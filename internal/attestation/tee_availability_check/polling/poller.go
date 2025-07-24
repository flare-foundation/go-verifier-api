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

func SampleAllTees(ctx context.Context, teeVerifier *verifier.TeeVerifier) {
	activeTees, err := getActiveTees(teeVerifier)
	if err != nil {
		logger.Errorf("Failed to get active TEEs:", err)
	}
	for i, teeId := range activeTees.TeeIds {
		proxyUrl := activeTees.Urls[i]
		valid, _ := queryTeeInfoAndValidate(ctx, teeVerifier, proxyUrl) // TODO: consider error handling

		teeVerifier.SamplesMu.Lock()
		samples := teeVerifier.TeeSamples[teeId]
		samples = append(samples, valid)
		if len(samples) > teeVerifier.SamplesToConsider {
			samples = samples[1:] // sliding window: drop oldest
		}
		teeVerifier.TeeSamples[teeId] = samples
		teeVerifier.SamplesMu.Unlock()
	}
}

func queryTeeInfoAndValidate(ctx context.Context, teeVerifier *verifier.TeeVerifier, proxyUrl string) (bool, error) {
	valid, err := teeVerifier.FetchTEEInfoResultAndValidate(ctx, proxyUrl)
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
