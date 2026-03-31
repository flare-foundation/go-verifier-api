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
	"strings"
	"sync"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/contracts/relay"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teemachineregistry"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/attestation/googlecloud"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/fetcher"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	teeattestation "github.com/flare-foundation/tee-node/pkg/attestation"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/flare-foundation/tee-node/pkg/utils"
)

const (
	fetchChallengeTimeout   = 4 * time.Second
	BlockFreshnessInSeconds = 150 // verifier polling every minute + proxy polling every minute + retrieve result buffer 30s
	chainMaxAttempts        = 1
	chainRetryDelay         = 400 * time.Millisecond
	chainFetchTimeout       = 3 * time.Second
	blockStalenessThreshold = 120             // seconds — warn if latest block is older than this
	SamplesToConsider       = 5               // NOTE: SamplesToConsider and SampleInterval need to be correlated.
	SampleInterval          = 1 * time.Minute // NOTE: SamplesToConsider and SampleInterval need to be correlated.
)

var (
	E2ETestPlatform = common.HexToHash("544553545f504c4154464f524d00000000000000000000000000000000000000")
	E2ETestCodeHash = common.HexToHash("194844cf417dde867073e5ab7199fa4d21fd82b5dbe2bdea8b3d7fc18d10fdc2")

	ErrInsufficientSamples  = errors.New("insufficient samples")
	ErrTEEDataValidation    = errors.New("TEE data validation failed")
	ErrActionResultNotFound = errors.New("action result not found")
)

type TeeVerifier struct {
	Cfg                      *config.TeeAvailabilityCheckConfig
	EthClient                EthClient
	TeeMachineRegistryCaller TeeMachineRegistryCallerInterface
	RelayCaller              RelayCallerInterface
	ValidateURL              bool
	CRLCache                 *CRLCache
	TeeSamples               map[common.Address][]verifiertypes.TeeSampleValue
	SamplesMu                sync.RWMutex
	magicPassLogged          sync.Map // tracks TEE IDs that have already logged a MagicPass warning
}

type EthClient interface {
	BlockByHash(ctx context.Context, hash common.Hash) (*ethtypes.Block, error)
	BlockByNumber(ctx context.Context, number *big.Int) (*ethtypes.Block, error)
}

type RelayCallerInterface interface {
	ToSigningPolicyHash(opts *bind.CallOpts, id *big.Int) ([32]byte, error)
}

type TeeMachineRegistryCallerInterface interface {
	GetAllActiveTeeMachines(opts *bind.CallOpts, start *big.Int, end *big.Int) (struct {
		TeeIds      []common.Address
		Urls        []string
		TotalLength *big.Int
	}, error)
	GetActiveTeeMachines(opts *bind.CallOpts, extensionId *big.Int) (struct {
		TeeIds []common.Address
		Urls   []string
	}, error)
}

func NewVerifier(cfg *config.TeeAvailabilityCheckConfig) (attestation.Verifier[connector.ITeeAvailabilityCheckRequestBody, connector.ITeeAvailabilityCheckResponseBody], error) {
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Flare node at %s: %w", cfg.RPCURL, err)
	}
	teeMachineRegistryCaller, err := teemachineregistry.NewTeeMachineRegistryCaller(common.HexToAddress(cfg.TeeMachineRegistryContractAddress), client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("cannot create TeeMachineRegistry caller at %s: %w", cfg.TeeMachineRegistryContractAddress, err)
	}
	relayCaller, err := relay.NewRelayCaller(common.HexToAddress(cfg.RelayContractAddress), client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("cannot create Relay caller at %s: %w", cfg.RelayContractAddress, err)
	}
	return &TeeVerifier{
		Cfg:                      cfg,
		EthClient:                client,
		TeeMachineRegistryCaller: teeMachineRegistryCaller,
		RelayCaller:              relayCaller,
		CRLCache:                 NewCRLCache(),
		TeeSamples:               make(map[common.Address][]verifiertypes.TeeSampleValue),
	}, nil
}

func (v *TeeVerifier) Verify(ctx context.Context, req connector.ITeeAvailabilityCheckRequestBody) (connector.ITeeAvailabilityCheckResponseBody, error) {
	var zero connector.ITeeAvailabilityCheckResponseBody
	// Fetch from TEE proxy /action/result/<instructionID>
	response, dataSigner, err := FetchTEEChallengeResult(ctx, v.FormatProxyURL(req.Url), req.InstructionId, v.Cfg.AllowPrivateNetworks)
	if err != nil {
		// check polled data
		isDown, infoErr := v.IsTEEInfoDown(req.TeeId)
		if infoErr != nil { // Not enough data has been polled
			return zero, infoErr
		}
		if isDown {
			return connector.ITeeAvailabilityCheckResponseBody{Status: uint8(DOWN)}, nil
		}
		return zero, fmt.Errorf("cannot fetch TEE data for (TEE=%s, URL=%s, instruction=%x) and determine its status: %w", req.TeeId, req.Url, req.InstructionId[:], err)
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
		info, err := v.DataVerification(ctx, response, req.TeeId, false)
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
	return connector.ITeeAvailabilityCheckResponseBody{
		Status:                 uint8(statusInfo.Status),
		TeeTimestamp:           infoData.TeeTimestamp,
		CodeHash:               statusInfo.CodeHash,
		Platform:               statusInfo.Platform,
		InitialSigningPolicyId: infoData.InitialSigningPolicyID,
		LastSigningPolicyId:    infoData.LastSigningPolicyID,
		State: connector.ITeeAvailabilityCheckTeeState{
			SystemState:        infoData.State.SystemState,
			SystemStateVersion: infoData.State.SystemStateVersion,
			State:              infoData.State.State,
			StateVersion:       infoData.State.StateVersion,
		},
	}, nil
}

func (v *TeeVerifier) DataVerification(ctx context.Context, response teenodetypes.TeeInfoResponse, expectedTeeID common.Address, pollerContext bool) (StatusInfo, error) {
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
	if string(response.Attestation) == teeattestation.MagicPass {
		platform := E2ETestPlatform
		codeHash := E2ETestCodeHash
		if pollerContext {
			if _, alreadyLogged := v.magicPassLogged.LoadOrStore(expectedTeeID, true); !alreadyLogged {
				logger.Warnf("TEE %s: MagicPass bypass active (non-production mode). Skipping all attestation validation. Do not use in production.", expectedTeeID.Hex())
			}
		} else {
			logger.Warnf("TEE %s: MagicPass bypass active (non-production mode). Skipping all attestation validation. Do not use in production.", expectedTeeID.Hex())
		}
		return StatusInfo{
			Status:   OK,
			CodeHash: codeHash,
			Platform: platform,
		}, nil
	}

	// TEE returned a real attestation — clear MagicPass tracking so it re-logs if it switches back.
	v.magicPassLogged.Delete(expectedTeeID)

	attestationToken := response.Attestation
	infoData := response.TeeInfo

	// Fetch CRLs for revocation checking (strict: fail verification if CRL fetch fails)
	var leafCRL, intermediateCRL *x509.RevocationList
	if v.CRLCache != nil {
		var crlErr error
		leafCRL, intermediateCRL, crlErr = v.CRLCache.GetCRLsForToken(ctx, string(attestationToken), v.Cfg.GoogleRootCertificate)
		if crlErr != nil {
			return StatusInfo{}, fmt.Errorf("CRL fetch failed: %w", crlErr)
		}
	}

	// Certificate checks - check if we can trust the data in token
	_, claims, err := googlecloud.ParseAndValidatePKIToken(string(attestationToken), v.Cfg.GoogleRootCertificate, leafCRL, intermediateCRL)
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
	statusInfo, err := ValidateClaims(claims, infoData, v.Cfg.AllowTeeDebug)
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
		hash, state, err := v.GetSigningPolicyHashFromChainWithRetry(ctx, teeInfoData.InitialSigningPolicyID, chainMaxAttempts, chainRetryDelay)
		initialSigningCh <- result{hash, state, err}
	}()
	go func() {
		hash, state, err := v.GetSigningPolicyHashFromChainWithRetry(ctx, teeInfoData.LastSigningPolicyID, chainMaxAttempts, chainRetryDelay)
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

func (v *TeeVerifier) GetSigningPolicyHashFromChain(ctx context.Context, signingPolicyID uint32) (common.Hash, verifiertypes.TeeSampleState, error) {
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

func (v *TeeVerifier) GetSigningPolicyHashFromChainWithRetry(
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
			h, state, err := v.GetSigningPolicyHashFromChain(ctx, signingPolicyID)
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
			"getSigningPolicyHashFromChainWithRetry failed after %d attempts: %w",
			maxAttempts, err,
		)
	}
	return hash, finalState, nil
}

func (v *TeeVerifier) CheckInfoChallengeIsValid(ctx context.Context, blockHash common.Hash) (verifiertypes.TeeSampleState, error) {
	challengeBlock, err := v.EthClient.BlockByHash(ctx, blockHash)
	if err != nil {
		return verifiertypes.MapFetchErrorToState("fetch challenge block", err)
	}
	latestBlock, err := v.EthClient.BlockByNumber(ctx, nil)
	if err != nil {
		return verifiertypes.MapFetchErrorToState("fetch latest block", err)
	}
	now := time.Now().Unix()
	blockAge := now - int64(latestBlock.Time())
	blockFreshness := int64(blockStalenessThreshold)
	if blockAge > blockFreshness {
		logger.Warnf("Latest block is stale: %d seconds old (%d, %d)", blockAge, latestBlock.NumberU64(), latestBlock.Time())
	}
	if latestBlock.Time()-challengeBlock.Time() <= BlockFreshnessInSeconds {
		return verifiertypes.TeeSampleValid, nil
	}
	return verifiertypes.TeeSampleInvalid, fmt.Errorf("challenge too old: %d seconds old (challenge: %d, latest: %d)", latestBlock.Time()-challengeBlock.Time(), challengeBlock.NumberU64(), latestBlock.NumberU64())
}

func (v *TeeVerifier) IsTEEInfoDown(teeID common.Address) (bool, error) {
	v.SamplesMu.RLock()
	samples := v.TeeSamples[teeID]
	v.SamplesMu.RUnlock()

	if len(samples) < SamplesToConsider {
		return false, fmt.Errorf("insufficient samples to determine TEE %s status: %w", teeID.Hex(), ErrInsufficientSamples)
	}
	for _, sample := range samples {
		if sample.State == verifiertypes.TeeSampleValid || sample.State == verifiertypes.TeeSampleIndeterminate {
			return false, nil
		}
	}
	return true, nil
}

// ClearMagicPassLogged removes the MagicPass log tracking for a TEE,
// allowing the warning to fire again if the TEE returns MagicPass in the future.
func (v *TeeVerifier) ClearMagicPassLogged(teeID common.Address) {
	v.magicPassLogged.Delete(teeID)
}

func (v *TeeVerifier) Close() error {
	var ethErr error
	if closer, ok := v.EthClient.(io.Closer); ok {
		ethErr = closer.Close()
	}
	var crlErr error
	if v.CRLCache != nil {
		crlErr = v.CRLCache.Close()
	}
	return errors.Join(ethErr, crlErr)
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
) (teenodetypes.TeeInfoResponse, common.Address, error) {
	var zero teenodetypes.TeeInfoResponse
	var zeroAdd common.Address
	url := fmt.Sprintf("%s/action/result/%s", baseURL, hex.EncodeToString(challengeInstructionID.Bytes()))
	resolved, err := ResolveExternalURL(ctx, baseURL, allowPrivateNetworks)
	if err != nil {
		return zero, zeroAdd, err
	}
	dialAddr, hostHeader, serverName := BuildPinnedAddr(resolved)
	actionResp, err := fetcher.GetJSONPinned[teenodetypes.ActionResponse](ctx, url, fetchChallengeTimeout, dialAddr, hostHeader, serverName)
	if err != nil {
		if errors.Is(err, fetcher.ErrNotFound) {
			return zero, zeroAdd, fmt.Errorf("%w: %w", ErrActionResultNotFound, err)
		}
		return zero, zeroAdd, err
	}
	if len(actionResp.Result.Data) == 0 {
		return zero, zeroAdd, fmt.Errorf("TEE challenge result data is empty")
	}
	if !json.Valid(actionResp.Result.Data) {
		return zero, zeroAdd, fmt.Errorf("TEE challenge result data is not valid JSON: %q", actionResp.Result.Data)
	}
	// teeInfo is marshaled inside actionResponse.Result.Data
	var teeInfo teenodetypes.TeeInfoResponse
	err = json.Unmarshal(actionResp.Result.Data, &teeInfo)
	if err != nil {
		return zero, zeroAdd, fmt.Errorf("unmarshal TEE result: %w", err)
	}
	// recover signer
	signer, err := utils.SignatureToSignersAddress(crypto.Keccak256(actionResp.Result.Data), actionResp.ProxySignature)
	if err != nil {
		return zero, zeroAdd, fmt.Errorf("recover signer: %w", err)
	}

	return teeInfo, signer, nil
}

// Ensure *TeeVerifier implements io.Closer at compile time.
var _ io.Closer = (*TeeVerifier)(nil)
