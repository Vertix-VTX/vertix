# Tendermint KMS (`tmkms`) for Vertix Validators

**Audience:** validator operators preparing for **mainnet** (`chain_id = vertix-1`).  
**Related:** [Validator setup (sentry topology & keys)](./validator-setup.md) · [Architecture §5.2](./architecture.md#52-validator-architecture-production) · [Devnet runbook](./devnet.md) (local keys only — not a `tmkms` deployment)

Vertix requires **remote signing** for the **consensus (priv_validator) key** on any public network. [Tendermint KMS](https://github.com/iqlusioninc/tmkms) (`tmkms`) holds that key off the validator process and signs blocks over a dedicated privval socket. It does **not** replace your operator or feeder keys — see [Vertix key separation](#vertix-key-separation-consensus-vs-feeder-vs-operator) below.

---

## What `tmkms` protects (and what it does not)

| Key / role | Used for | Protected by `tmkms`? |
|------------|----------|------------------------|
| **Consensus** (`priv_validator`) | CometBFT block signing, double-sign exposure | **Yes** — this is the only key `tmkms` manages |
| **Operator** (cold / hardware wallet) | `MsgCreateValidator`, `MsgEditValidator`, `MsgSetFeeder`, governance, bond/unbond | **No** — keep offline; never on the feeder host |
| **Feeder** (hot, delegated) | Signs `MsgSubmitFeed` for `x/oracle` | **No** — separate keyring on the feeder sidecar host |

**Why isolate consensus:** A compromised consensus key can double-sign and trigger tombstoning. Moving signing to `tmkms` (ideally HSM-backed) shrinks the attack surface on the validator VM and enables state tracking that blocks equivocation.

**What stays on `vertixd`:** The **node key** (`node_key.json`) for p2p identity is *not* the consensus key. Sentry nodes hold their own node keys; the validator’s node key is only used on the private validator ↔ sentry links.

---

## Supported signing backends

`tmkms` selects a **provider** that performs the actual signature operation. Vertix operators typically choose one of:

| Backend | Provider block | Typical use | Notes |
|---------|----------------|-------------|--------|
| **softsign** | `[[providers.softsign]]` | Testnet / staging | Key material on disk under `tmkms` secrets dir; encrypt at rest; not for high-value mainnet |
| **YubiHSM 2** | `[[providers.yubihsm]]` | Production mainnet | Keys never leave the HSM; requires YubiHSM setup and connector |
| **Ledger** | `[[providers.ledger]]` | Production (smaller ops) | Tendermint app on Ledger; manual approval per sign; good for low-frequency failover drills |

Build `tmkms` with the features you need (see upstream README). Example (softsign only):

```bash
cargo install tmkms --features=softsign
```

For YubiHSM or Ledger, enable the corresponding feature flags in your build or use a prebuilt binary from your distro’s packaging pipeline.

---

## Reference layout

```
/opt/tmkms/
├── tmkms.toml
├── state/
│   └── priv_validator_state.json   # height/round/step — critical for anti-double-sign
└── secrets/
    └── priv_validator_key          # softsign only; HSM/Ledger use device keys instead
```

On the **validator host** (private network):

```
~/.vertix/
└── config/
    └── config.toml                 # priv_validator_laddr → tmkms listener
```

`vertixd` must **not** load `priv_validator_key.json` locally when using remote signing — remove or never generate it on the validator; only `tmkms` holds the consensus private key.

---

## `tmkms.toml` example (`chain_id = vertix-1`)

Vertix Bech32 prefixes (from `app/app.go`): account `vtx`, validator `vtxvaloper`, consensus `vtxvalcons` / `vtxvalconspub`.

```toml
# /opt/tmkms/tmkms.toml — Vertix mainnet (vertix-1)
# Adjust paths, validator IP, and provider to your environment.

[chain]
id = "vertix-1"
key_format = { type = "bech32", account_key_prefix = "vtx", consensus_key_prefix = "vtxvalconspub" }
state_file = "/opt/tmkms/state/priv_validator_state.json"

# Listen for privval connections from vertixd (validator host connects inbound,
# or place tmkms on the same private LAN and bind accordingly).
[[validator]]
addr = "tcp://0.0.0.0:26659"
chain_id = "vertix-1"
reconnect = true
# secret_key only for softsign — omit for YubiHSM/Ledger and use provider config instead
secret_key = "/opt/tmkms/secrets/priv_validator_key"

[[providers.softsign]]
chain_ids = ["vertix-1"]
```

**YubiHSM (sketch):** replace `[[providers.softsign]]` with `[[providers.yubihsm]]` per [upstream YubiHSM docs](https://github.com/iqlusioninc/tmkms/blob/main/README.md#yubihsm2); point `[[validator]]` at the HSM-backed key label instead of `secret_key`.

**Ledger (sketch):** use `[[providers.ledger]]` with `chain_ids = ["vertix-1"]` and configure the validator connection without a filesystem `secret_key`.

> **Genesis / key migration:** Import or generate the consensus key **once**, register the matching `vtxvalconspub` consensus pubkey in your validator creation tx, then delete local `priv_validator_key.json` from the validator node. Keep backups of HSM-wrapped or softsign secrets only in operator-controlled vaults.

---

## Connecting `tmkms` to `vertixd` (privval socket)

CometBFT signs blocks via the **privval** interface. With `tmkms`, `vertixd` acts as a **client**; `tmkms` is the **signer server**.

### 1. Configure `vertixd` (`config.toml`)

On the **validator** machine (not on public sentries):

```toml
# ~/.vertix/config/config.toml

priv_validator_laddr = "tcp://127.0.0.1:26659"
```

- Point `127.0.0.1:26659` at the host where `tmkms` listens (same machine, or a private IP if `tmkms` runs on a dedicated signing appliance).
- Ensure **no** local `priv_validator_key.json` is present in `config/` when remote signing is active.

### 2. Start order

1. Start `tmkms` and confirm it loads `priv_validator_state.json` and the provider.
2. Start `vertixd` (or restart after config change).
3. Verify logs: CometBFT should report a remote privval connection, not file-based signing.

### 3. Network placement

| Deployment | `tmkms` bind | `priv_validator_laddr` |
|------------|--------------|---------------------------|
| Co-located on validator | `tcp://127.0.0.1:26659` | `tcp://127.0.0.1:26659` |
| Dedicated signing host | `tcp://0.0.0.0:26659` on private NIC | `tcp://<tmkms-private-ip>:26659` |

Firewall **26659** so only the validator host (or HA pair) can reach `tmkms`. Never expose privval to the public Internet.

### 4. systemd example (operator sketch)

```ini
# /etc/systemd/system/tmkms.service
[Unit]
Description=Tendermint KMS for Vertix
After=network-online.target

[Service]
User=tmkms
ExecStart=/usr/local/bin/tmkms start -c /opt/tmkms/tmkms.toml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Run `vertixd` under a separate unit that starts **after** `tmkms`.

---

## Double-sign protection

`tmkms` tracks validator state in `state_file` (`priv_validator_state.json`): last signed **height**, **round**, and **step**. Before signing a new block or prevote/precommit, it compares incoming requests against that state and **refuses** signatures that would equivocate.

**Operator practices:**

1. **One active signer per consensus pubkey** — never run `vertixd` with a local `priv_validator_key.json` and `tmkms` against the same key simultaneously.
2. **Preserve `state_file` across restarts** — back it up with the same care as key material; restoring an old copy while the chain advanced can also cause rejections (safer than double-signing).
3. **Never copy `priv_validator_state.json` between two live nodes** unless you are performing a controlled failover (see below).
4. Use **monitoring** on missed blocks and privval errors ([validator-setup.md § Monitoring](./validator-setup.md#monitoring-hooks-phase-8-dashboards)).

If double-signing occurs despite `tmkms`, the chain will tombstone the validator; recovery is governance and operational, not a key rotation alone.

---

## Failover (standby validator + `tmkms`)

A common pattern is **one consensus key, one active validator process, one hot `tmkms`**, and a **standby** stack that stays stopped until failover.

### Controlled failover procedure

1. **Stop the active `vertixd`** cleanly and wait for the process to exit (no new signatures).
2. **Stop `tmkms`** on the active signing host.
3. **Copy or replicate** the latest `priv_validator_state.json` to the standby `tmkms` host (rsync from backup, or shared highly durable volume — not NFS without careful locking).
4. Start **`tmkms` on standby**, then start **`vertixd` on standby** with the same `priv_validator_laddr` targeting the new `tmkms`.
5. Confirm the node catches up and signs blocks; alert on `tmkms` and CometBFT metrics.

**Do not** run two `tmkms` instances with the same key and divergent state files — that is how double-signs happen.

### Warm standby without signing

- Standby `vertixd` may run in **non-validator** mode (removed from `priv_validator_laddr`, or not started) for state sync testing; only promote after step 1–4 above.
- HSM failover: use vendor guidance (YubiHSM mirrored keys / backup device) so the standby `tmkms` uses the **same** consensus pubkey.

### Disaster recovery

- Maintain offline backups: `tmkms.toml`, `state_file`, and provider-specific key backup (or HSM recovery kit).
- Document **valoper address**, **consensus pubkey**, and **node IDs** of sentries in your runbook ([validator-setup.md](./validator-setup.md)).

---

## Vertix key separation (consensus vs feeder vs operator)

Vertix has **three** distinct signing roles. `tmkms` covers **only** the first.

```
┌─────────────────────────────────────────────────────────────────┐
│  Operator key (cold)                                            │
│  MsgCreateValidator, MsgSetFeeder, governance, staking ops      │
│  Storage: hardware wallet / offline — NOT on feeder host        │
└────────────────────────────┬────────────────────────────────────┘
                             │ MsgSetFeeder (one-time / rotation)
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  Feeder key (hot, low value)          vertix-feeder sidecar     │
│  Signs MsgSubmitFeed only             keyring: file | os        │
│  Must match on-chain delegation       Prometheus :9200        │
└────────────────────────────┬────────────────────────────────────┘
                             │ gRPC broadcast (local sentry/val)
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  vertixd + CometBFT                                             │
│  Consensus signing ───────────────► tmkms (privval :26659)      │
└─────────────────────────────────────────────────────────────────┘
```

### Consensus key → `tmkms`

- Managed exclusively by `tmkms` on mainnet.
- Never used for SDK transactions.

### Feeder key → `vertix-feeder`

- Delegated via `x/oracle` [`MsgSetFeeder`](./technical-design.md#2-oracle-module-xoracle) (`vertixd tx oracle set-feeder <valoper> <feeder-account> --from <operator>`).
- The sidecar signs **`MsgSubmitFeed`** with the feeder account; the message’s `validator` field is your **valoper**.
- If no delegation exists, the chain treats the **operator account** as the implicit feeder — convenient for solo dev, **avoid on mainnet**; always delegate a dedicated feeder key.
- Config: [`feeder-config.example.yaml`](../feeder-config.example.yaml) — set `chain_id: vertix-1`, `key_name: feeder`, and `validator: <your-valoper>`.

**Security contract (Phase 1 / Phase 4):** the feeder key is **low-value and rotatable**. Compromise allows feed manipulation (bounded by outlier slashing) but **not** fund theft. Rotate by broadcasting a new `MsgSetFeeder` from the operator key.

### Operator key → never on feeder host

- Used sparingly for staking, parameter votes, and feeder delegation.
- **Do not** copy the operator key to the machine running `vertix-feeder`.
- **Do not** configure `key_name` in feeder YAML to the operator key — submissions would violate the operational contract even if the chain accepted them when delegation defaults to the operator address.

### Local devnet exception

The Docker devnet ([`docs/devnet.md`](./devnet.md)) uses **file-based** `priv_validator_key.json` under `infra/devnet/keys/` for determinism. That layout is **not** a production model. Use it to learn feeder delegation and monitoring; use **`tmkms` + sentries** for testnet/mainnet.

---

## Checklist (mainnet readiness)

- [ ] Consensus key in `tmkms` (HSM or softsign per policy); no `priv_validator_key.json` on validator
- [ ] `chain_id = vertix-1` in both `tmkms.toml` and `vertixd` genesis/config
- [ ] `priv_validator_laddr` points to `tmkms`; port 26659 firewalled to validator only
- [ ] `priv_validator_state.json` backed up; failover runbook tested on testnet
- [ ] Dedicated feeder key created, funded for tx fees, authorized with `MsgSetFeeder`
- [ ] Operator key offline; feeder host has **only** feeder key material
- [ ] Prometheus scrapes CometBFT `:26660` and feeder `:9200` ([validator-setup.md](./validator-setup.md))

---

## References

- [iqlusioninc/tmkms](https://github.com/iqlusioninc/tmkms) — upstream configuration and providers
- [CometBFT privval](https://docs.cometbft.com/v0.38/core/configuration) — `priv_validator_laddr`
- [Vertix validator setup](./validator-setup.md) — sentry topology, firewall, monitoring
- [Vertix technical design §2 & §5](./technical-design.md) — oracle feeder delegation and sidecar
- [Vertix architecture §5.2 & §8](./architecture.md) — production validator diagram and security table
