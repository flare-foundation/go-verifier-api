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
		// Non-empty Policy inputs so DataVerification reaches the CRL fetch
		// (the guard above it rejects empty Audience/AllowedImageIDs first).
		Cfg: &config.TeeAvailabilityCheckConfig{
			TeeAudience:        "test-audience",
			TeeAllowedImageIDs: map[common.Hash]struct{}{{0x01}: {}},
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

	_, err := v.DataVerification(context.Background(), resp, common.Address{})
	require.ErrorContains(t, err, "CRL fetch failed")
}
