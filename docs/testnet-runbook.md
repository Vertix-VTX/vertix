# Testnet Runbook — Public Testnet v1

**Chain:** `vertix-testnet-1` (v1) · `vertix-testnet-2` (v2, Phase 9) · **Related:** [Validator onboarding](./validator-onboarding.md) · [RWA quickstart](./rwa-quickstart.md) · [Bug bounty](./bug-bounty.md) · [Phase 8 plan](./plans/2026-05-29-phase-8-public-testnet.md) · [Phase 9 plan](./plans/2026-05-29-phase-9-testnet-v2-genesis-rehearsal.md) · [Phase 9 design spec](./specs/2026-05-29-phase-9-testnet-v2-genesis-rehearsal-design.md)

Live-operations guide for founders and operators running `vertix-testnet-1`. Covers launch, coordination, monitoring, incident response, bug bounty ops, stability tracking, and the Phase 9 promotion gate.

---

## 1. Launch sequence

Execute in order:

```bash
# 1. Build genesis artifacts (reproducible)
make testnet-genesis
# Outputs: infra/testnet/genesis/genesis.json, genesis.sha256, seeds.txt

# 2. Publish artifacts (hosting / CDN / GitHub release)
#    - genesis.json
#    - genesis.sha256
#    - seeds.txt

# 3. Start founder stack (validators + sentries + feeders; no ghcr.io pulls)
make testnet-up
# Optional services (ghcr.io images may need `docker logout ghcr.io` first):
make testnet-up-faucet        # Cosmfaucet HTTP :8000
make testnet-up-monitoring    # Prometheus :9095 + Grafana :3000
make testnet-up-tenderduty    # Tenderduty :8888
make testnet-up-explorer      # Ping.pub :8080
# Or all optional layers: make testnet-up-all

# 4. Verify block production (~30s after up)
curl -s http://localhost:26657/status | jq -r '.result.sync_info.latest_block_height'
# Or from inside the Docker network:
docker run --rm --network vertix-testnet vertix:testnet \
  sh -c "curl -s http://sentry1:26657/status" | jq -r '.result.sync_info.latest_block_height'
# Expected: height >= 1

# 5. Open public services
#    - Faucet (Cosmfaucet): infra/testnet/faucet/
#    - Explorer (Ping.pub): infra/explorer/chains/vertix-testnet.json
#    - Monitoring: Grafana :3000, Prometheus :9090

# 6. Run lifecycle demos
make testnet-demo
./scripts/testnet/rwa-dispute-demo.sh   # waits ~305s for gov voting period

# 7. Announce + open onboarding
#    - Publish RPC/LCD/gRPC endpoints, faucet URL, genesis URL + SHA256
#    - Point validators to docs/validator-onboarding.md
#    - Open Discord/comms channel for coordination
```

Tear down (maintenance only):

```bash
make testnet-down
```

---

## 2. Validator coordination

- **Comms channel:** Discord `#testnet-validators` (or equivalent) — founders post seed updates, state-sync trust height/hash refreshes, and scheduled maintenance windows.
- **Onboarding flow:** Direct operators to [`docs/validator-onboarding.md`](./validator-onboarding.md) — the three-key model, mandatory genesis hash verification, feeder setup, and `create-validator`.
- **Genesis / seeds distribution:**
  - `GENESIS_URL` → hosted `genesis.json`
  - `genesis.sha256` → operators run `sha256sum -c`
  - `seeds.txt` → persistent peer / seed node IDs
- **State-sync / snapshots:** Publish fresh `STATESYNC_TRUST_HEIGHT` + `STATESYNC_TRUST_HASH` weekly (or after upgrades). Update `infra/testnet/node-kit/env.example` and announce in comms.
- **Join smoke:** Founders run `make testnet-join-smoke` after each genesis rebuild to prove external join path.

---

## 3. Faucet operations

Configuration: `infra/testnet/faucet/cosmfaucet.env`

| Parameter | Purpose |
|-----------|---------|
| Per-address cooldown | Prevent single-account drain |
| Per-IP rate limit | Abuse resistance |
| Daily cap | Total drip budget per 24h |
| Drip amount | `uvtx` per successful credit |

**Refill:** Transfer `uvtx` from the testnet treasury/founder account to the faucet module account when balance is low:

```bash
vertixd query bank balances <faucet-account> --node tcp://sentry1:26657
vertixd tx bank send <treasury> <faucet-account> <amount>uvtx \
  --chain-id vertix-testnet-1 --node tcp://sentry1:26657 -y
```

**Abuse response:**

1. Tighten cooldown in `cosmfaucet.env` → restart faucet container
2. Blocklist abusive IPs at reverse-proxy / firewall
3. Announce temporary faucet pause in comms if drain is ongoing

Credit endpoint (for operators):

```bash
curl -X POST "http://<faucet-host>/credit" \
  -H "Content-Type: application/json" \
  -d '{"denom":"uvtx","address":"vtx1..."}'
```

---

## 4. Monitoring operations

**Stack:** `infra/testnet/monitoring/`

| Component | Path | Purpose |
|-----------|------|---------|
| Prometheus | `prometheus.yml` | Scrape CometBFT + feeders |
| Alert rules | `alert-rules.yml` | Feeder stall, missed blocks, chain halt |
| Grafana | `grafana/dashboards/vertix-testnet.json` | Block time, oracle, fees, RWA |
| Tenderduty | `tenderduty/config.yml` | Validator miss alerts |
| PANIC | `panic/config.ini` | Multi-node health monitoring |

**Deploy Tenderduty + PANIC:** Wire an alert channel (Telegram/PagerDuty) in Tenderduty config and PANIC's alerting section before going public. Founders monitor founder validators; external operators self-monitor per [node-kit README](../infra/testnet/node-kit/README.md).

**Dashboard URL:** `http://<monitoring-host>:3000` (Grafana)

**Alert → action mapping:**

| Alert | Severity | Action |
|-------|----------|--------|
| `FeederSubmissionsStalled` | warning | Check feeder process, key funding, provider connectivity; operator restarts `vertix-feeder` |
| `FeederFailuresRising` | warning | Inspect feeder logs; check RPC/gRPC reachability |
| `ConsensusMissingBlocks` | critical | Check validator signing, TMKMS, sentry connectivity |
| `BlockProductionStalled` | critical | **Incident** — chain halt playbook (§5a) |

---

## 5. Incident response playbooks

Each playbook: **Detection → Triage → Mitigation → Comms**

### (a) Chain halt

- **Detection:** `BlockProductionStalled` alert; RPC height flat for >5 min
- **Triage:** Check founder validator logs (`docker logs founder1`); inspect CometBFT consensus state; look for panic in ABCI logs
- **Mitigation:** If single validator fault → restart/jail recovery; if app panic → coordinate binary rollback or governance param fix; if network partition → verify sentry peering
- **Comms:** Post status in `#testnet-validators` within 15 min; update every 30 min until resolved

### (b) Bad / stalled feed

- **Detection:** `FeederSubmissionsStalled`, flat `feeds_submitted_total`, rising miss counters
- **Triage:** Identify affected validators/pairs; check provider API status; verify feeder key has funds
- **Mitigation:** Operator restarts feeder; founders check aggregation if all feeds stale (possible oracle module issue)
- **Comms:** If chain-wide → public notice; if single operator → DM + feed-incident template

### (c) Faucet drain

- **Detection:** Faucet balance near zero; abnormal credit rate
- **Triage:** Review access logs for IP patterns; check daily cap enforcement
- **Mitigation:** Tighten rate limits; blocklist; pause faucet; refill from treasury
- **Comms:** Announce pause/resume in Discord + explorer banner

### (d) Crisis-invariant trip

| Route | Meaning | Response |
|-------|---------|----------|
| `oracle/prices` | Non-positive or unlisted price stored | Halt new attestations; governance param fix or binary patch |
| `fees/reconcile` | Supply ≠ genesis − burned (**21M cap violation**) | **Critical incident** — halt chain; private security advisory; no public exploit details |
| `fees/module-balance` | `x/fees` holding uvtx at block boundary | EndBlock bug — patch + coordinated upgrade |
| `rwa/bonds` | Unbonded ACTIVE asset or escrow deficit | Pause RWA registrations; governance slash investigation |
| `rwa/denoms` | Pre-mint asset has rwa/* supply | Mint bypass — patch + audit affected assets |

Detection: crisis module halts block production with invariant failure message in logs.

### (e) Mass validator jailing

- **Detection:** Low validator count; many `ConsensusMissingBlocks` alerts
- **Triage:** Network-wide outage vs. coordinated maintenance; check unbonding/jailing events
- **Mitigation:** Founders maintain minimum liveness; operators restore feeders/signing; consider temporary `min_signed_per_window` governance adjustment only as last resort
- **Comms:** Validator-wide ping; link to onboarding troubleshooting

---

## 6. Bug-bounty operations

1. **Platform listing:** Create Immunefi or HackerOne program referencing [`docs/bug-bounty.md`](./bug-bounty.md) scope and severity rubric.
2. **Submission flow:**
   - Researcher → [`SECURITY.md`](../SECURITY.md) (private advisory or email)
   - Triage team → severity mapping (48h ack, 5-day classification)
   - Fix → branch + testnet upgrade
   - Disclosure → coordinated public advisory + credit
3. **Backlog:** Non-Critical fixes triaged to GitHub `v2-backlog` label after patch
4. **Severity rubric:** Same table as [bug-bounty.md §Severity rubric](./bug-bounty.md#severity-rubric-mapped-to-crisis-invariants)

---

## 7. Triage rubric

| Label | Severity | SLA | Action |
|-------|----------|-----|--------|
| `critical` | Critical | 24h response, 7d fix target | Private advisory; founder war room |
| `high` | High | 48h response, 14d fix target | Priority sprint |
| `medium` | Medium | 5d response | Scheduled fix → `v2-backlog` |
| `low` | Low | Best effort | `v2-backlog` or close |
| `testnet` | — | — | All public testnet issues |
| `onboarding` | — | — | Validator join problems |
| `feed-incident` | — | — | Oracle feeder stalls |

**Crisis-invariant mapping:** See [bug-bounty.md](./bug-bounty.md#severity-rubric-mapped-to-crisis-invariants). Any report claiming `fees/reconcile` or `rwa/bonds` bypass is automatically Critical.

**`v2-backlog`:** GitHub project section for Phase 9 fixes — audit findings, Medium/Low bounty items, and UX improvements deferred from testnet v1.

---

## 8. 4-week stability tracker

Update weekly during the public testnet endurance period:

| Metric | Week 1 | Week 2 | Week 3 | Week 4 |
|--------|--------|--------|--------|--------|
| Uptime (%) | | | | |
| Avg block time (s) | | | | |
| Missed-block rate (founder avg) | | | | |
| Feed coverage (independent sidecars / active validators) | | | | |
| Fee-burn reconciliation (`genesisSupply − supply(uvtx) == cumulativeBurned`) | ✅/❌ | | | |
| External validator count | | | | |
| Open Criticals | | | | |
| Open Highs | | | | |

**Reconciliation check:**

```bash
# genesisSupply from genesis.json bank supply
# supply(uvtx) from live query
vertixd query bank total --node tcp://<rpc>:26657 -o json | jq '.supply[] | select(.denom=="uvtx")'
# Compare against x/fees cumulative burn events / crisis invariant
```

Target: 4 consecutive weeks with uptime ≥99%, reconciliation passing, and zero unaddressed Critical/High.

---

## 9. Phase 9 promotion gate

Testnet v1 graduates to Phase 9 (audit + testnet v2 + genesis rehearsal) when **all** gates pass:

- [ ] **4 consecutive clean weeks** on the stability tracker (§8)
- [ ] **≥10 external validators** in the active set with independent feeders
- [ ] **Independent feeders operating** — feed coverage ≥80% of active validators
- [ ] **≥1 public RWA lifecycle** demonstrated (happy path + dispute)
  - `make testnet-demo` PASS
  - `rwa-dispute-demo.sh` PASS
- [ ] **No unaddressed Critical/High** from bug bounty or public reports
- [ ] **Backlog triaged** — all Medium/Low items in `v2-backlog` with owners

Document gate review in a GitHub milestone or governance forum post before initiating Phase 9.

---

## 10. Testnet v2 (Phase 9)

**Chain ID:** `vertix-testnet-2` · **Artifacts:** `infra/testnet/v2/genesis/` · **Scripts:** [`scripts/testnet/v2/`](../scripts/testnet/v2/) · **Design:** [Phase 9 design spec](./specs/2026-05-29-phase-9-testnet-v2-genesis-rehearsal-design.md)

Phase 9 runs a **mainnet-equivalent dry run** on v2 while **v1 stays live**. v1 and v2 use separate chain IDs, genesis files, and published hashes. Do not mix artifacts.

| Make target | Purpose |
|-------------|---------|
| `make testnet-v2-genesis-base` | Base genesis (accounts, params; no gentxs) |
| `make testnet-v2-genesis` | Internal founder gentxs + collect + publish SHA256 |
| `make testnet-v2-up` | Founder-only v2 Compose stack |
| `make testnet-v2-upgrade` | Cosmovisor E2E (gov → swap → resume) |
| `make testnet-v2-verify` | Block production + crisis invariants |
| `make testnet-v2-load-test` | Module-realistic harness; calibrates SLO in [`docs/load-test.md`](./load-test.md) |
| `make testnet-v2-down` | Tear down v2 stack |

### 10.1 Internal gate overview

v2 launches **founder-only** first. External validators are not invited until the internal gate (§10.4) passes.

```
Founders only                    Public open (after gate)
────────────────────             ─────────────────────────
gentx ceremony + hash publish →  invite v1 validators
make testnet-v2-up              →  publish RPC/LCD/gRPC + seeds
Cosmovisor upgrade test         →  registry PR drafted
load harness + 2-week stability →  Keplr/Leap verified
```

`vertix-testnet-1` operations (§§1–8) continue unchanged during Phase 9.

### 10.2 Gentx coordinator playbook

Step 1 rehearses coordinated pre-launch genesis (v1 skipped this with auto-generated founder gentxs).

**Coordinator (founder ops lead):**

1. Build base genesis: `make testnet-v2-genesis-base` → `infra/testnet/v2/genesis/base/base-genesis.json`.
2. Publish base genesis + **submission deadline** (UTC) + gentx filename convention (`gentx-<moniker>.json`) to all ceremony participants.
3. After deadline: collect gentx JSONs into `infra/testnet/v2/gentxs/`.
4. Merge: `./scripts/testnet/v2/collect-gentxs.sh` (or `make testnet-v2-genesis` for the internal founder path).
5. Validate: `vertixd genesis validate-genesis infra/testnet/v2/genesis/genesis.json`.
6. Publish final `genesis.json` + `genesis.sha256` to `infra/testnet/v2/genesis/` and the hosting URL used in node-kit `GENESIS_URL`.
7. Announce **coordinated start UTC**; all participants verify hash before `vertixd start`.

**Each ceremony participant (before chain start):**

```bash
vertixd init "$MONIKER" --chain-id vertix-testnet-2 --home "$VERTIXD_HOME"
# Fund operator account from published base genesis allocation instructions
vertixd genesis gentx "$MONIKER" <self-delegation>uvtx \
  --chain-id vertix-testnet-2 \
  --home "$VERTIXD_HOME" \
  --moniker "$MONIKER" \
  --commission-rate 0.10 \
  --commission-max-rate 0.20 \
  --commission-max-change-rate 0.01 \
  --min-self-delegation 1 \
  --output-document "gentx-$MONIKER.json"
# Submit gentx-$MONIKER.json to coordinator before deadline
```

**Hash verification (all participants):**

```bash
curl -fsSL "$GENESIS_URL" -o genesis.json
echo "<published-sha256>  genesis.json" | sha256sum -c -
```

If hash verification fails: **do not start.** Coordinator re-runs collect; participants re-verify. v2 is disposable — wipe and retry without affecting v1.

Full script reference: [`scripts/testnet/v2/`](../scripts/testnet/v2/) (`build-genesis-base.sh`, `collect-gentxs.sh`, `build-genesis-internal.sh`).

### 10.3 Cosmovisor upgrade procedure

**Upgrade plan name:** `v0.2.0-testnet` (store migration via `RunMigrations`; see `app/upgrades/v020/`).

**Pre-upgrade checklist:**

- [ ] v2 stack running and stable (`make testnet-v2-up`)
- [ ] Snapshot or backup `$DAEMON_HOME/data` on each validator
- [ ] v0.2.x binary built: `make build`
- [ ] Upgrade binary staged on every validator (see layout below)
- [ ] Gov voting period allows proposal to pass before upgrade height

**Binary placement (each validator host):**

```bash
export DAEMON_NAME=vertixd
export DAEMON_HOME=/home/vertix/.vertixd   # or your node home

mkdir -p "$DAEMON_HOME/cosmovisor/upgrades/v0.2.0-testnet/bin"
cp build/vertixd "$DAEMON_HOME/cosmovisor/upgrades/v0.2.0-testnet/bin/vertixd"
chmod +x "$DAEMON_HOME/cosmovisor/upgrades/v0.2.0-testnet/bin/vertixd"
```

Cosmovisor layout:

```
$DAEMON_HOME/
├── cosmovisor/
│   ├── current -> genesis/   # or upgrades/<prior>/bin
│   ├── genesis/bin/vertixd
│   └── upgrades/
│       └── v0.2.0-testnet/
│           └── bin/vertixd   # staged before upgrade height
```

**Execution:**

1. Submit gov `SoftwareUpgrade` proposal: `name: v0.2.0-testnet`, `height: H+N` (default lead: 50 blocks in `upgrade-test.sh`).
2. Validators vote; ensure binary is staged before height.
3. At upgrade height: chain halts → Cosmovisor swaps → `RunMigrations` → resume.
4. Verify: `make testnet-v2-verify` (block production + five crisis invariants).
5. Confirm oracle feeds resume within 2 vote windows (`feeds_submitted_total` on feeders).

**Automated rehearsal:** `make testnet-v2-upgrade` (`scripts/testnet/v2/upgrade-test.sh`).

**Rollback:** If swap or migration fails, restore from pre-upgrade snapshot; do not retry a broken migration binary on live v2. Re-schedule at `H+N` after fix.

### 10.4 Internal → public open gate

All must pass before inviting external validators (from [design spec §5.1](./specs/2026-05-29-phase-9-testnet-v2-genesis-rehearsal-design.md)):

| # | Gate | Verification |
|---|------|--------------|
| 1 | Gentx ceremony complete | All founders verify SHA256; blocks producing |
| 2 | Cosmovisor upgrade success | Zero downtime; height advances post-halt |
| 3 | Crisis invariants post-upgrade | `make testnet-v2-verify` — all five routes |
| 4 | Load SLO met | Calibrated bar in [`docs/load-test.md`](./load-test.md) § Phase 9 SLO |
| 5 | 2-week stability | Uptime ≥99%, fee reconciliation, feeds green |
| 6 | Oracle feeds post-upgrade | Sidecars submitting within 2 vote windows |

### 10.5 Public v2 open

After the internal gate:

1. **Publish endpoints** — public RPC (`26657`), LCD (`1317`), gRPC (`9090`), seeds, `GENESIS_URL`, `GENESIS_SHA256` (update `infra/testnet/node-kit/env.example` and comms).
2. **Invite validators** — point to [`docs/validator-onboarding.md`](./validator-onboarding.md) v2 paths (post-open uses `MsgCreateValidator` like v1).
3. **Enable faucet** (if not already) for external joiners.
4. **Open monitoring** — Grafana/Tenderduty with `vertix-testnet-2` labels.
5. **Chain Registry** — open draft PR from `infra/chain-registry/testnet/` against `cosmos/chain-registry` (draft only in Phase 9; merge is Phase 10).
6. **Wallets** — test Keplr + Leap `suggestChain` against live v2 RPC.

**Phase 9 completion gate (public):** ≥3 external validators in active set; registry PR drafted; runbook + onboarding updated; upgrade path documented for reuse at mainnet.

---

## Appendix — gentx ceremony (Step 1 procedure rehearsal)

Phase 8 (`vertix-testnet-1`) uses **founder-launch + post-genesis join** (`MsgCreateValidator`). Phase 9 Step 1 rehearses the **coordinated pre-launch gentx ceremony** that mainnet will use.

**Do not run this procedure on live `vertix-testnet-1`.**

Coordinator workflow and Makefile targets: **§10.2** above. Automation lives under [`scripts/testnet/v2/`](../scripts/testnet/v2/):

| Script | Role |
|--------|------|
| `build-genesis-base.sh` | Base genesis (no gentxs) |
| `build-genesis-internal.sh` | Founder gentxs for internal rehearsal |
| `collect-gentxs.sh` | Merge submitted gentxs → final genesis + SHA256 |
| `prepare-compose-gen.sh` | Wire genesis into v2 Compose |
| `upgrade-test.sh` | Cosmovisor E2E |
| `integration-verify.sh` | Crisis invariants + liveness |
| `load-test.sh` | Module-realistic tx flood |

**Participant quick reference** (before coordinated start):

```bash
vertixd init "$MONIKER" --chain-id vertix-testnet-2 --home "$VERTIXD_HOME"

vertixd genesis gentx "$MONIKER" <self-delegation>uvtx \
  --chain-id vertix-testnet-2 \
  --home "$VERTIXD_HOME" \
  --moniker "$MONIKER" \
  --commission-rate 0.10 \
  --commission-max-rate 0.20 \
  --commission-max-change-rate 0.01 \
  --min-self-delegation 1 \
  --output-document "gentx-$MONIKER.json"

# Coordinator only:
./scripts/testnet/v2/collect-gentxs.sh infra/testnet/v2/gentxs
vertixd genesis validate-genesis infra/testnet/v2/genesis/genesis.json
sha256sum infra/testnet/v2/genesis/genesis.json > infra/testnet/v2/genesis/genesis.sha256

# All participants before start:
echo "<hash>  genesis.json" | sha256sum -c -
```

Step 2 (stretch): mainnet allocation dry run — see [Phase 9 design spec](./specs/2026-05-29-phase-9-testnet-v2-genesis-rehearsal-design.md) §4.6.
