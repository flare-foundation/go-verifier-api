package types

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
)

// MaxUtxoPublicKeys is Bitcoin's OP_CHECKMULTISIG consensus cap
// (MAX_PUBKEYS_PER_MULTISIG). An address with more keys can be created but its
// funds become permanently unspendable, so the limit is enforced up front.
const MaxUtxoPublicKeys = 20

// MaxUtxoAnchors caps the number of anchor chains a wallet may declare; it
// mirrors btcaddr.MaxAnchors (which also reserves derivation indices [0, 32)).
const MaxUtxoAnchors = 32

// anchorTxidLen is the byte length of a Bitcoin txid.
const anchorTxidLen = 32

// Anchor names one genesis anchor UTXO by outpoint. Its position in the anchors
// array is its chain identity and derivation index (order-significant).
type Anchor struct {
	GenesisAnchorTxid hexutil.Bytes `json:"genesisAnchorTxid" validate:"required" example:"0xabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcd"`
	GenesisAnchorVout uint32        `json:"genesisAnchorVout" example:"0"`
}

type PMWMultisigUtxoConfiguredRequestBody struct {
	AccountIndex uint32 `json:"accountIndex" example:"0"`
	// PublicKeys are the account's parent xpubs, each carried as the bytes of its
	// base58check string (e.g. "xpub6…") — the canonical form the chain registers
	// and the verifier expects. The example below is the hex of a base58 xpub string.
	PublicKeys []hexutil.Bytes `json:"publicKeys" validate:"required,min=1" example:"0x787075623636314d79...."`
	Threshold  uint64          `json:"threshold" validate:"gte=1" example:"2"`
	Anchors    []Anchor        `json:"anchors" validate:"required,min=1"`
}

// ValidateUtxoMultisigRequest enforces the request bounds shared by the JSON
// decoder (ToInternal) and the verifier: at least one public key up to the
// OP_CHECKMULTISIG cap, no empty key, a non-zero threshold no larger than the
// key count, and an anchor set of 1..MaxUtxoAnchors. The verifier re-runs the
// full btcaddr.ValidateV1 (xpub parsing, network, depth, outpoint distinctness);
// this mirror stops malformed direct-ABI requests that skip JSON validation.
func ValidateUtxoMultisigRequest(publicKeys [][]byte, threshold uint64, anchorCount int) error {
	if len(publicKeys) == 0 {
		return errors.New("publicKeys must not be empty")
	}
	if len(publicKeys) > MaxUtxoPublicKeys {
		return fmt.Errorf("too many public keys: %d (max %d)", len(publicKeys), MaxUtxoPublicKeys)
	}
	seen := make(map[string]int, len(publicKeys))
	for i, pk := range publicKeys {
		if len(pk) == 0 {
			return fmt.Errorf("public key at index %d is empty", i)
		}
		// Reject duplicate signer keys: a repeated key collapses k-of-n signer
		// independence. The verifier re-checks this on the derived pubkeys.
		if j, dup := seen[string(pk)]; dup {
			return fmt.Errorf("public key at index %d duplicates index %d; signers must be distinct", i, j)
		}
		seen[string(pk)] = i
	}
	if threshold == 0 {
		return errors.New("threshold must be greater than zero")
	}
	if threshold > uint64(len(publicKeys)) {
		return fmt.Errorf("threshold %d exceeds public key count %d", threshold, len(publicKeys))
	}
	if anchorCount < 1 || anchorCount > MaxUtxoAnchors {
		return fmt.Errorf("anchors set has %d elements; must be 1..%d", anchorCount, MaxUtxoAnchors)
	}
	return nil
}

func (requestBody PMWMultisigUtxoConfiguredRequestBody) ToInternal() (fdc2.IPMWMultisigUtxoConfiguredRequestBody, error) {
	publicKeys := make([][]byte, len(requestBody.PublicKeys))
	for i, pk := range requestBody.PublicKeys {
		publicKeys[i] = pk
	}
	if err := ValidateUtxoMultisigRequest(publicKeys, requestBody.Threshold, len(requestBody.Anchors)); err != nil {
		return fdc2.IPMWMultisigUtxoConfiguredRequestBody{}, err
	}

	anchors := make([]fdc2.IPMWMultisigUtxoConfiguredAnchor, len(requestBody.Anchors))
	for i, a := range requestBody.Anchors {
		if len(a.GenesisAnchorTxid) != anchorTxidLen {
			return fdc2.IPMWMultisigUtxoConfiguredRequestBody{}, fmt.Errorf("anchors[%d] genesisAnchorTxid must be %d bytes, got %d", i, anchorTxidLen, len(a.GenesisAnchorTxid))
		}
		var txid [32]byte
		copy(txid[:], a.GenesisAnchorTxid)
		anchors[i] = fdc2.IPMWMultisigUtxoConfiguredAnchor{
			GenesisAnchorTxid: txid,
			GenesisAnchorVout: a.GenesisAnchorVout,
		}
	}

	return fdc2.IPMWMultisigUtxoConfiguredRequestBody{
		AccountIndex: requestBody.AccountIndex,
		PublicKeys:   publicKeys,
		Threshold:    requestBody.Threshold,
		Anchors:      anchors,
	}, nil
}

type PMWMultisigUtxoConfiguredResponseBody struct {
	Status         uint8  `json:"status"`
	AccountAddress string `json:"accountAddress"`
}

type PMWMultisigUtxoConfiguredStatus int

const (
	PMWMultisigUtxoStatusOK PMWMultisigUtxoConfiguredStatus = iota
	PMWMultisigUtxoStatusERROR
)

func (s PMWMultisigUtxoConfiguredResponseBody) FromInternal(data fdc2.IPMWMultisigUtxoConfiguredResponseBody) ResponseConvertible[fdc2.IPMWMultisigUtxoConfiguredResponseBody] {
	return PMWMultisigUtxoConfiguredResponseBody{
		Status:         data.Status,
		AccountAddress: data.AccountAddress,
	}
}

func (s PMWMultisigUtxoConfiguredResponseBody) Log() {
	logger.Debugf("PMWMultisigUtxoConfigured result: Status=%d, AccountAddress=%s",
		s.Status, s.AccountAddress)
}

func LogPMWMultisigUtxoConfiguredRequestBody(req fdc2.IPMWMultisigUtxoConfiguredRequestBody) {
	logger.Debugf("PMWMultisigUtxoConfigured request: AccountIndex=%d, Threshold=%d, PublicKeys=%d keys, Anchors=%d",
		req.AccountIndex, req.Threshold, len(req.PublicKeys), len(req.Anchors))
}
