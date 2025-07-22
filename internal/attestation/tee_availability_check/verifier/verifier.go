package verifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/relay"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeregistry"
	types "gitlab.com/urskak/verifier-api/internal/api/type"
	teeavailabilitycheckconfig "gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/config"
	"gitlab.com/urskak/verifier-api/internal/attestation/utils"
	verifierinterface "gitlab.com/urskak/verifier-api/internal/verifier_interface"
)

const (
	regOperationType        = "REG"
	teeAttestationType      = "TEE_ATTESTATION"
	fetchTimeout            = 5 * time.Second
	blockFreshnessInSeconds = 150 // verifier polling every minute + proxy polling every minute + retrieve result buffer 30s
)

type TeeVerifier struct {
	cfg               *teeavailabilitycheckconfig.TeeAvailabilityCheckConfig
	ethClient         *ethclient.Client
	TeeRegistryCaller *teeregistry.TeeRegistryCaller
	RelayCaller       *relay.RelayCaller
	TeeSamples        map[common.Address][]bool
	SamplesToConsider int
}

func NewVerifier(cfg *teeavailabilitycheckconfig.TeeAvailabilityCheckConfig) (verifierinterface.VerifierInterface[types.TeeAvailabilityRequestData, types.TeeAvailabilityResponseData], error) {
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}
	teeRegistryCaller, err := teeregistry.NewTeeRegistryCaller(common.HexToAddress(cfg.TeeRegistryContractAddress), client)
	if err != nil {
		return nil, fmt.Errorf("failed to create contract TeeRegistry caller: %w", err)
	}
	relayCaller, err := relay.NewRelayCaller(common.HexToAddress(cfg.RelayContractAddress), client)
	if err != nil {
		return nil, fmt.Errorf("failed to create contract Relay caller: %w", err)
	}
	samplesToConsider := 5
	return &TeeVerifier{cfg: cfg, ethClient: client, TeeRegistryCaller: teeRegistryCaller, RelayCaller: relayCaller, SamplesToConsider: samplesToConsider}, nil
}

func GetVerifier(cfg *teeavailabilitycheckconfig.TeeAvailabilityCheckConfig) (verifierinterface.VerifierInterface[types.TeeAvailabilityRequestData, types.TeeAvailabilityResponseData], error) {
	return NewVerifier(cfg)
}

func (v *TeeVerifier) Verify(ctx context.Context, req types.TeeAvailabilityRequestData) (types.TeeAvailabilityResponseData, error) {
	// Build challenge instruction id
	challengeInstructionId := v.generateChallengeInstructionId(req.TeeId, req.Challenge)
	// Fetch from tee proxy /action/result/<challengeInstructionId>
	response, err := v.fetchTEEAvailabilityResult(ctx, req.Url, challengeInstructionId)
	// Result is not yet available
	if response.Attestation == "" && err == nil {
		// check polled data
		valid, infoErr := v.isTeeInfoValid(req.TeeId)
		if infoErr != nil { // Not enough data has been polled
			return types.TeeAvailabilityResponseData{}, fmt.Errorf("Insufficient polling data: %v", infoErr)
		}
		if !valid { // No response in the last 5 minutes
			var responseData types.TeeAvailabilityResponseData
			responseData.Status = uint8(types.DOWN)
			responseData.TeeTimestamp = 0
			responseData.CodeHash = common.Hash{}
			responseData.Platform = common.Hash{}
			responseData.InitialSigningPolicyId = uint32(0)
			responseData.LastSigningPolicyId = uint32(0)
			responseData.StateHash = common.Hash{}

			return responseData, nil
		}
	}
	// Error while fetching from tee proxy /action/result/<challengeInstructionId>
	if err != nil {
		return types.TeeAvailabilityResponseData{}, fmt.Errorf("Cannot fetch tee %s data: %v", req.TeeId, err)
	}
	statusInfo, err := v.dataVerification(response)
	if err != nil {
		return types.TeeAvailabilityResponseData{}, err
	}
	infoData := response.TeeInfo
	var responseData types.TeeAvailabilityResponseData
	responseData.Status = uint8(statusInfo.Status)
	responseData.TeeTimestamp = infoData.TeeTimestamp
	responseData.CodeHash = statusInfo.CodeHash
	responseData.Platform = statusInfo.Platform
	responseData.InitialSigningPolicyId = infoData.InitialSigningPolicyId
	responseData.LastSigningPolicyId = infoData.LastSigningPolicyId
	responseData.StateHash = infoData.StateHash

	return responseData, nil
}

func (v *TeeVerifier) dataVerification(response types.ProxyInfoResponseBody) (StatusInfo, error) {
	if response.Platform != "google" { //TODO
		return StatusInfo{}, fmt.Errorf("platform %s is not supported", response.Platform)
	}
	attestationToken := response.Attestation
	infoData := response.TeeInfo
	// Certificate checks
	token, err := ValidatePKIToken(v.cfg.GoogleRootCertificate, attestationToken)
	if err != nil {
		return StatusInfo{}, fmt.Errorf("failed to validate certificate signature: %v", err)
	}
	lastSigningPolicyHash, err := v.getLastSigningPolicyHashFromChain(infoData.LastSigningPolicyId)
	if err != nil {
		return StatusInfo{}, fmt.Errorf("failed to retrieve last signing policy hash: %v", err)
	}
	if lastSigningPolicyHash != infoData.LastSigningPolicyHash {
		return StatusInfo{}, errors.New("failed to validate last signing policy hash")
	}
	statusInfo, err := ValidateClaims(token, infoData)
	if err != nil {
		return StatusInfo{}, fmt.Errorf("failed to validate claims: %v", err)
	}
	return statusInfo, nil
}

func (v *TeeVerifier) fetchTEEAvailabilityResult(ctx context.Context, baseURL string, challengeInstructionId common.Hash) (types.ProxyInfoResponseBody, error) {
	return v.fetchTEEData(ctx, baseURL, fmt.Sprintf("/action/result/%s", challengeInstructionId))
}

func (v *TeeVerifier) FetchTEEInfoResultAndValidate(ctx context.Context, baseURL string) (bool, error) {
	infoResponse, err := v.fetchTEEData(ctx, baseURL, "/info")
	if err != nil {
		return false, err
	}
	checkInfoChallenge, err := v.checkInfoChallenge(ctx, infoResponse.TeeInfo.Challenge)
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

func (v *TeeVerifier) fetchTEEData(ctx context.Context, baseURL, path string) (types.ProxyInfoResponseBody, error) {
	url := fmt.Sprintf("%s%s", baseURL, path)
	httpClient := &http.Client{
		Timeout: fetchTimeout,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return types.ProxyInfoResponseBody{}, fmt.Errorf("creating HTTP request failed: %v", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return types.ProxyInfoResponseBody{}, fmt.Errorf("error making request to tee: %v", err)
	}
	defer resp.Body.Close()
	// No result yet available
	if resp.StatusCode == http.StatusNotFound {
		return types.ProxyInfoResponseBody{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return types.ProxyInfoResponseBody{}, fmt.Errorf("teeProxy %s returned non-200 status: %d", baseURL, resp.StatusCode)
	}
	var result types.ProxyInfoResponseBody //TODO -> do it with ToInternal()
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return types.ProxyInfoResponseBody{}, fmt.Errorf("error decoding tee response: %v", err)
	}
	return result, nil
}

func (v *TeeVerifier) generateChallengeInstructionId(teeId common.Address, challenge common.Hash) common.Hash {
	REG_OP_TYPE := utils.Bytes32(regOperationType)
	TEE_ATTESTATION := utils.Bytes32(teeAttestationType)
	buf := new(bytes.Buffer)
	buf.Write(REG_OP_TYPE[:])
	buf.Write(TEE_ATTESTATION[:])
	buf.Write(common.LeftPadBytes(teeId.Bytes(), 32))
	buf.Write(challenge.Bytes())
	challengeInstructionId := crypto.Keccak256Hash(buf.Bytes())
	return challengeInstructionId
}

func (v *TeeVerifier) getLastSigningPolicyHashFromChain(lastSigningPolicyId uint32) (common.Hash, error) {
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}
	lastSigningPolicyIdBigInt := new(big.Int).SetUint64(uint64(lastSigningPolicyId))
	lastSigningPolicyHashBytes, err := v.RelayCaller.ToSigningPolicyHash(callOpts, lastSigningPolicyIdBigInt)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to call ToSigningPolicyHash: %w", err)
	}
	return common.Hash(lastSigningPolicyHashBytes), nil
}

func (v *TeeVerifier) checkInfoChallenge(ctx context.Context, blockHash common.Hash) (bool, error) {
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
	samples := v.TeeSamples[teeId]
	if len(samples) < v.SamplesToConsider {
		return false, fmt.Errorf("not enough polling data for tee %s (%d samples: %+v)", teeId, len(samples), samples)
	}
	for _, sample := range samples {
		if sample {
			return true, nil
		}
	}
	return false, nil
}
