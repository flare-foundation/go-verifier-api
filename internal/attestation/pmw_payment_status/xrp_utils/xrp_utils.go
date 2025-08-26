package xrputils

import (
	"fmt"
	"math/big"

	xrptypes "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/types"
	pmwpaymentutils "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/utils"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
)

func GetTransactionStatus(result string) (xrptypes.TransactionStatus, error) {
	const transactionResultPrefixLength = 3
	if len(result) < transactionResultPrefixLength {
		return 0, fmt.Errorf("transaction result too short: %q", result)
	}
	prefix := result[:3]
	switch prefix {
	case "tes":
		return xrptypes.Success, nil
	case "tec":
		switch result {
		case "tecDST_TAG_NEEDED",
			"tecNO_DST",
			"tecNO_DST_INSUF_XRP",
			"tecNO_PERMISSION":
			return xrptypes.ReceiverFault, nil
		case "tecCANT_ACCEPT_OWN_NFTOKEN_OFFER",
			"tecCLAIM",
			"tecCRYPTOCONDITION_ERROR",
			"tecDIR_FULL",
			"tecDUPLICATE",
			"tecEXPIRED",
			"tecFAILED_PROCESSING",
			"tecFROZEN",
			"tecHAS_OBLIGATIONS",
			"tecINSUF_RESERVE_LINE",
			"tecINSUF_RESERVE_OFFER",
			"tecINSUFF_FEE",
			"tecINSUFFICIENT_FUNDS",
			"tecINSUFFICIENT_PAYMENT",
			"tecINSUFFICIENT_RESERVE",
			"tecINTERNAL",
			"tecINVARIANT_FAILED",
			"tecKILLED",
			"tecMAX_SEQUENCE_REACHED",
			"tecNEED_MASTER_KEY",
			"tecNFTOKEN_BUY_SELL_MISMATCH",
			"tecNFTOKEN_OFFER_TYPE_MISMATCH",
			"tecNO_ALTERNATIVE_KEY",
			"tecNO_AUTH",
			"tecNO_ENTRY",
			"tecNO_ISSUER",
			"tecNO_LINE",
			"tecNO_LINE_INSUF_RESERVE",
			"tecNO_LINE_REDUNDANT",
			"tecNO_REGULAR_KEY",
			"tecNO_SUITABLE_NFTOKEN_PAGE",
			"tecNO_TARGET",
			"tecOBJECT_NOT_FOUND",
			"tecOVERSIZE",
			"tecOWNERS",
			"tecPATH_DRY",
			"tecPATH_PARTIAL",
			"tecTOO_SOON",
			"tecUNFUNDED",
			"tecUNFUNDED_ADD",
			"tecUNFUNDED_PAYMENT",
			"tecUNFUNDED_OFFER":
			return xrptypes.SenderFault, nil
		default:
			return 0, fmt.Errorf("unknown tec error code: %s", result)
		}
	case "tef", "tel", "tem", "ter":
		return xrptypes.SenderFault, nil

	default:
		return 0, fmt.Errorf("unexpected transaction status prefix: %s", prefix)
	}
}

func FindReceivedAmountForAddress(meta *xrptypes.TransactionMetaData, receiver string) (*big.Int, error) {
	receivedAmounts, err := GetReceivedAmount(meta)
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

func GetReceivedAmount(meta *xrptypes.TransactionMetaData) ([]xrptypes.AddressAmount, error) {
	if meta == nil {
		return nil, fmt.Errorf("transaction meta is not available, thus received amounts cannot be calculated")
	}
	var received []xrptypes.AddressAmount

	for _, node := range meta.AffectedNodes {
		if mod := node.ModifiedNode; mod != nil && mod.LedgerEntryType == "AccountRoot" {
			finalFields := mod.FinalFields
			previousFields := mod.PreviousFields

			if finalFields == nil || previousFields == nil {
				continue
			}
			account, ok1 := pmwpaymentutils.GetStringField(finalFields, "Account")
			finalBalStr, ok2 := pmwpaymentutils.GetStringField(finalFields, "Balance")
			prevBalStr, ok3 := pmwpaymentutils.GetStringField(previousFields, "Balance")
			if !ok1 || !ok2 || !ok3 {
				continue
			}
			finalBal, err := utils.NewBigIntFromString(finalBalStr)
			if err != nil {
				return nil, fmt.Errorf("invalid final balance format: %s", finalBalStr)
			}
			prevBal, err := utils.NewBigIntFromString(prevBalStr)
			if err != nil {
				return nil, fmt.Errorf("invalid previous balance format: %s", prevBalStr)
			}
			diff := new(big.Int).Sub(finalBal, prevBal)
			if diff.Sign() > 0 {
				received = append(received, xrptypes.AddressAmount{
					Address: account,
					Amount:  diff,
				})
			}
		} else if created := node.CreatedNode; created != nil && created.LedgerEntryType == "AccountRoot" {
			newFields := created.NewFields
			if newFields == nil {
				continue
			}
			account, ok1 := pmwpaymentutils.GetStringField(newFields, "Account")
			balanceStr, ok2 := pmwpaymentutils.GetStringField(newFields, "Balance")
			if !ok1 || !ok2 {
				continue
			}
			balance, err := utils.NewBigIntFromString(balanceStr)
			if err != nil {
				return nil, fmt.Errorf("invalid balance format in CreatedNode: %s", balanceStr)
			}

			received = append(received, xrptypes.AddressAmount{
				Address: account,
				Amount:  balance,
			})
		}
	}
	return received, nil
}
