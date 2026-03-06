#!/usr/bin/env bash

DOCKER_COMPOSE_FILE="internal/tests/docker/docker-compose.yaml"

GOBIN=${GOBIN:-$(go env GOPATH)/bin}

# Install go-test-coverage if not already installed
if ! command -v "${GOBIN}/go-test-coverage" >/dev/null 2>&1; then
  echo "Installing go-test-coverage..."
  go install github.com/vladopajic/go-test-coverage/v2@latest
fi


# Docker setup
docker compose -f "$DOCKER_COMPOSE_FILE" up -d

# Function to wait for a service to become healthy
wait_for_health() {
  local service=$1
  local retries=30
  echo "Waiting for $service to be healthy..."
  for i in $(seq 1 $retries); do
    status=$(docker inspect --format='{{.State.Health.Status}}' $(docker compose -f "$DOCKER_COMPOSE_FILE" ps -q $service) 2>/dev/null || echo "unknown")
    if [ "$status" == "healthy" ]; then
      echo "$service is healthy"
      return 0
    fi
    sleep 2
  done
  echo "$service did not become healthy in time"
  exit 1
}


# Wait for DB services
wait_for_health c-chain-db
wait_for_health xrp-indexer-db

# Run Go tests
go test -v -coverpkg=./... -coverprofile=coverage.out ./...
# go tool cover -html=coverage.out # This opens coverage for each file in the browser.

# Run go-test-coverage
"${GOBIN}/go-test-coverage" --config=./.testcoverage.yml

# Shut down Docker services
docker compose -f "$DOCKER_COMPOSE_FILE" down --remove-orphans
