package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/api"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

func SetupServer(t *testing.T, attestationType connector.AttestationType, sourceID config.SourceName, config config.EnvConfig) (string, common.Hash, common.Hash, func()) {
	t.Helper()
	config.AttestationType = attestationType
	config.SourceID = sourceID
	config.Env = "development"

	stop := api.RunServerForTest(t, config)
	waitForServer(t, fmt.Sprintf("http://localhost:%s/api/health", config.Port))

	url := fmt.Sprintf("http://localhost:%s/verifier/%s/%s", config.Port, strings.ToLower(string(sourceID)), attestationType)
	attTypeEncoded, sourceIdEncoded := prepareAttestationTypeAndSourceID(t, attestationType, sourceID)

	return url, attTypeEncoded, sourceIdEncoded, stop
}

func prepareAttestationTypeAndSourceID(t *testing.T, attestationType connector.AttestationType, sourceID config.SourceName) (common.Hash, common.Hash) {
	t.Helper()
	var attestationTypeBytes, sourceIDBytes [32]byte
	copy(attestationTypeBytes[:], attestationType)
	copy(sourceIDBytes[:], sourceID)
	return common.BytesToHash(attestationTypeBytes[:]), common.BytesToHash(sourceIDBytes[:])
}

func waitForServer(t *testing.T, url string) {
	t.Helper()
	timeout := 2 * time.Second
	deadline := time.After(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("Server did not become healthy within %s", timeout)
		case <-ticker.C:
			resp, err := http.Get(url)
			if err == nil && resp.StatusCode == http.StatusOK {
				_ = resp.Body.Close()
				return
			}
			if resp != nil {
				_ = resp.Body.Close()
			}
		}
	}
}
