package main

import (
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/e2e"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

func main() {
	e2e.RunServer(config.EnvConfig{
		RPCURL:                                 "http://localhost:8545",
		RelayContractAddress:                   "0x5A0773Ff307Bf7C71a832dBB5312237fD3437f9F",
		TeeMachineRegistryContractAddress:      "0x053568617FFccEe2F75073975CC0e1549Ff9db71",
		TeeWalletManagerContractAddress:        "0xD036a8F254ef782cb93af4F829A1568E992c3864",
		TeeWalletProjectManagerContractAddress: "0x26d1E94963C8b382Ad66320826399E4B30347404",
		DatabaseURL:                            "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
		CChainDatabaseURL:                      "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
		Env:                                    "development",
		Port:                                   e2e.PMWPaymentStatusPort,
		ApiKeys:                                []string{"12345"},
		AttestationType:                        connector.PMWPaymentStatus,
		SourceID:                               "XRP",
	})
}
