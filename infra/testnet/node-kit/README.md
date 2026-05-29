# Vertix public testnet — external validator node kit

Portable kit for operators running their own validator + oracle feeder on `vertix-testnet-1`. Config is rendered from a single `env` file (no hand-editing TOML). See `docs/validator-onboarding.md` for the full join contract.

## Prerequisites

- Linux host (systemd), dedicated `vertix` user recommended
- `vertixd` and `vertix-feeder` binaries installed (matching the published testnet release)
- [Cosmovisor](https://docs.cosmos.network/main/build/tooling/cosmovisor) at `/usr/local/bin/cosmovisor` with upgrade layout under `DAEMON_HOME`
- `curl`, `jq`, `sha256sum`
- Published genesis URL + SHA256, seeds, and (optional) state-sync trusted height/hash from the founders
- Three keys: **operator** (create-validator txs), **consensus** (`priv_validator`), **feeder** (oracle votes; separate from operator)

## Quick start

1. **Configure**

   ```bash
   cd infra/testnet/node-kit
   cp env.example env
   # Edit MONIKER, GENESIS_*, SEEDS, STATESYNC_* (if using fast join), VERTIXD_HOME
   ```

2. **Render node config**

   ```bash
   ./setup-node.sh
   ```

3. **Feeder config**

   ```bash
   cp feeder.example.yaml "$VERTIXD_HOME/feeder.yaml"   # or /home/vertix/.vertixd/feeder.yaml
   # Set validator (valoper), key_name, keyring paths; fund the feeder key for tx fees
   vertixd keys add feeder --home "$VERTIXD_HOME" --keyring-backend file
   ```

4. **Install systemd units**

   ```bash
   sudo cp systemd/vertixd.service systemd/vertix-feeder.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable vertixd vertix-feeder
   ```

5. **Start**

   ```bash
   sudo systemctl start vertixd
   # Wait until synced (or state-sync completes), then:
   sudo systemctl start vertix-feeder
   ```

6. **Join**

   Fund your **operator** account from the testnet faucet, then submit `MsgCreateValidator` (gentx flow) per `docs/validator-onboarding.md`. Do not reuse the feeder key for staking or consensus.

## Is my validator healthy?

- [ ] Signing: `curl -s localhost:26657/status | jq .result.validator_info` shows your address with voting power
- [ ] Synced: `curl -s localhost:26657/status | jq .result.sync_info.catching_up` is `false`
- [ ] Peered: `curl -s localhost:26657/net_info | jq .result.n_peers` >= 3
- [ ] Feeding: `curl -s localhost:9200/metrics | grep feeds_submitted_total` increases each vote window
- [ ] Not missing: Tenderduty (port 8888) shows 0 recent missed blocks
- [ ] Disk OK: data dir has headroom; pruning configured
