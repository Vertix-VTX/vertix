# Builds a single image carrying both vertixd and vertix-feeder for the devnet.
# Toolchain: go1.25.4-alpine (Cosmos/gin sonic does not link on Go 1.26+).
FROM golang:1.25.4-alpine AS builder
RUN apk add --no-cache git make
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO off: sonic (via gin) fails to link with CGO_ENABLED=1 on Go 1.25+.
# GOTOOLCHAIN matches Makefile pin; build outputs under WORKDIR (not /out at FS root).
ENV CGO_ENABLED=0 GOTOOLCHAIN=go1.25.4
RUN make build BUILD_DIR=/src/out VERSION=devnet \
 && make feeder-build BUILD_DIR=/src/out VERSION=devnet

FROM alpine:3.20
RUN apk add --no-cache ca-certificates libstdc++ bash jq curl
COPY --from=builder /src/out/vertixd /usr/local/bin/vertixd
COPY --from=builder /src/out/vertix-feeder /usr/local/bin/vertix-feeder
EXPOSE 26656 26657 1317 9090 26660 9200
# No ENTRYPOINT: compose sets the command per service (vertixd or vertix-feeder).
