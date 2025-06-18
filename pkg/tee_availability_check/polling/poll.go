package polling

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"gitlab.com/urskak/verifier-api/pkg/tee_availability_check/types"
)

type TeeInfoValidity uint8

const (
	VALID TeeInfoValidity = iota
	INVALID
)

type TeeInfoData struct {
	TeeId        common.Address
	URL          string
	TeeTimestamp uint64
	Timestamp    uint64
	Validity     TeeInfoValidity
}

var TeeSamples = make(map[common.Address][]bool)

const (
	sampleInterval    = 1 * time.Minute
	samplesToConsider = 5
)

func SampleAllTees(client *ethclient.Client) {
	activeTees, err := GetActiveTees(client)
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
