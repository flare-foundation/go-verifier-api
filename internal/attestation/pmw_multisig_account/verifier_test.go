package multisigservice

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	pmwmultisigaccountconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account/config"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var envConfig = config.EnvConfig{
	RPCURL:          "https://s.altnet.rippletest.net:51234",
	SourceID:        "XRP",
	AttestationType: connector.PMWMultisigAccountConfigured,
}

func TestMultisigService(t *testing.T) {
	t.Run("Should successfully create MultisigService", func(t *testing.T) {
		service, err := NewMultisigService(envConfig)
		require.NoError(t, err)
		require.NotNil(t, service)
		require.NotNil(t, service.GetVerifier())
		require.NotNil(t, service.GetConfig())
	})

	t.Run("Missing fields in env config", func(t *testing.T) {
		pmwmultisigaccountconfig.ClearPMWMultisigAccountConfigForTest()
		badEnvConfig := config.EnvConfig{
			RPCURL:          "",
			SourceID:        "XRP",
			AttestationType: connector.PMWMultisigAccountConfigured,
		}
		service, err := NewMultisigService(badEnvConfig)
		require.Error(t, err)
		require.Nil(t, service)
	})

	t.Run("Using unsupported source ID", func(t *testing.T) {
		pmwmultisigaccountconfig.ClearPMWMultisigAccountConfigForTest()
		badEnvConfig := config.EnvConfig{
			RPCURL:          "https://s.altnet.rippletest.net:51234",
			SourceID:        "UNSUPPORTED_SOURCE",
			AttestationType: connector.PMWMultisigAccountConfigured,
		}
		service, err := NewMultisigService(badEnvConfig)
		require.Error(t, err)
		require.Nil(t, service)
	})

	pmwmultisigaccountconfig.ClearPMWMultisigAccountConfigForTest()
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
	service, err := NewMultisigService(envConfig)
	require.NoError(t, err)

	pubkey1, err := hexutil.Decode("0xd6dfbae2c2feae24a61bfe596125cf98d433d83126bd187f79cfb054fe6e1f0b201cb19fb9786903acb4b55f422fd40571f973e48fa95305416ec9afce905dbc")
	require.NoError(t, err)
	pubkey2, err := hexutil.Decode("0xfe18c14c27e66cfdf4f2bd885d25e88adb493bf9469b8a415cd518505f604a6649938a258f15b476acd1f92bccd6074bff2dadd4031544a7f7057f5866c5d83b")
	require.NoError(t, err)
	pubkey3, err := hexutil.Decode("0x64104c4e9096d4f8d0cfff1759840a1f44dbaac001265c1194e7af24d9c52c4aec4999de13d74a54acc57b70543a3363f31212c8b0ab0fe01d3b8ddd76c8b76f")
	require.NoError(t, err)

	verifier := service.GetVerifier()
	verify, err := verifier.Verify(t.Context(), connector.IPMWMultisigAccountConfiguredRequestBody{
		WalletAddress: "rGnsRVdAseq7uMW4hjRuydPiV9cZsUWiyN",
		PublicKeys:    [][]byte{pubkey1, pubkey2, pubkey3},
		Threshold:     1,
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
