# Vertix — Implementation Difficulty Analysis

**Date:** 2026-05-29
**Status:** Analysis (informational)
**Source of truth:** [`docs/full-design-spec.md`](../full-design-spec.md) · [`docs/technical-design.md`](../technical-design.md)
**Purpose:** Rank each phase and each major feature by implementation difficulty, expose where the real risk lives, and give the team a heat map for planning, staffing, and audit focus.

> This is an **advisory** document. It does not change phase order, contracts, or invariants — those are locked by the full design spec. It only estimates *how hard* each locked item is to build correctly.

---

## 1. How difficulty is scored

Each item is scored on five dimensions (1 = trivial, 5 = very hard), then rolled up into an overall rating.

| Dimension | What it measures |
|---|---|
| **Logic** | Algorithmic / state-machine complexity of the on-chain or off-chain code |
| **Consensus risk** | Determinism, ABCI hook correctness, non-determinism / panic surface |
| **Integration** | Cross-module wiring, SDK keeper coupling, ordering dependencies |
| **External** | Reliance on things outside our code (3rd-party APIs, relayers, external validators, auditors) |
| **Testing** | Effort to prove it correct (sim, fuzz, E2E, multi-node, adversarial) |

**Overall difficulty bands:**

- 🟢 **Low** — well-trodden SDK patterns, mostly wiring + config.
- 🟡 **Medium** — custom logic, but contained within one module and testable in unit/integration.
- 🟠 **High** — custom consensus-critical logic, multi-module coupling, or determinism hazards.
- 🔴 **Very High** — adversarial / economic correctness, external coordination, or irreversible (mainnet) consequences.

---

## 2. Phase difficulty heat map

| Phase | Name | Logic | Consensus | Integration | External | Testing | **Overall** |
|---|---|:--:|:--:|:--:|:--:|:--:|:--:|
| 0 | Chain Foundation | 2 | 2 | 3 | 1 | 2 | 🟢 **Low** |
| 1 | `x/oracle` (aggregation + slashing) | 4 | 5 | 4 | 2 | 4 | 🔴 **Very High** |
| 2 | `x/fees` (burn + distribute) | 2 | 4 | 3 | 1 | 3 | 🟡 **Medium** |
| 3 | `x/rwa` (lifecycle + bonding) | 4 | 3 | 5 | 2 | 4 | 🟠 **High** |
| 4 | `vertix-feeder` sidecar | 3 | 1 | 3 | 5 | 3 | 🟠 **High** |
| 5 | IBC enablement (ICS-20) | 2 | 2 | 4 | 4 | 4 | 🟡 **Medium** |
| 6 | Devnet (multi-validator) | 2 | 2 | 5 | 4 | 4 | 🟠 **High** |
| 7 | Security hardening | 3 | 4 | 4 | 3 | 5 | 🟠 **High** |
| 8 | Public Testnet v1 | 1 | 2 | 4 | 5 | 4 | 🟠 **High** |
| 9 | Audit + Testnet v2 + genesis rehearsal | 2 | 3 | 4 | 5 | 4 | 🔴 **Very High** |
| 10 | Mainnet launch (gate) | 1 | 3 | 4 | 5 | 3 | 🔴 **Very High** |

### Difficulty profile (overall band)

```
Phase 0  🟢 Low
Phase 1  🔴 Very High   ◄── hardest build-time engineering
Phase 2  🟡 Medium
Phase 3  🟠 High
Phase 4  🟠 High
Phase 5  🟡 Medium
Phase 6  🟠 High
Phase 7  🟠 High
Phase 8  🟠 High
Phase 9  🔴 Very High   ◄── hardest coordination / irreversibility
Phase 10 🔴 Very High   ◄── one-shot, no rollback
```

**Two difficulty peaks:** Phase **1** (hardest *engineering*) and Phases **9–10** (hardest *coordination & irreversibility*). They are hard for different reasons — see §5.

---

## 3. Feature-level difficulty

### Phase 0 — Chain Foundation 🟢
| Feature | Difficulty | Why |
|---|:--:|---|
| Chain identity / genesis params | 🟢 Low | Config; values already specified in technical-design §1.2. |
| Standard module wiring | 🟢 Low | Boilerplate `app.go` wiring. |
| **Removing `x/mint` + `TestNoMintModule`** | 🟡 Medium | Easy to drop the module; the *guard test* and ensuring staking rewards still work without mint is the subtle part. |
| Makefile / CI / lint / proto tooling | 🟢 Low | Standard Ignite/Cosmos tooling. |

### Phase 1 — `x/oracle` 🔴 (highest engineering risk)
| Feature | Difficulty | Why |
|---|:--:|---|
| `MsgSubmitFeed` + params + genesis | 🟡 Medium | Standard proto/keeper plumbing. |
| **Stake-weighted median aggregation** | 🟠 High | Must be deterministic across all validators; tie-breaking, LegacyDec precision, sort stability all matter. A subtle bug halts consensus. |
| **`EndBlock` vote-window algorithm** | 🔴 Very High | Consensus-critical. Bounded iteration only, no panics, exact window math, feed clearing. Any non-determinism = chain halt. |
| TWAP histories (1h/24h) | 🟠 High | Time-windowed store with big-endian keyed pruning; off-by-one on expiry corrupts price history. |
| **Miss/outlier slashing** | 🔴 Very High | Touches validator funds. Threshold math, outlier statistical test (still an *open question*, App. C), and avoiding accidental mass-jailing. |
| `OracleKeeper` interface (for `x/rwa`) | 🟡 Medium | Small surface, but it is a frozen cross-phase contract. |

### Phase 2 — `x/fees` 🟡
| Feature | Difficulty | Why |
|---|:--:|---|
| Fee-collector sweep at `EndBlock` | 🟡 Medium | Simple loop, but consensus-critical and must never panic on empty/odd balances. |
| Burn via `x/bank.BurnCoins` | 🟡 Medium | Directly affects total supply — must be exact and irreversible-safe. |
| Distribute via `x/distribution` FeePool | 🟡 Medium | Correct keeper call + rounding (`distr = total − burn` to avoid dust loss). |
| `BurnRatio + DistributionRatio == 1` validation | 🟢 Low | Pure param validation. |

### Phase 3 — `x/rwa` 🟠 (most cross-module coupling)
| Feature | Difficulty | Why |
|---|:--:|---|
| Lifecycle state machine (DRAFT→ATTESTED→ACTIVE→SETTLED) | 🟠 High | Many guarded transitions; every negative path needs a test. |
| **Issuer bond escrow** | 🟠 High | Locks user funds; release-vs-slash correctness; invariant-backed. |
| Oracle-price attestation | 🟡 Medium | Consumes `OracleKeeper.GetPrice`; must handle stale/missing price. |
| `rwa/{asset-id}` factory-denom mint/burn | 🟡 Medium | Standard `x/bank` mint, but namespace discipline matters for IBC. |
| **Transfer restrictions + ante decorator** | 🔴 Very High | Must block bypass via plain `x/bank.MsgSend`; ante-handler logic is easy to get subtly wrong and security-sensitive. |
| Mint/settle fee routing | 🟡 Medium | Floor math + atomic debit into fee collector. |

### Phase 4 — `vertix-feeder` 🟠 (off-chain, external-dependency heavy)
| Feature | Difficulty | Why |
|---|:--:|---|
| Provider abstraction (CoinGecko/Binance/…) | 🟠 High | Each external API differs; rate limits, schema drift, outages (open question App. C). |
| Cross-source median | 🟡 Medium | Simple math; the hard part is handling missing/lagging sources. |
| Tx signing + broadcast each window | 🟠 High | Hot keys, sequence/nonce handling, retry without double-submit, fit inside `VoteWindow`. |
| Prometheus metrics | 🟢 Low | Standard instrumentation. |
| *Not consensus-critical* | — | A feeder bug slashes one validator, it does not halt the chain — lowers blast radius vs Phase 1. |

### Phase 5 — IBC Enablement 🟡
| Feature | Difficulty | Why |
|---|:--:|---|
| Confirm `x/ibc` + `x/transfer` wiring | 🟢 Low | Mostly already scaffolded in Phase 0. |
| **`rwa/*` portability w/o losing restriction semantics** | 🟠 High | The tricky bit: restriction is enforced at source chain; needs explicit design + tests. |
| `interchaintest` ICS-20 suite | 🟠 High | Spinning two chains + relayer in CI is environment-heavy and flaky-prone. |
| Hermes / `rly` relayer recipes | 🟡 Medium | Config + docs; external tool behavior. |

### Phase 6 — Devnet 🟠
| Feature | Difficulty | Why |
|---|:--:|---|
| 3-validator docker-compose stack | 🟡 Medium | Compose + genesis orchestration. |
| Per-validator feeders | 🟠 High | First time all moving parts run together; timing/keys/network. |
| **End-to-end integration (oracle→rwa→fees→IBC)** | 🟠 High | Exercises *every* cross-phase contract simultaneously — first real integration truth. |
| Explorer + faucet + reset/seed | 🟡 Medium | Tooling integration. |
| 1-week stability bar | 🟠 High | Latent non-determinism / memory leaks only show over time. |

### Phase 7 — Security Hardening 🟠
| Feature | Difficulty | Why |
|---|:--:|---|
| `x/crisis` invariants (oracle/rwa/fees) | 🟠 High | Must be cheap, correct, and bounded — they run on-chain. |
| Simulation / fuzz tests | 🔴 Very High | Writing meaningful sim ops for custom modules is genuinely hard. |
| Adversarial tests (oracle manip, bond bypass) | 🟠 High | Requires thinking like an attacker against your own economics. |
| CodeQL / static analysis in CI | 🟡 Medium | Setup + triage. |
| Load test / TPS baseline | 🟡 Medium | TPS target still an open question (App. C). |

### Phase 8 — Public Testnet v1 🟠
| Feature | Difficulty | Why |
|---|:--:|---|
| 10+ **external** validators | 🔴 Very High | First real-world test of Invariant 3; you control neither their ops nor uptime. |
| Independent feeders in the wild | 🟠 High | Provider disagreement and real network conditions surface here. |
| Public faucet + bug bounty + monitoring | 🟡 Medium | Operational, not algorithmic. |
| Stay stable 4+ weeks | 🟠 High | Endurance + incident response under public load. |

### Phase 9 — Audit + Testnet v2 + Genesis Rehearsal 🔴
| Feature | Difficulty | Why |
|---|:--:|---|
| **External audit + fix integration** | 🔴 Very High | Externally gated, time-unpredictable; fixes can ripple across modules. |
| Cosmovisor upgrade test | 🟠 High | Store migrations must be exact; a bad migration bricks an upgrade. |
| Genesis ceremony rehearsal | 🟠 High | `gentx`/`collect-gentxs`/hash coordination; multi-party. |
| Chain Registry + wallet (`suggestChain`) | 🟡 Medium | External PR + wallet behavior. |
| Frozen genesis template | 🟠 High | This output becomes the irreversible mainnet template. |

### Phase 10 — Mainnet Launch 🔴
| Feature | Difficulty | Why |
|---|:--:|---|
| Final genesis + SHA256 + 20+ validator coordination | 🔴 Very High | One-shot, multi-party, time-boxed, no rollback. |
| Day-1 IBC channels | 🟠 High | Live counterparties (Cosmos Hub) outside our control. |
| Registry/wallet/explorer/monitoring go-live | 🟡 Medium | Operational launch checklist. |
| Incident response (first 48h) | 🔴 Very High | Real value at stake; mistakes are permanent/public. |

---

## 4. The hardest features (ranked)

A focused list of the items that most deserve senior attention, early prototyping, and audit budget:

| Rank | Feature | Phase | Band | Core reason |
|---|---|---|:--:|---|
| 1 | Oracle `EndBlock` aggregation + slashing | 1 | 🔴 | Consensus-critical + touches validator funds; non-determinism = chain halt. |
| 2 | Stake-weighted median (deterministic) | 1 | 🟠 | Precision/tie-break bugs are silent until they diverge across nodes. |
| 3 | Outlier detection slash (method TBD) | 1 | 🔴 | Open design question + economic correctness + slashing. |
| 4 | RWA transfer-restriction ante decorator | 3 | 🔴 | Security bypass surface via plain bank sends. |
| 5 | External audit + fix integration | 9 | 🔴 | Externally gated; ripple risk; pre-mainnet. |
| 6 | Mainnet genesis ceremony / launch | 10 | 🔴 | Irreversible, multi-party, one-shot. |
| 7 | Issuer bond escrow (lock/release/slash) | 3 | 🟠 | User funds + invariant-backed correctness. |
| 8 | Module simulation/fuzz tests | 7 | 🔴 | Hard to author well for custom modules. |
| 9 | TWAP history windowing/pruning | 1 | 🟠 | Time-keyed store, off-by-one corrupts price history. |
| 10 | `rwa/*` IBC portability w/ restrictions | 5 | 🟠 | Restriction semantics across chain boundary. |

---

## 5. Why difficulty clusters where it does

- **Phase 1 is the engineering apex.** It is the only phase that is simultaneously *consensus-critical*, *funds-touching* (slashing), *math-sensitive* (deterministic median, TWAP), and *contract-defining* (the `OracleKeeper` interface everything downstream consumes). A defect here is the worst kind: a chain halt or an unjust slash. **Recommendation:** prototype the `WeightedMedian` + `EndBlock` path first, with exhaustive determinism tests and a resolved outlier-detection method (App. C) *before* building outward.

- **Phase 3 is the integration apex.** `x/rwa` is the only module that depends on two others (oracle + fees) and adds an ante-handler attack surface. Its difficulty is *coupling*, not raw algorithm.

- **Phase 4's risk is external, not consensus.** The feeder is off-chain; a bug slashes one validator rather than halting the chain. Its hard parts are flaky third-party APIs and safe broadcast — important, but a smaller blast radius.

- **Phases 9–10 are hard despite low code volume.** Almost no new logic, but maximum *irreversibility* and *external coordination* (auditors, external validators, counterparty chains). Difficulty migrates from "writing code" to "not making a one-shot mistake."

---

## 6. Recommendations

1. **Front-load Phase 1.** Treat the oracle `EndBlock` + median + slashing as a spike before committing the rest of the phase. Resolve the outlier-detection open question (App. C) early — it gates a 🔴 feature.
2. **Determinism test harness early.** Build a multi-validator simulation that asserts identical aggregation output across nodes; reuse it through Phases 6–8.
3. **Security-review the RWA ante decorator** as its own task — it is the most likely place for a silent bypass.
4. **Parallelize the 🟢/🟡 work** (Phases 2 and 5 can run alongside Phase 1 per the dependency graph) to keep the critical path on Phases 1 → 3 → 6.
5. **Budget calendar time, not just effort, for Phases 9–10.** They are externally gated (audit, external validators, registry PRs) and cannot be compressed by adding engineers.

---

## 7. Caveats

- Difficulty ≠ duration. Phase 8 is "low logic" but spans 4+ weeks of wall-clock endurance testing.
- Scores are relative within this project, not absolute.
- Open questions in Appendix C (outlier method, dispute resolution, provider set, TPS target, Day-1 channels) can each raise their phase's difficulty once resolved; revisit this analysis after each per-phase brainstorm.
```
