# Vertix Validator Setup (Production)

**Audience:** operators running validators on **testnet** or **mainnet** (`chain_id = vertix-1`).  
**Related:** [Tendermint KMS (`tmkms`)](./tmkms.md) · [Devnet runbook](./devnet.md) · [Architecture §5.2](./architecture.md#52-validator-architecture-production) · [Technical design §5](./technical-design.md#5-vertix-feeder-sidecar--design)

This guide describes **sentry-node isolation**, **network exposure**, the **three-key security model**, and **monitoring hooks** used by Phase 8 dashboards. The local Docker devnet is a simplified topology for development — see [Comparison to devnet](#comparison-to-devnet).

---

## Overview

A production Vertix validator should:

1. Stay **off the public p2p mesh** — only **sentry** nodes expose `26656`.
2. Sign blocks with a **remote consensus key** via [`tmkms`](./tmkms.md).
3. Run **`vertix-feeder`** with a **delegated feeder key** (never the operator key on that host).
4. Expose **Prometheus** metrics on CometBFT `:26660` and feeder `:9200` for monitoring and alerting.

```
                         Internet / public peers
                                    │
                    ┌───────────────┴───────────────┐
                    ▼                               ▼
             ┌────────────┐                  ┌────────────┐
             │  Sentry 1  │ ◄──── p2p ────► │  Sentry 2  │   (≥2 recommended)
             │  :26656    │                  │  :26656    │
             └─────┬──────┘                  └─────┬──────┘
                   │    private_peer_ids /           │
                   │    persistent_peers             │
                   └────────────┬────────────────────┘
                                ▼
                    ┌───────────────────────┐
                    │  Validator (private)   │
                    │  vertixd + feeder      │
                    │  NO public :26656      │
                    └───────────┬───────────┘
                                │ privval :26659
                                ▼
                    ┌───────────────────────┐
                    │  tmkms (signing)       │  ← docs/tmkms.md
                    └───────────────────────┘
```

---

## Sentry-node architecture

**Goal:** Validators never advertise a dialable public address. Sentries absorb peer discovery, gossip, and denial-of-service noise; the validator maintains **private** outbound connections to sentries only.

| Role | Public p2p (`26656`) | Signs blocks | Holds consensus key |
|------|----------------------|--------------|---------------------|
| Sentry | Yes | No | No (node key only) |
| Validator | **No** | Yes (via `tmkms`) | No on disk — `tmkms` only |
| `tmkms` | **No** | Yes | Yes (HSM / softsign) |

Run **at least two** sentries in independent failure domains (AZ/region). Three is common for mesh redundancy.

### Sentry `config.toml` (each sentry)

```toml
# P2P — public face
p2p.laddr = "tcp://0.0.0.0:26656"
p2p.external_address = "<public-ip-or-dns>:26656"
p2p.pex = true
# Seed / persistent peers: other sentries, trusted boot nodes, NOT the validator node id

p2p.persistent_peers = "<peer_id>@sentry2:26656,<peer_id>@seed:26656"
# Do NOT list the validator's node id in any public config

# RPC — localhost or management VPN only (see Firewall)
rpc.laddr = "tcp://127.0.0.1:26657"
```

### Validator `config.toml` (private)

```toml
p2p.laddr = "tcp://0.0.0.0:26656"
p2p.external_address = ""              # no public advertisement
p2p.pex = false                        # do not join public peer exchange

# Only sentries — use node IDs from `vertixd tendermint show-node-id` on each sentry
p2p.persistent_peers = "<sentry1-id>@<sentry1-private-ip>:26656,<sentry2-id>@<sentry2-private-ip>:26656"

# Hide validator node id from peer exchange (repeat for each sentry id if needed)
p2p.private_peer_ids = "<sentry1-node-id>,<sentry2-node-id>"

# Always reconnect to sentries even when pex is false
p2p.unconditional_peer_ids = "<sentry1-node-id>,<sentry2-node-id>"

# Remote consensus signing — see docs/tmkms.md
priv_validator_laddr = "tcp://127.0.0.1:26659"

# Prometheus (validator — scrape from monitoring VPC)
[instrumentation]
prometheus = true
prometheus_listen_addr = ":26660"
```

**`private_peer_ids`:** prevents the validator from gossiping sentry addresses into the public peer store.  
**`unconditional_peer_ids`:** keeps sentry links up under load or peer-score churn.  
**`pex = false` on validator:** eliminates accidental public mesh membership.

### Sentry ↔ validator wiring

1. On each **sentry**, add the **validator node id** to `private_peer_ids` (so sentries do not broadcast the validator) and add validator to `persistent_peers` on a **private** IP if sentries initiate (common pattern: validator dials sentries only — then sentries need validator in `persistent_peers` on private nets).
2. Confirm bidirectional block sync: validator height advances, sentries follow the same network.
3. Register the validator with **consensus pubkey** from `tmkms` / `tendermint show-validator`, not from a file on the validator disk.

Operational details vary by cloud; document your chosen direction (validator-initiated vs sentry-initiated) in your internal runbook.

---

## Firewall and port guidance

Principle: **minimize listening sockets**; **p2p public only on sentries**; **RPC/gRPC not on the Internet**.

### Port matrix

| Port | Service | Sentry (public) | Validator (private) | Notes |
|------|---------|-----------------|---------------------|--------|
| **26656** | CometBFT p2p | **Open** to Internet (or allowlist) | **Closed** inbound from Internet | Only sentries need inbound 26656 |
| **26657** | RPC | `127.0.0.1` or admin VPN | `127.0.0.1` or admin VPN | Never `0.0.0.0` on mainnet |
| **1317** | REST / API | Optional localhost | Optional localhost | Disable if unused |
| **9090** | gRPC | Loopback or private net | Loopback or private net | Feeder uses gRPC to **local** `vertixd` |
| **26659** | privval (`tmkms`) | Closed | **tmkms host only** | See [tmkms.md](./tmkms.md) |
| **26660** | CometBFT Prometheus | Optional | Scrape from monitoring net | `/metrics` |
| **9200** | `vertix-feeder` Prometheus | N/A | Scrape from monitoring net | `/metrics` |

### Example host firewall (illustrative `ufw`)

**Sentry:**

```bash
ufw default deny incoming
ufw allow 22/tcp comment 'SSH (restrict source in production)'
ufw allow 26656/tcp comment 'CometBFT p2p'
# Do NOT ufw allow 26657 to the world
ufw enable
```

**Validator:**

```bash
ufw default deny incoming
ufw allow from <monitoring-subnet> to any port 26660 proto tcp
ufw allow from <monitoring-subnet> to any port 9200 proto tcp
ufw allow from <validator-ip> to <tmkms-ip> port 26659 proto tcp
# Private RFC1918 between validator and sentries only for 26656
ufw enable
```

Bind RPC explicitly in `config.toml`:

```toml
rpc.laddr = "tcp://127.0.0.1:26657"
grpc.laddr = "tcp://127.0.0.1:9090"
api.enable = true
api.swagger = false
api.address = "tcp://127.0.0.1:1317"
```

Use SSH tunnels or a private observability stack to reach localhost services from your workstation.

---

## Three-key separation model

Vertix deliberately splits custody so a single host compromise does not equal full validator + fund loss.

| Key | Bech32 examples | Signs | Where it lives | Rotation |
|-----|-----------------|-------|----------------|----------|
| **Consensus** | `vtxvalconspub…` | Block votes | **`tmkms` only** | On-chain validator key change (rare); see [tmkms.md](./tmkms.md) |
| **Operator** | `vtx1…` / `vtxvaloper…` | Staking, gov, `MsgSetFeeder` | Hardware wallet / cold station | Standard account rotation |
| **Feeder** | `vtx1…` (dedicated account) | `MsgSubmitFeed` only | Feeder host keyring (`file` / `os`) | `vertixd tx oracle set-feeder …` |

### Consensus (tmkms)

- Required on mainnet; see [tmkms.md](./tmkms.md).
- Never submit SDK transactions with this key.

### Operator (cold)

- Creates the validator and delegates the feeder:

```bash
vertixd tx oracle set-feeder <valoper> <feeder-account> \
  --from <operator> --chain-id vertix-1 --gas auto
```

- Used for `MsgEditValidator`, unjail, governance, and RWA/bond operations if you act as an issuer.
- **Never** install on the `vertix-feeder` machine.

### Feeder (hot, delegated)

- Run [`vertix-feeder`](../feeder/) on the validator host (or a dedicated sidecar VM on the same private LAN).
- Configure `key_name`, `validator` (valoper), and `node_grpc` pointing at **local** `vertixd` (e.g. `localhost:9090`).
- Fund the feeder account with a small VTX balance for tx fees only.

**Implicit feeder warning:** if you skip `MsgSetFeeder`, the chain defaults to the operator address as feeder — that places a hot path on your cold key. Always delegate a dedicated feeder before mainnet.

---

## `vertix-feeder` on the validator host

Every **active** validator must run the sidecar ([`technical-design.md` §5](./technical-design.md#5-vertix-feeder-sidecar--design)).

Minimal production settings (`feeder-config.yaml`):

```yaml
chain_id: vertix-1
node_grpc: localhost:9090
validator: vtxvaloper1...          # your valoper
key_name: feeder
keyring_backend: file              # or os
keyring_dir: /var/lib/vertix-feeder/keyring
prometheus:
  enabled: true
  port: 9200
```

- **gRPC** should hit the co-located `vertixd`, not a public RPC endpoint.
- Restrict egress from the feeder VM to price-provider APIs only.
- Example reference: [`feeder-config.example.yaml`](../feeder-config.example.yaml) (defaults to `vertix-devnet-1` — change for mainnet).

---

## Monitoring hooks (Phase 8 dashboards)

Phase 6 introduced scrape targets; Phase 8 expands dashboards and alerting. Enable these endpoints in production so Prometheus (or an agent) can reach them from a **monitoring VPC**.

### CometBFT Prometheus (`:26660`)

In `config.toml`:

```toml
[instrumentation]
prometheus = true
prometheus_listen_addr = ":26660"
```

**Useful series (non-exhaustive):**

| Metric family | Purpose |
|---------------|---------|
| `cometbft_consensus_height` | Block production |
| `cometbft_consensus_validator_missed_blocks` | Liveness / slashing risk |
| `cometbft_consensus_validator_power` | Voting weight |
| `cometbft_p2p_peers` | Sentry connectivity |

Scrape example:

```yaml
# prometheus.yml (snippet)
- job_name: vertix-cometbft
  metrics_path: /metrics
  static_configs:
    - targets: ["validator-internal:26660"]
      labels: { chain: vertix-1, role: validator }
```

Devnet reference: validators expose `26660` — see [`infra/monitoring/prometheus.yml`](../infra/monitoring/prometheus.yml) and [`devnet.md` § Explorer, faucet, and dashboards](./devnet.md#explorer-faucet-and-dashboards).

### Feeder Prometheus (`:9200`)

With `prometheus.enabled: true` in feeder config, metrics are served at `http://<host>:9200/metrics`.

| Metric | Type | Labels | Purpose |
|--------|------|--------|---------|
| `vertix_feeder_feeds_submitted_total` | Counter | `pair` | Successful window submissions |
| `vertix_feeder_feeds_failed_total` | Counter | `pair`, `reason` | Broadcast / validation failures |
| `vertix_feeder_provider_latency_ms` | Histogram | `provider`, `pair` | Price source SLO |
| `vertix_feeder_provider_errors_total` | Counter | `provider` | Provider health |
| `vertix_feeder_last_submit_timestamp` | Gauge | `pair` | Staleness detection |

```yaml
- job_name: vertix-feeders
  metrics_path: /metrics
  static_configs:
    - targets: ["validator-internal:9200"]
```

**Alerting hints (Phase 8):** rising `feeds_failed_total`, flat `feeds_submitted_total` across vote windows, and increasing CometBFT missed blocks often precede oracle miss slashes (0.5%) — correlate feeder and consensus metrics.

Testnet reference configs: [`infra/testnet/monitoring/`](../infra/testnet/monitoring/) (Grafana dashboards, [`alert-rules.yml`](../infra/testnet/monitoring/alert-rules.yml), Tenderduty, PANIC). External validators: follow [`docs/validator-onboarding.md`](./validator-onboarding.md) for self-monitoring setup.

### Other operational metrics

- **SDK / module metrics** — extend via application instrumentation as Phase 8 adds chain-wide dashboards.
- **Tenderduty / PANIC** — recommended for validator miss alerts ([`architecture.md` §7](./architecture.md#7-operational-stack-devnet--mainnet)).
- **Supply / fee burn** — query `bank` total supply and `x/fees` events; devnet Grafana already trends supply ([`devnet.md`](./devnet.md)).

---

## Comparison to devnet

The Phase 6 [devnet runbook](./devnet.md) intentionally differs from production:

| Topic | Devnet (`vertix-devnet-1`) | Production (`vertix-1`) |
|-------|----------------------------|------------------------|
| Topology | 3 validators on one Docker network, public p2p between them | Validator + ≥2 sentries; validator not on public p2p |
| Consensus key | `infra/devnet/keys/.../priv_validator_key.json` in repo | [`tmkms`](./tmkms.md); no local privval key |
| Feeder keys | Test keyring + `setup-feeders.sh` | Dedicated feeder + `MsgSetFeeder` |
| Metrics | Host `26660`, feeders `9201–9203` → container `9200` | Internal `:26660` + `:9200` scrape |
| Purpose | CI smoke, module integration, Grafana starter | Mainnet security posture |

Use devnet to validate feeder authorization, oracle aggregation, and dashboard wiring — then apply this document and [tmkms.md](./tmkms.md) for public networks.

---

## Bring-up checklist

### Infrastructure

- [ ] ≥2 sentries with public `26656`; validator without public inbound p2p
- [ ] `pex`, `persistent_peers`, `private_peer_ids`, `unconditional_peer_ids` configured per above
- [ ] Firewall: RPC/gRPC localhost; monitoring subnet → `26660` / `9200`

### Keys

- [ ] Consensus key in `tmkms`; `priv_validator_laddr` configured ([tmkms.md](./tmkms.md))
- [ ] Operator key offline; feeder key created and funded
- [ ] `MsgSetFeeder` committed; `vertixd query oracle feeder <valoper>` matches feeder address

### Workloads

- [ ] `vertixd` synced; validator registered with correct consensus pubkey
- [ ] `vertix-feeder` running; `query oracle price` returns data for accepted pairs
- [ ] Prometheus targets **UP** for CometBFT and feeder

### Documentation cross-links

- Remote signing: [tmkms.md](./tmkms.md)
- Local stack & smoke tests: [devnet.md](./devnet.md)
- Oracle/feeder contract: [technical-design.md §2 & §5](./technical-design.md)
- Security architecture table: [architecture.md §8](./architecture.md#8-security-architecture)

---

## References

- [Vertix `tmkms` guide](./tmkms.md)
- [Vertix devnet runbook](./devnet.md)
- [Vertix architecture — validator topology](./architecture.md)
- [Vertix technical design — oracle & feeder](./technical-design.md)
- [CometBFT configuration](https://docs.cometbft.com/v0.38/core/configuration)
- [Cosmos SDK validator guide](https://docs.cosmos.network/main/validators/validator-setup)
