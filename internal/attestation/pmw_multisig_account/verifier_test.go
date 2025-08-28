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
	SourceID:        "TESTXRP",
	AttestationType: connector.PMWMultisigAccountConfigured,
}

func TestMultisig(t *testing.T) {
	service, err := NewMultisigService(envConfig)
	require.NoError(t, err)

	pubkey1, err := hexutil.Decode("0xED3F12A88266246B4D6E3886E9906E3C905F4378ACA1CDBACF7F6989CD940FE7F7")
	require.NoError(t, err)

	pubkey2, err := hexutil.Decode("0xED5E7464EF81CA10829CADE12BF0EF298A3BDCF66BF68600B9D5C47147DCE394A2")
	require.NoError(t, err)

	verifier := service.GetVerifier()
	verify, err := verifier.Verify(t.Context(), connector.IPMWMultisigAccountConfiguredRequestBody{
		WalletAddress: "rGPKDtynj7g4s789Z4w84nWF6X1wZnavk3",
		PublicKeys:    [][]byte{pubkey1, pubkey2},
		Threshold:     2,
	})

	require.NoError(t, err)
	require.Equal(t, uint8(attestationtypes.PMWMultisigAccountStatusOK), verify.Status)
	require.Equal(t, uint64(9882340), verify.Sequence)
}

// Wallet without disabled master key should be rejected.
func TestMultisigWithoutDisabledMasterKey(t *testing.T) {
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

	pubkey1, err := hexutil.Decode("0xEDB0977AA35E892128197DBBA01D84BECC5AD66C6E5C966A544D20895F51DD0494")
	require.NoError(t, err)

	pubkey2, err := hexutil.Decode("0xED5CABB5E057B0341A0C9121B450CC348D5F6F516BF7B7A9963B42B962BE17F9BB")
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
