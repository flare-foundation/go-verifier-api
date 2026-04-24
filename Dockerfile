FROM golang:1.25.1 AS builder

# Build context is the parent directory (contains go-verifier-api/ and tee-node/).
# docker build -t local/go-verifier-api -f Dockerfile ..

WORKDIR /app/go-verifier-api

COPY tee-node/ /app/tee-node/
COPY go-verifier-api/go.mod go-verifier-api/go.sum ./

RUN go mod download

COPY go-verifier-api/ .
RUN CGO_ENABLED=0 go build -tags netgo -o ./go-verifier-api cmd/main.go
RUN git rev-parse HEAD > COMMIT_HASH

FROM debian:12-slim AS execution

RUN apt-get update && apt-get install -y curl

WORKDIR /app

COPY --from=builder /app/go-verifier-api/go-verifier-api .
COPY --from=builder /app/go-verifier-api/COMMIT_HASH .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

CMD [ "./go-verifier-api" ]
