package xrpverifier

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	nodetypes "github.com/flare-foundation/tee-node/pkg/types"

	"github.com/flare-foundation/go-flare-common/pkg/xrpl/address"

	apitypes "github.com/flare-foundation/go-verifier-api/internal/api/types"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigconfigured/xrp/client"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigconfigured/xrp/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

var (
	ErrValidationFailed = errors.New("multisig account validation failed")
	// ErrInvalidRequest is returned when the request shape violates documented
	// constraints (e.g. too many public keys, empty key entries). Maps to HTTP 400.
	ErrInvalidRequest = errors.New("invalid multisig request")
	// ErrNetworkMismatch marks an XRPL node confirmed to be on a different network
	// than the source requires (a wrong-chain node). Maps to HTTP 503.
	ErrNetworkMismatch = errors.New("XRPL node is on the wrong network")
)

type XRPVerifier struct {
	Config *config.PMWMultisigAccountConfig
	Client *client.Client

	// lastVerifiedNano is the unix-nano time the node's network was last confirmed
	// to match the source (0 = never); verifyMu serializes confirmation attempts.
	// Until it is fresh (within networkVerifyTTL) Verify fails closed, so a node
	// repointed to a different network is re-detected within the TTL.
	lastVerifiedNano atomic.Int64
	verifyMu         sync.Mutex
	now              func() time.Time // overridable in tests
}

func NewXRPVerifier(cfg *config.PMWMultisigAccountConfig) *XRPVerifier {
	client := client.NewClient(cfg.SourceRPCURL)

	return &XRPVerifier{Config: cfg, Client: client, now: time.Now}
}

const (
	// networkPinTimeout bounds a single server_info probe used to pin the network.
	networkPinTimeout = 5 * time.Second
	// networkVerifyTTL is how long a confirmed network is trusted before Verify
	// re-checks, so an endpoint repointed to a different network is caught within it.
	networkVerifyTTL = 30 * time.Minute
)

// clock returns the current time, using the injected now when set (tests).
func (x *XRPVerifier) clock() time.Time {
	if x.now != nil {
		return x.now()
	}
	return time.Now()
}

// verifiedFresh reports whether the network was confirmed within networkVerifyTTL.
func (x *XRPVerifier) verifiedFresh() bool {
	last := x.lastVerifiedNano.Load()
	return last != 0 && x.clock().Sub(time.Unix(0, last)) < networkVerifyTTL
}

// expectedNetworkID returns the XRPL network_id a source must be on — Mainnet (0)
// for XRP, Testnet (1) for testXRP — and whether the source has an XRPL network.
func expectedNetworkID(source config.SourceName) (uint32, bool) {
	switch source {
	case config.SourceXRP:
		return 0, true
	case config.SourceTestXRP:
		return 1, true
	default:
		return 0, false
	}
}

// checkNetwork probes the node's network_id once and classifies the result:
// nil (and marks the verifier verified) when it matches the source's expected
// network; ErrNetworkMismatch when it is a confirmed wrong chain; a wrapped fetch
// error when the network cannot be read (unreachable node or no network_id).
func (x *XRPVerifier) checkNetwork(ctx context.Context) error {
	expected, ok := expectedNetworkID(x.Config.SourceIDPair.SourceID)
	if !ok {
		x.lastVerifiedNano.Store(x.clock().UnixNano())
		return nil
	}
	got, present, err := x.Client.NetworkID(ctx)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("%w: node reports no network_id", client.ErrFetchServerInfo)
	}
	if got != expected {
		return fmt.Errorf("%w: node network_id %d but source %s requires %d",
			ErrNetworkMismatch, got, x.Config.SourceIDPair.SourceID, expected)
	}
	x.lastVerifiedNano.Store(x.clock().UnixNano())
	return nil
}

// VerifyNetwork pins the configured XRPL node to the network its SOURCE_ID
// expects — Mainnet (0) for XRP, Testnet (1) for testXRP — so a node
// misconfigured to the wrong chain, whose address encodings coincide, cannot
// attest wrong-chain data. Run once at startup: a confirmed wrong chain fails
// boot, while an unreachable node or one that reports no network_id does not
// block boot (nor the co-located payment/fee-proof types) — the request path
// stays fail-closed via ensureNetworkVerified until the network is confirmed.
func (x *XRPVerifier) VerifyNetwork(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, networkPinTimeout)
	defer cancel()

	err := x.checkNetwork(ctx)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrNetworkMismatch):
		return err
	default:
		logger.Warnf("PMWMultisigAccountConfigured: XRPL network not verified at startup for source %s: %v; requests are blocked until it verifies",
			x.Config.SourceIDPair.SourceID, err)
		return nil
	}
}

// ensureNetworkVerified fails closed until the node's network has been confirmed.
// A fresh confirmation (within networkVerifyTTL) is a lock-free no-op; otherwise
// it re-probes — so a wrong-chain or unreachable node keeps every request
// rejected, and a node repointed to a different network is caught within the TTL.
func (x *XRPVerifier) ensureNetworkVerified(ctx context.Context) error {
	if x.verifiedFresh() {
		return nil
	}
	x.verifyMu.Lock()
	defer x.verifyMu.Unlock()
	if x.verifiedFresh() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, networkPinTimeout)
	defer cancel()
	return x.checkNetwork(ctx)
}

func (x *XRPVerifier) Verify(ctx context.Context, req fdc2.IPMWMultisigAccountConfiguredRequestBody) (fdc2.IPMWMultisigAccountConfiguredResponseBody, error) {
	// Enforce request shape here so direct ABI callers (verify / prepareResponseBody)
	// cannot bypass the limits applied by the JSON ToInternal path: at least one
	// public key, no more than MaxPublicKeys, no empty entries, and a non-zero
	// threshold.
	if err := apitypes.ValidateMultisigRequest(req.PublicKeys, req.Threshold); err != nil {
		return fdc2.IPMWMultisigAccountConfiguredResponseBody{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	// Fail closed until the node's network is confirmed: a node that could not be
	// verified at startup (unreachable then, or a wrong chain) must not serve.
	if err := x.ensureNetworkVerified(ctx); err != nil {
		return fdc2.IPMWMultisigAccountConfiguredResponseBody{}, err
	}
	accountInfo, err := x.Client.FetchAccountInfo(ctx, req.AccountAddress)
	if err != nil {
		return fdc2.IPMWMultisigAccountConfiguredResponseBody{}, err
	}
	sequence, err := x.validateMultisigConfiguration(accountInfo, req)
	if err != nil {
		return fdc2.IPMWMultisigAccountConfiguredResponseBody{
			Status:   uint8(apitypes.PMWMultisigAccountStatusERROR),
			Sequence: 0,
		}, nil
	}
	return fdc2.IPMWMultisigAccountConfiguredResponseBody{
		Status:   uint8(apitypes.PMWMultisigAccountStatusOK),
		Sequence: sequence,
	}, nil
}

func (x *XRPVerifier) validateMultisigConfiguration(accountInfo *types.AccountInfoResponse, req fdc2.IPMWMultisigAccountConfiguredRequestBody) (uint64, error) {
	// Bind the response back to the requested account before trusting any of its
	// fields: a misbehaving/compromised RPC must not be able to answer with a
	// different (correctly-configured) account's data and have it accepted for
	// req.AccountAddress. The on-chain wallet is keyed by the raw accountAddress
	// (TeePayments._toAccountHash), and XRPL echoes the classic address in
	// account_data.Account, so a byte-for-byte comparison is the correct
	// canonical-form check (normalizing here but not in the contract would diverge).
	if accountInfo.Result.AccountData.Account != req.AccountAddress {
		return 0, fmt.Errorf("account_info returned account %q for requested %q: %w", accountInfo.Result.AccountData.Account, req.AccountAddress, ErrValidationFailed)
	}
	if accountInfo.Result.Validated == nil || !*accountInfo.Result.Validated {
		return 0, fmt.Errorf("account_info response is not from a validated ledger for %s: %w", accountInfo.Result.AccountData.Account, ErrValidationFailed)
	}
	// There is only a single signer list for an account.
	// From docs: If a future amendment allows multiple signer lists for an account, this may change.[https://xrpl.org/docs/references/protocol/ledger-data/ledger-entry-types/signerlist]
	signerLists := accountInfo.Result.ResolveSignerLists()
	if len(signerLists) == 0 {
		return 0, fmt.Errorf("no signer list for account %s: %w", accountInfo.Result.AccountData.Account, ErrValidationFailed)
	}
	signersValid := x.validateSignerList(signerLists[0], req)
	if !signersValid {
		return 0, fmt.Errorf("signer list invalid for account %s: %w", accountInfo.Result.AccountData.Account, ErrValidationFailed)
	}
	flags := accountInfo.Result.AccountFlags
	if flags == nil {
		return 0, fmt.Errorf("account_flags missing from account_info response for %s: %w", accountInfo.Result.AccountData.Account, ErrValidationFailed)
	}
	if err := checkAccountFlags(*flags); err != nil {
		return 0, fmt.Errorf("invalid flag for account%s: %w: %w", accountInfo.Result.AccountData.Account, err, ErrValidationFailed)
	}
	if accountInfo.Result.AccountData.RegularKey != "" {
		return 0, fmt.Errorf("account %s has regular key set: %w", accountInfo.Result.AccountData.Account, ErrValidationFailed)
	}
	return accountInfo.Result.AccountData.Sequence, nil
}

func (x *XRPVerifier) validateSignerList(signerList types.SignerList, req fdc2.IPMWMultisigAccountConfiguredRequestBody) bool {
	expectedAccounts := make(map[string]struct{}, len(req.PublicKeys))
	for _, pk := range req.PublicKeys {
		addrStr, err := XRPAddressFromPubKey(pk)
		if err != nil {
			logger.Warnf("Failed to convert public key %s to address: %v", hex.EncodeToString(pk), err)
			return false
		}
		expectedAccounts[addrStr] = struct{}{}
	}
	actualAccounts := signerList.AccountsMap()
	if len(actualAccounts) != len(expectedAccounts) {
		return false
	}
	for acc := range expectedAccounts {
		weight, found := actualAccounts[acc]
		if !found || weight != 1 {
			return false
		}
	}
	return signerList.SignerQuorum == req.Threshold
}

func XRPAddressFromPubKey(pubkey []byte) (string, error) {
	pk, err := nodetypes.ParsePubKeyBytes(pubkey)
	if err != nil {
		return "", err
	}
	compressed := crypto.CompressPubkey(pk)
	return address.PubToAddress(hex.EncodeToString(compressed))
}

func checkAccountFlags(flags types.AccountFlags) error {
	switch {
	case !flags.DisableMasterKey:
		return errors.New("master key is not disabled")
	case flags.DepositAuth:
		return errors.New("deposit authorization is enabled")
	case flags.RequireDestinationTag:
		return errors.New("destination tag is required")
	case flags.DisallowIncomingXRP:
		return errors.New("incoming XRP is disallowed")
	}
	return nil
}
