# Vertix — Chain Registry drafts (Phase 9)

Local drafts for a future PR to [cosmos/chain-registry](https://github.com/cosmos/chain-registry). These files are **not** published to the upstream registry until testnet v2 is publicly open and endpoints are final.

## Layout

| Path | Chain ID | Registry folder (upstream) |
|------|----------|----------------------------|
| `testnet/` | `vertix-testnet-2` | `testnets/vertixtestnet/` |
| `mainnet/` | `vertix-1` | `vertix/` |

| File | Purpose |
|------|---------|
| `chain.json` | Chain metadata (RPC/LCD, bech32, fees, staking) |
| `assetlist.json` | Native `uvtx` / display `vtx` (6 decimals, symbol VTX) |
| `testnet/keplr.json` | Keplr `experimentalSuggestChain` payload |
| `testnet/leap.json` | Leap Wallet `suggestChain` payload (same shape as Keplr) |

## Placeholder endpoints

All `*.example` URLs are intentional placeholders. Replace them before opening a registry PR or testing wallets on a live network:

| Placeholder | Replace with |
|-------------|----------------|
| `https://rpc.testnet.vertix.example` | Public CometBFT RPC (e.g. sentry RPC on `26657`) |
| `https://lcd.testnet.vertix.example` | Public REST/gRPC-gateway LCD (e.g. `1317`) |
| `https://rpc.vertix.example` | Mainnet RPC (when live) |
| `https://lcd.vertix.example` | Mainnet LCD (when live) |

After the internal gate opens testnet v2 to the public, copy the real URLs into `testnet/chain.json`, then regenerate `keplr.json` and `leap.json` from the same values (chain ID, RPC, REST, bech32 prefixes, and coin metadata must stay in sync).

Document the final URLs in [`docs/testnet-runbook.md`](../../docs/testnet-runbook.md) when swapping placeholders.

## Bech32 prefix

Registry drafts use bech32 prefix **`vertix`** (account `vertix1…`, validator `vertixvaloper1…`), aligned with Phase 9 wallet prep and [`docs/specs/2026-05-29-phase-9-testnet-v2-genesis-rehearsal-design.md`](../../docs/specs/2026-05-29-phase-9-testnet-v2-genesis-rehearsal-design.md).

The current `vertixd` binary still uses prefix **`vtx`** in [`app/app.go`](../../app/app.go) for devnet and testnet v1. Wallet `suggestChain` JSON must match the **live** chain prefix at test time; reconcile prefix in a dedicated change before mainnet if registry and binary diverge.

## Mainnet (`mainnet/`)

`mainnet/chain.json` is a **pre-launch** draft: `status` is `upcoming`, `stage` is `pre-launch`, and APIs are placeholders. Do not submit mainnet entries to cosmos/chain-registry until Phase 10 launch readiness.

## Upstream registry PR procedure

1. **Gate:** Internal Phase 9 gate passed; `vertix-testnet-2` is publicly reachable; genesis hash and APIs are stable.
2. **Fork** [cosmos/chain-registry](https://github.com/cosmos/chain-registry) and create a branch.
3. **Testnet:** Add `testnets/vertixtestnet/chain.json` and `testnets/vertixtestnet/assetlist.json` from this repo’s `testnet/` copies (update endpoints, add `logo_URIs` / explorers if available).
4. **Validate:** Run upstream CI locally if possible; ensure `chain_name` matches folder name and `assetlist.json` `chain_name` matches `chain.json`.
5. **PR:** Open against `cosmos/chain-registry` `master` with title e.g. `Add Vertix testnet (vertix-testnet-2)`; link to official docs or announcement.
6. **Wallets:** After merge (or from this repo before merge), test Keplr and Leap with `testnet/keplr.json` / `testnet/leap.json` via `experimentalSuggestChain` against live RPC.
7. **Mainnet:** Repeat under `vertix/` only after mainnet launch (Phase 10); use `mainnet/` drafts as the starting template.

## References

- Phase 9 plan: [`docs/plans/2026-05-29-phase-9-testnet-v2-genesis-rehearsal.md`](../../docs/plans/2026-05-29-phase-9-testnet-v2-genesis-rehearsal.md) (Task 10)
- Denom metadata: [`config.yml`](../../config.yml) (`uvtx` / `vtx`, 6 decimals)
- Tokenomics: [`docs/tokenomics.md`](../../docs/tokenomics.md)
