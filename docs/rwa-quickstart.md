# RWA Quickstart — Public Testnet

**Chain:** `vertix-testnet-1` · **Base denom:** `uvtx` · **Related:** [Technical design §3](./technical-design.md#3-xrwa-module) · [Testnet runbook](./testnet-runbook.md)

---

## Introduction

A Real World Asset (RWA) on Vertix is an on-chain registry entry backed by an issuer bond, linked to a live oracle price at attestation, and represented by a factory denom `rwa/{asset-id}` once minted. The lifecycle is: **register** (lock bond) → **attest** (oracle price link) → **mint** (factory denom) → **settle** (burn tokens, release bond). See [technical design §3](./technical-design.md#3-xrwa-module) for module internals, state machine, and crisis invariants (`rwa/bonds`, `rwa/denoms`).

---

## Prerequisites

1. **`vertixd` installed** — build from source (`make build`) or download the published testnet release binary.
2. **A funded key** — create a local key and fund it from the public faucet:

   ```bash
   vertixd keys add me --keyring-backend test
   ADDR=$(vertixd keys show me -a --keyring-backend test)
   curl -X POST "http://<faucet-host>/credit" \
     -H "Content-Type: application/json" \
     -d "{\"denom\":\"uvtx\",\"address\":\"$ADDR\"}"
   ```

   You need enough `uvtx` for the issuer bond (≥ `MinIssuerBond`, default **10,000 VTX** = `10000000000uvtx`) plus transaction fees.

3. **RPC endpoint** — set `NODE` to a public testnet RPC, e.g. `tcp://rpc.vertix-testnet.example:26657`.

4. **Live oracle price** — confirm `VTX:USD` is aggregating before attesting:

   ```bash
   vertixd query oracle price VTX:USD --node "$NODE" -o json
   ```

---

## Step 1 — Register

Register the asset and lock the issuer bond in module escrow. The bond must be ≥ `MinIssuerBond` (10,000 VTX on testnet).

```bash
CHAIN_ID=vertix-testnet-1
NODE=tcp://<rpc-host>:26657
ASSET=demo-gold-$(date +%s)
BOND=10000000000uvtx   # 10,000 VTX — >= MinIssuerBond

vertixd tx rwa register-asset "$ASSET" "Demo Gold Bar" VTX:USD "$BOND" \
  --from me \
  --chain-id "$CHAIN_ID" \
  --node "$NODE" \
  --keyring-backend test \
  --fees 4000uvtx \
  --gas 400000 \
  -y
```

This matches the register step in [`scripts/testnet/rwa-demo.sh`](../scripts/testnet/rwa-demo.sh).

---

## Step 2 — Attest

Link the asset to the live aggregated oracle price for `VTX:USD`. Attestation transitions the asset toward `ACTIVE` eligibility.

```bash
vertixd tx rwa attest-asset "$ASSET" \
  --from me \
  --chain-id "$CHAIN_ID" \
  --node "$NODE" \
  --keyring-backend test \
  --fees 4000uvtx \
  --gas 400000 \
  -y

# Verify post-attest status
vertixd query rwa asset "$ASSET" --node "$NODE" -o json | jq -r '.asset.status // .status'
```

Confirm the oracle price is live before attesting:

```bash
vertixd query oracle price VTX:USD --node "$NODE" -o json
```

---

## Step 3 — Mint

Mint factory-denom tokens `rwa/$ASSET` once the asset is attested:

```bash
vertixd tx rwa mint-rwa "$ASSET" 1000000000 \
  --from me \
  --chain-id "$CHAIN_ID" \
  --node "$NODE" \
  --keyring-backend test \
  --fees 4000uvtx \
  --gas 400000 \
  -y

# Check rwa/* supply
vertixd query bank total --node "$NODE" -o json | jq '.supply[] | select(.denom | test("rwa/"))'
```

---

## Step 4 — Settle

Burn outstanding RWA tokens and release the issuer bond:

```bash
vertixd tx rwa settle-rwa "$ASSET" \
  --from me \
  --chain-id "$CHAIN_ID" \
  --node "$NODE" \
  --keyring-backend test \
  --fees 4000uvtx \
  --gas 400000 \
  -y

vertixd query rwa asset "$ASSET" --node "$NODE" -o json | jq -r '.asset.status // .status'
# Expected: SETTLED (or equivalent settled state)
```

---

## Querying

| Query | Command |
|-------|---------|
| Asset record | `vertixd query rwa asset <id> --node "$NODE"` |
| All assets | `vertixd query rwa assets --node "$NODE"` |
| Module params | `vertixd query rwa params --node "$NODE"` |
| Oracle price | `vertixd query oracle price VTX:USD --node "$NODE"` |
| RWA denom balance | `vertixd query bank balances <addr> --node "$NODE"` |

**Lifecycle states:** `DRAFT` → `ATTESTED` → `ACTIVE` → `SETTLED`. An asset cannot reach `ACTIVE` without a locked bond ≥ `MinIssuerBond` and a valid oracle attestation.

---

## The dispute path

If an asset is misrepresented, governance can force-settle it via `MsgSlashBond`. The issuer bond is routed to the community pool instead of being returned. This is governance-gated — submit a proposal carrying the slash message, vote, and wait for the voting period.

The full adversarial demo is scripted in [`scripts/testnet/rwa-dispute-demo.sh`](../scripts/testnet/rwa-dispute-demo.sh):

1. Register + attest a disputable asset
2. Build a gov proposal with `MsgSlashBond` (authority = gov module account)
3. Vote yes from validators
4. Wait out the voting period (`voting_period=300s` on testnet)
5. Confirm force-settled status + community pool increase

Example proposal message shape:

```json
{
  "@type": "/vertix.rwa.v1.MsgSlashBond",
  "authority": "<gov-module-address>",
  "asset_id": "<asset-id>",
  "reason": "misrepresented backing"
}
```

Submit with:

```bash
vertixd tx gov submit-proposal /path/to/slash-prop.json \
  --from me --chain-id "$CHAIN_ID" --node "$NODE" \
  --keyring-backend test --fees 4000uvtx --gas 400000 -y
```

---

## One-command demo

Against a running founder stack (Docker), run the full happy-path lifecycle:

```bash
make testnet-demo
```

This executes [`scripts/testnet/rwa-demo.sh`](../scripts/testnet/rwa-demo.sh): register → attest → mint → settle, with oracle price checks and supply assertions. Expected output ends with `RWA DEMO: PASS`.

For the dispute path:

```bash
./scripts/testnet/rwa-dispute-demo.sh
```

Expected output ends with `RWA DISPUTE DEMO: PASS`.
