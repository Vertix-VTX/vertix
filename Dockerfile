FROM golang:1.25.4-alpine AS builder
RUN apk add --no-cache git make build-base linux-headers
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=1
RUN make build BUILD_DIR=/out VERSION=docker

FROM alpine:3.20
RUN apk add --no-cache ca-certificates libstdc++
COPY --from=builder /out/vertixd /usr/local/bin/vertixd
EXPOSE 26656 26657 1317 9090
ENTRYPOINT ["vertixd"]
