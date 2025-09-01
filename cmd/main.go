package main

import (
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-verifier-api/internal/api"
)

func main() {
	envConfig, err := api.LoadEnvConfig()
	if err != nil {
		logger.Fatal(err)
	}
	api.RunServer(envConfig)
}
