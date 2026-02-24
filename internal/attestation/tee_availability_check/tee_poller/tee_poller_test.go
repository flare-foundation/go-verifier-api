package teepoller

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/flare-foundation/go-verifier-api/internal/tests/helpers"
	"github.com/stretchr/testify/require"
)

func TestSampleAllTees(t *testing.T) {
	setup := func() (*verifier.TeeVerifier, context.Context, context.CancelFunc) {
		t.Helper()
		tmpV, err := verifier.NewVerifier(&config.TeeAvailabilityCheckConfig{
			RPCURL:                            "https://coston-api.flare.network/ext/C/rpc",
			RelayContractAddress:              "0x92a6E1127262106611e1e129BB64B6D8654273F7",
			TeeMachineRegistryContractAddress: "0x053568617FFccEe2F75073975CC0e1549Ff9db71",
			AllowTeeDebug:                     false,
			DisableAttestationCheckE2E:        false,
		})
		require.NoError(t, err)

		v, ok := tmpV.(*verifier.TeeVerifier)
		require.True(t, ok, "tmpV should be *TeeVerifier")

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		return v, ctx, cancel
	}
	t.Run("success", func(t *testing.T) {
		v, ctx, cancel := setup()
		defer cancel()
		mock := &mockTeeMachineRegistryCaller{
			getAllActiveFunc: func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error) {
				return mockAllActiveTeeMAchines(t, []string{"0x1"}, []string{"url"}), nil
			},
		}
		v.TeeMachineRegistryCaller = mock
		fakeValidator := func(ctx context.Context, v *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error) {
			return verifiertypes.TeeSampleValid, nil
		}
		s := NewTeePoller(v)
		s.sampleAllTees(ctx, fakeValidator)
		v.SamplesMu.RLock()
		defer v.SamplesMu.RUnlock()
		require.Len(t, v.TeeSamples, 1)
		require.NotEmpty(t, v.TeeSamples[common.HexToAddress("0x1")])
	})
	t.Run("fallback to cache", func(t *testing.T) {
		v, ctx, cancel := setup()
		defer cancel()
		mock := &mockTeeMachineRegistryCaller{
			getAllActiveFunc: func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error) {
				return teeMachinesResult{}, errors.New("boom")
			},
		}
		v.TeeMachineRegistryCaller = mock
		s := NewTeePoller(v)
		s.updateActiveTees(mockActiveTees(t, []string{"0x2"}, []string{"url"}))
		fakeValidator := func(ctx context.Context, v *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error) {
			return verifiertypes.TeeSampleIndeterminate, nil
		}
		s.sampleAllTees(ctx, fakeValidator)
		v.SamplesMu.RLock()
		defer v.SamplesMu.RUnlock()
		require.Contains(t, v.TeeSamples, common.HexToAddress("0x2"))
	})
	t.Run("try to fallback to cache (empty cache)", func(t *testing.T) {
		v, ctx, cancel := setup()
		defer cancel()
		mock := &mockTeeMachineRegistryCaller{
			getAllActiveFunc: func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error) {
				return teeMachinesResult{}, errors.New("boom")
			},
		}
		v.TeeMachineRegistryCaller = mock
		s := NewTeePoller(v)
		s.updateActiveTees(mockActiveTees(t, []string{}, []string{}))
		fakeValidator := func(ctx context.Context, v *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error) {
			return verifiertypes.TeeSampleIndeterminate, nil
		}
		s.sampleAllTees(ctx, fakeValidator)
		v.SamplesMu.RLock()
		defer v.SamplesMu.RUnlock()
		require.Empty(t, v.TeeSamples)
	})
	t.Run("truncate old samples", func(t *testing.T) {
		v, ctx, cancel := setup()
		defer cancel()
		mock := &mockTeeMachineRegistryCaller{
			getAllActiveFunc: func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error) {
				return mockAllActiveTeeMAchines(t, []string{"0x1"}, []string{"url"}), nil
			},
		}
		v.TeeMachineRegistryCaller = mock
		s := NewTeePoller(v)
		query := func(ctx context.Context, v *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error) {
			return verifiertypes.TeeSampleValid, nil
		}
		// Call multiple times to exceed SamplesToConsider
		for i := 0; i < verifier.SamplesToConsider+2; i++ {
			s.sampleAllTees(ctx, query)
		}
		v.SamplesMu.RLock()
		defer v.SamplesMu.RUnlock()
		require.Len(t, v.TeeSamples[common.HexToAddress("0x1")], verifier.SamplesToConsider)
	})
	t.Run("query failure does not crash and logs error", func(t *testing.T) {
		ver, _, cancel := setup()
		defer cancel()
		mock := &mockTeeMachineRegistryCaller{
			getAllActiveFunc: func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error) {
				return mockAllActiveTeeMAchines(t, []string{"0x1"}, []string{"url"}), nil
			},
		}
		ver.TeeMachineRegistryCaller = mock
		s := NewTeePoller(ver)
		query := func(ctx context.Context, v *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error) {
			return verifiertypes.TeeSampleInvalid, errors.New("query failed")
		}
		s.sampleAllTees(context.Background(), query)
		ver.SamplesMu.RLock()
		defer ver.SamplesMu.RUnlock()
		require.Len(t, ver.TeeSamples[common.HexToAddress("0x1")], 1)
		require.Equal(t, verifiertypes.TeeSampleInvalid, ver.TeeSamples[common.HexToAddress("0x1")][0].State)
	})
	t.Run("remove inactive TEEs", func(t *testing.T) {
		active := mockActiveTees(t, []string{"0x1", "0x2"}, []string{"url", "url2"})
		ver := &verifier.TeeVerifier{
			TeeSamples: make(map[common.Address][]verifiertypes.TeeSampleValue),
		}
		ver.TeeSamples[common.HexToAddress("0x1")] = []verifiertypes.TeeSampleValue{{State: verifiertypes.TeeSampleValid}}
		ver.TeeSamples[common.HexToAddress("0x2")] = []verifiertypes.TeeSampleValue{{State: verifiertypes.TeeSampleInvalid}}
		ver.TeeSamples[common.HexToAddress("0x3")] = []verifiertypes.TeeSampleValue{{State: verifiertypes.TeeSampleIndeterminate}} // inactive

		s := NewTeePoller(ver)
		s.filterTeeSamplesToActive(active)
		ver.SamplesMu.RLock()
		defer ver.SamplesMu.RUnlock()

		require.Contains(t, ver.TeeSamples, common.HexToAddress("0x1"))
		require.Contains(t, ver.TeeSamples, common.HexToAddress("0x2"))
		require.NotContains(t, ver.TeeSamples, common.HexToAddress("0x3")) // removed
		require.Len(t, ver.TeeSamples, 2)
	})
	t.Run("clear all when active list empty", func(t *testing.T) {
		ver := &verifier.TeeVerifier{
			TeeSamples: map[common.Address][]verifiertypes.TeeSampleValue{
				common.HexToAddress("0x1"): {{State: verifiertypes.TeeSampleValid}},
				common.HexToAddress("0x2"): {{State: verifiertypes.TeeSampleInvalid}},
			},
		}
		s := NewTeePoller(ver)
		s.filterTeeSamplesToActive(teeList{})
		ver.SamplesMu.RLock()
		defer ver.SamplesMu.RUnlock()
		require.Empty(t, ver.TeeSamples)
	})
}

func TestCachedActiveTees(t *testing.T) {
	expected := mockActiveTees(t, []string{"0xcafe"}, []string{"http://cached"})
	s := NewTeePoller(&verifier.TeeVerifier{})
	s.updateActiveTees(expected)

	got := s.getCachedActiveTees()
	require.Equal(t, expected, got)
}

func TestGetAllActiveTeeMachines(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockTeeMachineRegistryCaller{
			getAllActiveFunc: func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error) {
				ids := []string{"0xabc", "0xbcd", "0xcde"}
				urls := []string{"http://tee-abc", "http://tee-bce", "http://tee-cde"}
				s := int(start.Int64())
				e := int(end.Int64())
				if s < 0 {
					s = 0
				}
				if e > len(ids) {
					e = len(ids)
				}
				if s > e {
					s = e
				}
				return mockAllActiveTeeMAchines(t, ids[s:e], urls[s:e]), nil
			},
		}
		ver := &verifier.TeeVerifier{TeeMachineRegistryCaller: mock}
		s := NewTeePoller(ver)
		ctx := context.Background()
		list, err := s.getAllActiveTeeMachines(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, 3, len(list.TeeIDs))
		require.Equal(t, "http://tee-abc", list.URLs[0])
	})
	t.Run("error", func(t *testing.T) {
		mock := &mockTeeMachineRegistryCaller{
			getAllActiveFunc: func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error) {
				return teeMachinesResult{}, errors.New("contract failed")
			},
		}
		ver := &verifier.TeeVerifier{TeeMachineRegistryCaller: mock}
		s := NewTeePoller(ver)

		ctx := context.Background()
		list, err := s.getAllActiveTeeMachines(ctx, 1)
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
			return mockAllActiveTeeMAchines(t, []string{"0x123"}, []string{"http://tee-123"}), nil
		},
	}
	ver := &verifier.TeeVerifier{TeeMachineRegistryCaller: mock}
	s := NewTeePoller(ver)

	ctx := context.Background()
	list, err := s.getAllActiveTeesWithRetry(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(list.TeeIDs))
	require.Equal(t, "http://tee-123", list.URLs[0])
	require.GreaterOrEqual(t, callCount, 2, "should retry at least once")
}

func TestStartTeePoller_Close(t *testing.T) {
	ver := &verifier.TeeVerifier{
		TeeSamples: make(map[common.Address][]verifiertypes.TeeSampleValue),
	}
	ver.TeeMachineRegistryCaller = &mockTeeMachineRegistryCaller{
		getAllActiveFunc: func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error) {
			return mockAllActiveTeeMAchines(t, nil, nil), nil
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	service := NewTeePoller(ver)
	service.StartTeePoller(ctx)
	require.NotNil(t, service)
	err := service.Close()
	require.NoError(t, err)
}

func TestQueryTeeInfoAndValidate(t *testing.T) {
	// verifier setup
	verIface, err := verifier.NewVerifier(&config.TeeAvailabilityCheckConfig{
		RPCURL:                            "https://coston-api.flare.network/ext/C/rpc",
		RelayContractAddress:              "0x92a6E1127262106611e1e129BB64B6D8654273F7",
		TeeMachineRegistryContractAddress: "0x053568617FFccEe2F75073975CC0e1549Ff9db71",
		AllowTeeDebug:                     true,
		DisableAttestationCheckE2E:        true,
	})
	require.NoError(t, err)
	ver, ok := verIface.(*verifier.TeeVerifier)
	require.True(t, ok, "verIface should be *TeeVerifier")
	ver.ValidateURL = nil
	// eth client
	// #nosec G115: only used in test, integer overflow not a concern
	now := uint64(time.Now().Unix())
	challengeHash := common.HexToHash("0x123")
	failedChallengeHash := common.HexToHash("0x1")
	challengeBlock := types.NewBlockWithHeader(&types.Header{Time: now - 10})
	failedBlock := types.NewBlockWithHeader(&types.Header{Time: now - 300})
	latestBlock := types.NewBlockWithHeader(&types.Header{Time: now})
	mockClient := &helpers.MockEthClient{
		BlockByHashFn: func(ctx context.Context, hash common.Hash) (*types.Block, error) {
			if hash == challengeHash {
				return challengeBlock, nil
			} else {
				return failedBlock, nil
			}
		},
		BlockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {
			return latestBlock, nil
		},
	}
	ver.EthClient = mockClient
	t.Run("success", func(t *testing.T) {
		server, privTEEKey := makeTeeInfoServer(t, challengeHash, false, false)
		defer server.Close()
		// test
		sampleState, err := queryTeeInfoAndValidate(context.Background(), ver, server.URL, crypto.PubkeyToAddress(privTEEKey.PublicKey))
		require.NoError(t, err)
		require.Equal(t, verifiertypes.TeeSampleValid, sampleState)
	})
	t.Run("invalid challenge", func(t *testing.T) {
		server, privTEEKey := makeTeeInfoServer(t, failedChallengeHash, false, false)
		defer server.Close()
		// test
		sampleState, err := queryTeeInfoAndValidate(context.Background(), ver, server.URL, crypto.PubkeyToAddress(privTEEKey.PublicKey))
		require.Equal(t, verifiertypes.TeeSampleInvalid, sampleState)
		require.ErrorContains(t, err, "challenge too old: 300 seconds old")
	})
	t.Run("signing policy fail", func(t *testing.T) {
		server, privTEEKey := makeTeeInfoServer(t, challengeHash, true, false)
		defer server.Close()
		// test
		sampleState, err := queryTeeInfoAndValidate(context.Background(), ver, server.URL, crypto.PubkeyToAddress(privTEEKey.PublicKey))
		require.Equal(t, verifiertypes.TeeSampleInvalid, sampleState)
		require.ErrorContains(t, err, fmt.Sprintf("signing policy check failed for TEE %s: failed to validate initial signing policy hash", crypto.PubkeyToAddress(privTEEKey.PublicKey)))
	})
	t.Run("teeInfo fail", func(t *testing.T) {
		server, privTEEKey := makeTeeInfoServer(t, challengeHash, true, true)
		defer server.Close()
		// test
		sampleState, err := queryTeeInfoAndValidate(context.Background(), ver, server.URL, crypto.PubkeyToAddress(privTEEKey.PublicKey))
		require.Equal(t, verifiertypes.TeeSampleInvalid, sampleState)
		require.ErrorContains(t, err, fmt.Sprintf("cannot fetch TEE info from %s: resource not found (404)", server.URL))
	})
	t.Run("data verification fail", func(t *testing.T) {
		verIfaceInt, err := verifier.NewVerifier(&config.TeeAvailabilityCheckConfig{
			RPCURL:                            "https://coston-api.flare.network/ext/C/rpc",
			RelayContractAddress:              "0x92a6E1127262106611e1e129BB64B6D8654273F7",
			TeeMachineRegistryContractAddress: "0x053568617FFccEe2F75073975CC0e1549Ff9db71",
			AllowTeeDebug:                     false,
			DisableAttestationCheckE2E:        false,
		})
		require.NoError(t, err)
		verInt, ok := verIfaceInt.(*verifier.TeeVerifier)
		require.True(t, ok, "verIface should be *TeeVerifier")
		verInt.ValidateURL = func(context.Context, string) error { return nil }
		// eth client
		// #nosec G115: only used in test, integer overflow not a concern
		mockClient := &helpers.MockEthClient{
			BlockByHashFn: func(ctx context.Context, hash common.Hash) (*types.Block, error) {
				return challengeBlock, nil
			},
			BlockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {
				return latestBlock, nil
			},
		}
		verInt.EthClient = mockClient
		server, privTEEKey := makeTeeInfoServer(t, challengeHash, false, false)
		defer server.Close()
		// test
		sampleState, err := queryTeeInfoAndValidate(context.Background(), verInt, server.URL, crypto.PubkeyToAddress(privTEEKey.PublicKey))
		require.Equal(t, verifiertypes.TeeSampleInvalid, sampleState)
		require.ErrorContains(t, err, fmt.Sprintf("data verification failed for TEE %s: cannot validate certificate signature: parsing and verifying: token is malformed: token contains an invalid number of segments", crypto.PubkeyToAddress(privTEEKey.PublicKey)))
	})
}

type teeMachinesResult = struct {
	TeeIds      []common.Address
	Urls        []string
	TotalLength *big.Int
}
type mockTeeMachineRegistryCaller struct {
	getAllActiveFunc func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error)
}

func (m *mockTeeMachineRegistryCaller) GetAllActiveTeeMachines(
	opts *bind.CallOpts, start, end *big.Int,
) (teeMachinesResult, error) {
	return m.getAllActiveFunc(opts, start, end)
}
func mockAllActiveTeeMAchines(t *testing.T, ids []string, urls []string) teeMachinesResult {
	t.Helper()
	addresses := make([]common.Address, len(ids))
	for i, id := range ids {
		addresses[i] = common.HexToAddress(id)
	}
	return teeMachinesResult{
		TeeIds:      addresses,
		Urls:        urls,
		TotalLength: big.NewInt(int64(len(ids))),
	}
}

func mockActiveTees(t *testing.T, ids []string, urls []string) teeList {
	t.Helper()
	addresses := make([]common.Address, len(ids))
	for i, id := range ids {
		addresses[i] = common.HexToAddress(id)
	}
	return teeList{
		TeeIDs: addresses,
		URLs:   urls,
	}
}

func makeTeeInfoServer(t *testing.T, challenge common.Hash, failSigningPolicy bool, notFound bool) (*httptest.Server, *ecdsa.PrivateKey) {
	t.Helper()
	handler := http.NewServeMux()
	resp, privKey := helpers.GetTeeInfoResponse(t, challenge)
	if notFound {
		handler.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
	} else {
		handler.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if failSigningPolicy {
				resp.TeeInfo.InitialSigningPolicyID = 4800
			}
			require.NoError(t, json.NewEncoder(w).Encode(resp))
		})
	}
	server := httptest.NewServer(handler)
	return server, privKey
}
