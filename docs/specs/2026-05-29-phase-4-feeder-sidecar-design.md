# Phase 4 — `vertix-feeder` Sidecar — Design Spec

**Date:** 2026-05-29
**Status:** Approved (brainstorm output)
**Phase:** 4 of the canonical phase map ([`full-design-spec.md`](../full-design-spec.md) §7.2)
**Depends on:** Phase 1 (`x/oracle`) — `MsgSubmitFeed` wire format, feeder-delegation model, `BASE:QUOTE` pair convention, `VoteWindow`/`MaxPriceAge` params · **Enables:** Phase 6 (live per-validator feeds on the devnet stack)
**Owns:** the off-chain `vertix-feeder` Go binary — price fetching, cross-source median + quality filtering, per-window batched `MsgSubmitFeed` broadcast signed by the delegated feeder key, Prometheus metrics, and the operator-facing YAML config.

> This is the per-phase design spec produced by the brainstorm of Phase 4. It refines — and must stay consistent with — the program source of truth ([`full-design-spec.md`](../full-design-spec.md) Phase 4) and the engineering reference ([`technical-design.md`](../technical-design.md) §5). Where this doc and the program spec disagree on phase order or cross-phase contracts, the program spec wins. Module/binary internals defer to `technical-design.md`, **except** where this doc explicitly refines it (flagged as doc-sync items in §12 — notably the §5.2 config surface and the hybrid loop model). The step-by-step execution plan lives in `docs/plans/` (produced after this spec).
>
> **Canonical phase note.** Per [`full-design-spec.md`](../full-design-spec.md) §7.2, "Phase 4" is the `vertix-feeder` sidecar. The `roadmap.md` timeline numbers IBC as its "Phase 4"; the program spec's canonical ordering supersedes the roadmap numbering, and this spec uses it.

---

## 1. Goal & Deliverable Boundary

Deliver a working, production-shaped `vertix-feeder` binary that every active validator runs to keep the oracle data loop live. On a steady cadence it fetches external prices for each accepted pair, computes a **cross-source median** with strict data-quality guards, and broadcasts a **single batched `MsgSubmitFeed` transaction once per `VoteWindow`** — signed by the **delegated feeder key only** (never the operator key) — against the validator's local `vertixd` node. It is resilient (never crashes on transient errors), observable (Prometheus on `:9200`), and honors the Phase 1 `MsgSubmitFeed` wire format and `BASE:QUOTE` convention exactly.

This completes the oracle loop: external APIs → `vertix-feeder` → `MsgSubmitFeed` → on-chain stake-weighted median aggregation (Phase 1).

**In scope:**

- `feeder/` package + `cmd/vertix-feeder` Cobra binary, in the **same Go module** as the chain (`github.com/vertix-network/vertix`) so it imports `x/oracle/types` and the app codec directly — no duplicate proto generation.
- `PriceProvider` abstraction with three adapters: **CoinGecko**, **Binance**, and **Static** (bootstrap/manual).
- Cross-source median + data-quality guards (`min_providers`, `max_deviation`, `max_quote_age`).
- SDK-based broadcaster (Approach A): `client.Context` + `tx.Factory` + generated gRPC `tx`/`auth` clients; keyring signing; local sequence tracking with chain re-sync.
- YAML config + startup validation; `feeder-config.example.yaml`.
- Prometheus metrics surface (`technical-design.md` §5.4) on `:9200`.
- `make feeder-build` target; unit + integration tests (mock providers/clients, no live network in CI).

**Out of scope (deferred):**

- On-chain aggregation, TWAP, and slashing — owned by Phase 1 (`x/oracle`, already implemented). The feeder is a pure off-chain producer of `MsgSubmitFeed`.
- Operator feeder-delegation UX — operators delegate with the **existing** chain CLI `vertixd tx oracle set-feeder` (the `x/oracle` module already ships `MsgSetFeeder` + autocli). The feeder binary does not re-implement it.
- Multi-validator deployment, docker-compose, per-validator wiring — Phase 6 (devnet).
- A real VTX market price source — VTX is unlisted pre-launch; the `static` provider is the documented bootstrap placeholder, replaced post-listing by a real market adapter (e.g. an Osmosis VTX pool / DEX TWAP) as a future `PriceProvider`.

---

## 2. Locked Decisions (from this brainstorm)

| # | Decision | Choice | Rationale |
|---|---|---|---|
| D1 | **Loop model** | **Hybrid.** A **price loop** ticks every `feed_interval` (5s), fetches + quality-filters + caches the median per pair; an independent **submit loop** polls block height and broadcasts the cached medians **exactly once per `VoteWindow`**. | A pure ticker (spec default) is blind to windows — it over-submits (wasted gas + sequence churn) or, on slow blocks, drifts out of a window into a slashable miss. A fully block-event-driven loop couples the whole binary to a stream. The hybrid separates "fetch cadence" from "submit correctness": slow/flaky REST providers never affect submission timing, and submission is exactly-once-per-window. **Trade-off accepted:** slightly more code (two loops + shared cache) than the single-loop sketch in `technical-design.md` §5.1 — doc-sync §12. |
| D2 | **VTX:USD source** | **`static` provider**, implemented and wired to `VTX:USD` in the example config as a **documented bootstrap placeholder**. | VTX has no external market pre-mainnet, yet `VTX:USD` is first in the on-chain `accept_list`. A configurable static price lets the pair reach quorum so the full loop is exercisable end-to-end on devnet (Phase 6), and the same provider is invaluable for deterministic tests. The real source later is just another `PriceProvider` adapter — no architectural change. |
| D3 | **Data-quality policy** | **Configurable, strict defaults.** `min_providers` (default 2), `max_deviation` from the cross-source median (default `0.10`, drop deviating sources before the final median), `max_quote_age` (drop stale quotes). If `< min_providers` healthy sources remain after filtering, **skip the pair this window** (do not submit a low-confidence price); record `feeds_failed_total{reason}`. | The chain punishes both misses (0.5%) and outliers (1.0%); the feeder must avoid submitting junk *and* avoid needless skips. Hard-coded guards are too rigid (devnet may run one source / `static`); best-effort is unsafe. Tunable knobs with safe defaults resolve [`full-design-spec.md`](../full-design-spec.md) Appendix C "provider set + weighting". |
| D4 | **Tx shape** | **One batched tx per window.** Pack all healthy pairs' `MsgSubmitFeed` into a single tx, signed once, broadcast `BROADCAST_MODE_SYNC` via gRPC. | One signature, one sequence increment, atomic. Cosmos txs natively allow multiple messages. Cheaper and simpler sequence handling than one-tx-per-pair (which races on sequence and multiplies signatures). |
| D5 | **Signing & sequence** | **Cosmos SDK keyring** (`test`/`file`/`os`), load feeder key by `key_name`; query account number once at startup, **track sequence locally**, re-sync from chain on a sequence-mismatch error. | Standard, integrates with `vertixd keys`, signs exactly as the chain expects. Local sequence avoids a per-window account query; re-sync on mismatch self-heals after node restarts / out-of-band txs. |
| D6 | **Resilience posture** | **Resilient & self-healing.** Never crash on transient errors: per-provider timeouts, bounded retry-with-backoff on broadcast within the window, auto-reconnect to the node, graceful per-pair/per-provider degradation, everything surfaced via metrics + structured logs. Crash (exit non-zero) **only** on fatal startup misconfig. | The feeder runs unattended beside a validator; a crash on a transient provider/node blip would cause avoidable misses. Fail-fast is left to a process supervisor only for truly fatal startup conditions. |
| D7 | **Chain client** | **Approach A — Cosmos SDK `client` primitives.** `client.Context` + `tx.Factory` for build/sign; SDK keyring; generated gRPC `tx.ServiceClient` (broadcast) + `auth` query client (account/sequence). Import `oracletypes.MsgSubmitFeed` directly. | Canonical, matches `vertixd` signing (same codec, sign mode, gas), no third-party wrapper, full control over batching/retry, easily testable with mocked clients. Rejected: Ignite `cosmosclient` (heavy dep, less control) and hand-rolled `TxRaw` (re-implements consensus-sensitive signing — most risk). |
| D8 | **Delegated key only** | Sign `MsgSubmitFeed` with the **feeder** key; the `MsgSubmitFeed.validator` field carries the **valoper**; the feeder account must equal the on-chain delegation (or the operator's own acc address if none). Never sign with the operator key. | Phase 1 security contract (spec D10/C1): the feeder key is low-value and rotatable; a compromise is bounded by outlier slashing and touches no funds. |
| D9 | **Binary creation** | Hand-written Go under `feeder/` in the existing module (no Ignite scaffold — the feeder is not a chain module). Reuses the chain's codec, `oracletypes`, and Bech32 config (`vtx`/`vtxvaloper`). | Consistent with `project-structure.md` §5 ("single Go module for chain + feeder"); avoids duplicate proto generation and import cycles. |

These inherit and must not contradict the program-level locked decisions in [`full-design-spec.md`](../full-design-spec.md) §§1–6, nor the architectural invariants §5. The feeder is **off-chain** and by Invariant 5 never influences on-chain aggregation/slashing math — it only produces `MsgSubmitFeed` txs the chain independently validates and aggregates.

---

## 3. Target Layout (Phase 4 outputs)

Realizes `project-structure.md` §5. The main loop is split into focused files (refinement vs. the single-`feeder.go` sketch — D1):

```
feeder/
├── cmd/
│   └── vertix-feeder/
│       └── main.go              Cobra root: flags (--config, --log-level), config load, lifecycle, signal handling
├── feeder/
│   ├── config.go                YAML Config struct, Validate(), loader
│   ├── feeder.go                Orchestrator: constructs providers/cache/broadcaster, starts both loops, graceful shutdown
│   ├── priceloop.go             Price loop: tick → fetch all pairs → quality filter → median → cache
│   ├── submitloop.go            Submit loop: poll height → new-window detection → build batch → broadcast → retry
│   ├── broadcaster.go           SDK client.Context + tx.Factory; build/sign/broadcast; account & sequence mgmt
│   ├── aggregate.go             Cross-source median + max_deviation outlier drop + staleness filter
│   ├── cache.go                 Concurrency-safe latest-median-per-pair store with timestamps + health flag
│   ├── metrics.go               Prometheus registry + counters/gauges/histograms; :9200 HTTP server
│   ├── provider/
│   │   ├── provider.go          PriceProvider interface + name→constructor registry
│   │   ├── coingecko.go         CoinGecko REST adapter
│   │   ├── binance.go           Binance spot REST adapter
│   │   ├── static.go            Static/manual provider (VTX:USD bootstrap + deterministic tests)
│   │   └── provider_test.go     Mock (httptest) unit tests
│   ├── aggregate_test.go        median / deviation drop / staleness
│   ├── config_test.go           load + Validate (positive + negative)
│   └── feeder_test.go           Integration: mock providers → median → mock broadcaster → asserted MsgSubmitFeed batch
└── feeder-config.example.yaml   Documented example configuration
```

---

## 4. Chain Client (Approach A, D7)

The broadcaster wraps the SDK client stack and is the only component that talks to the node:

- **Codec / interfaces:** reuse the app's `encodingConfig` (interface registry + codec) so `MsgSubmitFeed` is registered identically to `vertixd`.
- **Keyring:** `keyring.New(...)` with the configured backend/dir; resolve `key_name` → feeder record + acc address.
- **Account/sequence:** at startup query `auth.QueryClient.Account` (via gRPC) for the feeder account's number + sequence; cache number, track sequence locally; on a broadcast error indicating sequence mismatch, re-query and retry once (D5).
- **Build/sign/broadcast:** `tx.Factory` configured with chain_id, account number/sequence, gas (`auto` → simulate × `gas_adjustment`, or fixed), fees; `tx.BuildUnsignedTx` → `tx.Sign` → `tx.ServiceClient.BroadcastTx` in `SYNC` mode; inspect `TxResponse.Code` (0 = accepted into mempool).

**Interface seam for tests** — the submit loop depends on a small interface, not the concrete gRPC client, so `feeder_test.go` injects a mock:

```go
type Broadcaster interface {
    // SubmitFeeds builds, signs, and broadcasts one tx containing a MsgSubmitFeed per entry.
    SubmitFeeds(ctx context.Context, feeds []FeedSubmission) (txhash string, err error)
}

type FeedSubmission struct {
    Pair  string          // "BASE:QUOTE"
    Price math.LegacyDec
}
```

The concrete implementation fills `MsgSubmitFeed{Feeder: <feeder acc>, Validator: <cfg.validator valoper>, Pair, Price: price.String()}` and runs `ValidateBasic` before signing.

---

## 5. Configuration (`feeder-config.example.yaml`)

Extends `technical-design.md` §5.2 with the knobs from D1/D3/D4/D5/D6 (doc-sync §12):

```yaml
chain_id: vertix-devnet-1
node_grpc: localhost:9090          # gRPC: account/sequence queries + tx broadcast (Approach A)
validator: vtxvaloper1...          # valoper this feeder submits for (-> MsgSubmitFeed.validator)
key_name: feeder                   # feeder key in the keyring (signs the tx; never the operator key)
keyring_backend: test              # test | file | os
keyring_dir: ~/.vertix

fees: 2000uvtx                     # flat fee per window tx (chain min_gas_prices = 0.025uvtx)
gas: auto                          # "auto" (simulate + adjust) or a fixed integer
gas_adjustment: 1.3

feed_interval: 5s                  # price-loop tick
submit_poll_interval: 1s           # submit-loop height poll
vote_window: 0                     # 0 = query from chain oracle params at startup; >0 overrides
broadcast_retries: 3               # bounded retry-with-backoff within the window

quality:
  min_providers: 2
  max_deviation: "0.10"            # drop a source > 10% from the cross-source median
  max_quote_age: 30s               # drop quotes older than this

providers: ["coingecko", "binance", "static"]
static_prices:                     # consumed by the static provider
  "VTX:USD": "0.10"                # BOOTSTRAP PLACEHOLDER — replace with a real market source once VTX lists

pairs:
  - pair: "VTX:USD"
    symbols: { static: "VTX:USD" }
  - pair: "BTC:USD"
    symbols: { coingecko: "bitcoin",  binance: "BTCUSDT" }
  - pair: "ETH:USD"
    symbols: { coingecko: "ethereum", binance: "ETHUSDT" }
  - pair: "ATOM:USD"
    symbols: { coingecko: "cosmos",   binance: "ATOMUSDT" }
  - pair: "USDC:USD"
    symbols: { coingecko: "usd-coin", binance: "USDCUSDT" }

prometheus: { enabled: true, port: 9200 }
log_level: info
```

`Config.Validate()` (fatal at startup, D6):

- `chain_id`, `node_grpc`, `key_name` non-empty; `validator` parses as a `vtxvaloper` address.
- every `pairs[].pair` matches `^[A-Z0-9]+:[A-Z0-9]+$` and has a `symbols` entry for at least one **enabled** provider.
- `quality.min_providers ≥ 1`; `max_deviation` parses as a `LegacyDec` in `(0, 1]`; `max_quote_age`, `feed_interval`, `submit_poll_interval` are positive durations.
- if the `static` provider is enabled and any pair maps to it, `static_prices` has a parseable positive price for that pair.
- `fees` parses as `sdk.Coins`; `gas` is `auto` or a positive integer; `gas_adjustment > 1` when `gas: auto`.

> **Identity note.** `validator` is the **valoper** (goes in the message body). The **signing identity** is the **feeder key** resolved from the keyring; its account address must equal the on-chain feeder delegation for `validator` (set out-of-band via `vertixd tx oracle set-feeder`), or — if no delegation exists — the operator's own account address (the chain's `ResolveAuthorizedFeeder` default). A mismatch yields on-chain `ErrFeederNotAuthorized`; the feeder logs the rejected tx and the operator fixes the delegation.

---

## 6. Data Flow

```
price loop  (every feed_interval)
  for pair in cfg.pairs:
     quotes = []
     for prov in providers supporting pair:
        q, latency = prov.Fetch(ctx_with_timeout, symbol)   # error → provider_errors_total, skip source
        record provider_latency_ms{provider,pair}
        if q ok && age(q) <= max_quote_age: quotes.append(q)
     filtered = dropDeviating(quotes, max_deviation)         # drop sources > max_deviation from median(quotes)
     if len(filtered) < min_providers:
        mark cache[pair].healthy = false                     # feeds_failed_total{pair,reason="insufficient_sources"}
     else:
        cache[pair] = { price: median(filtered), at: now, healthy: true }

submit loop (every submit_poll_interval)
  h = latestHeight(node)                                     # gRPC; node unreachable → log, retry next poll
  w = h / VoteWindow                                         # VoteWindow from chain params at startup (or cfg override)
  if w == lastSubmittedWindow: continue
  feeds = [ {pair, price} for pair in cache if healthy && fresh(at, max_quote_age) ]
  if feeds empty: continue                                   # nothing confident to submit this window
  hash, err = broadcaster.SubmitFeeds(ctx, feeds)            # one batched, signed tx (SYNC)
  if err (incl. retries exhausted): log; continue            # next poll re-attempts within the window
  lastSubmittedWindow = w
  for f in feeds: feeds_submitted_total{f.pair}++ ; last_submit_timestamp{f.pair}=now
```

The two loops communicate only through the concurrency-safe `cache` (D1). The price loop never blocks submission; the submit loop submits whatever fresh, healthy medians exist at window boundary. `VoteWindow` is read from the chain's oracle params at startup so the feeder stays correct if governance changes it; `vote_window > 0` in config forces an override (useful for tests).

---

## 7. Provider Interface & Adapters

Exactly the `technical-design.md` §5.3 contract:

```go
type PriceProvider interface {
    Name() string
    Fetch(ctx context.Context, symbol string) (math.LegacyDec, error)
}
```

- **CoinGecko / Binance:** REST adapters over a context-timeout'd `http.Client`; map `symbol` → endpoint, parse JSON → `LegacyDec`; return a typed error on HTTP/parse failure (counted, never fatal).
- **Static:** returns configured `static_prices[symbol]`; powers `VTX:USD` bootstrap and deterministic tests.
- A registry maps enabled provider names → constructors; unknown name → fatal startup error. The future real-VTX source is added here as one more adapter with no other change.

**Cross-source median (`aggregate.go`).** Collect healthy, fresh quotes; if `≥ min_providers`, drop those deviating `> max_deviation` from the provisional median, then take the median of the survivors (even count → average of the two middle values, in `LegacyDec`). Deterministic, no external state.

---

## 8. Metrics (Prometheus, `:9200/metrics`)

Exactly `technical-design.md` §5.4:

| Metric | Type | Labels |
|---|---|---|
| `vertix_feeder_feeds_submitted_total` | Counter | `pair` |
| `vertix_feeder_feeds_failed_total` | Counter | `pair`, `reason` |
| `vertix_feeder_provider_latency_ms` | Histogram | `provider`, `pair` |
| `vertix_feeder_provider_errors_total` | Counter | `provider` |
| `vertix_feeder_last_submit_timestamp` | Gauge | `pair` |

A missed window surfaces as `last_submit_timestamp{pair}` failing to advance; operators alert on that (the slashing itself is on-chain, Phase 1).

---

## 9. Error Handling & Resilience (D6)

- **Startup (fatal, exit non-zero):** invalid config, missing keyring/key, cannot dial the node, `chain_id` mismatch with the node, unknown provider name.
- **Runtime (never crash):**
  - provider error/timeout → log + `provider_errors_total{provider}`, skip that source.
  - pair has `< min_providers` healthy sources → keep last cache, mark unhealthy, `feeds_failed_total{pair,reason="insufficient_sources"}`; do not submit it this window.
  - node unreachable in submit loop → log, auto-retry next poll (auto-reconnect); no crash.
  - broadcast failure → bounded retry-with-backoff within the window (`broadcast_retries`).
  - sequence-mismatch error → re-query account sequence and retry once (D5).
- **Shutdown:** SIGINT/SIGTERM → cancel context, stop loops, close the metrics server cleanly.

All retries are bounded and never block past the current window; correctness of "submit once per window" is preserved by `lastSubmittedWindow`.

---

## 10. Build, CLI, Logging

- **Binary:** `cmd/vertix-feeder/main.go` Cobra root with `--config` (path, required) and `--log-level` flags; reads YAML, validates, constructs the feeder, runs until signal.
- **Makefile:** add `feeder-build` → `go build -o build/vertix-feeder ./feeder/cmd/vertix-feeder` (mirrors `AGENTS.md` §8). `make build`/`make test`/`make lint` continue to cover the whole module (the feeder is in-module).
- **Logging:** structured logger (the SDK's `cosmossdk.io/log` for consistency), level from config/flag.

---

## 11. Acceptance Gate Mapping

| Spec gate ([`full-design-spec.md`](../full-design-spec.md) Phase 4) | How satisfied |
|---|---|
| Fetch from ≥2 providers, compute median | `provider_test.go` (httptest-mocked CoinGecko + Binance); `aggregate_test.go` (median + deviation drop + staleness) |
| Sign + broadcast each window against a local node | `feeder_test.go` integration with a mock `Broadcaster` asserting the per-window batched `MsgSubmitFeed` set; `broadcaster.go` builds real SDK txs and runs `ValidateBasic` (verified against imported `oracletypes`) |
| Metrics exposed | metrics unit test scrapes `:9200/metrics` and asserts the five series |
| Missing/lagging providers handled gracefully | quality-filter + unhealthy-skip tests; per-provider timeout test; node-unreachable retry test |
| Honors `MsgSubmitFeed` format + `BASE:QUOTE` convention | imports `x/oracle/types` directly; round-trip `ValidateBasic` on built messages; pair-regex validation in config |
| Inherited Phase 0 gate (`make lint && make test && make build`) | CI stays green with the new in-module package + `make feeder-build` |

Tests use mock providers and a mock broadcaster — **no live network in CI** (coding-standards).

---

## 12. Cross-Phase Contracts Honored + Doc-Syncs

**Cross-phase contracts honored:**

- **Phase 1 `MsgSubmitFeed` wire format** (consumed here): `feeder`, `validator`, `pair` (`"BASE:QUOTE"`), `price` (`LegacyDec` string) — built from imported `oracletypes`, never redefined.
- **`BASE:QUOTE` pair convention:** shared by config, providers, and the message body; validated by the same regex the chain uses.
- **Feeder-delegation security contract** (Phase 1 D10/C1): sign with the delegated feeder key only (D8); delegation is established out-of-band via `vertixd tx oracle set-feeder`.
- **Operational contract** ([`full-design-spec.md`](../full-design-spec.md) Phase 4): one feeder per validator; submission cadence fits within `VoteWindow` (D1).
- **Inherited Phase 0 gates:** `make lint && make test && make build` bar; the feeder lives in the same Go module/CI.

**Doc-syncs this phase forces (land alongside this spec / its plan):**

1. **`technical-design.md` §5.1** — the single-loop "ticker → fetch → median → broadcast" sketch is refined to the **hybrid two-loop** model (price loop + window-gated submit loop) of §6 here, to satisfy the once-per-window submission contract and decouple provider latency from submission timing (D1).
2. **`technical-design.md` §5.2** — the example config gains the `quality` block (`min_providers`/`max_deviation`/`max_quote_age`), `static_prices`, `submit_poll_interval`, `vote_window`, `broadcast_retries`, and `fees`/`gas` fields (D3/D4/D5). The config field `validator_key`/`key_name` pair is clarified: `validator` (valoper, message body) vs `key_name` (feeder signing key).
3. **[`full-design-spec.md`](../full-design-spec.md) Appendix C "Provider set + weighting"** — resolved here by D2 (CoinGecko + Binance + Static; static for VTX:USD bootstrap) and D3 (configurable strict quality guards). Appendix C entry can be marked resolved by Phase 4.

---

## 13. Risks & Open Items

| Item | Risk | Mitigation |
|---|---|---|
| Static VTX:USD price drifts from any future real value | Misleading on-chain VTX price on devnet/testnet | Documented as a bootstrap placeholder (D2); replaced by a real market `PriceProvider` once VTX lists; devnet-only impact |
| Provider API shape/rate-limit changes | Fetch failures, fewer healthy sources | Per-provider isolation + metrics; `min_providers`/skip logic prevents bad submissions; adapters easy to patch |
| Feeder key not delegated / wrong account | Every submission rejected (`ErrFeederNotAuthorized`) | Identity note §5; rejected-tx logging; operator runs `set-feeder`; documented in config comments |
| Slow blocks vs `feed_interval` | Cache staleness near a window boundary | `max_quote_age` freshness check before submit; window-gated submit loop independent of fetch cadence |
| Local sequence drift after node restart / out-of-band tx | Broadcast rejected on sequence mismatch | Re-query account sequence and retry once (D5) |
| Over-aggressive `max_deviation` | Healthy sources dropped → pair skipped → misses | Tunable (D3); default `0.10` is generous; metrics expose skip reasons |
| `EndBlock`/window math off-by-one | Submitting in wrong window | `VoteWindow` read from chain params; `lastSubmittedWindow` integer-division gate; unit-tested with `vote_window` override |

---

## 14. Definition of Done

- `vertix-feeder` builds via `make feeder-build`, lives in the chain's Go module, and imports `x/oracle/types` directly (no duplicate proto generation).
- The hybrid loop runs: price loop fetches + quality-filters + caches medians; submit loop broadcasts a single batched, feeder-signed `MsgSubmitFeed` tx **once per `VoteWindow`**, with bounded retry and sequence re-sync.
- Three providers implemented (CoinGecko, Binance, Static); cross-source median + `min_providers`/`max_deviation`/`max_quote_age` guards enforced; under-covered pairs skipped (not submitted).
- Prometheus metrics (`technical-design.md` §5.4) served on `:9200`; resilient posture (D6) — no crash on transient provider/node/broadcast errors; fatal only on startup misconfig.
- `feeder-config.example.yaml` documents every field including the VTX:USD bootstrap note and the `validator` (valoper) vs `key_name` (feeder) distinction.
- Unit + integration tests pass (provider mocks, median/deviation/staleness, config validate, mock-broadcaster batch assertion, metrics scrape); `make lint && make test && make build` stay green in CI.
- The `technical-design.md` doc-syncs (§12 items 1–2) are landed; Appendix C provider open-question (item 3) is marked resolved.
- Phase 4 acceptance gate (§11) satisfied — providing the live per-validator feed producer that Phase 6 (devnet) integrates.

---

## Appendix A — Document Map

| Doc | Relationship to this spec |
|---|---|
| [`full-design-spec.md`](../full-design-spec.md) | Program source of truth; Phase 4 section is the parent of this spec; Appendix C provider open-question resolved here |
| [`technical-design.md`](../technical-design.md) | Owns §5 feeder internals referenced here; §5.1 (loop model) and §5.2 (config) receive doc-sync (§12) |
| [`2026-05-29-phase-1-oracle-module-design.md`](./2026-05-29-phase-1-oracle-module-design.md) | The `MsgSubmitFeed` wire format, feeder-delegation model, `VoteWindow`/`MaxPriceAge` params, and `BASE:QUOTE` convention this feeder consumes |
| [`2026-05-29-phase-0-chain-foundation-design.md`](./2026-05-29-phase-0-chain-foundation-design.md) | The Go module, codec, Bech32 config, and CI/lint/test gates the feeder builds on |
| [`project-structure.md`](../project-structure.md) | §5 target layout for `feeder/` |
| [`coding-standards.md`](../coding-standards.md) | Lint set, test patterns (mock providers/clients, no live network), structured logging |
| `docs/plans/` | Step-by-step execution plan derived from this spec (next step) |
