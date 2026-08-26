package verifier

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestDataVerification_CRLFetchFailure(t *testing.T) {
	signedToken, _, _, _, _ := buildTestTokenWithCRLDists(t,
		[]string{"http://example.com/leaf.crl"},
		[]string{"http://example.com/intermediate.crl"},
	)

	v := &TeeVerifier{
		// Non-empty Audience so DataVerification reaches the CRL fetch
		// (the guard above it rejects an empty Audience first).
		Cfg: &config.TeeAvailabilityCheckConfig{
			TeeAudience: "test-audience",
		},
		CRLCache: &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				return nil, errors.New("fetch failed")
			},
		},
	}

	resp := teenodetypes.TeeInfoResponse{
		Attestation: signedToken,
	}

	// A genuine CRL network-fetch failure is TRANSIENT (retryable), never a
	// terminal attestation-invalid rejection.
	_, err := v.DataVerification(context.Background(), resp, common.Address{})
	require.ErrorIs(t, err, ErrTEERevocationUnavailable)
	require.NotErrorIs(t, err, ErrTEEDataValidation,
		"a CRL fetch outage must not be classifiable as a terminal TEE validation failure")
}

// TestDataVerification_CorruptCRLIsTerminal: when the fetch SUCCEEDS but the served
// bytes are not a valid CRL, that is a deterministic problem — it must NOT be tagged
// transient, so it fails closed as an invalid attestation rather than retrying forever.
func TestDataVerification_CorruptCRLIsTerminal(t *testing.T) {
	signedToken, _, _, _, _ := buildTestTokenWithCRLDists(t,
		[]string{"http://example.com/leaf.crl"},
		[]string{"http://example.com/intermediate.crl"},
	)
	v := &TeeVerifier{
		Cfg: &config.TeeAvailabilityCheckConfig{TeeAudience: "test-audience"},
		CRLCache: &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				return []byte("not a CRL"), nil // reachable, but garbage → parse fails (deterministic)
			},
		},
	}
	resp := teenodetypes.TeeInfoResponse{Attestation: signedToken}

	_, err := v.DataVerification(context.Background(), resp, common.Address{})
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrTEERevocationUnavailable,
		"a corrupt CRL (fetch succeeded) is deterministic and must not be retryable")
}
