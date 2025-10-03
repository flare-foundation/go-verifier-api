package verifier

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/relay"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teemachineregistry"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/keyutil"
	teetype "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/type"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
)

const (
	fetchInfoTimeout        = 5 * time.Second
	blockFreshnessInSeconds = 150 // verifier polling every minute + proxy polling every minute + retrieve result buffer 30s
	chainMaxAttemps         = 1
	chainRetryDelay         = 400 * time.Millisecond
	chainFetchTimeout       = 4 * time.Second
	samplesToConsider       = 5
)

var (
	ErrIndeterminate = errors.New("indeterminate verifier status")
)

type TeeVerifier struct {
	cfg                      *config.TeeAvailabilityCheckConfig
	ethClient                EthClient
	TeeMachineRegistryCaller TeeMachineRegistryCallerInterface
	RelayCaller              RelayCallerInterface
	TeeSamples               map[common.Address][]teetype.TeePollerSample
	SamplesToConsider        int
	SamplesMu                sync.RWMutex
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
}

func NewVerifier(cfg *config.TeeAvailabilityCheckConfig) (verifierinterface.VerifierInterface[connector.ITeeAvailabilityCheckRequestBody, connector.ITeeAvailabilityCheckResponseBody], error) {
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Flare node at %s: %w", cfg.RPCURL, err)
	}
	teeRegistryCaller, err := teemachineregistry.NewTeeMachineRegistryCaller(common.HexToAddress(cfg.TeeMachineRegistryContractAddress), client)
	if err != nil {
		return nil, fmt.Errorf("cannot create TeeRegistry caller at %s: %w", cfg.TeeMachineRegistryContractAddress, err)
	}
	relayCaller, err := relay.NewRelayCaller(common.HexToAddress(cfg.RelayContractAddress), client)
	if err != nil {
		return nil, fmt.Errorf("cannot create Relay caller at %s: %w", cfg.RelayContractAddress, err)
	}
	return &TeeVerifier{cfg: cfg, ethClient: client, TeeMachineRegistryCaller: teeRegistryCaller, RelayCaller: relayCaller, SamplesToConsider: samplesToConsider}, nil
}

func GetVerifier(cfg *config.TeeAvailabilityCheckConfig) (verifierinterface.VerifierInterface[connector.ITeeAvailabilityCheckRequestBody, connector.ITeeAvailabilityCheckResponseBody], error) {
	return NewVerifier(cfg)
}

func (v *TeeVerifier) Verify(ctx context.Context, req connector.ITeeAvailabilityCheckRequestBody) (connector.ITeeAvailabilityCheckResponseBody, error) {
	var zero connector.ITeeAvailabilityCheckResponseBody
	// Build challenge instruction id
	challengeInstructionID, err := instruction.GenerateChallengeInstructionID(req.TeeId, req.Challenge)
	if err != nil {
		return zero, fmt.Errorf("cannot generate challenge instruction id: %w", err)
	}
	// Fetch from TEE proxy /action/result/<challengeInstructionID>
	response, dataSigner, err := v.fetchTEEChallengeResult(ctx, v.FormatProxyURL(req.Url), challengeInstructionID, coreutil.GetJSON[teenodetypes.ActionResponse])
	if err != nil && !errors.Is(err, coreutil.ErrNotFound) {
		return zero, fmt.Errorf("cannot fetch TEE data for TeeID %s: %w", req.TeeId, err)
	}
	if errors.Is(err, coreutil.ErrNotFound) {
		// check polled data
		isDown, infoErr := v.isTEEInfoDown(req.TeeId)
		if infoErr != nil { // Not enough data has been polled
			return zero, infoErr
		}
		if isDown {
			return connector.ITeeAvailabilityCheckResponseBody{Status: uint8(teetype.DOWN)}, nil
		} else {
			return zero, ErrIndeterminate
		}
	}
	// Check proxy signature.
	if dataSigner != req.TeeProxyId {
		return zero, fmt.Errorf("proxy signer does not match: expected %s, got: %s", req.TeeProxyId.Hex(), dataSigner.Hex())
	}
	// Verify info data.
	statusInfo, err := v.DataVerification(response, req.TeeId)
	if err != nil {
		return zero, err
	}
	infoData := response.TeeInfo
	_, err = v.CheckSigningPolicies(ctx, infoData)
	if err != nil {
		return zero, err
	}

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

func (v *TeeVerifier) DataVerification(response teenodetypes.TeeInfoResponse, expectedTeeID common.Address) (teetype.StatusInfo, error) {
	// if response.Platform != "google" { //TODO (platform) - add after teeInfo.Platform is defined
	// 	return StatusInfo{}, fmt.Errorf("platform %s is not supported", response.Platform)
	// }
	if v.cfg.DisableAttestationCheckE2E {
		platform := common.HexToHash("4743505f494e54454c5f54445800000000000000000000000000000000000000")
		codeHash := common.HexToHash("194844cf417dde867073e5ab7199fa4d21fd82b5dbe2bdea8b3d7fc18d10fdc2")
		logger.Warnf("Attestation check disabled for E2E (using DISABLE_ATTESTATION_CHECK_E2E=true). Do not use in production. Status %d, Codehash %s, Platform %s", teetype.OK, codeHash, platform)
		return teetype.StatusInfo{
			Status:   teetype.OK,
			CodeHash: codeHash,
			Platform: platform,
		}, nil

	}
	attestationToken := response.Attestation
	infoData := response.TeeInfo
	// Certificate checks - check if we can trust the data in token
	token, err := ValidatePKIToken(v.cfg.GoogleRootCertificate, string(attestationToken))
	if err != nil {
		return teetype.StatusInfo{}, fmt.Errorf("failed to validate certificate signature: %w", err)
	}
	if !token.Valid {
		return teetype.StatusInfo{}, fmt.Errorf("attestation token is invalid: %v", token)
	}
	// Check claims
	claims, ok := token.Claims.(*teetype.GoogleTeeClaims)
	if !ok {
		return teetype.StatusInfo{}, errors.New("cannot parse claims")
	}
	statusInfo, err := ValidateClaims(claims, infoData, v.cfg.AllowTeeDebug)
	if err != nil {
		return teetype.StatusInfo{}, fmt.Errorf("failed to validate claims: %w", err)
	}
	// Validate teeID
	receivedTeeID, err := keyutil.RetrieveAddressFromPublicKey(infoData.PublicKey)
	if err != nil {
		return teetype.StatusInfo{}, fmt.Errorf("failed to retrieve TEE ID from: %w", err)
	}
	if expectedTeeID != receivedTeeID {
		return teetype.StatusInfo{}, fmt.Errorf("expected TEE ID %s, got: %s", expectedTeeID.Hex(), receivedTeeID.Hex())
	}
	return statusInfo, nil
}

func (v *TeeVerifier) CheckSigningPolicies(ctx context.Context, teeInfoData teenodetypes.TeeInfo) (teetype.TeePollerSampleState, error) {
	type result struct {
		hash  common.Hash
		state teetype.TeePollerSampleState
		err   error
	}
	initialSigningCh := make(chan result, 1)
	lastSigningCh := make(chan result, 1)
	// Fetch policies
	go func() {
		hash, state, err := v.getSigningPolicyHashFromChainWithRetry(ctx, teeInfoData.InitialSigningPolicyID, chainMaxAttemps, chainRetryDelay)
		initialSigningCh <- result{hash, state, err}
	}()
	go func() {
		hash, state, err := v.getSigningPolicyHashFromChainWithRetry(ctx, teeInfoData.LastSigningPolicyID, chainMaxAttemps, chainRetryDelay)
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
		return teetype.TeePollerSampleInvalid, errors.New("failed to validate initial signing policy hash")
	}
	if lastSigningRes.hash != teeInfoData.LastSigningPolicyHash {
		return teetype.TeePollerSampleInvalid, errors.New("failed to validate last signing policy hash")
	}
	return teetype.TeePollerSampleValid, nil
}

func (v *TeeVerifier) fetchTEEChallengeResult(
	ctx context.Context,
	baseURL string,
	challengeInstructionID common.Hash,
	fetchFn func(context.Context, string, time.Duration) (teenodetypes.ActionResponse, error),
) (teenodetypes.TeeInfoResponse, common.Address, error) {
	var zero teenodetypes.TeeInfoResponse
	var zeroAdd common.Address
	url := fmt.Sprintf("%s/action/result/%s", baseURL, hex.EncodeToString(challengeInstructionID.Bytes()))
	// ActionResponse = https://gitlab.com/flarenetwork/tee/tee-node/-/blob/brezTilna/internal/processor/direct/getcoreutil/tee.go?ref_type=heads#L12
	actionResp, err := fetchFn(ctx, url, fetchInfoTimeout)
	if err != nil {
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
	signer, err := keyutil.RecoverSigner(actionResp.Result.Data, actionResp.ProxySignature)
	if err != nil {
		return zero, zeroAdd, fmt.Errorf("recover signer: %w", err)
	}
	return teeInfo, signer, nil
}

func (v *TeeVerifier) getSigningPolicyHashFromChain(ctx context.Context, signingPolicyID uint32) (common.Hash, teetype.TeePollerSampleState, error) {
	ctx, cancel := context.WithTimeout(ctx, chainFetchTimeout)
	defer cancel()
	callOpts := &bind.CallOpts{
		Context: ctx,
	}
	signingPolicyIDBigInt := new(big.Int).SetUint64(uint64(signingPolicyID))
	signingPolicyHashBytes, err := v.RelayCaller.ToSigningPolicyHash(callOpts, signingPolicyIDBigInt)
	if err != nil {
		state, classifiedErr := coreutil.MapFetchErrorToState("ToSigningPolicyHash", err)
		return common.Hash{}, state, classifiedErr
	}
	return common.Hash(signingPolicyHashBytes), teetype.TeePollerSampleValid, nil
}

func (v *TeeVerifier) getSigningPolicyHashFromChainWithRetry(
	ctx context.Context,
	signingPolicyID uint32,
	maxAttempts int,
	delay time.Duration,
) (common.Hash, teetype.TeePollerSampleState, error) {
	var (
		hash       common.Hash
		finalState teetype.TeePollerSampleState
	)
	_, err := coreutil.Retry(
		maxAttempts,
		delay,
		func() (struct{}, error) {
			h, state, err := v.getSigningPolicyHashFromChain(ctx, signingPolicyID)
			if err != nil {
				finalState = state
				return struct{}{}, err
			}
			hash = h
			finalState = state
			return struct{}{}, nil
		},
		func(err error) bool {
			return finalState == teetype.TeePollerSampleInvalid
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

func (v *TeeVerifier) CheckInfoChallengeIsValid(ctx context.Context, blockHash common.Hash) (teetype.TeePollerSampleState, error) {
	challengeBlock, err := v.ethClient.BlockByHash(ctx, blockHash)
	if err != nil {
		return coreutil.MapFetchErrorToState("fetch challenge block", err)
	}
	latestBlock, err := v.ethClient.BlockByNumber(ctx, nil)
	if err != nil {
		if errors.Is(err, coreutil.ErrInvalidInput) {
			return teetype.TeePollerSampleIndeterminate, fmt.Errorf("fetch latest block: %w", err)
		}
		return coreutil.MapFetchErrorToState("fetch latest block", err)
	}
	if latestBlock.Time()-challengeBlock.Time() <= blockFreshnessInSeconds {
		return teetype.TeePollerSampleValid, nil
	}
	return teetype.TeePollerSampleInvalid, fmt.Errorf("challenge too old: %d seconds old", latestBlock.Time()-challengeBlock.Time())
}

func (v *TeeVerifier) isTEEInfoDown(teeID common.Address) (bool, error) {
	v.SamplesMu.RLock()
	samples := v.TeeSamples[teeID]
	v.SamplesMu.RUnlock()

	if len(samples) < v.SamplesToConsider {
		return false, fmt.Errorf("insufficient samples to determine TEE %s status", teeID.Hex())
	}
	for _, sample := range samples {
		if sample.State == teetype.TeePollerSampleValid || sample.State == teetype.TeePollerSampleIndeterminate {
			return false, nil
		}
	}
	return true, nil
}

func (v *TeeVerifier) Close() error {
	if closer, ok := v.ethClient.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (v *TeeVerifier) FormatProxyURL(url string) string {
	if v.cfg.DisableAttestationCheckE2E {
		url = strings.Replace(url, "localhost", "host.docker.internal", -1)
	}
	return url
}
