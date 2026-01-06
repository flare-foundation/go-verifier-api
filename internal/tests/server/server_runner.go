package server

import (
	"context"
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

const (
	port            = "3121"
	apiKey          = "test-api-key"
	serverTimeout   = 5 * time.Second
	serverTickDelay = 10 * time.Millisecond
)

type TestSetupServer struct {
	URL                    string
	AttestationTypeEncoded common.Hash
	SourceIDEncoded        common.Hash
	Stop                   func()
	Port                   string
	APIKey                 string
}

func SetupServer(t *testing.T, attestationType connector.AttestationType, sourceID config.SourceName, config config.EnvConfig) TestSetupServer {
	t.Helper()
	config.AttestationType = attestationType
	config.SourceID = sourceID
	config.Port = port
	config.APIKeys = []string{apiKey}

	stop := RunServerForTest(t, config)
	waitForServer(t, fmt.Sprintf("http://localhost:%s/api/health", config.Port))

	url := fmt.Sprintf("http://localhost:%s/verifier/%s/%s", config.Port, strings.ToLower(string(sourceID)), attestationType)
	attTypeEncoded, sourceIDEncoded := prepareAttestationTypeAndSourceID(t, attestationType, sourceID)

	return TestSetupServer{URL: url, AttestationTypeEncoded: attTypeEncoded, SourceIDEncoded: sourceIDEncoded, Stop: stop, Port: port, APIKey: apiKey}
}

func RunServerForTest(t *testing.T, envConfig config.EnvConfig) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	srv, closers := api.StartServer(ctx, envConfig)

	stop = func() {
		cancel()
		api.ShutdownServer(srv, closers)
	}

	return stop
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
	deadline := time.After(serverTimeout)
	ticker := time.NewTicker(serverTickDelay)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("Server did not become healthy within %s", serverTimeout)
		case <-ticker.C:
			// #nosec G107: URL is controlled in test setup
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
