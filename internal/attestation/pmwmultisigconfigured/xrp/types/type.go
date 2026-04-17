package types

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
	Validated    *bool         `json:"validated,omitempty"`
	LedgerIndex  *uint64       `json:"ledger_index,omitempty"`
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
