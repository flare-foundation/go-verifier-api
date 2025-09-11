package transaction

import (
	"fmt"

	types "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/type"
)

func GetTransactionStatus(result string) (types.TransactionStatus, error) {
	const transactionResultPrefixLength = 3
	if len(result) < transactionResultPrefixLength {
		return 0, fmt.Errorf("transaction result too short: %q", result)
	}
	prefix := result[:3]
	switch prefix {
	case "tes":
		return types.Success, nil
	case "tec":
		switch result {
		case "tecDST_TAG_NEEDED",
			"tecNO_DST",
			"tecNO_DST_INSUF_XRP",
			"tecNO_PERMISSION":
			return types.ReceiverFault, nil
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
			return types.SenderFault, nil
		default:
			return 0, fmt.Errorf("unknown tec error code: %s", result)
		}
	case "tef", "tel", "tem", "ter":
		return types.SenderFault, nil

	default:
		return 0, fmt.Errorf("unexpected transaction status prefix: %s", prefix)
	}
}
