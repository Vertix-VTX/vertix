# Phase 5 — IBC Enablement (ICS-20) — Design Spec

**Date:** 2026-05-29
**Status:** Approved (brainstorm output)
**Phase:** 5 of the canonical phase map ([`full-design-spec.md`](../full-design-spec.md) §7.2)
**Depends on:** Phase 0 (Chain Foundation) — the scaffolded `app/ibc.go` IBC stack, `config.yml` genesis surface, CI/lint/build gates · **Enables:** Phase 6 (cross-chain transfers in the multi-validator devnet stack)
**Owns:** the cross-chain transfer surface of Vertix — confirming/locking the ICS-20 (`x/transfer`) wiring, the launch IBC application posture (ICA/ICS-29 kept wired but inert), the transfer genesis params, the layered IBC test strategy (in-process `ibctesting` + `interchaintest`), the chain Docker image used by E2E, and the relayer recipes (`docs/relayer.md` + `infra/hermes/config.toml`).

> This is the per-phase design spec produced by the brainstorm of Phase 5. It refines — and must stay consistent with — the program source of truth ([`full-design-spec.md`](../full-design-spec.md) Phase 5). Where this doc and the program spec disagree on phase order or cross-phase contracts, the program spec wins. Module internals defer to [`technical-design.md`](../technical-design.md). The step-by-step execution plan lives in `docs/plans/` (produced after this spec).
>
> **Canonical phase note.** Per [`full-design-spec.md`](../full-design-spec.md) §7.2, "Phase 5" is **IBC Enablement (ICS-20)**. The historical `plans/` numbering used "05" for the oracle-feeder sidecar; the program spec's canonical ordering supersedes that numbering, and this spec uses it. (The feeder is Phase 4 — see [`2026-05-29-phase-4-feeder-sidecar-design.md`](./2026-05-29-phase-4-feeder-sidecar-design.md).)

---

## 1. Goal & Deliverable Boundary

Make **VTX (`uvtx`) transferable cross-chain over ICS-20**, lock the IBC application surface down to exactly what launch needs, and prove it with a layered, reproducible test strategy plus worked relayer recipes. Additionally, **define — but do not yet activate — the `rwa/*` portability contract** that Phase 3 (`x/rwa`) will implement against.

The IBC core stack is already scaffolded in `app/ibc.go` (Phase 0 / Ignite): `ibc-core`, `transfer`, Interchain Accounts (host + controller), and the ICS-29 fee middleware are all wired, with the capability keeper, scoped keepers, and a sealed port router. Phase 5 is therefore **mostly confirmation, lock-down, params, and test/docs** — not new module code.

**In scope:**

- **Confirm + lock the ICS-20 wiring** in `app/ibc.go` and add a guard test (`app/ibc_test.go`) that pins the launch surface (ICS-20 enabled; ICA host locked; router sealed).
- **Genesis params:** `transfer` `send_enabled`/`receive_enabled = true`; ICA host `host_enabled = true` with `allow_messages = []`; ICA controller disabled. Surfaced in `config.yml` so devnet inherits them.
- **Layered tests (Approach C):** fast in-process `ibctesting` assertions in the main module (run on every PR via `make test`) **and** an `e2e/` `interchaintest` suite (real `vertixd` binary + real Hermes relayer) in a dedicated Docker CI job.
- **Chain `Dockerfile`** for `interchaintest` to consume.
- **`e2e/` as a separate Go module** (isolates `interchaintest`'s heavy dependency tree from the chain build, per `project-structure.md` §8).
- **Relayer recipes:** `docs/relayer.md` (Hermes v1.8.x primary, fully worked; `rly` as a shorter alternative) + `infra/hermes/config.toml` consumed by the E2E suite.
- **`Makefile` targets:** `e2e` (and any Docker image build helper the E2E job needs).

**Out of scope (deferred):**

- **Live `rwa/{id}` cross-chain transfer + restriction enforcement** — depends on `x/rwa` (Phase 3, not yet implemented). Phase 5 ships the **written contract (§4)** and a **build-tagged, ready-to-enable E2E stub**, closed by a small follow-up task after Phase 3 lands.
- **Cross-chain oracle queries (ICS-004 / interchain queries)** — post-mainnet (out of scope at launch per [`full-design-spec.md`](../full-design-spec.md) §1).
- **Mainnet channel operation + Day-1 counterparty channels** — Phase 10.
- **Interchain Accounts usage** — kept wired but inert; no host actions, no controller flows at launch.
- **Multi-validator devnet topology, per-validator relayer deployment** — Phase 6.

---

## 2. Locked Decisions (from this brainstorm)

| # | Decision | Choice | Rationale |
|---|---|---|---|
| D1 | **Phase scope vs. `rwa/*`** | Ship **VTX ICS-20 now**; deliver `rwa/*` as a written cross-phase contract (§4) + a build-tagged, ready-to-enable `interchaintest` stub that is `t.Skip`-ped until `x/rwa` exists. | Honors the §7.3 dependency graph: Phase 5 depends only on Phase 0 and is explicitly meant to run **in parallel** with Phases 1/2 — blocking it on Phase 3 would contradict that and stall the critical path. The VTX test already exercises 100% of the hard infrastructure (two chains + relayer, channel handshake, packet relay, escrow/voucher round-trip); the `rwa/*` test adds only non-native-denom transfer + the restriction check, which are meaningless before Phase 3. |
| D2 | **E2E topology** | **Vertix↔Vertix** (two instances of our own image) as the **required CI default**; **Vertix↔gaia** behind a build tag (`realnet`, opt-in, not in default CI). | Self-contained + deterministic (same image both sides, no external counterparty) for a phase the difficulty analysis flags as "environment-heavy and flaky-prone." Matches `project-structure.md` §8 ("ICS-20 VTX transfer between two Vertix chains"). A real Cosmos Hub counterparty is explicitly a **Phase 10** concern; gaia adds realism for those who want it at zero cost to per-PR CI stability. |
| D3 | **Relayer** | **Hermes v1.8.x primary**, fully worked recipe (config + channel-create + relay commands); **`rly` documented as a shorter alternative**. | Matches the program spec. The same `infra/hermes/config.toml` is consumed by the E2E suite, so the documented config is a **tested artifact**, not docs that rot. |
| D4 | **IBC application surface** | **Keep ICA (host + controller) + ICS-29 fee wired** (no code removal) but **inert at genesis**: ICA host `host_enabled = true` with `allow_messages = []`; ICA controller disabled; ICS-29 active only on fee-enabled channels (none opened at launch). Add a **guard test** pinning this posture. | Invariant 6 ("standard SDK modules wired unmodified to preserve auditability and ecosystem compatibility") cuts against ripping the scaffolded stack out of `app.go` and risks disturbing the sealed router / depinject graph. An empty `allow_messages` removes the open-ICA-host **launch attack surface** without deleting code, and the guard test prevents silent drift. |
| D5 | **Test strategy** | **Approach C (layered):** in-process `ibctesting` (ibc-go `Coordinator`, in-memory, per-PR, deterministic) for ICS-20 correctness (escrow/voucher math, channel handshake, the ICA-locked guard) **+** `e2e/` `interchaintest` (real built binary + real Hermes) for the high-fidelity acceptance proof. | Fast, deterministic per-PR coverage that catches regressions instantly **and** the real-binary/relayer proof the acceptance gate demands. The in-process layer keeps the flaky Docker layer off the inner dev loop; the E2E layer validates the actual operator experience. |
| D6 | **CI integration** | A **dedicated GitHub Actions job** builds a `vertixd` Docker image and runs the `e2e/` module (separate `go.mod`) on a timeout, **off the fast unit-test path**. Requires adding a chain `Dockerfile`. | Keeps `interchaintest`'s Docker weight and multi-minute runtime out of the per-PR unit-test path while still gating merges on a real cross-chain run. |
| D7 | **`rwa/*` restriction rule** | **Restricted assets (non-empty allowlist/denylist) are NOT IBC-exportable**; **unrestricted `rwa/{id}` transfers freely** over ICS-20. Enforced at the **source boundary** via a bank `SendRestrictionFn` registered by `x/rwa` (Phase 3). | Once an `rwa/{id}` leaves over ICS-20 it becomes an `ibc/<hash>` voucher on the counterparty, where our chain has **zero** enforcement power. The only honest reading of "without losing restriction semantics at the source chain" is that a **supervised asset can never escape supervision by being bridged away**. A `SendRestrictionFn` (not a handler-only check) also closes the "IBC escrow bypass" hole. See §4. |

These inherit and must not contradict the program-level locked decisions in [`full-design-spec.md`](../full-design-spec.md) §§1–6 and the architectural invariants §5. In particular, IBC introduces **no new VTX issuance** (Invariant 1/2): ICS-20 escrows on send and un-escrows on receive; it never mints base supply.

---

## 3. Target Layout (Phase 5 outputs)

Realizes `project-structure.md` §8 (`e2e/`) and §9 (`infra/hermes/`). Net-new files are marked `+`; confirmed/edited existing files are marked `~`.

```
+ Dockerfile                         minimal vertixd image for interchaintest (static build, alpine/distroless)
~ app/ibc.go                         confirmed; no functional change expected beyond comments/lock-in
+ app/ibc_test.go                    guard test: ICS-20 transfer route present; ICA host allow_messages == [];
                                     controller disabled; port router sealed
+ app/ibc_flow_test.go               in-process ibctesting: VTX ICS-20 round-trip (escrow → voucher → return → unescrow)
~ config.yml                         transfer send/receive enabled; ICA host allow_messages: []; controller disabled
e2e/                                 separate Go module (isolates interchaintest dependency tree)
├── + go.mod / go.sum
├── + ibc_transfer_test.go           Vertix↔Vertix ICS-20 VTX round-trip (default CI)
├── + gaia_transfer_test.go          Vertix↔gaia VTX round-trip (build tag `realnet`, opt-in)
├── + rwa_transfer_test.go           build tag `rwa`: unrestricted rwa/{id} round-trips + restricted rejected at source
│                                     (t.Skip until x/rwa lands — Phase 3 follow-up enables it)
└── + helpers/
    └── chain.go                     ChainSpec(s), image tag, genesis overrides (transfer on, ICA locked), funded users
infra/
└── hermes/
    └── + config.toml                Hermes v1.8.x config for a Vertix↔counterparty link
docs/
└── + relayer.md                     Hermes worked recipe (config + create-channel + relay), rly alternative
~ Makefile                           + `e2e` target; + docker image build helper for the E2E job
~ .github/workflows/ci.yml           + dedicated `e2e` job (build image → run e2e module, timeout, off fast path)
```

> Naming note: `app/ibc_flow_test.go` is the in-process `ibctesting` layer; if the repo prefers, it can live under a dedicated package (e.g. `app/ibctest/`). The plan settles the exact filename — both are valid.

---

## 4. The `rwa/*` Portability Contract (handed to Phase 3)

This realizes the spec's hardest Phase 5 item — *"the `rwa/*` namespace must transfer correctly over ICS-20 without losing restriction semantics at the source chain."* It is the **most important design content** Phase 5 produces, because Phase 3 must honor it.

**The enforcement reality.** ICS-20 of a chain-native denom (which `rwa/{id}` is, on Vertix) works by **escrowing** the tokens in the `transfer` module account on send and **un-escrowing** on return. On the counterparty chain the asset exists only as an `ibc/<hash>` voucher that **Vertix cannot govern**. Therefore restriction semantics are enforceable **only at the source boundary** — the escrow-on-send leg and the unescrow-on-receive leg.

**Contract (Phase 5 defines, Phase 3 implements):**

1. **Enforce via a bank `SendRestrictionFn`, not a handler-only check.** `x/rwa` MUST register its allowlist/denylist enforcement through `bankkeeper.AppendSendRestriction` (a `SendRestrictionFn`). This guarantees the ICS-20 escrow send — a plain `x/bank` send to the `transfer` module account — is subject to the same check, closing the bypass that a check living only in `MsgTransferRWA` would leave open. *(This dovetails with the Phase 3 difficulty-analysis item "transfer-restriction ante decorator"; a `SendRestrictionFn` is the SDK-canonical, lower-risk mechanism and is the recommended realization.)*

2. **Restricted assets are not IBC-exportable.** For an `rwa/{id}` with a **non-empty** allowlist or denylist, the `SendRestrictionFn` MUST reject sends into the IBC escrow account. **Unrestricted `rwa/{id}` (no allow/deny constraints) transfers freely** over ICS-20. This satisfies *"`rwa/*` transferable cross-chain"* for the unrestricted case while making it impossible for a supervised asset to escape supervision via bridging.

3. **Receipt of returning vouchers** un-escrows back to the source; the `SendRestrictionFn` applies on that leg too, so a returning restricted asset (should one ever have left) is still checked at the boundary.

**Phase 5 deliverable for this contract:** the contract text above (carried into Phase 3's brainstorm) **plus** `e2e/rwa_transfer_test.go` (build tag `rwa`) asserting: (a) an unrestricted `rwa/{id}` round-trips Vertix↔Vertix; (b) a restricted `rwa/{id}` send is rejected at source. The test is `t.Skip`-ped with a clear message until `x/rwa` is available; a Phase 3 follow-up task removes the skip.

---

## 5. Genesis / Params to Lock

| Module | Param | Value | Why |
|---|---|---|---|
| `transfer` | `params.send_enabled` | `true` | VTX (and later unrestricted `rwa/*`) can be sent cross-chain. |
| `transfer` | `params.receive_enabled` | `true` | Vertix can receive vouchers (incl. returning VTX). |
| `interchainaccounts` host | `host_enabled` | `true` | Module on (standard), but… |
| `interchainaccounts` host | `allow_messages` | `[]` | …no host action is executable — inert ICA surface (D4). |
| `interchainaccounts` controller | `controller_enabled` | `false` | No outbound ICA flows at launch (D4). |

These are surfaced in `config.yml` so the Phase 6 devnet genesis inherits them, and pinned by `app/ibc_test.go` so they cannot silently drift.

---

## 6. Test Matrix

| Layer | Where | Runs | Asserts |
|---|---|---|---|
| **Guard** | `app/ibc_test.go` | per-PR (`make test`) | ICS-20 route registered; ICA host `allow_messages == []`; controller disabled; router sealed. |
| **In-process flow** | `app/ibc_flow_test.go` (`ibctesting`) | per-PR (`make test`) | Channel handshake; VTX escrow→voucher→return→unescrow; balances conserved; no base-supply change. |
| **E2E (default)** | `e2e/ibc_transfer_test.go` | dedicated CI Docker job | Real `vertixd`×2 + real Hermes: open channel, transfer VTX, assert counterparty `ibc/<hash>` balance + round-trip. |
| **E2E (opt-in)** | `e2e/gaia_transfer_test.go` (`realnet`) | manual / scheduled | Vertix↔gaia VTX transfer against a published gaia image. |
| **E2E (`rwa`, stubbed)** | `e2e/rwa_transfer_test.go` (`rwa`) | enabled post-Phase 3 | Unrestricted `rwa/{id}` round-trips; restricted `rwa/{id}` rejected at source. |

---

## 7. CI Design

- New `e2e` job in `.github/workflows/ci.yml`, **separate** from the existing fast `lint`/`test`/`build` jobs:
  1. Build the `vertixd` Docker image from the new `Dockerfile` (tag it for `interchaintest` to pick up).
  2. `cd e2e && go test ./... -timeout 30m -v` (default tags only → Vertix↔Vertix + skipped `rwa`).
- The job has its own timeout and does **not** block the fast unit-test feedback loop.
- `realnet` and `rwa` build tags are excluded from the default CI run (opt-in / future).

---

## 8. Acceptance Gate Mapping

| [`full-design-spec.md`](../full-design-spec.md) Phase 5 gate clause | How Phase 5 satisfies it |
|---|---|
| "ICS-20 VTX transfers pass in `interchaintest`" | `e2e/ibc_transfer_test.go` (Vertix↔Vertix) with real Hermes, in the CI Docker job. |
| "`rwa/{id}` transfers work cross-chain" | **Documented carry-over:** contract (§4) + `rwa`-tagged E2E stub; fully closed by a small follow-up task once Phase 3 (`x/rwa`) lands. |
| "relayer setup documented and reproducible" | `docs/relayer.md` (Hermes worked recipe + `rly` alt) + `infra/hermes/config.toml`, exercised by the E2E suite. |

> **Honest gate note:** because `x/rwa` (Phase 3) is not yet implemented, the `rwa/{id}` clause of the Phase 5 gate is **partially carried over** by design (D1). This does not violate the dependency graph (Phase 5 depends only on Phase 0); it reflects that the `rwa/*` *contract* is delivered here while its *live test* activates after Phase 3. The carry-over is tracked explicitly so it is not forgotten.

---

## 9. Cross-Phase Contracts Produced

- **To Phase 3 (`x/rwa`):** the §4 contract — restrictions enforced via a bank `SendRestrictionFn`; **restricted assets are not IBC-exportable, unrestricted ones transfer freely**. Phase 3's brainstorm must honor this (it also resolves the [`full-design-spec.md`](../full-design-spec.md) Appendix C "transfer-restriction model" open question in a way consistent with it).
- **To Phase 6 (devnet):** the locked `transfer`/ICA genesis params (§5) and `infra/hermes/config.toml` become the devnet relayer baseline; the `e2e/helpers` chain spec informs the compose genesis.
- **Denom portability contract (from Phase 3, honored here):** the reserved `rwa/{asset-id}` namespace is treated as a transferable native denom subject to §4.

---

## 10. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| `interchaintest` flakiness in CI (Docker, relayer timing). | Layered strategy (D5): correctness proven cheaply in-process; the heavy job is isolated, timeout-bounded, and off the fast path. Pin image tags and relayer version. |
| ICA host accidentally opened later (attack surface). | `allow_messages == []` guard test (D4) fails CI on drift. |
| `rwa/*` IBC bypass (restriction skipped via plain bank send / escrow). | §4 mandates a bank `SendRestrictionFn`, which intercepts the escrow send itself — not a handler-only check. |
| Phase 5 "done" but `rwa/{id}` clause unmet. | Explicitly tracked carry-over (§8) + skipped-but-present E2E stub so the follow-up is unambiguous. |
| Chain `Dockerfile` drift vs. release build. | Reuse the same build flags as the release pipeline where possible; keep the image minimal (static binary). |

---

## 11. Doc-Sync Items

On implementation, update:

- `docs/project-structure.md` — confirm `e2e/` (§8) file set matches; note `Dockerfile` at repo root and `infra/hermes/config.toml`.
- `AGENTS.md` — add `docs/relayer.md` to the doc index; note the `e2e` CI job in §8 command reference.
- `docs/full-design-spec.md` Appendix C — when Phase 3 lands, mark the "transfer-restriction model" question resolved consistent with §4/D7.
- `AGENTS.md` §2 still lists a historical `plans/2026-05-09-06-ibc-devnet.md` (IBC + devnet combined) that does not exist as a file; the canonical split is IBC = Phase 5, devnet = Phase 6, and the new per-phase plans supersede that historical entry.

---

## 12. Open Questions (deferred to the plan / Phase 3)

- **Exact in-process test package location** (`app/ibc_flow_test.go` vs. a dedicated `app/ibctest/` package) — settled in the plan.
- **gaia image tag/version** for the `realnet` opt-in test — pinned in the plan (a recent ibc-go v8-compatible gaia release).
- **Precise `SendRestrictionFn` storage shape** for allow/deny lists — owned by the Phase 3 brainstorm; §4 fixes only the *behavioral* contract.
