package transaction

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/helper"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/xrp/types"
)

func FindReceivedAmountForAddress(meta *types.TransactionMetaData, receiver string) (*big.Int, error) {
	receivedAmounts, err := ReceivedAmount(meta)
	if err != nil {
		return nil, err
	}
	for _, ra := range receivedAmounts {
		if ra.Address == receiver {
			return ra.Amount, nil
		}
	}
	return big.NewInt(0), nil
}

func ReceivedAmount(meta *types.TransactionMetaData) ([]types.AddressAmount, error) {
	if meta == nil {
		return nil, errors.New("transaction meta is not available, thus received amounts cannot be calculated")
	}
	var received []types.AddressAmount

	for _, node := range meta.AffectedNodes {
		if mod := node.ModifiedNode; mod != nil {
			aa, err := extractFromModifiedNode(mod)
			if err != nil {
				return nil, err
			}
			if aa != nil {
				received = append(received, *aa)
			}
			continue
		}
		if created := node.CreatedNode; created != nil {
			aa, err := extractFromCreatedNode(created)
			if err != nil {
				return nil, err
			}
			if aa != nil {
				received = append(received, *aa)
			}
		}
	}
	return received, nil
}

func extractFromModifiedNode(mod *types.ModifiedNode) (*types.AddressAmount, error) {
	if mod.LedgerEntryType != "AccountRoot" {
		return nil, nil
	}
	finalFields := mod.FinalFields
	previousFields := mod.PreviousFields
	if finalFields == nil || previousFields == nil {
		return nil, nil
	}
	account, ok1 := getStringField(finalFields, "Account")
	finalBalStr, ok2 := getStringField(finalFields, "Balance")
	prevBalStr, ok3 := getStringField(previousFields, "Balance")
	if !ok1 || !ok2 || !ok3 {
		return nil, nil
	}
	finalBal, err := helper.ParseNonNegativeBigInt(finalBalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid final balance format for account %s: %s", account, finalBalStr)
	}
	// A native XRP balance cannot exceed the total supply; a larger value is corrupt
	// or forged metadata and must fail closed even if the final−previous delta looks
	// plausible (e.g. final=supply+1000, previous=supply).
	if helper.ExceedsMaxXRPDrops(finalBal) {
		return nil, fmt.Errorf("final balance %s for account %s exceeds total XRP supply in drops: %w", finalBal, account, db.ErrDatabase)
	}
	prevBal, err := helper.ParseNonNegativeBigInt(prevBalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid previous balance format for account %s: %s", account, prevBalStr)
	}
	if helper.ExceedsMaxXRPDrops(prevBal) {
		return nil, fmt.Errorf("previous balance %s for account %s exceeds total XRP supply in drops: %w", prevBal, account, db.ErrDatabase)
	}
	diff := new(big.Int).Sub(finalBal, prevBal)
	if diff.Sign() <= 0 {
		return nil, nil
	}
	return &types.AddressAmount{
		Address: account,
		Amount:  diff,
	}, nil
}

func extractFromCreatedNode(created *types.CreatedNode) (*types.AddressAmount, error) {
	if created.LedgerEntryType != "AccountRoot" {
		return nil, nil
	}
	newFields := created.NewFields
	if newFields == nil {
		return nil, nil
	}
	account, ok1 := getStringField(newFields, "Account")
	balanceStr, ok2 := getStringField(newFields, "Balance")
	if !ok1 || !ok2 {
		return nil, nil
	}
	balance, err := helper.ParseNonNegativeBigInt(balanceStr)
	if err != nil {
		return nil, fmt.Errorf("invalid balance format in CreatedNode for account %s: %s", account, balanceStr)
	}
	if helper.ExceedsMaxXRPDrops(balance) {
		return nil, fmt.Errorf("created-node balance %s for account %s exceeds total XRP supply in drops: %w", balance, account, db.ErrDatabase)
	}
	return &types.AddressAmount{
		Address: account,
		Amount:  balance,
	}, nil
}

func getStringField(m map[string]any, key string) (string, bool) {
	val, ok := m[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}
