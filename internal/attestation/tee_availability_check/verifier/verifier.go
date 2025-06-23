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
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	teeavailabilitycheckconfig "gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/config"
	"gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/types"
	verifierinterface "gitlab.com/urskak/verifier-api/internal/verifier_interface"
)

const regOperationType = "REG"
const attestationType = "ATTESTATION_TYPE"

type TeeVerifier struct {
	cfg    *teeavailabilitycheckconfig.TeeAvailabilityCheckConfig
	client *ethclient.Client
}

func NewVerifier(cfg *teeavailabilitycheckconfig.TeeAvailabilityCheckConfig) (verifierinterface.VerifierInterface[connector.ITeeAvailabilityCheckRequestBody, connector.ITeeAvailabilityCheckResponseBody], error) {
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}
	return &TeeVerifier{cfg: cfg, client: client}, nil
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
		return connector.ITeeAvailabilityCheckResponseBody{}, err
	}
	attestationToken := response.AttestationInfo.Attestation
	infoData := response.Data
	// Certificate checks
	cert, err := LoadRootCert()
	if err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, fmt.Errorf("failed to load root cert: %w", err)
	}
	token, err := ValidatePKIToken(cert, attestationToken)
	if err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, fmt.Errorf("failed to load root cert: %w", err)
	}
	if !token.Valid {
		return connector.ITeeAvailabilityCheckResponseBody{}, fmt.Errorf("attestation token is invalid: %s", attestationToken)
	}
	statusInfo, err := ValidateClaims(token, infoData)
	if err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, fmt.Errorf("failed to validate claims: %w", err)
	}
	lastSigningPolicyHash, err := v.getLastSigningPolicyHashFromChain(v.client, infoData.LastSigningPolicyId)
	if lastSigningPolicyHash != infoData.LastSigningPolicyHash {
		return connector.ITeeAvailabilityCheckResponseBody{}, fmt.Errorf("failed to validate last signing policy hash")
	}
	if err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, fmt.Errorf("failed to retrieve last signing policy hash: %w", err)
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

func (v *TeeVerifier) fetchTEEAvailabilityResult(ctx context.Context, baseURL, challengeInstructionId string) (types.ProxyInfoResponseBody, error) {
	url := fmt.Sprintf("%s/action/result/%s", baseURL, challengeInstructionId)
	client := &http.Client{
		Timeout: 5 * time.Second,
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

func (v *TeeVerifier) getLastSigningPolicyHashFromChain(client *ethclient.Client, lastSigningPolicyId *big.Int) (common.Hash, error) {
	contractAddrStr := v.cfg.RelayContractAddress
	contractAddress := common.HexToAddress(contractAddrStr)
	relayCaller, err := relay.NewRelayCaller(contractAddress, client)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to create contract caller: %w", err)
	}
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}
	lastSigningPolicyHash, err := relayCaller.ToSigningPolicyHash(callOpts, lastSigningPolicyId)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to call ToSigningPolicyHash: %w", err)
	}
	return lastSigningPolicyHash, nil
}
