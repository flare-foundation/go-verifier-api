# WARN:(janezicmatej) Wonky Dockerfile. This expects to build with .. context to have
# tee-node local dependency available
# - somedir/
#   - tee-node
#   - go-verifier-api/Dockerfile -> run `docker build -t tag -f Dockerfile ..`
FROM golang:1.24.4 AS builder

WORKDIR /app
COPY tee-node ./tee-node

WORKDIR /app/go-verifier-api

COPY go-verifier-api/go.mod go-verifier-api/go.sum ./

RUN go mod download

COPY go-verifier-api .
RUN go build -o ./go-verifier-api cmd/main.go

FROM debian:latest AS execution

RUN apt-get update && apt-get install -y curl

WORKDIR /app

COPY --from=builder /app/go-verifier-api/go-verifier-api .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

CMD [ "./go-verifier-api" ]
