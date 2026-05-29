# Validator Onboarding — Public Testnet

**Chain:** `vertix-testnet-1` · **Related:** [Validator setup](./validator-setup.md) · [TMKMS](./tmkms.md) · [Node kit](../infra/testnet/node-kit/) · [Testnet runbook](./testnet-runbook.md)

This document is the **onboarding contract** for external validators joining `vertix-testnet-1`. Follow every step in order; skipping genesis verification or feeder setup will cause join failures or oracle slashes.

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
