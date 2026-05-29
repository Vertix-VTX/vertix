BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
COMMIT := $(shell git log -1 --format='%H')
APPNAME := vertix

# Cosmos SDK deps (sonic) do not link on Go 1.26+; pin the compiler for all targets.
export GOTOOLCHAIN ?= go1.25.4

# don't override user values
ifeq (,$(VERSION))
  VERSION := $(shell git describe --exact-match 2>/dev/null)
  # if VERSION is empty, then populate it with branch's name and raw commit hash
  ifeq (,$(VERSION))
    VERSION := $(BRANCH)-$(COMMIT)
  endif
endif

# Update the ldflags with the app, client & server names
ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=$(APPNAME) \
	-X github.com/cosmos/cosmos-sdk/version.AppName=$(APPNAME)d \
	-X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
	-X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT)

BUILD_FLAGS := -ldflags '$(ldflags)'

##############
###  Test  ###
##############

test-unit:
	@echo Running unit tests...
	@go test -mod=readonly -v -timeout 30m ./...

test-race:
	@echo Running unit tests with race condition reporting...
	@go test -mod=readonly -v -race -timeout 30m ./...

test-cover:
	@echo Running unit tests and creating coverage report...
	@go test -mod=readonly -v -timeout 30m -coverprofile=$(COVER_FILE) -covermode=atomic ./...
	@go tool cover -html=$(COVER_FILE) -o $(COVER_HTML_FILE)
	@rm $(COVER_FILE)

bench:
	@echo Running unit tests with benchmarking...
	@go test -mod=readonly -v -timeout 30m -bench=. ./...

###################
###  Simulation ###
###################

SIM_NUM_BLOCKS ?= 50
SIM_BLOCK_SIZE ?= 50
SIM_SEED ?= 42

test-sim-nondeterminism:
	@echo "Running non-determinism simulation..."
	@go test -mod=readonly ./app -run TestAppStateDeterminism -Enabled=true \
		-NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=true -Period=1 -v -timeout 30m

test-sim-fullapp:
	@echo "Running full-app simulation..."
	@go test -mod=readonly ./app -run TestFullAppSimulation -Enabled=true \
		-NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=true -Seed=$(SIM_SEED) -Period=1 -v -timeout 30m

test-sim-import-export:
	@echo "Running import/export simulation..."
	@go test -mod=readonly ./app -run TestAppImportExport -Enabled=true \
		-NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=true -Seed=$(SIM_SEED) -Period=1 -v -timeout 30m

.PHONY: test-sim-nondeterminism test-sim-fullapp test-sim-import-export

test: govet test-race

.PHONY: test test-unit test-race test-cover bench

#################
###  Install  ###
#################

all: install

install:
	@echo "--> ensure dependencies have not been modified"
	@go mod verify
	@echo "--> installing $(APPNAME)d"
	@go install $(BUILD_FLAGS) -mod=readonly ./cmd/$(APPNAME)d

.PHONY: all install

##################
###  Protobuf  ###
##################

# Use this target if you do not want to use Ignite for generating proto files
GOLANG_PROTOBUF_VERSION=1.28.1
GRPC_GATEWAY_VERSION=1.16.0
GRPC_GATEWAY_PROTOC_GEN_OPENAPIV2_VERSION=2.20.0

proto-deps:
	@echo "Installing proto deps"
	@go install github.com/bufbuild/buf/cmd/buf@v1.50.0
	@go install github.com/cosmos/gogoproto/protoc-gen-gogo@latest
	@go install github.com/cosmos/cosmos-proto/cmd/protoc-gen-go-pulsar@latest
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@v$(GOLANG_PROTOBUF_VERSION)
	@go install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@v$(GRPC_GATEWAY_VERSION)
	@go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v$(GRPC_GATEWAY_PROTOC_GEN_OPENAPIV2_VERSION)
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto-gen:
	@echo "Generating protobuf files..."
	@ignite generate proto-go --yes

.PHONY: proto-deps proto-gen

#################
###  Linting  ###
#################

golangci_lint_cmd=golangci-lint
golangci_version=v1.64.8

lint:
	@echo "--> Running linter"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(golangci_version)
	@$(golangci_lint_cmd) run ./... --timeout 15m

lint-fix:
	@echo "--> Running linter and fixing issues"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(golangci_version)
	@$(golangci_lint_cmd) run ./... --fix --timeout 15m

.PHONY: lint lint-fix

###################
### Development ###
###################

govet:
	@echo Running go vet...
	@go vet $$(go list ./... | grep -v '/api/')

govulncheck:
	@echo Running govulncheck...
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@govulncheck ./...

.PHONY: govet govulncheck

###################
###   Build     ###
###################

BUILD_DIR ?= build
COVER_FILE ?= coverage.out
COVER_HTML_FILE ?= coverage.html

build:
	@echo "--> Building $(APPNAME)d"
	@go build $(BUILD_FLAGS) -mod=readonly -o $(BUILD_DIR)/$(APPNAME)d ./cmd/$(APPNAME)d

feeder-build:
	@echo "--> Building vertix-feeder"
	@go build $(BUILD_FLAGS) -mod=readonly -o $(BUILD_DIR)/vertix-feeder ./feeder/cmd/vertix-feeder

clean:
	@echo "--> Cleaning build artifacts"
	@rm -rf $(BUILD_DIR) $(COVER_FILE) $(COVER_HTML_FILE)

.PHONY: build feeder-build clean

###################
###   Devnet (Docker, Phase 6) ###
###################

DEVNET_DIR ?= infra/devnet
COMPOSE ?= docker compose --env-file $(DEVNET_DIR)/.env --env-file $(DEVNET_DIR)/mnemonics.env -f $(DEVNET_DIR)/docker-compose.yml

devnet-docker-build:
	@echo "--> Building vertix:devnet image"
	@docker build -f $(DEVNET_DIR)/Dockerfile -t vertix:devnet .

localnet-genesis:
	@./scripts/devnet/init-genesis.sh

localnet-up: devnet-docker-build localnet-genesis
	@echo "--> Starting devnet stack"
	@$(COMPOSE) up -d

localnet-down:
	@echo "--> Stopping devnet stack"
	@$(COMPOSE) down -v

localnet-reset: localnet-down localnet-up

devnet-smoke:
	@./scripts/devnet/smoke.sh --bootstrap

.PHONY: devnet-docker-build localnet-genesis localnet-up localnet-down localnet-reset devnet-smoke

###################
###  Testnet (Public, Phase 8) ###
###################

TESTNET_DIR ?= infra/testnet
TESTNET_COMPOSE ?= docker compose --env-file $(TESTNET_DIR)/.env --env-file $(TESTNET_DIR)/mnemonics.env -f $(TESTNET_DIR)/docker-compose.public.yml

testnet-genesis:
	@./scripts/testnet/build-genesis.sh

testnet-up: devnet-docker-build testnet-genesis
	@docker tag vertix:devnet vertix:testnet
	@echo "--> Starting public testnet founder stack"
	@$(TESTNET_COMPOSE) up -d

testnet-down:
	@$(TESTNET_COMPOSE) down -v

testnet-join-smoke:
	@./scripts/testnet/join-smoke.sh

testnet-demo:
	@./scripts/testnet/rwa-demo.sh

testnet-integration-verify:
	@./scripts/testnet/integration-verify.sh

.PHONY: testnet-genesis testnet-up testnet-down testnet-join-smoke testnet-demo testnet-integration-verify

###################
###  Load test  ###
###################

LOADTEST_ENDPOINT ?= ws://localhost:26657/websocket
LOADTEST_DURATION ?= 60
LOADTEST_RATE ?= 200
LOADTEST_CONNS ?= 4

load-test:
	@echo "Running tm-load-test against $(LOADTEST_ENDPOINT) (devnet must be up)..."
	@tm-load-test -c $(LOADTEST_CONNS) -T $(LOADTEST_DURATION) -r $(LOADTEST_RATE) \
		--broadcast-tx-method sync --endpoints $(LOADTEST_ENDPOINT)

.PHONY: load-test

###################
###  Genesis    ###
###################

validate-genesis: build
	@echo "--> Validating genesis"
	@tmpdir=$$(mktemp -d) && \
		$(BUILD_DIR)/$(APPNAME)d init validate --chain-id vertix-devnet-1 --home $$tmpdir && \
		$(BUILD_DIR)/$(APPNAME)d genesis validate-genesis --home $$tmpdir

.PHONY: validate-genesis

###################
###  Devnet     ###
###################

devnet-reset:
	@echo "--> Starting fresh devnet"
	@ignite chain serve --reset-once --verbose

devnet:
	@echo "--> Starting devnet (keeping state)"
	@ignite chain serve --verbose

.PHONY: devnet-reset devnet

###################
###  Clients    ###
###################

ts-gen:
	@echo "--> Generating TypeScript client"
	@ignite generate ts-client --yes

.PHONY: ts-gen

###################
###    E2E      ###
###################

E2E_IMAGE ?= vertix-network/vertixd:local

e2e-image:
	@echo "--> Building e2e docker image $(E2E_IMAGE)"
	@docker build -t $(E2E_IMAGE) .

e2e: e2e-image
	@echo "--> Running interchaintest e2e suite"
	@cd e2e && go test ./... -timeout 30m -v

.PHONY: e2e-image e2e

###################
###    Hooks    ###
###################

hooks:
	@echo "--> Installing git hooks"
	@ln -sf ../../scripts/git-hooks/pre-commit .git/hooks/pre-commit
	@chmod +x scripts/git-hooks/pre-commit

.PHONY: hooks