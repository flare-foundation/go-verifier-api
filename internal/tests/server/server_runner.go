package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
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

func SetupServer(t *testing.T, attestationType fdc2.AttestationType, sourceID config.SourceName, config config.EnvConfig) TestSetupServer {
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

// MockEthRPC starts a minimal JSON-RPC server that answers eth_call with the
// ABI-encoding of initialNonce (a uint64), letting the PMW verifiers' on-chain
// initial-nonce lookup resolve without a live Flare node. It ignores the call's
// account argument and returns the same initialNonce for every account, which
// suits the single-wallet fixtures. The returned server is closed via
// t.Cleanup; pass its URL as the RPC_URL env value.
func MockEthRPC(t *testing.T, initialNonce uint64) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)

		// eth_call returns the ABI-encoded uint64 (32-byte big-endian word); any
		// other method gets a benign quantity so ethclient bootstrapping never errors.
		result := "0x1"
		if req.Method == "eth_call" {
			result = fmt.Sprintf("0x%064x", initialNonce)
		}
		id := req.ID
		if len(id) == 0 {
			id = json.RawMessage("1")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%q}`, id, result)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func prepareAttestationTypeAndSourceID(t *testing.T, attestationType fdc2.AttestationType, sourceID config.SourceName) (common.Hash, common.Hash) {
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
