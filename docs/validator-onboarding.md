# Validator Onboarding — Public Testnet

**Chain:** `vertix-testnet-1` (v1) · `vertix-testnet-2` (v2, Phase 9) · **Related:** [Validator setup](./validator-setup.md) · [TMKMS](./tmkms.md) · [Node kit](../infra/testnet/node-kit/) · [Testnet runbook](./testnet-runbook.md) · [Phase 9 design spec](./specs/2026-05-29-phase-9-testnet-v2-genesis-rehearsal-design.md)

This document is the **onboarding contract** for external validators joining Vertix public testnets. Follow every step in order; skipping genesis verification or feeder setup will cause join failures or oracle slashes.

---

## Join paths (v1 vs v2)

| Network | Phase | Join method | When |
|---------|-------|-------------|------|
| `vertix-testnet-1` | Phase 8 (v1) | Post-genesis `MsgCreateValidator` | Any time after v1 is public |
| `vertix-testnet-2` | Phase 9 ceremony | Pre-launch `genesis gentx` submitted to coordinator | Before v2 coordinated start only |
| `vertix-testnet-2` | Phase 9 post-open | Post-genesis `MsgCreateValidator` | After founders open v2 publicly |

- **v1:** §§3–10 below (`chain-id vertix-testnet-1`).
- **v2 ceremony:** Coordinator distributes base genesis; you submit `gentx-<moniker>.json` before the deadline. See [testnet runbook §10.2](./testnet-runbook.md#102-gentx-coordinator-playbook) and [`scripts/testnet/v2/`](../scripts/testnet/v2/).
- **v2 post-open:** Same flow as v1 but override node-kit `env` for v2 chain ID and genesis (§4.1). Use `vertix-testnet-2` in all `vertixd` commands.

---

## 1. Overview + the three-key model

Vertix validators use **three distinct keys** — never reuse one key for multiple roles:

| Key | Purpose | Storage |
|-----|---------|---------|
| **Operator** | `MsgCreateValidator`, governance votes, account funding | Hot wallet on operator host |
| **Consensus** | Block signing (`priv_validator`) | [`tmkms`](./tmkms.md) or HSM — never on a public-facing host |
| **Feeder** | Oracle `MsgSubmitFeed` submissions | Dedicated key on feeder host |

See [validator setup §Overview](./validator-setup.md#overview) for sentry topology and [tmkms.md](./tmkms.md) for consensus key isolation.

---

## 2. Prerequisites

- **Hardware:** 4+ CPU cores, 16 GB RAM, 500 GB SSD (NVMe preferred), stable uplink
- **OS:** Linux with systemd (Ubuntu 22.04+ or equivalent)
- **Pinned versions:**
  - Go **1.22+**
  - CometBFT **v0.38.x** (bundled with `vertixd`)
  - Cosmovisor **v1.5.x** at `/usr/local/bin/cosmovisor`
  - `vertixd` + `vertix-feeder` matching the published testnet release tag
- **Tools:** `curl`, `jq`, `sha256sum`

---

## 3. Install

Build from source or download release binaries:

```bash
# From source
git clone https://github.com/vertix-network/vertix.git
cd vertix
make build
sudo cp build/vertixd /usr/local/bin/
go build -o build/vertix-feeder ./feeder/cmd/vertix-feeder
sudo cp build/vertix-feeder /usr/local/bin/

# Verify
vertixd version
vertix-feeder version
```

Install Cosmovisor per [Cosmos SDK docs](https://docs.cosmos.network/main/build/tooling/cosmovisor).

---

## 4. Get the node kit

Copy the portable node kit from the repository:

```bash
git clone https://github.com/vertix-network/vertix.git
cd vertix/infra/testnet/node-kit
cp env.example env
```

Edit `env` with your values:

```bash
MONIKER=my-validator
GENESIS_URL=https://<founder-host>/genesis.json
GENESIS_SHA256=<published-sha256>
SEEDS=<seed-node-id>@<seed-host>:26656
STATESYNC_RPC=<public-rpc>:26657
STATESYNC_TRUST_HEIGHT=<recent-height>
STATESYNC_TRUST_HASH=<block-hash-at-height>
VERTIXD_HOME=/home/vertix/.vertixd
```

See [`infra/testnet/node-kit/README.md`](../infra/testnet/node-kit/README.md) for field descriptions.

### 4.1 Node kit env overrides for v2

For `vertix-testnet-2` (post-open join or ceremony node prep), set at minimum:

```bash
CHAIN_ID=vertix-testnet-2
GENESIS_URL=https://<founder-host>/testnet-v2/genesis.json
GENESIS_SHA256=<from infra/testnet/v2/genesis/genesis.sha256>
SEEDS=<v2-seed-node-id>@<host>:26656
STATESYNC_RPC=<v2-public-rpc>:26657
STATESYNC_TRUST_HEIGHT=<recent-height>
STATESYNC_TRUST_HASH=<block-hash-at-height>
```

Published artifacts live under `infra/testnet/v2/genesis/` after `make testnet-v2-genesis`. **Never** reuse v1 `GENESIS_URL` or hash for v2.

During the **internal gate**, founders use the private v2 Compose stack (`make testnet-v2-up`); external operators wait until [runbook §10.4](./testnet-runbook.md#104-internal--public-open-gate) passes.

---

## 5. Fetch + verify genesis

**Verifying the hash is mandatory.** Never start a node against an unverified genesis file.

```bash
curl -fsSL "$GENESIS_URL" -o "$VERTIXD_HOME/config/genesis.json"
echo "<published-sha256>  $VERTIXD_HOME/config/genesis.json" | sha256sum -c -
```

The published `genesis.sha256` is distributed alongside `genesis.json` by the founders (see [testnet runbook §Launch sequence](./testnet-runbook.md#1-launch-sequence)). If the hash does not match, **stop** — do not proceed. Report a hash mismatch via the [onboarding problem issue template](../.github/ISSUE_TEMPLATE/onboarding_problem.yml).

---

## 6. Configure + fast-join

Render node configuration and enable state-sync:

```bash
cd infra/testnet/node-kit
./setup-node.sh
```

This writes `config.toml`, `app.toml`, and `client.toml` under `$VERTIXD_HOME/config/` with your seeds, state-sync peers, and pruning settings.

---

## 7. Keys

Create operator and feeder keys (consensus key is managed separately via TMKMS):

```bash
vertixd keys add operator --home "$VERTIXD_HOME" --keyring-backend file
vertixd keys add feeder   --home "$VERTIXD_HOME" --keyring-backend file

OPERATOR=$(vertixd keys show operator -a --home "$VERTIXD_HOME" --keyring-backend file)
curl -X POST "http://<faucet-host>/credit" \
  -H "Content-Type: application/json" \
  -d "{\"denom\":\"uvtx\",\"address\":\"$OPERATOR\"}"
```

Fund the feeder key separately for oracle tx fees after the node is running.

---

## 8. Start

Install and enable systemd units:

```bash
sudo cp systemd/vertixd.service systemd/vertix-feeder.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable vertixd vertix-feeder
sudo systemctl start vertixd
```

Wait until synced (or state-sync completes):

```bash
curl -s localhost:26657/status | jq '.result.sync_info.catching_up'
# false = ready
```

Then start the feeder:

```bash
sudo systemctl start vertix-feeder
```

---

## 9. Join the active set

Once synced and funded, create a validator:

```bash
PUBKEY=$(vertixd tendermint show-validator --home "$VERTIXD_HOME")

cat > /tmp/validator.json <<EOF
{
  "pubkey": $PUBKEY,
  "amount": "1000000000000uvtx",
  "moniker": "$MONIKER",
  "commission-rate": "0.10",
  "commission-max-rate": "0.20",
  "commission-max-change-rate": "0.01",
  "min-self-delegation": "1"
}
EOF

vertixd tx staking create-validator /tmp/validator.json \
  --from operator \
  --chain-id vertix-testnet-1 \
  --node tcp://localhost:26657 \
  --home "$VERTIXD_HOME" \
  --keyring-backend file \
  --fees 5000uvtx \
  --gas 500000 \
  -y
```

Verify voting power:

```bash
curl -s localhost:26657/status | jq '.result.validator_info'
```

---

## 10. Run your feeder

Configure the feeder sidecar:

```bash
cp feeder.example.yaml "$VERTIXD_HOME/feeder.yaml"
# Edit: validator (valoper address), key_name=feeder, chain_id, rpc/grpc endpoints
```

Confirm submissions are incrementing:

```bash
curl -s localhost:9200/metrics | grep feeds_submitted_total
# Counter should increase each vote window (~10 blocks)
```

Every active validator **must** run a feeder. Missed feed windows accumulate toward oracle slashes (0.5% of bonded stake).

---

## 11. Self-monitoring

Use the node-kit health checklist ([`infra/testnet/node-kit/README.md`](../infra/testnet/node-kit/README.md#is-my-validator-healthy)):

- [ ] Signing: `curl -s localhost:26657/status | jq .result.validator_info` shows your address with voting power
- [ ] Synced: `catching_up` is `false`
- [ ] Peered: `curl -s localhost:26657/net_info | jq .result.n_peers` ≥ 3
- [ ] Feeding: `feeds_submitted_total` increases each vote window
- [ ] Not missing: Tenderduty shows 0 recent missed blocks
- [ ] Disk OK: data dir has headroom

Enable [Tenderduty](../infra/testnet/monitoring/tenderduty/config.yml) locally or point it at your validator RPC. Alert on missed blocks before slashing thresholds.

---

## 12. Sentry hardening (recommended)

Run your validator **behind ≥2 sentry nodes** — only sentries expose public p2p (`26656`). See [validator setup §Sentry topology](./validator-setup.md#overview) for the production pattern:

```
Internet → Sentry 1, Sentry 2 → Validator (private) → TMKMS
```

On testnet you may run a simplified single-node join, but sentries are strongly recommended before mainnet.

---

## 13. Troubleshooting

| Symptom | Likely cause | Action |
|---------|--------------|--------|
| `sha256sum: FAILED` on genesis | Wrong or tampered genesis | Re-download from official URL; verify hash with founders |
| Node not signing / zero voting power | Validator not created or jailed | Check `vertixd query staking validator <valoper>`; re-submit `create-validator` if needed |
| `feeds_submitted_total` flat | Feeder misconfigured or unfunded | Check `feeder.yaml` valoper + key; fund feeder key; check logs: `journalctl -u vertix-feeder` |
| State-sync fails | Stale trust height/hash | Request fresh `STATESYNC_TRUST_*` from founders |
| Oracle slash incoming | Missed feed windows | Restart feeder; verify provider connectivity |

File issues using the matching GitHub template:

- **Genesis / state-sync / create-validator / feeder / monitoring** → [Onboarding problem](../.github/ISSUE_TEMPLATE/onboarding_problem.yml)
- **Oracle feed stalls or provider errors** → [Feed incident](../.github/ISSUE_TEMPLATE/feed_incident.yml)
- **Module bugs** → [Bug report](../.github/ISSUE_TEMPLATE/bug_report.yml)

---

## 14. Cosmovisor upgrade (v0.2.0-testnet)

When the chain schedules upgrade plan **`v0.2.0-testnet`**, every validator must stage the new binary before the upgrade height. This proves the Cosmovisor path used for all future upgrades (Phase 9 rehearsal on `vertix-testnet-2`).

**Environment:**

```bash
export DAEMON_NAME=vertixd
export DAEMON_HOME=/home/vertix/.vertixd   # match your node-kit VERTIXD_HOME
```

**Stage binary (before upgrade height):**

```bash
mkdir -p "$DAEMON_HOME/cosmovisor/upgrades/v0.2.0-testnet/bin"
cp /path/to/vertixd-v0.2.x "$DAEMON_HOME/cosmovisor/upgrades/v0.2.0-testnet/bin/vertixd"
chmod +x "$DAEMON_HOME/cosmovisor/upgrades/v0.2.0-testnet/bin/vertixd"
```

**Expected layout:**

```
$DAEMON_HOME/
├── cosmovisor/
│   ├── current -> genesis/          # symlink managed by Cosmovisor
│   ├── genesis/bin/vertixd            # binary at chain start
│   └── upgrades/
│       └── v0.2.0-testnet/
│           └── bin/vertixd          # must exist before upgrade height
├── data/
└── config/
```

**At upgrade height:** CometBFT halts → Cosmovisor swaps `current` to `upgrades/v0.2.0-testnet/bin` → `x/upgrade` handler runs `RunMigrations` → chain resumes.

**Verify after upgrade:**

```bash
curl -s localhost:26657/status | jq '.result.sync_info | {height, catching_up}'
# Height should advance; catching_up false after sync
curl -s localhost:9200/metrics | grep feeds_submitted_total
# Feeder counter should resume within ~2 vote windows
```

**Snapshot before upgrade:** Back up `$DAEMON_HOME/data` (or use your volume snapshot policy). If migration or swap fails, restore the snapshot and coordinate a new upgrade height with founders — do not restart with a broken binary.

Founders run the automated rehearsal: `make testnet-v2-upgrade`. Operators follow the same binary path documented in [testnet runbook §10.3](./testnet-runbook.md#103-cosmovisor-upgrade-procedure).
