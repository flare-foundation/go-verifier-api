package verifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
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
	// Fetch from tee proxy
	response, err := v.fetchTEEAvailabilityResult(ctx, req.Url, challengeInstructionId)
	if err != nil {
		valid, infoErr := v.isTeeInfoValid(req.TeeId)
		if infoErr != nil { // Not enough data has been polled
			return types.TeeAvailabilityResponseData{}, huma.Error503ServiceUnavailable(fmt.Sprintf("Insufficient polling data %v", infoErr))
		}
		if !valid { // No response in the last 5 minutes
			var responseBody types.TeeAvailabilityResponseData
			responseBody.Status = uint8(types.DOWN)
			responseBody.TeeTimestamp = 0
			responseBody.CodeHash = common.Hash{}
			responseBody.Platform = common.Hash{}
			responseBody.InitialSigningPolicyId = uint32(0)
			responseBody.LastSigningPolicyId = uint32(0)
			responseBody.StateHash = common.Hash{}

			return responseBody, nil
		}
		// There are valid responses from /info, but no response on /action/result/<challengeInstructionId>
		return types.TeeAvailabilityResponseData{}, huma.Error503ServiceUnavailable(fmt.Sprintf("Tee %s data not available %v", req.TeeId, err))
	}
	//TODO - continue
	statusInfo, err := v.dataVerification(response)
	infoData := response.TeeInfo
	if err != nil {
		return types.TeeAvailabilityResponseData{}, err
	}

	var responseBody types.TeeAvailabilityResponseData
	responseBody.Status = uint8(statusInfo.Status)
	responseBody.TeeTimestamp = infoData.TeeTimestamp
	responseBody.CodeHash = statusInfo.CodeHash
	responseBody.Platform = statusInfo.Platform
	responseBody.InitialSigningPolicyId = infoData.InitialSigningPolicyId
	responseBody.LastSigningPolicyId = infoData.LastSigningPolicyId
	responseBody.StateHash = infoData.StateHash

	return responseBody, nil
}

func (v *TeeVerifier) dataVerification(response types.ProxyInfoResponseBody) (StatusInfo, error) {
	if response.Platform != "google" { //TODO
		return StatusInfo{}, huma.Error501NotImplemented(fmt.Sprintf("Platform %s is not supported", response.Platform))
	}
	attestationToken := response.Attestation
	infoData := response.TeeInfo
	// Certificate checks
	cert, err := LoadRootCert()
	if err != nil {
		return StatusInfo{}, huma.Error500InternalServerError("failed to load root cert: %w", err)
	}
	token, err := ValidatePKIToken(cert, attestationToken)
	if err != nil {
		return StatusInfo{}, err
	}
	if !token.Valid {
		return StatusInfo{}, huma.Error400BadRequest(fmt.Sprintf("attestation token is invalid: %s", attestationToken))
	}
	lastSigningPolicyHash, err := v.getLastSigningPolicyHashFromChain(infoData.LastSigningPolicyId)
	if err != nil {
		return StatusInfo{}, huma.Error503ServiceUnavailable("failed to retrieve last signing policy hash: %w", err)
	}
	if lastSigningPolicyHash != infoData.LastSigningPolicyHash {
		return StatusInfo{}, huma.Error400BadRequest("failed to validate last signing policy hash")
	}
	statusInfo, err := ValidateClaims(token, infoData)
	if err != nil {
		return StatusInfo{}, huma.Error400BadRequest("failed to validate claims: %w", err)
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
		return types.ProxyInfoResponseBody{}, huma.Error500InternalServerError(fmt.Sprintf("fetchTEEData: creating HTTP request failed: %v", err))
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return types.ProxyInfoResponseBody{}, huma.Error503ServiceUnavailable(fmt.Sprintf("fetchTEEData: error making request to tee: %v", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return types.ProxyInfoResponseBody{}, huma.Error503ServiceUnavailable(fmt.Sprintf("fetchTEEData: teeProxy %s returned non-200 status: %d", baseURL, resp.StatusCode))
	}
	var result types.ProxyInfoResponseBody //TODO -> do it with ToInternal()
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return types.ProxyInfoResponseBody{}, huma.Error400BadRequest(fmt.Sprintf("error decoding tee response: %v", err))
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
		return false, fmt.Errorf("failed to get block: %w", err)
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
		return false, fmt.Errorf("not enough data for tee %s (%d samples: %+v)", teeId, len(samples), samples)
	}
	for _, sample := range samples {
		if sample {
			return true, nil
		}
	}
	return false, nil
}
