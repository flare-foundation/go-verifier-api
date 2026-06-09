package verifier

import (
	"context"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/contracts/relay"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	csigning "github.com/flare-foundation/go-flare-common/pkg/signing"
	"github.com/flare-foundation/go-flare-common/pkg/tee/attestation/googlecloud"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/fetcher"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	teeattestation "github.com/flare-foundation/tee-node/pkg/attestation"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/flare-foundation/tee-node/pkg/utils"
)

const (
	fetchChallengeTimeout = 4 * time.Second
	chainMaxAttempts      = 1
	chainRetryDelay       = 400 * time.Millisecond
	chainFetchTimeout     = 3 * time.Second
)

var (
	E2ETestPlatform = common.HexToHash("544553545f504c4154464f524d00000000000000000000000000000000000000")
	E2ETestCodeHash = common.HexToHash("194844cf417dde867073e5ab7199fa4d21fd82b5dbe2bdea8b3d7fc18d10fdc2")

	// Expected (OPType, OPCommand) on an availability-check action result.
	// Not covered by ActionResult.Hash(), so a malicious proxy can change
	// them without breaking the TEE signature — checked explicitly here.
	// (op.Get, op.TEEInfo) is intentionally not allowed: it is reachable
	// only via the proxy's API-key-gated /direct endpoint, not the FDC2
	// availability-check flow.
	expectedAvailabilityOPType    = op.Reg.Hash()
	expectedAvailabilityOPCommand = op.TEEAttestation.Hash()

	ErrTEEDataValidation    = errors.New("TEE data validation failed")
	ErrActionResultNotFound = errors.New("action result not found")
)

type TeeVerifier struct {
	Cfg         *config.TeeAvailabilityCheckConfig
	EthClient   *ethclient.Client
	RelayCaller RelayCallerInterface
	CRLCache    *CRLCache
}

type RelayCallerInterface interface {
	ToSigningPolicyHash(opts *bind.CallOpts, id *big.Int) ([32]byte, error)
}

func NewVerifier(cfg *config.TeeAvailabilityCheckConfig) (attestation.Verifier[fdc2.ITeeAvailabilityCheckRequestBody, fdc2.ITeeAvailabilityCheckResponseBody], error) {
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Flare node at %s: %w", cfg.RPCURL, err)
	}
	relayCaller, err := relay.NewRelayCaller(cfg.RelayContractAddress, client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("cannot create Relay caller at %s: %w", cfg.RelayContractAddress.Hex(), err)
	}
	return &TeeVerifier{
		Cfg:         cfg,
		EthClient:   client,
		RelayCaller: relayCaller,
		CRLCache:    NewCRLCache(),
	}, nil
}

func (v *TeeVerifier) Verify(ctx context.Context, req fdc2.ITeeAvailabilityCheckRequestBody) (fdc2.ITeeAvailabilityCheckResponseBody, error) {
	var zero fdc2.ITeeAvailabilityCheckResponseBody
	// Fetch from TEE proxy /action/result/<instructionID>
	actionResp, response, dataSigner, err := FetchTEEChallengeResult(ctx, v.FormatProxyURL(req.Url), req.InstructionId, v.Cfg.AllowPrivateNetworks)
	if err != nil {
		return zero, fmt.Errorf("cannot fetch TEE data for (TEE=%s, URL=%s, instruction=%x): %w", req.TeeId, req.Url, req.InstructionId[:], err)
	}
	// Check corresponding challenge.
	challengeHex := common.BytesToHash(req.Challenge[:])
	if response.TeeInfo.Challenge != challengeHex {
		return zero, fmt.Errorf("challenge does not match: expected %s, got %s: %w", challengeHex.Hex(), response.TeeInfo.Challenge.Hex(), ErrTEEDataValidation)
	}
	// Check proxy signature.
	if dataSigner != req.TeeProxyId {
		return zero, fmt.Errorf("proxy signer does not match: expected %s, got %s: %w", req.TeeProxyId.Hex(), dataSigner.Hex(), ErrTEEDataValidation)
	}
	// Verify the action result is from the expected TEE and bound to this instruction.
	if err := verifyActionResult(actionResp, req.InstructionId, req.TeeId, response.TeeInfo.ChainID); err != nil {
		return zero, fmt.Errorf("%w: %w", ErrTEEDataValidation, err)
	}
	// Run DataVerification and CheckSigningPolicies in parallel (independent after challenge fetch).
	infoData := response.TeeInfo

	type dataVerResult struct {
		info StatusInfo
		err  error
	}
	type sigPolicyResult struct {
		err error
	}
	dvCh := make(chan dataVerResult, 1)
	spCh := make(chan sigPolicyResult, 1)

	go func() {
		info, err := v.DataVerification(ctx, response, req.TeeId)
		dvCh <- dataVerResult{info, err}
	}()
	go func() {
		_, err := v.CheckSigningPolicies(ctx, infoData)
		spCh <- sigPolicyResult{err}
	}()

	dvRes := <-dvCh
	spRes := <-spCh

	if dvRes.err != nil {
		return zero, fmt.Errorf("%w: %w", ErrTEEDataValidation, dvRes.err)
	}
	if spRes.err != nil {
		return zero, spRes.err
	}

	statusInfo := dvRes.info
	return fdc2.ITeeAvailabilityCheckResponseBody{
		Status:                 uint8(statusInfo.Status),
		TeeTimestamp:           infoData.TeeTimestamp,
		CodeHash:               statusInfo.CodeHash,
		Platform:               statusInfo.Platform,
		InitialSigningPolicyId: infoData.InitialSigningPolicyID,
		LastSigningPolicyId:    infoData.LastSigningPolicyID,
		State: fdc2.ITeeAvailabilityCheckTeeState{
			SystemState:        infoData.State.SystemState,
			SystemStateVersion: infoData.State.SystemStateVersion,
			State:              infoData.State.State,
			StateVersion:       infoData.State.StateVersion,
		},
	}, nil
}

func (v *TeeVerifier) DataVerification(ctx context.Context, response teenodetypes.TeeInfoResponse, expectedTeeID common.Address) (StatusInfo, error) {
	if v.Cfg.DisableAttestationCheckE2E {
		platform := E2ETestPlatform
		codeHash := E2ETestCodeHash
		logger.Warnf("Attestation check disabled for E2E (using DISABLE_ATTESTATION_CHECK_E2E=true). Do not use in production. Status %d, Codehash %s, Platform %s", OK, codeHash, platform)
		return StatusInfo{
			Status:   OK,
			CodeHash: codeHash,
			Platform: platform,
		}, nil
	}

	// WARNING: MagicPass bypass — accepts TEE nodes running in non-production mode
	// (settings.Mode != 0) which return "magic_pass" instead of a real attestation token.
	// This skips ALL attestation validation (PKI, claims, CRL). Any TEE returning this
	// string will be trusted unconditionally. Do NOT rely on this in production — a
	// compromised or malicious TEE could return "magic_pass" to bypass verification.
	// This exists to support hackathon and development environments where real Google
	// Confidential Space attestation is unavailable.
	if response.Attestation == teeattestation.MagicPass {
		logger.Warnf("TEE %s: MagicPass bypass active (non-production mode). Skipping all attestation validation. Do not use in production.", expectedTeeID.Hex())
		return StatusInfo{
			Status:   OK,
			CodeHash: E2ETestCodeHash,
			Platform: E2ETestPlatform,
		}, nil
	}

	attestationToken := response.Attestation
	infoData := response.TeeInfo

	// Fetch CRLs for revocation checking (strict: fail verification if CRL fetch fails)
	var leafCRL, intermediateCRL *x509.RevocationList
	if v.CRLCache != nil {
		var crlErr error
		leafCRL, intermediateCRL, crlErr = v.CRLCache.FetchCRLsForToken(ctx, attestationToken, v.Cfg.GoogleRootCertificate)
		if crlErr != nil {
			return StatusInfo{}, fmt.Errorf("CRL fetch failed: %w", crlErr)
		}
	}

	teeInfoHash, err := infoData.Hash()
	if err != nil {
		return StatusInfo{}, fmt.Errorf("cannot create hash of teeInfo: %w", err)
	}
	// go-flare-common replaced the AllowDebug bool with an AllowedDebugStatuses
	// allowlist. Preserve prior semantics: when TEE debug is not allowed (production),
	// require the production dbgstat "disabled-since-boot"; otherwise leave the
	// allowlist empty to skip the dbgstat check.
	var allowedDebugStatuses []string
	if !v.Cfg.AllowTeeDebug {
		allowedDebugStatuses = []string{"disabled-since-boot"}
	}
	policy := googlecloud.Policy{
		Audience:             v.Cfg.TeeAudience,
		AllowedImageIDs:      v.Cfg.TeeAllowedImageIDs,
		EATNonce:             hex.EncodeToString(teeInfoHash),
		AllowedDebugStatuses: allowedDebugStatuses,
		Issuer:               googlecloud.ConfidentialSpaceIssuer,
	}
	// Certificate checks - check if we can trust the data in token
	_, claims, err := googlecloud.ParseAndValidatePKIToken(attestationToken, v.Cfg.GoogleRootCertificate, leafCRL, intermediateCRL, policy)
	if err != nil {
		return StatusInfo{}, fmt.Errorf("cannot validate certificate signature: %w", err)
	}
	// Validate teeID
	receivedTeeIDs, err := utils.PubKeysToAddresses([]teenodetypes.PublicKey{infoData.PublicKey})
	if err != nil {
		return StatusInfo{}, fmt.Errorf("cannot retrieve TEE ID from: %w", err)
	}
	if expectedTeeID != receivedTeeIDs[0] {
		return StatusInfo{}, fmt.Errorf("expected TEE ID %s, got: %s", expectedTeeID.Hex(), receivedTeeIDs[0].Hex())
	}
	// Check claims
	statusInfo, err := ValidateClaims(claims)
	if err != nil {
		return StatusInfo{}, fmt.Errorf("cannot validate claims: %w", err)
	}
	return statusInfo, nil
}

func (v *TeeVerifier) CheckSigningPolicies(ctx context.Context, teeInfoData teenodetypes.TeeInfo) (verifiertypes.TeeSampleState, error) {
	type result struct {
		hash  common.Hash
		state verifiertypes.TeeSampleState
		err   error
	}
	initialSigningCh := make(chan result, 1)
	lastSigningCh := make(chan result, 1)
	// Fetch policies
	go func() {
		hash, state, err := v.FetchSigningPolicyHashFromChainWithRetry(ctx, teeInfoData.InitialSigningPolicyID, chainMaxAttempts, chainRetryDelay)
		initialSigningCh <- result{hash, state, err}
	}()
	go func() {
		hash, state, err := v.FetchSigningPolicyHashFromChainWithRetry(ctx, teeInfoData.LastSigningPolicyID, chainMaxAttempts, chainRetryDelay)
		lastSigningCh <- result{hash, state, err}
	}()
	// Wait for results
	initialSigningRes := <-initialSigningCh
	lastSigningRes := <-lastSigningCh
	// Check
	if initialSigningRes.err != nil {
		return initialSigningRes.state, fmt.Errorf("cannot retrieve initial signing policy hash for ID %d: %w", teeInfoData.InitialSigningPolicyID, initialSigningRes.err)
	}
	if lastSigningRes.err != nil {
		return lastSigningRes.state, fmt.Errorf("cannot retrieve last signing policy hash for ID %d: %w", teeInfoData.LastSigningPolicyID, lastSigningRes.err)
	}
	if initialSigningRes.hash != teeInfoData.InitialSigningPolicyHash {
		return verifiertypes.TeeSampleInvalid, fmt.Errorf("failed to validate initial signing policy hash: %w", ErrTEEDataValidation)
	}
	if lastSigningRes.hash != teeInfoData.LastSigningPolicyHash {
		return verifiertypes.TeeSampleInvalid, fmt.Errorf("failed to validate last signing policy hash: %w", ErrTEEDataValidation)
	}

	return verifiertypes.TeeSampleValid, nil
}

func (v *TeeVerifier) FetchSigningPolicyHashFromChain(ctx context.Context, signingPolicyID uint32) (common.Hash, verifiertypes.TeeSampleState, error) {
	ctx, cancel := context.WithTimeout(ctx, chainFetchTimeout)
	defer cancel()
	callOpts := &bind.CallOpts{
		Context: ctx,
	}
	signingPolicyIDBigInt := new(big.Int).SetUint64(uint64(signingPolicyID))
	signingPolicyHashBytes, err := v.RelayCaller.ToSigningPolicyHash(callOpts, signingPolicyIDBigInt)
	if err != nil {
		state, classifiedErr := verifiertypes.MapFetchErrorToState("ToSigningPolicyHash", err)
		return common.Hash{}, state, classifiedErr
	}

	return common.Hash(signingPolicyHashBytes), verifiertypes.TeeSampleValid, nil
}

func (v *TeeVerifier) FetchSigningPolicyHashFromChainWithRetry(
	ctx context.Context,
	signingPolicyID uint32,
	maxAttempts int,
	delay time.Duration,
) (common.Hash, verifiertypes.TeeSampleState, error) {
	var (
		hash       common.Hash
		finalState verifiertypes.TeeSampleState
	)
	_, err := fetcher.Retry(
		ctx,
		maxAttempts,
		delay,
		func() (struct{}, error) {
			h, state, err := v.FetchSigningPolicyHashFromChain(ctx, signingPolicyID)
			if err != nil {
				finalState = state
				return struct{}{}, err
			}
			hash = h
			finalState = state
			return struct{}{}, nil
		},
		func(err error) bool {
			return finalState == verifiertypes.TeeSampleInvalid
		},
	)
	if err != nil {
		return common.Hash{}, finalState, fmt.Errorf(
			"fetchSigningPolicyHashFromChainWithRetry failed after %d attempts: %w",
			maxAttempts, err,
		)
	}
	return hash, finalState, nil
}

func (v *TeeVerifier) Close() error {
	if v.EthClient != nil {
		v.EthClient.Close()
	}
	if v.CRLCache != nil {
		return v.CRLCache.Close()
	}
	return nil
}

func (v *TeeVerifier) FormatProxyURL(url string) string {
	if v.Cfg.DisableAttestationCheckE2E {
		url = strings.ReplaceAll(url, "localhost", "host.docker.internal")
	}
	return url
}

func FetchTEEChallengeResult(
	ctx context.Context,
	baseURL string,
	challengeInstructionID common.Hash,
	allowPrivateNetworks bool,
) (teenodetypes.ActionResponse, teenodetypes.TeeInfoResponse, common.Address, error) {
	var zeroAction teenodetypes.ActionResponse
	var zeroInfo teenodetypes.TeeInfoResponse
	var zeroAdd common.Address
	// Compose the path safely instead of string concatenation: url.JoinPath handles
	// a trailing slash on baseURL, escapes the path segment, and rejects a malformed
	// base URL — while preserving any path prefix the operator configured.
	reqURL, err := url.JoinPath(baseURL, "action", "result", hex.EncodeToString(challengeInstructionID.Bytes()))
	if err != nil {
		return zeroAction, zeroInfo, zeroAdd, fmt.Errorf("invalid proxy URL %q: %w", baseURL, err)
	}
	resolved, err := ResolveExternalURL(ctx, baseURL, allowPrivateNetworks)
	if err != nil {
		return zeroAction, zeroInfo, zeroAdd, err
	}
	dialAddr, hostHeader, serverName := BuildPinnedAddr(resolved)
	actionResp, err := fetcher.FetchJSONPinned[teenodetypes.ActionResponse](ctx, reqURL, fetchChallengeTimeout, dialAddr, hostHeader, serverName)
	if err != nil {
		if errors.Is(err, fetcher.ErrNotFound) {
			return zeroAction, zeroInfo, zeroAdd, fmt.Errorf("%w: %w", ErrActionResultNotFound, err)
		}
		return zeroAction, zeroInfo, zeroAdd, err
	}
	if len(actionResp.Result.Data) == 0 {
		return zeroAction, zeroInfo, zeroAdd, errors.New("TEE challenge result data is empty")
	}
	if !json.Valid(actionResp.Result.Data) {
		preview := actionResp.Result.Data
		if len(preview) > 128 {
			preview = preview[:128]
		}
		return zeroAction, zeroInfo, zeroAdd, fmt.Errorf("TEE challenge result data is not valid JSON (len=%d, preview=%q)", len(actionResp.Result.Data), preview)
	}
	// teeInfo is marshaled inside actionResponse.Result.Data
	var teeInfo teenodetypes.TeeInfoResponse
	err = json.Unmarshal(actionResp.Result.Data, &teeInfo)
	if err != nil {
		return zeroAction, zeroInfo, zeroAdd, fmt.Errorf("unmarshal TEE result: %w", err)
	}
	// recover signer over the domain-separated PROXY_ACTION_RESULT preimage,
	// chain-bound with the chainID carried inside the attestation payload.
	proxySignHash, err := csigning.NewPayload(csigning.ProxyActionResult, teeInfo.TeeInfo.ChainID, common.BytesToHash(actionResp.Result.Hash())).Hash()
	if err != nil {
		return zeroAction, zeroInfo, zeroAdd, fmt.Errorf("hashing proxy signature payload: %w", err)
	}
	signer, err := utils.SignatureToSignersAddress(proxySignHash[:], actionResp.ProxySignature)
	if err != nil {
		return zeroAction, zeroInfo, zeroAdd, fmt.Errorf("recover signer: %w", err)
	}

	return actionResp, teeInfo, signer, nil
}

// verifyActionResult checks that the action result is genuinely from the
// expected TEE and bound to the requested instruction. It complements the
// proxy-signature check by establishing TEE proof-of-possession on the action
// result itself, preventing a malicious proxy from replaying an older valid
// result or serving a different TEE's response.
//
// The TEE signs DomainHash(TEE_ACTION_RESULT, chainID, Result.Hash()), which
// binds Data, ID, SubmissionTag, and Status (see tee-node pkg/types/actions.go
// Hash()) plus the chain ID. OPType and OPCommand are NOT bound by the
// signature, so they are checked explicitly against the expected
// (op.Reg, op.TEEAttestation) pair — the only pair that legitimately reaches
// the verifier via the FDC2 availability-check flow. chainID is the TEE's
// reported TeeInfo.ChainID; a wrong value simply fails recovery (fail-closed),
// since only the genuine TEE can produce a signature over its real chain ID.
func verifyActionResult(
	actionResp teenodetypes.ActionResponse,
	expectedInstructionID common.Hash,
	expectedTeeID common.Address,
	chainID uint64,
) error {
	if actionResp.Result.ID != expectedInstructionID {
		return fmt.Errorf("action result instruction ID mismatch: expected %s, got %s",
			expectedInstructionID.Hex(), actionResp.Result.ID.Hex())
	}
	// regutils.TEEAttestation returns Status=1 on success. Status=0 is
	// processorutils.Invalid, Status=3 is DeadlineExceeded.
	if actionResp.Result.Status != 1 {
		return fmt.Errorf("action result status not success: status=%d, log=%q, additionalStatus=%x",
			actionResp.Result.Status, actionResp.Result.Log, []byte(actionResp.Result.AdditionalResultStatus))
	}
	if actionResp.Result.OPType != expectedAvailabilityOPType {
		return fmt.Errorf("action result OPType mismatch: expected %s, got %s",
			expectedAvailabilityOPType.Hex(), actionResp.Result.OPType.Hex())
	}
	if actionResp.Result.OPCommand != expectedAvailabilityOPCommand {
		return fmt.Errorf("action result OPCommand mismatch: expected %s, got %s",
			expectedAvailabilityOPCommand.Hex(), actionResp.Result.OPCommand.Hex())
	}
	if len(actionResp.Signature) == 0 {
		return errors.New("missing TEE signature on action result")
	}
	signHash, err := csigning.NewPayload(csigning.TEEActionResult, chainID, common.BytesToHash(actionResp.Result.Hash())).Hash()
	if err != nil {
		return fmt.Errorf("computing action result sign hash: %w", err)
	}
	if err := utils.VerifySignature(signHash[:], actionResp.Signature, expectedTeeID); err != nil {
		return fmt.Errorf("TEE signature on action result does not match expected TEE %s: %w",
			expectedTeeID.Hex(), err)
	}
	return nil
}

// Ensure *TeeVerifier implements io.Closer at compile time.
var _ io.Closer = (*TeeVerifier)(nil)
