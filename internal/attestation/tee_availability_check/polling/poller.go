package polling

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeregistry"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	teeavailabilitycheckconfig "gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/config"
	"gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/types"
)

var TeeSamples = make(map[common.Address][]bool)

const (
	sampleInterval    = 1 * time.Minute
	samplesToConsider = 5
)

func SampleAllTees(client *ethclient.Client) {
	activeTees, err := getActiveTees(client)
	if err != nil {
		logger.Errorf("Failed to get active TEEs:", err)
	}
	for i, teeId := range activeTees.TeeIds {
		url := activeTees.Urls[i]
		valid := queryTeeInfo(url)
		// Sliding window
		TeeSamples[teeId] = append(TeeSamples[teeId], valid)
		if len(TeeSamples[teeId]) > samplesToConsider {
			TeeSamples[teeId] = TeeSamples[teeId][1:]
		}

	}
}

func queryTeeInfo(url string) bool {
	httpClient := http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Get(url + "/info")
	if err != nil {
		logger.Errorf("Failed to connect to TEE %s: %v", url, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("Invalid response from Tee:", url)
		return false
	}
	var info types.ProxyInfoResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		logger.Errorf("Failed to decode '/info' response:", err)
		return false
	}
	// TODO validation logic:
	return true
}

func IsTeeInfoValid(teeId common.Address) (bool, error) {
	samples := TeeSamples[teeId]
	if len(samples) < samplesToConsider {
		return false, fmt.Errorf("tee %s invalid after %d samples: %+v", teeId.Hex(), len(samples), samples)
	}
	for _, sample := range samples {
		if sample {
			return true, nil
		}
	}
	return false, nil
}

type ActiveTees struct {
	TeeIds []common.Address
	Urls   []string
}

func getActiveTees(client *ethclient.Client) (ActiveTees, error) {
	cfg, err := teeavailabilitycheckconfig.LoadTeeAvailabilityCheckConfig()
	if err != nil {
		return ActiveTees{}, fmt.Errorf("failed to load config: %w", err)
	}
	contractAddrStr := cfg.TeeRegistryContractAddress
	contractAddress := common.HexToAddress(contractAddrStr)
	teeregistryCaller, err := teeregistry.NewTeeRegistryCaller(contractAddress, client)
	if err != nil {
		return ActiveTees{}, fmt.Errorf("failed to create contract caller: %w", err)
	}
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}
	activeTees, err := teeregistryCaller.GetActiveTees(callOpts)
	if err != nil {
		return ActiveTees{}, fmt.Errorf("failed to call GetActiveTeeIds: %w", err)
	}
	return activeTees, nil
}
