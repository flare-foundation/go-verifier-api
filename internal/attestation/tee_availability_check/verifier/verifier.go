package verifier

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/flare-foundation/go-flare-common/pkg/tee/constants"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/relay"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teemachineregistry"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	config "github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
	teeTypes "github.com/flare-foundation/tee-node/pkg/types"
)

const (
	regOperationConst       = "F_REG"
	teeAttestationConst     = "TEE_ATTESTATION"
	fetchTimeout            = 5 * time.Second
	blockFreshnessInSeconds = 150 // verifier polling every minute + proxy polling every minute + retrieve result buffer 30s
)

var (
	ErrIndeterminate = errors.New("indeterminate verifier status")
)

type TeeVerifier struct {
	cfg                      *config.TeeAvailabilityCheckConfig
	ethClient                *ethclient.Client
	TeeMachineRegistryCaller *teemachineregistry.TeeMachineRegistryCaller
	RelayCaller              *relay.RelayCaller
	TeeSamples               map[common.Address][]bool
	SamplesToConsider        int
	SamplesMu                sync.RWMutex
}

func NewVerifier(cfg *config.TeeAvailabilityCheckConfig) (verifierinterface.VerifierInterface[types.TeeAvailabilityRequestData, types.TeeAvailabilityResponseData], error) {
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}
	teeRegistryCaller, err := teemachineregistry.NewTeeMachineRegistryCaller(common.HexToAddress(cfg.TeeRegistryContractAddress), client)
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

func GetVerifier(cfg *config.TeeAvailabilityCheckConfig) (verifierinterface.VerifierInterface[types.TeeAvailabilityRequestData, types.TeeAvailabilityResponseData], error) {
	return NewVerifier(cfg)
}

func (v *TeeVerifier) Verify(ctx context.Context, req types.TeeAvailabilityRequestData) (types.TeeAvailabilityResponseData, error) {
	// Build challenge instruction id
	challengeInstructionId, err := v.generateChallengeInstructionId(req.TeeId, req.Challenge)
	if err != nil {
		return types.TeeAvailabilityResponseData{}, fmt.Errorf("cannot generate challenge instruction id: %v", v)
	}
	// Fetch from tee proxy /action/result/<challengeInstructionId>
	response, err := v.fetchTEEChallengeResult(ctx, req.Url, challengeInstructionId)
	if err != nil {
		if errors.Is(err, utils.ErrNotFound) {
			// check polled data
			valid, infoErr := v.isTeeInfoValid(req.TeeId)
			if infoErr != nil { // Not enough data has been polled
				return types.TeeAvailabilityResponseData{}, fmt.Errorf("insufficient polling data to determine status: %v", infoErr)
			}
			if valid {
				return types.TeeAvailabilityResponseData{}, ErrIndeterminate
			} else { // No response in the last 5 minutes => tee is down
				return types.TeeAvailabilityResponseData{Status: uint8(types.DOWN)}, nil
			}
		} else {
			return types.TeeAvailabilityResponseData{}, fmt.Errorf("cannot fetch tee data %s: %v", req.TeeId, err)
		}
	}
	statusInfo, err := v.dataVerification(response)
	if err != nil {
		return types.TeeAvailabilityResponseData{}, err
	}
	infoData := response.TeeInfo
	return types.TeeAvailabilityResponseData{
		Status:                 uint8(statusInfo.Status),
		TeeTimestamp:           infoData.TeeTimestamp,
		CodeHash:               statusInfo.CodeHash,
		Platform:               statusInfo.Platform,
		InitialSigningPolicyId: infoData.InitialSigningPolicyID,
		LastSigningPolicyId:    infoData.LastSigningPolicyID,
		//StateHash:              infoData.,
	}, nil
}

func (v *TeeVerifier) dataVerification(response teeTypes.TeeInfoResponse) (StatusInfo, error) {
	// if response.Platform != "google" { //TODO - add after teeInfo.Platform is defined
	// 	return StatusInfo{}, fmt.Errorf("platform %s is not supported", response.Platform)
	// }
	attestationToken := response.Attestation
	infoData := response.TeeInfo

	_ = attestationToken
	_ = infoData

	// TODO
	// Certificate checks - check if we can trust the data in token
	//token, err := ValidatePKIToken(v.cfg.GoogleRootCertificate, string(attestationToken))
	//if err != nil {
	//	return StatusInfo{}, fmt.Errorf("failed to validate certificate signature: %v", err)
	//}
	//// check claims
	//statusInfo, err := ValidateClaims(token, infoData)
	//if err != nil {
	//	return StatusInfo{}, fmt.Errorf("failed to validate claims: %v", err)
	//}
	statusInfo := StatusInfo{
		Status: types.OK,
	}
	// check last signing policy hash
	// TODO [Jakob] Uncomment when everything is properly deployed
	lastSigningPolicyHash, err := v.getSigningPolicyHashFromChain(infoData.LastSigningPolicyID)
	if err != nil {
		return StatusInfo{}, fmt.Errorf("failed to retrieve last signing policy hash: %v", err)
	}
	if lastSigningPolicyHash != infoData.LastSigningPolicyHash {
		return StatusInfo{}, errors.New("failed to validate last signing policy hash")
	}
	// check initial signing policy hash
	initialSigningPolicyHash, err := v.getSigningPolicyHashFromChain(infoData.InitialSigningPolicyID)
	if err != nil {
		return StatusInfo{}, fmt.Errorf("failed to retrieve initial signing policy hash: %v", err)
	}
	if initialSigningPolicyHash != infoData.InitialSigningPolicyHash {
		return StatusInfo{}, errors.New("failed to validate initial signing policy hash")
	}
	return statusInfo, nil
	//return StatusInfo{Status: types.OK}, nil // TODO [Jakob] Remove
}

func (v *TeeVerifier) fetchTEEChallengeResult(ctx context.Context, baseURL string, challengeInstructionId common.Hash) (teeTypes.TeeInfoResponse, error) {
	url := fmt.Sprintf("%s/action/result/%s", baseURL, hex.EncodeToString(challengeInstructionId.Bytes()))
	fmt.Println("url: ", url)
	// ActionResponse = https://gitlab.com/flarenetwork/tee/tee-node/-/blob/brezTilna/internal/processor/direct/getutils/tee.go?ref_type=heads#L12
	actionResp, err := utils.FetchJSON[teeTypes.ActionResponse](ctx, url, fetchTimeout)
	if err != nil {
		return teeTypes.TeeInfoResponse{}, err
	}
	// teeInfo is marshaled inside actionResponse.Result.Data
	var teeInfo teeTypes.TeeInfoResponse
	err = json.Unmarshal(actionResp.Result.Data, &teeInfo)
	if err != nil {
		return teeTypes.TeeInfoResponse{}, fmt.Errorf("unmarshal tee result: %w", err)
	}
	return teeInfo, nil
}

func (v *TeeVerifier) FetchTEEInfoResultAndValidate(ctx context.Context, baseURL string) (bool, error) {
	infoResponse, err := v.fetchTEEInfoData(ctx, baseURL, "/info")
	if err != nil {
		return false, err
	}
	checkInfoChallenge, err := v.checkInfoChallengeIsValid(ctx, infoResponse.TeeInfo.Challenge)
	if err != nil {
		return false, err
	}
	if !checkInfoChallenge {
		return false, nil
	}
	_, err = v.dataVerification(infoResponse)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (v *TeeVerifier) fetchTEEInfoData(ctx context.Context, baseURL, path string) (teeTypes.TeeInfoResponse, error) {
	url := fmt.Sprintf("%s%s", baseURL, path)
	return utils.FetchJSON[teeTypes.TeeInfoResponse](ctx, url, fetchTimeout)
}

func (v *TeeVerifier) generateChallengeInstructionId(teeId common.Address, challenge common.Hash) (common.Hash, error) {
	REG_OP_TYPE, err := utils.Bytes32(fmt.Sprintf("%s", constants.Reg))
	if err != nil {
		return common.Hash{}, err
	}
	TEE_ATTESTATION, err := utils.Bytes32(fmt.Sprintf("%s", constants.TEEAttestation))
	if err != nil {
		return common.Hash{}, err
	}
	buf := new(bytes.Buffer)
	buf.Write(REG_OP_TYPE[:])
	buf.Write(TEE_ATTESTATION[:])
	buf.Write(common.LeftPadBytes(teeId.Bytes(), 32))
	buf.Write(challenge.Bytes())
	challengeInstructionId := crypto.Keccak256Hash(buf.Bytes())
	return challengeInstructionId, nil
}

func (v *TeeVerifier) getSigningPolicyHashFromChain(signingPolicyId uint32) (common.Hash, error) {
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}
	signingPolicyIdBigInt := new(big.Int).SetUint64(uint64(signingPolicyId))
	signingPolicyHashBytes, err := v.RelayCaller.ToSigningPolicyHash(callOpts, signingPolicyIdBigInt)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to call ToSigningPolicyHash: %w", err)
	}
	return common.Hash(signingPolicyHashBytes), nil
}

func (v *TeeVerifier) checkInfoChallengeIsValid(ctx context.Context, blockHash common.Hash) (bool, error) {
	challengeBlock, err := v.ethClient.BlockByHash(ctx, blockHash)
	if err != nil {
		return false, fmt.Errorf("failed to get challenge block: %w", err)
	}
	latestBlock, err := v.ethClient.BlockByNumber(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get latest block: %w", err)
	}
	if latestBlock.Time()-challengeBlock.Time() <= blockFreshnessInSeconds {
		return true, nil
	}
	return false, nil
}

func (v *TeeVerifier) isTeeInfoValid(teeId common.Address) (bool, error) {
	v.SamplesMu.RLock()
	samples := v.TeeSamples[teeId]
	v.SamplesMu.RUnlock()

	if len(samples) < v.SamplesToConsider {
		return false, fmt.Errorf("tee %s (%d samples: %+v)", teeId.Hex(), len(samples), samples)
	}
	for _, sample := range samples {
		if sample {
			return true, nil
		}
	}
	return false, nil
}
