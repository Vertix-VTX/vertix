FROM golang:1.25.4 AS builder
RUN apt-get update && apt-get install -y --no-install-recommends make git \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local
RUN mkdir -p /src/out && make build BUILD_DIR=/src/out VERSION=docker
RUN go install cosmossdk.io/tools/cosmovisor/cmd/cosmovisor@v1.5.0

FROM alpine:3.20
RUN apk add --no-cache ca-certificates libstdc++
COPY --from=builder /src/out/vertixd /usr/local/bin/vertixd
COPY --from=builder /go/bin/cosmovisor /usr/local/bin/cosmovisor
EXPOSE 26656 26657 1317 9090
ENTRYPOINT ["vertixd"]
