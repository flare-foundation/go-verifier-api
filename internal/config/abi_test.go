package config_test

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGetAbiArguments(t *testing.T) {
	origABI := connector.ConnectorMetaData.ABI
	defer func() { connector.ConnectorMetaData.ABI = origABI }()

	connector.ConnectorMetaData.ABI = `[
		{
			"constant": false,
			"inputs": [{"name": "arg1","type": "uint256"}],
			"name": "TestMethod",
			"outputs": [],
			"payable": false,
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`

	t.Run("valid struct", func(t *testing.T) {
		arg, err := config.GetAbiArguments("TestMethod")
		require.NoError(t, err)
		require.Equal(t, "uint256", arg.Type.String())
	})
	t.Run("method not found", func(t *testing.T) {
		_, err := config.GetAbiArguments("MissingMethod")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid method definition")
	})
	t.Run("invalid ABI", func(t *testing.T) {
		connector.ConnectorMetaData.ABI = "not json"
		_, err := config.GetAbiArguments("TestMethod")
		require.Contains(t, err.Error(), "failed to parse ABI")
	})
}
