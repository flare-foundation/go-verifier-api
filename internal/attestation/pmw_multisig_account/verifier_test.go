package multisigservice

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var envConfig = config.EnvConfig{
	RPCURL:          "https://s.altnet.rippletest.net:51234",
	SourceID:        "XRP",
	AttestationType: connector.PMWMultisigAccountConfigured,
}

func TestMultisig(t *testing.T) {
	service, err := NewMultisigService(envConfig)
	require.NoError(t, err)

	pubkey1, err := hexutil.Decode("0x51003727e9d42e8be45a851c3b86386d27df8e01630f27aaf0ea254dcb6390920d7015365559f9546f3593dd48baae0120495fef2986f87873ca116c39416240")
	require.NoError(t, err)
	pubkey2, err := hexutil.Decode("0x06276df7b93cd7fdc34c95a93e3b23466ae3416ad56d59a746fc53ab4446104ac5e545cc021561ff80bd80c411006af1c0711492259894482d995a80cd6c7e8f")
	require.NoError(t, err)
	pubkey3, err := hexutil.Decode("0x76e4a85207c1012283a7190b1df628e29ba1a687404ec35a766e7eddba94ba42a07f356ccc847540b4ed23f15f3feb07c406c3f815a361983c321740fa998cdb")
	require.NoError(t, err)

	verifier := service.GetVerifier()
	verify, err := verifier.Verify(t.Context(), connector.IPMWMultisigAccountConfiguredRequestBody{
		WalletAddress: "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
		PublicKeys:    [][]byte{pubkey1, pubkey2, pubkey3},
		Threshold:     1,
	})

	require.NoError(t, err)
	require.Equal(t, uint8(attestationtypes.PMWMultisigAccountStatusOK), verify.Status)
	require.Equal(t, uint64(10136106), verify.Sequence)
}

// Wallet without disabled master key should be rejected.
func TestMultisigWithoutDisabledMasterKey(t *testing.T) {
	t.Skip() // TODO Fix with correct pubkey form!
	service, err := NewMultisigService(envConfig)
	require.NoError(t, err)

	pubkey1, err := hexutil.Decode("0xEDB0977AA35E892128197DBBA01D84BECC5AD66C6E5C966A544D20895F51DD0494")
	require.NoError(t, err)

	pubkey2, err := hexutil.Decode("0xED5CABB5E057B0341A0C9121B450CC348D5F6F516BF7B7A9963B42B962BE17F9BB")
	require.NoError(t, err)

	verifier := service.GetVerifier()
	verify, err := verifier.Verify(t.Context(), connector.IPMWMultisigAccountConfiguredRequestBody{
		WalletAddress: "rnk1bYEjfr24uvQHnGM9DkMU5HtwsjYW6N",
		PublicKeys:    [][]byte{pubkey1, pubkey2},
		Threshold:     2,
	})

	require.NoError(t, err)
	require.Equal(t, uint8(attestationtypes.PMWMultisigAccountStatusERROR), verify.Status)
	require.Equal(t, uint64(0), verify.Sequence)
}

// Wallet without multiple signer should be rejected.
func TestSingleSig(t *testing.T) {
	service, err := NewMultisigService(envConfig)
	require.NoError(t, err)

	pubkey1, err := hexutil.Decode("0x51003727e9d42e8be45a851c3b86386d27df8e01630f27aaf0ea254dcb6390920d7015365559f9546f3593dd48baae0120495fef2986f87873ca116c39416240")
	require.NoError(t, err)
	pubkey2, err := hexutil.Decode("0x06276df7b93cd7fdc34c95a93e3b23466ae3416ad56d59a746fc53ab4446104ac5e545cc021561ff80bd80c411006af1c0711492259894482d995a80cd6c7e8f")
	require.NoError(t, err)

	verifier := service.GetVerifier()
	verify, err := verifier.Verify(t.Context(), connector.IPMWMultisigAccountConfiguredRequestBody{
		WalletAddress: "rnVdREAw4HkdS7TJzwx1XpqfAz1v8iGHxr",
		PublicKeys:    [][]byte{pubkey1, pubkey2},
		Threshold:     2,
	})

	require.NoError(t, err)
	require.Equal(t, uint8(attestationtypes.PMWMultisigAccountStatusERROR), verify.Status)
	require.Equal(t, uint64(0), verify.Sequence)
}
