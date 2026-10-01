# checkov:skip=CKV_DOCKER_2: Healthcheck is handled by the container orchestrator
FROM golang:1.26.6 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -tags netgo -o ./go-verifier-api cmd/main.go && \
    git rev-parse HEAD > COMMIT_HASH

FROM debian:12-slim AS execution

# hadolint ignore=DL3008
RUN apt-get update && \
    apt-get install -y --no-install-recommends curl && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd -g 10001 app && useradd -u 10001 -g app -m app

WORKDIR /app

COPY --from=builder /app/go-verifier-api .
COPY --from=builder /app/COMMIT_HASH .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER app

CMD [ "./go-verifier-api" ]
