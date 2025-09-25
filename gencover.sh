#!/bin/bash

DOCKER_COMPOSE_FILE="internal/test_helper/docker/docker-compose.yaml"

# Docker setup
docker compose -f "$DOCKER_COMPOSE_FILE" up -d

# Function to wait for service health
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

# Run tests
go test -v -coverpkg=./... -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Docker shutdown
docker compose -f "$DOCKER_COMPOSE_FILE" down
