package types

import (
	"bytes"
	"fmt"
	"strconv"
)

// Derived from https://xrpl.org/docs/references/http-websocket-apis/public-api-methods/account-methods/account_info

type AccountData struct {
	Account     string       `json:"Account"`
	Sequence    uint64       `json:"Sequence"`
	RegularKey  string       `json:"RegularKey,omitempty"`
	SignerLists []SignerList `json:"signer_lists"`
}

type AccountFlags struct {
	DisableMasterKey      bool `json:"disableMasterKey"`
	DepositAuth           bool `json:"depositAuth"`
	RequireDestinationTag bool `json:"requireDestinationTag"`
	DisallowIncomingXRP   bool `json:"disallowIncomingXRP"`
}

type AccountInfoResult struct {
	AccountData  AccountData   `json:"account_data"`
	AccountFlags *AccountFlags `json:"account_flags,omitempty"`
	SignerLists  []SignerList  `json:"signer_lists"` // API v2/Clio returns signer_lists at result level
	Status       string        `json:"status"`
	// Error is the XRPL error code on a non-success response (status == "error"),
	// e.g. "actNotFound" (deterministic) or "noNetwork"/"tooBusy" (transient). The
	// code lives here, not in Status, so classification must read this field.
	Error       string  `json:"error,omitempty"`
	Validated   *bool   `json:"validated,omitempty"`
	LedgerIndex *uint64 `json:"ledger_index,omitempty"`
}

// ResolveSignerLists returns signer lists from whichever location they appear in the response.
// API v1 (rippled) nests them inside account_data; API v2/Clio places them at the result level.
func (r *AccountInfoResult) ResolveSignerLists() []SignerList {
	if len(r.AccountData.SignerLists) > 0 {
		return r.AccountData.SignerLists
	}
	return r.SignerLists
}

type AccountInfoResponse struct {
	Result AccountInfoResult `json:"result"`
}

// ServerInfoResponse carries the server_info fields the verifier needs to pin the
// node's network. network_id is 0 on Mainnet, 1 on Testnet, 2 on Devnet.
type ServerInfoResponse struct {
	Result struct {
		Status string `json:"status"`
		Info   struct {
			NetworkID NetworkID `json:"network_id"`
		} `json:"info"`
	} `json:"result"`
}

// NetworkID is an XRPL network id. rippled reports it as a JSON number, Clio as a
// JSON string, and either may omit it — so it is unmarshalled leniently. Present
// is false when the field is absent or null.
type NetworkID struct {
	Value   uint32
	Present bool
}

func (n *NetworkID) UnmarshalJSON(b []byte) error {
	// Accept a number (rippled) or a quoted string (Clio); treat null as absent.
	s := string(bytes.Trim(bytes.TrimSpace(b), `"`))
	if s == "" || s == "null" {
		return nil
	}
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid network_id %q: %w", s, err)
	}
	n.Value = uint32(v)
	n.Present = true
	return nil
}

type SignerEntry struct {
	Account      string `json:"Account"`
	SignerWeight uint16 `json:"SignerWeight"`
}

type SignerEntryWrapper struct {
	SignerEntry SignerEntry `json:"SignerEntry"`
}

type SignerList struct {
	SignerQuorum  uint64               `json:"SignerQuorum"`
	SignerEntries []SignerEntryWrapper `json:"SignerEntries"`
}

func (sl *SignerList) AccountsMap() map[string]uint16 {
	m := make(map[string]uint16, len(sl.SignerEntries))
	for _, entry := range sl.SignerEntries {
		m[entry.SignerEntry.Account] = entry.SignerEntry.SignerWeight
	}
	return m
}
