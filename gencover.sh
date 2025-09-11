#!/bin/bash

# Docker setup
docker compose -f internal/test_util/docker/docker-compose.yaml up -d
docker compose wait c-chain-db xrp-indexer-db

# Run tests
go test -v -coverpkg=./... -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Docker shutdown
docker compose down
