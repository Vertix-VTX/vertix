# Vertix Relayer Guide (ICS-20)

Phase 5 enables ICS-20 transfers of VTX (`uvtx`) — and, after Phase 3, unrestricted
`rwa/{id}` denoms. This guide covers opening and operating a transfer channel with
**Hermes v1.8.x** (primary) and **`rly`** (alternative).

> Restriction semantics (spec §4): a **restricted** `rwa/{id}` asset is **not**
> IBC-exportable; only **unrestricted** `rwa/{id}` and VTX transfer over ICS-20.

## 1. Hermes (primary)

### 1.1 Install

    cargo install ibc-relayer-cli --bin hermes --version 1.8.4 --locked
    hermes version

### 1.2 Configure

Use [`infra/hermes/config.toml`](../infra/hermes/config.toml). Copy it to
`~/.hermes/config.toml` and edit the two `[[chains]]` blocks for your endpoints
(chain id, rpc/grpc, websocket). Validate:

    hermes --config ~/.hermes/config.toml config validate

### 1.3 Add relayer keys

For each chain, import a funded relayer key (mnemonic in `relayer.mnemonic`):

    hermes keys add --chain vertix-devnet-1 --mnemonic-file relayer.mnemonic --key-name vertix-relayer
    hermes keys add --chain vertix-devnet-2 --mnemonic-file relayer.mnemonic --key-name vertix-relayer

### 1.4 Create a transfer channel

    hermes create channel \
      --a-chain vertix-devnet-1 \
      --b-chain vertix-devnet-2 \
      --a-port transfer --b-port transfer \
      --new-client-connection --yes

Record the printed `channel-N` ids for both ends.

### 1.5 Relay

    hermes start

### 1.6 Test a transfer

    vertixd tx ibc-transfer transfer transfer channel-0 \
      <dst-vtx-address> 1000000uvtx \
      --from alice --chain-id vertix-devnet-1 --fees 5000uvtx --yes

    # On the destination chain, confirm the ibc/<hash> voucher balance:
    vertixd q bank balances <dst-vtx-address> --node http://127.0.0.1:26667

## 2. rly (alternative)

    go install github.com/cosmos/relayer/v2@v2.5.2

    rly config init
    rly chains add-dir infra/rly/chains   # see below for chain json
    rly keys restore vertix-devnet-1 default "<relayer mnemonic>"
    rly keys restore vertix-devnet-2 default "<relayer mnemonic>"
    rly paths new vertix-devnet-1 vertix-devnet-2 vertix-link
    rly tx link vertix-link --src-port transfer --dst-port transfer
    rly start vertix-link

A minimal `rly` chain definition (`infra/rly/chains/vertix-devnet-1.json`):

    {
      "type": "cosmos",
      "value": {
        "key": "default",
        "chain-id": "vertix-devnet-1",
        "rpc-addr": "http://127.0.0.1:26657",
        "account-prefix": "vtx",
        "keyring-backend": "test",
        "gas-prices": "0.025uvtx",
        "gas-adjustment": 1.3,
        "trusting-period": "336h",
        "timeout": "20s"
      }
    }

## 3. Reproducibility

The `infra/hermes/config.toml` here is the same shape the `e2e/` interchaintest
suite drives (Hermes via `interchaintest.NewBuiltinRelayerFactory(ibc.Hermes, ...)`),
so a green `make e2e` is evidence this recipe works end-to-end.
