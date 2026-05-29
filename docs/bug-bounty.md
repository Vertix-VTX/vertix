# Bug Bounty Program — Vertix Testnet

**Chain:** `vertix-testnet-1` · **Report via:** [`SECURITY.md`](../SECURITY.md) · **Operations:** [Testnet runbook §Bug-bounty operations](./testnet-runbook.md#6-bug-bounty-operations)

---

## Scope

### In scope

| Component | Notes |
|-----------|-------|
| `x/oracle` | Aggregation, slashing, feed validation, TWAP |
| `x/rwa` | Lifecycle state machine, bond escrow, factory denoms, transfer restrictions |
| `x/fees` | EndBlock fee split, burn, distribution |
| Custom ante handlers | RWA transfer restriction decorator |
| Genesis configuration | Supply cap, module params, crisis invariant registration |

### Out of scope

- Standard Cosmos SDK modules (`x/bank`, `x/staking`, etc.) unless a Vertix-specific wiring bug in `app/app.go` enables exploitation of custom modules
- Infrastructure DoS against testnet RPC/faucet/explorer endpoints
- Third-party price-provider APIs used by `vertix-feeder` (CoinGecko, Binance, etc.)
- Social engineering, physical attacks, or mainnet/real-user-fund targeting
- Issues already known and tracked in the public backlog

---

## Severity rubric (mapped to crisis invariants)

Vertix registers five permanent `x/crisis` invariant routes. Severity is assessed by whether a bug can violate or bypass these checks:

| Severity | Criteria | Crisis routes affected |
|----------|----------|---------------------|
| **Critical** | Can mint/inflate beyond 21M cap; activate unbonded assets or drain bond escrow; halt consensus via oracle aggregation manipulation | `fees/reconcile`, `rwa/bonds`, `oracle/prices` (consensus halt) |
| **High** | Unjust oracle slash; transfer restriction bypass via bank send; premature `rwa/*` mint before attestation | `rwa/denoms`, `rwa/bonds` (partial), `fees/module-balance` |
| **Medium** | Griefing (non-fund-loss), incorrect-but-recoverable state, parameter edge cases with governance workaround | Any route under adversarial conditions |
| **Low** | Informational, documentation errors, non-exploitable code quality | None directly |
| **Informational** | Best-practice suggestions, gas optimizations | None |

### Invariant reference

| Route | Statement |
|-------|-----------|
| `oracle/prices` | Stored aggregated prices are accept-listed and strictly positive |
| `fees/reconcile` | `genesisSupply − supply(uvtx) == cumulativeBurned` (⇒ 21M cap) |
| `fees/module-balance` | `x/fees` module account holds zero `uvtx` at block boundary |
| `rwa/bonds` | Module bond escrow ≥ required bonds; every `ACTIVE` asset is bonded |
| `rwa/denoms` | Pre-mint assets have zero factory-denom supply |

---

## Reward tiers

Indicative ranges (finalized operationally when the platform listing goes live):

| Severity | Indicative reward (USD) |
|----------|-------------------------|
| Critical | $5,000 – $25,000 |
| High | $1,000 – $5,000 |
| Medium | $250 – $1,000 |
| Low / Informational | Recognition / swag |

Rewards are at Vertix's discretion based on impact, report quality, and duplicate status. Payment method and KYC requirements are finalized in the platform listing (Immunefi or HackerOne — see [runbook](./testnet-runbook.md#6-bug-bounty-operations)).

---

## Rules / safe harbor

1. **Testnet only** — test against `vertix-testnet-1`. Do not attack mainnet or third-party infrastructure.
2. **Good faith** — no data destruction, no privacy violations, no extortion.
3. **No public disclosure** before coordinated fix and team acknowledgement.
4. **One report per issue** — duplicates receive partial or no reward at Vertix's discretion.
5. **Minimal exploitation** — demonstrate impact without maximizing harm (e.g., do not drain the community pool beyond a PoC amount).
6. **Legal compliance** — you must comply with applicable laws in your jurisdiction.

Researchers acting in good faith within these rules will not be pursued legally by Vertix.

---

## How to submit

1. Read [`SECURITY.md`](../SECURITY.md) for the private reporting channel.
2. Use the [GitHub Security Advisory](https://github.com/vertix-network/vertix/security/advisories/new) form or email `security@vertix.network`.
3. Include: module, severity self-assessment (using the rubric above), reproduction steps, commit/tag, and impact analysis referencing the relevant crisis invariant route.
4. Wait for triage acknowledgement (48h SLA). Critical/High issues are tracked in the private advisory thread; Medium/Low may be moved to the public `v2-backlog` after fix.

Platform listing (Immunefi/HackerOne) steps and the submission→triage→fix workflow are documented in [testnet runbook §Bug-bounty operations](./testnet-runbook.md#6-bug-bounty-operations).
