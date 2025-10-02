package teepoller

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
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
	t.Run("success", func(t *testing.T) {
		v, ctx, cancel := setup()
		defer cancel()
		getTees := func(ctx context.Context, v *verifier.TeeVerifier) (teeList, error) {
			return teeList{
				TeeIDs: []common.Address{common.HexToAddress("0x1")},
				URLs:   []string{"url"},
			}, nil
		}
		fakeValidator := func(ctx context.Context, v *verifier.TeeVerifier, proxyURL string, teeID common.Address) (teetype.TeePollerSampleState, error) {
			return teetype.TeePollerSampleValid, nil
		}
		sampleAllTees(ctx, v, getTees, fakeValidator)
		v.SamplesMu.RLock()
		defer v.SamplesMu.RUnlock()
		require.Len(t, v.TeeSamples, 1)
		require.NotEmpty(t, v.TeeSamples[common.HexToAddress("0x1")])
	})
	t.Run("fallback to cache", func(t *testing.T) {
		v, ctx, cancel := setup()
		defer cancel()
		updateActiveTees(teeList{
			TeeIDs: []common.Address{common.HexToAddress("0x2")},
			URLs:   []string{"url"},
		})
		getTees := func(ctx context.Context, v *verifier.TeeVerifier) (teeList, error) {
			return teeList{}, errors.New("boom")
		}
		fakeValidator := func(ctx context.Context, v *verifier.TeeVerifier, proxyURL string, teeID common.Address) (teetype.TeePollerSampleState, error) {
			return teetype.TeePollerSampleIndeterminate, nil
		}
		sampleAllTees(ctx, v, getTees, fakeValidator)
		v.SamplesMu.RLock()
		defer v.SamplesMu.RUnlock()
		require.Contains(t, v.TeeSamples, common.HexToAddress("0x2"))
	})
	t.Run("truncate old samples", func(t *testing.T) {
		ver := &verifier.TeeVerifier{
			TeeSamples:        make(map[common.Address][]teetype.TeePollerSample),
			SamplesToConsider: 2,
		}
		getTees := func(ctx context.Context, v *verifier.TeeVerifier) (teeList, error) {
			return teeList{TeeIDs: []common.Address{common.HexToAddress("0x1")}, URLs: []string{"url"}}, nil
		}
		callCount := 0
		query := func(ctx context.Context, v *verifier.TeeVerifier, proxyURL string, teeID common.Address) (teetype.TeePollerSampleState, error) {
			callCount++
			return teetype.TeePollerSampleValid, nil
		}
		// Call multiple times to exceed SamplesToConsider
		for i := 0; i < 3; i++ {
			sampleAllTees(context.Background(), ver, getTees, query)
		}
		ver.SamplesMu.RLock()
		defer ver.SamplesMu.RUnlock()
		require.Len(t, ver.TeeSamples[common.HexToAddress("0x1")], 2) // only last 2 samples kept
	})
	t.Run("query failure does not crash and logs error", func(t *testing.T) {
		ver := &verifier.TeeVerifier{
			TeeSamples:        make(map[common.Address][]teetype.TeePollerSample),
			SamplesToConsider: 2,
		}
		getTees := func(ctx context.Context, v *verifier.TeeVerifier) (teeList, error) {
			return teeList{TeeIDs: []common.Address{common.HexToAddress("0x1")}, URLs: []string{"url"}}, nil
		}
		query := func(ctx context.Context, v *verifier.TeeVerifier, proxyURL string, teeID common.Address) (teetype.TeePollerSampleState, error) {
			return teetype.TeePollerSampleInvalid, errors.New("query failed")
		}
		sampleAllTees(context.Background(), ver, getTees, query)
		ver.SamplesMu.RLock()
		defer ver.SamplesMu.RUnlock()
		require.Len(t, ver.TeeSamples[common.HexToAddress("0x1")], 1)
		require.Equal(t, teetype.TeePollerSampleInvalid, ver.TeeSamples[common.HexToAddress("0x1")][0].State)
	})
	t.Run("remove inactive TEEs", func(t *testing.T) {
		active := teeList{
			TeeIDs: []common.Address{
				common.HexToAddress("0x1"),
				common.HexToAddress("0x2"),
			},
		}
		ver := &verifier.TeeVerifier{
			TeeSamples: make(map[common.Address][]teetype.TeePollerSample),
		}
		ver.TeeSamples[common.HexToAddress("0x1")] = []teetype.TeePollerSample{{State: teetype.TeePollerSampleValid}}
		ver.TeeSamples[common.HexToAddress("0x2")] = []teetype.TeePollerSample{{State: teetype.TeePollerSampleInvalid}}
		ver.TeeSamples[common.HexToAddress("0x3")] = []teetype.TeePollerSample{{State: teetype.TeePollerSampleIndeterminate}} // inactive

		filterTeeSamplesToActive(ver, active)
		ver.SamplesMu.RLock()
		defer ver.SamplesMu.RUnlock()

		require.Contains(t, ver.TeeSamples, common.HexToAddress("0x1"))
		require.Contains(t, ver.TeeSamples, common.HexToAddress("0x2"))
		require.NotContains(t, ver.TeeSamples, common.HexToAddress("0x3")) // removed
		require.Len(t, ver.TeeSamples, 2)
	})
	t.Run("clear all when active list empty", func(t *testing.T) {
		ver := &verifier.TeeVerifier{
			TeeSamples: map[common.Address][]teetype.TeePollerSample{
				common.HexToAddress("0x1"): {{State: teetype.TeePollerSampleValid}},
				common.HexToAddress("0x2"): {{State: teetype.TeePollerSampleInvalid}},
			},
		}
		filterTeeSamplesToActive(ver, teeList{})
		ver.SamplesMu.RLock()
		defer ver.SamplesMu.RUnlock()
		require.Empty(t, ver.TeeSamples)
	})
}

func (m *mockTeeMachineRegistryCaller) GetAllActiveTeeMachines(
	opts *bind.CallOpts, start, end *big.Int,
) (teeMachinesResult, error) {
	return m.getAllActiveFunc(opts, start, end)
}
func singleTee(id string, url string) teeMachinesResult {
	return teeMachinesResult{
		TeeIds:      []common.Address{common.HexToAddress(id)},
		Urls:        []string{url},
		TotalLength: big.NewInt(1),
	}
}

type teeMachinesResult = struct {
	TeeIds      []common.Address
	Urls        []string
	TotalLength *big.Int
}
type mockTeeMachineRegistryCaller struct {
	getAllActiveFunc func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error)
}

func TestGetAllActiveTeeMachines(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockTeeMachineRegistryCaller{
			getAllActiveFunc: func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error) {
				return singleTee("0xabc", "http://tee-abc"), nil
			},
		}

		ver := &verifier.TeeVerifier{TeeMachineRegistryCaller: mock}
		ctx := context.Background()

		list, err := getAllActiveTeeMachines(ctx, ver)
		require.NoError(t, err)
		require.Equal(t, 1, len(list.TeeIDs))
		require.Equal(t, "http://tee-abc", list.URLs[0])
	})
	t.Run("error", func(t *testing.T) {
		mock := &mockTeeMachineRegistryCaller{
			getAllActiveFunc: func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error) {
				return teeMachinesResult{}, errors.New("contract failed")
			},
		}

		ver := &verifier.TeeVerifier{TeeMachineRegistryCaller: mock}
		ctx := context.Background()

		list, err := getAllActiveTeeMachines(ctx, ver)
		require.ErrorContains(t, err, "contract failed")
		require.Empty(t, list.TeeIDs)
	})
}

func TestGetAllActiveTeesWithRetry(t *testing.T) {
	callCount := 0
	mock := &mockTeeMachineRegistryCaller{
		getAllActiveFunc: func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error) {
			callCount++
			if callCount == 1 {
				return teeMachinesResult{}, errors.New("boom")
			}
			return singleTee("0x123", "http://tee-123"), nil
		},
	}

	ver := &verifier.TeeVerifier{TeeMachineRegistryCaller: mock}
	ctx := context.Background()

	list, err := getAllActiveTeesWithRetry(ctx, ver)
	require.NoError(t, err)
	require.Equal(t, 1, len(list.TeeIDs))
	require.Equal(t, "http://tee-123", list.URLs[0])
	require.GreaterOrEqual(t, callCount, 2, "should retry at least once")
}

func TestCachedActiveTees(t *testing.T) {
	expected := teeList{
		TeeIDs: []common.Address{common.HexToAddress("0xcafe")},
		URLs:   []string{"http://cached"},
	}
	updateActiveTees(expected)

	got := getCachedActiveTees()
	require.Equal(t, expected, got)
}

func TestStartTeePoller_Close(t *testing.T) {
	ver := &verifier.TeeVerifier{
		TeeSamples: make(map[common.Address][]teetype.TeePollerSample),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	service := StartTeePoller(ctx, ver)
	require.NotNil(t, service)
	err := service.Close()
	require.NoError(t, err)
}
