package main

import (
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	e2e "github.com/flare-foundation/go-verifier-api/e2e"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

func main() {
	e2e.RunServer(config.EnvConfig{
		RPCURL:                                 "https://s.altnet.rippletest.net:51234",
		RelayContractAddress:                   "0x5A0773Ff307Bf7C71a832dBB5312237fD3437f9F",
		TeeMachineRegistryContractAddress:      "0x053568617FFccEe2F75073975CC0e1549Ff9db71",
		TeeWalletManagerContractAddress:        "",
		TeeWalletProjectManagerContractAddress: "",
		DatabaseURL:                            "",
		CChainDatabaseURL:                      "",
		Env:                                    "development",
		Port:                                   e2e.PMWMultisigPort,
		ApiKeys:                                []string{"12345"},
		AttestationType:                        connector.PMWMultisigAccountConfigured,
		SourceID:                               "XRP",
	})
}
