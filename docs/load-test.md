# Load test baseline (Phase 7)

Vertix Phase 7 establishes a **documented, operator-run throughput baseline** against the Phase 6 three-validator Docker devnet. This is descriptive infrastructure for Phase 9 comparison — **not** a CI gate and **not** a pass/fail TPS threshold (per [`specs/2026-05-29-phase-7-security-hardening-design.md`](./specs/2026-05-29-phase-7-security-hardening-design.md) §8 and `full-design-spec.md` Appendix C).

Harness: [`infra/loadtest/`](../infra/loadtest/) + `make load-test`.

---

## Method

| Item | Value |
|------|--------|
| Driver | [`tm-load-test`](https://github.com/informalsystems/tm-load-test) (standalone mode) |
| Entrypoint | `make load-test` from repo root |
| RPC target | `ws://localhost:26657/websocket` (`vertix-val1`, host-mapped) |
| Broadcast mode | `sync` (waits for `CheckTx` response) |
| Default load | 4 connections, 200 tx/s target rate, 60 s duration, 250-byte payloads |
| Config reference | [`infra/loadtest/loadtest.toml`](../infra/loadtest/loadtest.toml) |

**Operator responsibility:** baseline numbers are captured on real hardware after `make localnet-up`. They are **not** generated or enforced in CI for Phase 7.

**Tooling caveat:** `tm-load-test` ships with a **kvstore** client, not Vertix/Cosmos SDK transactions. The default run measures RPC submission and CometBFT handling of non-application-valid payloads unless a custom client is added. Record that context when publishing numbers; module-realistic mixes are a Phase 9 item.

---

## Topology

Aligned with [`docs/devnet.md`](./devnet.md):

| Component | Count | Notes |
|-----------|-------|--------|
| `vertixd` validators | 3 | `vertix-val1` … `vertix-val3`, peering on Docker `devnet` network |
| `vertix-feeder` | 3 | One per validator (not load-targeted by default harness) |
| Load RPC | 1 | Host `localhost:26657` → `vertix-val1` |
| Chain ID | `vertix-devnet-1` | |
| IBC / gaia / Hermes | Optional | Not required for the default `make load-test` RPC harness |

```
  [tm-load-test on host]
           │ ws://localhost:26657/websocket
           ▼
     vertix-val1 ──► vertix-val2 ◄──► vertix-val3
```

---

## Transaction mix (default harness)

| Mix | Share | Description |
|-----|-------|-------------|
| kvstore-style payloads | 100% (default tool) | Fixed-size writes via built-in `tm-load-test` client — **not** Vertix module messages |
| Vertix module txs | 0% | Requires custom `tm-load-test` client or `vertixd tx` scripting (future) |

Document any custom mix in the baseline table when you replace the default harness.

---

## Hardware / environment (fill on capture)

Record the machine used for each baseline refresh:

| Field | Value |
|-------|--------|
| Date (UTC) | _not yet captured_ |
| Host OS / kernel | _e.g. Linux 6.x_ |
| CPU model / cores | _operator fills_ |
| RAM | _operator fills_ |
| Disk (if relevant) | _operator fills_ |
| Docker / Compose version | _operator fills_ |
| `tm-load-test` version | `tm-load-test --version` or module pseudo-version from `go install` |

---

## Measured baseline

> **Status:** placeholder — devnet was not running during harness authoring. **Do not treat the table below as measured metrics.**

Run on your machine:

```bash
make localnet-up
# wait for block height > 0
make load-test
```

Then paste observed values here and in your change log / Phase 9 notes.

| Metric | Placeholder | Captured value |
|--------|-------------|----------------|
| Sustained TPS (achieved) | _run to refresh_ | |
| Target rate (`LOADTEST_RATE`) | 200 tx/s | |
| p50 broadcast → commit latency | _run to refresh_ | |
| p99 broadcast → commit latency | _run to refresh_ | |
| Observed mean block time | _query `/status` or Prometheus :26660_ | |
| Error / reject rate | _run to refresh_ | |

Example queries after the run:

```bash
curl -s http://localhost:26657/status | jq '.result.sync_info | {height: .latest_block_height, time: .latest_block_time}'
# Optional: tm-load-test --stats-output infra/loadtest/results/stats.csv
```

---

## Phase 7 policy (no threshold)

- **No pass/fail TPS or latency threshold** is defined in Phase 7.
- Numbers are **hardware- and topology-dependent**; use them for regression comparison on the **same** setup, not as absolute mainnet SLOs.
- The hard TPS/SLO target remains a **Phase 9** open item (`full-design-spec.md` Appendix C).

---

## How to re-run

1. Start (or reset) devnet: `make localnet-up` or `make localnet-reset`.
2. Confirm RPC: `curl -sf http://localhost:26657/status`.
3. Install tool (once): `go install github.com/informalsystems/tm-load-test/cmd/tm-load-test@latest`.
4. Run: `make load-test` (override `LOADTEST_*` as needed — see [`infra/loadtest/README.md`](../infra/loadtest/README.md)).
5. Update the **Measured baseline** and **Hardware / environment** sections above with real output.
6. Stop stack when finished: `make localnet-down`.

For Phase 9, re-run with the same parameter file and document any change in validator count, block size, or transaction mix.

---

## Related docs

- Devnet runbook: [`docs/devnet.md`](./devnet.md)
- Harness README: [`infra/loadtest/README.md`](../infra/loadtest/README.md)
- Phase 7 plan: [`docs/plans/2026-05-29-phase-7-security-hardening.md`](./plans/2026-05-29-phase-7-security-hardening.md) (Task 14)
