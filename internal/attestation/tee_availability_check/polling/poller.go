package polling

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
)

const SampleInterval = 1 * time.Minute

func SampleAllTees(teeVerifier *verifier.TeeVerifier) {
	activeTees, err := getActiveTees(teeVerifier)
	if err != nil {
		logger.Errorf("Failed to get active TEEs:", err)
	}
	for i, teeId := range activeTees.TeeIds {
		proxyUrl := activeTees.Urls[i]
		valid, _ := queryTeeInfoAndValidate(teeVerifier, proxyUrl) // TODO how to consider error? should it be undetermined of simply false
		// Sliding window
		teeVerifier.TeeSamples[teeId] = append(teeVerifier.TeeSamples[teeId], valid)
		if len(teeVerifier.TeeSamples[teeId]) > teeVerifier.SamplesToConsider {
			teeVerifier.TeeSamples[teeId] = teeVerifier.TeeSamples[teeId][1:]
		}

	}
}

func queryTeeInfoAndValidate(teeVerifier *verifier.TeeVerifier, proxyUrl string) (bool, error) {
	valid, err := teeVerifier.FetchTEEInfoResultAndValidate(context.Background(), proxyUrl) // TODO ctx??
	if err != nil {
		return false, err
	}
	if !valid {
		return false, nil
	}
	return true, nil
}

type ActiveTees struct {
	TeeIds []common.Address
	Urls   []string
}

func getActiveTees(teeVerifier *verifier.TeeVerifier) (ActiveTees, error) {
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}
	activeTees, err := teeVerifier.TeeRegistryCaller.GetActiveTees(callOpts)
	if err != nil {
		return ActiveTees{}, fmt.Errorf("failed to call GetActiveTeeIds: %w", err)
	}
	return activeTees, nil
}
