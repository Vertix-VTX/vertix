# Builds a single image carrying both vertixd and vertix-feeder for the devnet.
# Mirrors the root Dockerfile toolchain (CGO on, go1.25.4-alpine) and adds the feeder.
FROM golang:1.25.4-alpine AS builder
RUN apk add --no-cache git make build-base linux-headers
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=1
RUN make build BUILD_DIR=/out VERSION=devnet \
 && make feeder-build BUILD_DIR=/out VERSION=devnet

FROM alpine:3.20
RUN apk add --no-cache ca-certificates libstdc++ bash jq curl
COPY --from=builder /out/vertixd /usr/local/bin/vertixd
COPY --from=builder /out/vertix-feeder /usr/local/bin/vertix-feeder
EXPOSE 26656 26657 1317 9090 26660 9200
# No ENTRYPOINT: compose sets the command per service (vertixd or vertix-feeder).
