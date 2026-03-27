FROM golang:1.25.1 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .
RUN go build -o ./go-verifier-api cmd/main.go
RUN git rev-parse HEAD > COMMIT_HASH

FROM debian:12-slim AS execution

RUN apt-get update && apt-get install -y curl

WORKDIR /app

COPY --from=builder /app/go-verifier-api .
COPY --from=builder /app/COMMIT_HASH .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

CMD [ "./go-verifier-api" ]
