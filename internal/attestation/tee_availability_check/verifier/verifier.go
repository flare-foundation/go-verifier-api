package verifier

import (
	"context"
	"encoding/hex"
	"encoding/json"
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
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	teeavailabilitycheckconfig "gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/config"
	"gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/types"
	verifierinterface "gitlab.com/urskak/verifier-api/internal/verifier_interface"
)

const (
	regOperationType        = "REG"
	attestationType         = "ATTESTATION_TYPE"
	fetchTimeout            = 5 * time.Second
	blockFreshnessInSeconds = 30
)

type TeeVerifier struct {
	cfg               *teeavailabilitycheckconfig.TeeAvailabilityCheckConfig
	client            *ethclient.Client
	TeeRegistryCaller *teeregistry.TeeRegistryCaller
	RelayCaller       *relay.RelayCaller
	TeeSamples        map[common.Address][]bool
	SamplesToConsider int
}

func NewVerifier(cfg *teeavailabilitycheckconfig.TeeAvailabilityCheckConfig) (verifierinterface.VerifierInterface[connector.ITeeAvailabilityCheckRequestBody, connector.ITeeAvailabilityCheckResponseBody], error) {
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
	return &TeeVerifier{cfg: cfg, client: client, TeeRegistryCaller: teeRegistryCaller, RelayCaller: relayCaller, SamplesToConsider: samplesToConsider}, nil
}

func GetVerifier(cfg *teeavailabilitycheckconfig.TeeAvailabilityCheckConfig) (verifierinterface.VerifierInterface[connector.ITeeAvailabilityCheckRequestBody, connector.ITeeAvailabilityCheckResponseBody], error) {
	return NewVerifier(cfg)
}

func (v *TeeVerifier) Verify(ctx context.Context, req connector.ITeeAvailabilityCheckRequestBody) (connector.ITeeAvailabilityCheckResponseBody, error) {
	// Build challenge instruction id
	challengeInstructionId := v.generateChallengeInstructionId(req.TeeId, req.Challenge)
	// Fetch from tee proxy
	response, err := v.fetchTEEAvailabilityResult(ctx, req.Url, challengeInstructionId)
	if err != nil {
		// check polling data - TODO: needs to differentiate between not enough data to determine result and no valid result
		valid, err := v.isTeeInfoValid(req.TeeId)
		if err != nil { //TODO: cannot say anything - what kind of status do we return?
			return connector.ITeeAvailabilityCheckResponseBody{}, err
		}
		if !valid { //TODO: cannot say anything
			var responseBody connector.ITeeAvailabilityCheckResponseBody
			responseBody.Status = uint8(types.DOWN)
			responseBody.CodeHash = [32]byte{}
			responseBody.Platform = [32]byte{}
			responseBody.MachineStatus = uint8(types.UNDETERMINED)
			responseBody.TeeTimestamp = 0
			responseBody.InitialTeeId = common.Address{}
			responseBody.TeeGovernanceHash = [32]byte{}
			responseBody.RewardEpochId = &big.Int{}

			return responseBody, nil
		}
		//TODO: cannot say anything - what kind of status do we return?
		return connector.ITeeAvailabilityCheckResponseBody{}, err
	}

	statusInfo, err := v.dataVerification(response)
	infoData := response.Data
	if err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, err
	}

	var responseBody connector.ITeeAvailabilityCheckResponseBody
	responseBody.Status = uint8(statusInfo.Status)
	responseBody.CodeHash = statusInfo.CodeHash
	responseBody.Platform = statusInfo.Platform
	responseBody.MachineStatus = uint8(infoData.Status)
	responseBody.TeeTimestamp = infoData.TeeTimestamp
	responseBody.InitialTeeId = infoData.InitialTeeId
	responseBody.TeeGovernanceHash = infoData.TeeGovernanceHash
	responseBody.RewardEpochId = infoData.LastSigningPolicyId

	return responseBody, nil
}

func (v *TeeVerifier) dataVerification(response types.ProxyInfoResponseBody) (StatusInfo, error) {
	attestationToken := response.AttestationInfo.Attestation
	infoData := response.Data
	// Certificate checks
	cert, err := LoadRootCert()
	if err != nil {
		return StatusInfo{}, fmt.Errorf("failed to load root cert: %w", err)
	}
	token, err := ValidatePKIToken(cert, attestationToken)
	if err != nil {
		return StatusInfo{}, fmt.Errorf("failed to validate PKI token: %w", err)
	}
	if !token.Valid {
		return StatusInfo{}, fmt.Errorf("attestation token is invalid: %s", attestationToken)
	}
	statusInfo, err := ValidateClaims(token, infoData)
	if err != nil {
		return StatusInfo{}, fmt.Errorf("failed to validate claims: %w", err)
	}
	lastSigningPolicyHash, err := v.getLastSigningPolicyHashFromChain(infoData.LastSigningPolicyId)
	if lastSigningPolicyHash != infoData.LastSigningPolicyHash {
		return StatusInfo{}, fmt.Errorf("failed to validate last signing policy hash")
	}
	if err != nil {
		return StatusInfo{}, fmt.Errorf("failed to retrieve last signing policy hash: %w", err)
	}
	return statusInfo, nil
}

func (v *TeeVerifier) fetchTEEAvailabilityResult(ctx context.Context, baseURL, challengeInstructionId string) (types.ProxyInfoResponseBody, error) {
	return v.fetchTEEData(ctx, baseURL, fmt.Sprintf("/action/result/%s", challengeInstructionId))
}

func (v *TeeVerifier) FetchTEEInfoResultAndValidate(ctx context.Context, baseURL string) (bool, error) {
	infoResponse, err := v.fetchTEEData(ctx, baseURL, "/info")
	if err != nil {
		return false, err
	}
	checkInfoChallenge, err := v.checkInfoChallenge(ctx, infoResponse.Data.Challenge)
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
	client := &http.Client{
		Timeout: fetchTimeout,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return types.ProxyInfoResponseBody{}, fmt.Errorf("creating HTTP request failed: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return types.ProxyInfoResponseBody{}, fmt.Errorf("error making request to tee: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return types.ProxyInfoResponseBody{}, fmt.Errorf("tee returned non-200 status: %d", resp.StatusCode)
	}
	var result types.ProxyInfoResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return types.ProxyInfoResponseBody{}, fmt.Errorf("error decoding tee response: %w", err)
	}
	return result, nil
}

func (v *TeeVerifier) generateChallengeInstructionId(teeId common.Address, challenge *big.Int) string {
	reg := common.BytesToHash([]byte(regOperationType))
	teeAttestation := common.BytesToHash([]byte(attestationType))
	teeIdHash := common.BytesToHash(teeId.Bytes())
	challengeHash := common.BytesToHash(challenge.Bytes())
	challengeInstructionId := crypto.Keccak256(reg[:], teeAttestation[:], teeIdHash[:], challengeHash[:])
	return hex.EncodeToString(challengeInstructionId)
}

func (v *TeeVerifier) getLastSigningPolicyHashFromChain(lastSigningPolicyId *big.Int) (common.Hash, error) {
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}
	lastSigningPolicyHash, err := v.RelayCaller.ToSigningPolicyHash(callOpts, lastSigningPolicyId)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to call ToSigningPolicyHash: %w", err)
	}
	return lastSigningPolicyHash, nil
}

func (v *TeeVerifier) checkInfoChallenge(ctx context.Context, blockHash string) (bool, error) {
	block, err := v.client.BlockByHash(ctx, common.HexToHash(blockHash))
	if err != nil {
		return false, fmt.Errorf("failed to get block: %w", err)
	}
	latestBlock, err := v.client.BlockByNumber(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get latest block: %w", err)
	}
	if latestBlock.Time()-block.Time() <= blockFreshnessInSeconds {
		return true, nil
	}
	return false, nil
}

func (v *TeeVerifier) isTeeInfoValid(teeId common.Address) (bool, error) {
	samples := v.TeeSamples[teeId]
	if len(samples) < v.SamplesToConsider {
		return false, fmt.Errorf("not enough data for tee %s (%d samples: %+v)", teeId.Hex(), len(samples), samples)
	}
	for _, sample := range samples {
		if sample {
			return true, nil
		}
	}
	return false, nil
}
