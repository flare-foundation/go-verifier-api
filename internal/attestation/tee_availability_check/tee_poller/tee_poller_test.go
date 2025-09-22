package teepoller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	teetype "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/type"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	"github.com/stretchr/testify/require"
)

func TestSampleAllTees(t *testing.T) {
	setup := func() (*verifier.TeeVerifier, context.Context, context.CancelFunc) {
		v := &verifier.TeeVerifier{
			TeeSamples:        make(map[common.Address][]teetype.TeePollerSample),
			SamplesToConsider: 3,
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		return v, ctx, cancel
	}

	t.Run("Success", func(t *testing.T) {
		v, ctx, cancel := setup()
		defer cancel()

		getTees := func(ctx context.Context, v *verifier.TeeVerifier) (teeList, error) {
			return teeList{
				TeeIDs: []common.Address{common.HexToAddress("0x1")},
				URLs:   []string{"url"},
			}, nil
		}
		fakeValidator := func(ctx context.Context, v *verifier.TeeVerifier, proxyURL string) (teetype.TeePollerSampleState, error) {
			return teetype.TeePollerSampleValid, nil
		}

		sampleAllTees(ctx, v, getTees, fakeValidator)

		v.SamplesMu.RLock()
		defer v.SamplesMu.RUnlock()
		require.Len(t, v.TeeSamples, 1)
		require.NotEmpty(t, v.TeeSamples[common.HexToAddress("0x1")])
	})

	t.Run("FallbackToCache", func(t *testing.T) {
		v, ctx, cancel := setup()
		defer cancel()

		updateActiveTees(teeList{
			TeeIDs: []common.Address{common.HexToAddress("0x2")},
			URLs:   []string{"url"},
		})

		getTees := func(ctx context.Context, v *verifier.TeeVerifier) (teeList, error) {
			return teeList{}, errors.New("boom")
		}
		fakeValidator := func(ctx context.Context, v *verifier.TeeVerifier, proxyURL string) (teetype.TeePollerSampleState, error) {
			return teetype.TeePollerSampleIndeterminate, nil
		}

		sampleAllTees(ctx, v, getTees, fakeValidator)

		v.SamplesMu.RLock()
		defer v.SamplesMu.RUnlock()
		require.Contains(t, v.TeeSamples, common.HexToAddress("0x2"))
	})
}
