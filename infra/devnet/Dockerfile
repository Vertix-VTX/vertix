# Builds a single image carrying both vertixd and vertix-feeder for the devnet.
# Builder uses Debian/glibc (golang:1.25.4); runtime stays Alpine.
# Go 1.26+ breaks sonic (Cosmos/gin); the image pins 1.25.4 — use GOTOOLCHAIN=local here.
FROM golang:1.25.4 AS builder
RUN apt-get update && apt-get install -y --no-install-recommends make git \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local
RUN mkdir -p /src/out \
    && make build BUILD_DIR=/src/out VERSION=devnet \
    && make feeder-build BUILD_DIR=/src/out VERSION=devnet

FROM alpine:3.20
RUN apk add --no-cache ca-certificates libstdc++ bash jq curl
COPY --from=builder /src/out/vertixd /usr/local/bin/vertixd
COPY --from=builder /src/out/vertix-feeder /usr/local/bin/vertix-feeder
EXPOSE 26656 26657 1317 9090 26660 9200
# No ENTRYPOINT: compose sets the command per service (vertixd or vertix-feeder).
