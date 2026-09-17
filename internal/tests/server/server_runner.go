package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/api"
	cfgpkg "github.com/flare-foundation/go-verifier-api/internal/config"
)

const (
	port            = "3121"
	apiKey          = "test-api-key"
	serverTimeout   = 5 * time.Second
	serverTickDelay = 10 * time.Millisecond
)

// TestDestinationSlug is the destination-chain URL slug test servers run under
// when the test does not set one; it pairs with helpers.TestChainID (16).
const TestDestinationSlug = "coston"

type TestSetupServer struct {
	URL                    string
	AttestationTypeEncoded common.Hash
	SourceIDEncoded        common.Hash
	Stop                   func()
	Port                   string
	APIKey                 string
}

func SetupServer(t *testing.T, attestationType fdc2.AttestationType, sourceID cfgpkg.SourceName, config cfgpkg.EnvConfig) TestSetupServer {
	t.Helper()
	config.AttestationType = attestationType
	config.SourceID = sourceID
	config.Port = port
	config.APIKeys = []string{apiKey}
	if config.DestinationChainURLSlug == "" {
		config.DestinationChainURLSlug = TestDestinationSlug
	}

	stop := RunServerForTest(t, config)
	waitForServer(t, fmt.Sprintf("http://localhost:%s%s/api/health", config.Port, cfgpkg.DeploymentPrefix(sourceID, config.DestinationChainURLSlug)))

	url := fmt.Sprintf("http://localhost:%s%s/%s", config.Port, cfgpkg.DeploymentPrefix(sourceID, config.DestinationChainURLSlug), attestationType)
	attTypeEncoded, sourceIDEncoded := prepareAttestationTypeAndSourceID(t, attestationType, sourceID)

	return TestSetupServer{URL: url, AttestationTypeEncoded: attTypeEncoded, SourceIDEncoded: sourceIDEncoded, Stop: stop, Port: port, APIKey: apiKey}
}

// TestSetupMultiServer is a running server that serves several attestation types
// for one source (the per-source deployment shape).
type TestSetupMultiServer struct {
	BaseURL         string
	Stop            func()
	Port            string
	APIKey          string
	sourceID        cfgpkg.SourceName
	destinationSlug string
}

// SetupMultiServer starts one server that serves attestationTypes for sourceID,
// mirroring a per-source deployment. Per-type endpoint bases come from URL.
func SetupMultiServer(t *testing.T, sourceID cfgpkg.SourceName, attestationTypes []fdc2.AttestationType, cfg cfgpkg.EnvConfig) TestSetupMultiServer {
	t.Helper()
	cfg.AttestationTypes = attestationTypes
	cfg.SourceID = sourceID
	cfg.Port = port
	cfg.APIKeys = []string{apiKey}
	if cfg.DestinationChainURLSlug == "" {
		cfg.DestinationChainURLSlug = TestDestinationSlug
	}

	stop := RunServerForTest(t, cfg)
	waitForServer(t, fmt.Sprintf("http://localhost:%s%s/api/health", cfg.Port, cfgpkg.DeploymentPrefix(sourceID, cfg.DestinationChainURLSlug)))

	return TestSetupMultiServer{
		BaseURL:         "http://localhost:" + cfg.Port,
		Stop:            stop,
		Port:            port,
		APIKey:          apiKey,
		sourceID:        sourceID,
		destinationSlug: cfg.DestinationChainURLSlug,
	}
}

// URL returns the endpoint base for one attestation type served by this server.
func (s TestSetupMultiServer) URL(attestationType fdc2.AttestationType) string {
	return fmt.Sprintf("%s%s/%s", s.BaseURL, cfgpkg.DeploymentPrefix(s.sourceID, s.destinationSlug), attestationType)
}

func RunServerForTest(t *testing.T, envConfig cfgpkg.EnvConfig) (stop func()) {
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
// t.Cleanup; pass its URL as the FLARE_RPC_URL env value.
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
		if _, err := fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%q}`, id, result); err != nil {
			t.Errorf("write mock JSON-RPC response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func prepareAttestationTypeAndSourceID(t *testing.T, attestationType fdc2.AttestationType, sourceID cfgpkg.SourceName) (common.Hash, common.Hash) {
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
