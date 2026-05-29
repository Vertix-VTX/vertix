# Testnet Runbook — Public Testnet v1

**Chain:** `vertix-testnet-1` · **Related:** [Validator onboarding](./validator-onboarding.md) · [RWA quickstart](./rwa-quickstart.md) · [Bug bounty](./bug-bounty.md) · [Phase 8 plan](./plans/2026-05-29-phase-8-public-testnet.md)

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

## Appendix — gentx ceremony (deferred to Phase 9)

Phase 8 uses **founder-launch + post-genesis join** (`MsgCreateValidator`). Phase 9 rehearses the full coordinated genesis ceremony:

```bash
# On each participating validator host (before chain start):

# 1. Initialize node (if not already)
vertixd init "$MONIKER" --chain-id vertix-testnet-2

# 2. Create gentx
vertixd genesis gentx "$MONIKER" <self-delegation>uvtx \
  --chain-id vertix-testnet-2 \
  --moniker "$MONIKER" \
  --commission-rate 0.10 \
  --commission-max-rate 0.20 \
  --commission-max-change-rate 0.01 \
  --min-self-delegation 1 \
  --pubkey "$(vertixd tendermint show-validator)"

# 3. Collect gentxs (genesis coordinator only)
vertixd genesis collect-gentxs /path/to/gentxs/

# 4. Validate + publish hash
vertixd genesis validate-genesis
sha256sum genesis.json > genesis.sha256

# 5. Coordinated start — all validators verify hash and start at agreed block time
echo "<hash>  genesis.json" | sha256sum -c -
```

This appendix is the starting procedure for Phase 9 genesis rehearsal; do not run on live `vertix-testnet-1`.
