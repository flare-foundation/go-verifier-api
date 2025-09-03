package verifier

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"

	"github.com/flare-foundation/go-flare-common/pkg/tee/op"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/relay"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teemachineregistry"
	teetypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/types"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
)

const (
	fetchTimeout            = 5 * time.Second
	blockFreshnessInSeconds = 150 // verifier polling every minute + proxy polling every minute + retrieve result buffer 30s
	chainRetries            = 2
	chainRetryDelay         = 500 * time.Millisecond
)

var (
	ErrIndeterminate = errors.New("indeterminate verifier status")
)

type TeeVerifier struct {
	cfg                      *config.TeeAvailabilityCheckConfig
	ethClient                EthClient
	TeeMachineRegistryCaller *teemachineregistry.TeeMachineRegistryCaller
	RelayCaller              RelayCallerInterface
	TeeSamples               map[common.Address][]teetypes.TeePollerSample
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

func NewVerifier(cfg *config.TeeAvailabilityCheckConfig) (verifierinterface.VerifierInterface[connector.ITeeAvailabilityCheckRequestBody, connector.ITeeAvailabilityCheckResponseBody], error) {
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Flare node: %w", err)
	}
	teeRegistryCaller, err := teemachineregistry.NewTeeMachineRegistryCaller(common.HexToAddress(cfg.TeeMachineRegistryContractAddress), client)
	if err != nil {
		return nil, fmt.Errorf("failed to create contract TeeRegistry caller: %w", err)
	}
	relayCaller, err := relay.NewRelayCaller(common.HexToAddress(cfg.RelayContractAddress), client)
	if err != nil {
		return nil, fmt.Errorf("failed to create contract Relay caller: %w", err)
	}
	samplesToConsider := 5
	return &TeeVerifier{cfg: cfg, ethClient: client, TeeMachineRegistryCaller: teeRegistryCaller, RelayCaller: relayCaller, SamplesToConsider: samplesToConsider}, nil
}

func GetVerifier(cfg *config.TeeAvailabilityCheckConfig) (verifierinterface.VerifierInterface[connector.ITeeAvailabilityCheckRequestBody, connector.ITeeAvailabilityCheckResponseBody], error) {
	return NewVerifier(cfg)
}

func (v *TeeVerifier) Verify(ctx context.Context, req connector.ITeeAvailabilityCheckRequestBody) (connector.ITeeAvailabilityCheckResponseBody, error) {
	// Build challenge instruction id
	challengeInstructionId, err := v.generateChallengeInstructionId(req.TeeId, req.Challenge)
	if err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, fmt.Errorf("cannot generate challenge instruction id: %w", err)
	}
	// Fetch from TEE proxy /action/result/<challengeInstructionId>
	response, err := v.fetchTEEChallengeResult(ctx, req.Url, challengeInstructionId)
	if err != nil && !errors.Is(err, utils.ErrNotFound) {
		return connector.ITeeAvailabilityCheckResponseBody{}, fmt.Errorf("cannot fetch TEE data %s: %w", req.TeeId, err)
	}
	if errors.Is(err, utils.ErrNotFound) {
		// check polled data
		isDown, infoErr := v.isTEEInfoDown(req.TeeId)
		if infoErr != nil { // Not enough data has been polled
			return connector.ITeeAvailabilityCheckResponseBody{}, fmt.Errorf("insufficient polling data to determine TEE status: %w", infoErr)
		}
		if isDown {
			return connector.ITeeAvailabilityCheckResponseBody{Status: uint8(teetypes.DOWN)}, nil
		} else {
			return connector.ITeeAvailabilityCheckResponseBody{}, ErrIndeterminate
		}
	}
	statusInfo, err := v.DataVerification(response)
	if err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, err
	}
	infoData := response.TeeInfo
	_, err = v.CheckSigningPolicies(ctx, infoData)
	if err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, err
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

func (v *TeeVerifier) DataVerification(response teenodetypes.TeeInfoResponse) (teetypes.StatusInfo, error) {
	// if response.Platform != "google" { //TODO (platform) - add after teeInfo.Platform is defined
	// 	return StatusInfo{}, fmt.Errorf("platform %s is not supported", response.Platform)
	// }
	attestationToken := response.Attestation
	infoData := response.TeeInfo
	// Certificate checks - check if we can trust the data in token
	token, err := ValidatePKIToken(v.cfg.GoogleRootCertificate, string(attestationToken))
	if err != nil {
		return teetypes.StatusInfo{}, fmt.Errorf("failed to validate certificate signature: %w", err)
	}
	// check claims
	statusInfo, err := ValidateClaims(token, infoData)
	if err != nil {
		return teetypes.StatusInfo{}, fmt.Errorf("failed to validate claims: %w", err)
	}
	return statusInfo, nil
}

func (v *TeeVerifier) CheckSigningPolicies(ctx context.Context, teeInfoData teenodetypes.TeeInfo) (teetypes.TeePollerSampleState, error) {
	// check initial signing policy hash
	initialSigningPolicyHash, state, err := v.getSigningPolicyHashFromChainWithRetry(ctx, teeInfoData.InitialSigningPolicyID)
	if err != nil {
		return state, fmt.Errorf("failed to retrieve initial signing policy hash: %w", err)
	}
	if initialSigningPolicyHash != teeInfoData.InitialSigningPolicyHash {
		return teetypes.TeePollerSampleInvalid, errors.New("failed to validate initial signing policy hash")
	}
	// check last signing policy hash
	lastSigningPolicyHash, state, err := v.getSigningPolicyHashFromChainWithRetry(ctx, teeInfoData.LastSigningPolicyID)
	if err != nil {
		return state, fmt.Errorf("failed to retrieve last signing policy hash: %w", err)
	}
	if lastSigningPolicyHash != teeInfoData.LastSigningPolicyHash {
		return teetypes.TeePollerSampleInvalid, errors.New("failed to validate last signing policy hash")
	}
	return teetypes.TeePollerSampleValid, nil
}

func (v *TeeVerifier) fetchTEEChallengeResult(ctx context.Context, baseURL string, challengeInstructionId common.Hash) (teenodetypes.TeeInfoResponse, error) {
	url := fmt.Sprintf("%s/action/result/%s", baseURL, hex.EncodeToString(challengeInstructionId.Bytes()))
	// ActionResponse = https://gitlab.com/flarenetwork/tee/tee-node/-/blob/brezTilna/internal/processor/direct/getutils/tee.go?ref_type=heads#L12
	actionResp, err := utils.FetchJSON[teenodetypes.ActionResponse](ctx, url, fetchTimeout)
	if err != nil {
		return teenodetypes.TeeInfoResponse{}, err
	}
	if len(actionResp.Result.Data) == 0 {
		return teenodetypes.TeeInfoResponse{}, fmt.Errorf("TEE challenge result data is empty")
	}
	if !json.Valid(actionResp.Result.Data) {
		return teenodetypes.TeeInfoResponse{}, fmt.Errorf("TEE challenge result data is not valid JSON")
	}
	// teeInfo is marshaled inside actionResponse.Result.Data
	var teeInfo teenodetypes.TeeInfoResponse
	err = json.Unmarshal(actionResp.Result.Data, &teeInfo)
	if err != nil {
		return teenodetypes.TeeInfoResponse{}, fmt.Errorf("unmarshal TEE result: %w", err)
	}
	return teeInfo, nil
}

func (v *TeeVerifier) generateChallengeInstructionId(teeId common.Address, challenge common.Hash) (common.Hash, error) {
	REG_OP_TYPE, err := utils.Bytes32(string(op.Reg))
	if err != nil {
		return common.Hash{}, err
	}
	TEE_ATTESTATION, err := utils.Bytes32(string(op.TEEAttestation))
	if err != nil {
		return common.Hash{}, err
	}
	buf := new(bytes.Buffer)
	buf.Write(REG_OP_TYPE[:])
	buf.Write(TEE_ATTESTATION[:])
	buf.Write(common.LeftPadBytes(teeId.Bytes(), utils.Bytes32Size))
	buf.Write(challenge.Bytes())
	challengeInstructionId := crypto.Keccak256Hash(buf.Bytes())
	return challengeInstructionId, nil
}

func (v *TeeVerifier) getSigningPolicyHashFromChain(ctx context.Context, signingPolicyId uint32) (common.Hash, teetypes.TeePollerSampleState, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	callOpts := &bind.CallOpts{
		Context: ctx,
	}
	signingPolicyIdBigInt := new(big.Int).SetUint64(uint64(signingPolicyId))
	signingPolicyHashBytes, err := v.RelayCaller.ToSigningPolicyHash(callOpts, signingPolicyIdBigInt)
	if err != nil {
		state, classifiedErr := utils.ClassifyFetchError("ToSigningPolicyHash", err)
		return common.Hash{}, state, classifiedErr
	}
	return common.Hash(signingPolicyHashBytes), teetypes.TeePollerSampleValid, nil
}

func (v *TeeVerifier) getSigningPolicyHashFromChainWithRetry(ctx context.Context, signingPolicyId uint32) (common.Hash, teetypes.TeePollerSampleState, error) {
	var (
		hash       common.Hash
		finalState teetypes.TeePollerSampleState
	)
	_, err := utils.Retry(
		chainRetries,
		chainRetryDelay,
		func() (struct{}, error) {
			h, state, err := v.getSigningPolicyHashFromChain(ctx, signingPolicyId)
			if err != nil {
				finalState = state
				return struct{}{}, err
			}
			hash = h
			finalState = state
			return struct{}{}, nil
		},
		func(err error) bool {
			return finalState == teetypes.TeePollerSampleInvalid
		},
	)
	if err != nil {
		return common.Hash{}, finalState, fmt.Errorf(
			"getSigningPolicyHashFromChainWithRetry failed after %d retries: %w",
			chainRetries, err,
		)
	}
	return hash, finalState, nil
}

func (v *TeeVerifier) CheckInfoChallengeIsValid(ctx context.Context, blockHash common.Hash) (teetypes.TeePollerSampleState, error) {
	challengeBlock, err := v.ethClient.BlockByHash(ctx, blockHash)
	if err != nil {
		return utils.ClassifyFetchError("fetch challenge block", err)
	}
	latestBlock, err := v.ethClient.BlockByNumber(ctx, nil)
	if err != nil {
		if errors.Is(err, utils.ErrInvalidInput) {
			return teetypes.TeePollerSampleIndeterminate, fmt.Errorf("fetch latest block: %w", err)
		}
		return utils.ClassifyFetchError("fetch latest block", err)
	}
	if latestBlock.Time()-challengeBlock.Time() <= blockFreshnessInSeconds {
		return teetypes.TeePollerSampleValid, nil
	}
	return teetypes.TeePollerSampleInvalid, nil
}

func (v *TeeVerifier) isTEEInfoDown(teeId common.Address) (bool, error) {
	v.SamplesMu.RLock()
	samples := v.TeeSamples[teeId]
	v.SamplesMu.RUnlock()

	if len(samples) < v.SamplesToConsider {
		logger.Infof("TEE %s has insufficient samples (%d/%d). Samples: %+v", teeId.Hex(), len(samples), v.SamplesToConsider, samples)
		return false, fmt.Errorf("insufficient samples to determine TEE %s status", teeId.Hex())
	}
	for _, sample := range samples {
		if sample.State == teetypes.TeePollerSampleValid || sample.State == teetypes.TeePollerSampleIndeterminate {
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
