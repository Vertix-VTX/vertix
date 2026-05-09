# Vertix (VTX) — Tokenomics & Unlock Schedule

**Chain:** Vertix (`vertix-1`) | **Ticker:** VTX | **Category:** Oracle · Real World Assets

---

## 1. Denomination & Supply

| Field | Value |
|---|---|
| Base denom | `uvtx` (on-chain smallest unit) |
| Display denom | `VTX` |
| Decimals | 6 |
| Conversion | `1 VTX = 1,000,000 uvtx` |
| Total supply | `21,000,000 VTX` |
| Max supply | `21,000,000 VTX` (hard cap) |
| Total supply in uvtx | `21,000,000,000,000 uvtx` |
| Inflation | **None** — zero mint after genesis, ever |

The 21M hard cap is a deliberate design choice modeled on Bitcoin's scarcity. No governance proposal can mint new VTX beyond the genesis supply. Network security is funded by fee revenue, not dilution.

---

## 2. Token Allocation

| Category | % | VTX | uvtx |
|---|---|---|---|
| Team & Core Contributors | 22% | 4,620,000 | 4,620,000,000,000 |
| Foundation / Treasury | 20% | 4,200,000 | 4,200,000,000,000 |
| Strategic Investors | 13% | 2,730,000 | 2,730,000,000,000 |
| Ecosystem & Grants | 18% | 3,780,000 | 3,780,000,000,000 |
| Validator Incentives Pool | 12% | 2,520,000 | 2,520,000,000,000 |
| Public Liquidity | 8% | 1,680,000 | 1,680,000,000,000 |
| Community Airdrop | 7% | 1,470,000 | 1,470,000,000,000 |
| **Total** | **100%** | **21,000,000** | **21,000,000,000,000** |

**Team + Foundation = 42%** — execution-focused allocation to fund aggressive 12-month development.

---

## 3. Vesting Schedules

All vesting implemented via Cosmos SDK `cosmos.vesting.v1beta1` account types in genesis.

### 3.1 Team & Core Contributors — 4,620,000 VTX (22%)

- **Type:** PeriodicVestingAccount
- **Cliff:** 12 months (no tokens unlock before month 12)
- **Linear vesting:** 36 months after cliff (monthly periods)
- **Total duration:** 48 months (4 years) from TGE
- **Rationale:** Align long-term incentives with mainnet success

```
Month:  0    6   12   18   24   30   36   48
        ├────┼────┼────┼────┼────┼────┼────┤
Unlock  0%   0%  0%  ~8%  ~17% ~25% ~33% 100%
              ←cliff→ ←── linear monthly ───→
```

### 3.2 Foundation / Treasury — 4,200,000 VTX (20%)

- **Type:** PeriodicVestingAccount
- **Cliff:** 12 months
- **Linear vesting:** 48 months after cliff (quarterly periods)
- **Control:** Governance multisig (3-of-5); spending requires on-chain governance proposal
- **Total duration:** 60 months from TGE
- **Rationale:** Long-horizon runway; governance-controlled prevents unilateral spending

### 3.3 Strategic Investors — 2,730,000 VTX (13%)

- **Type:** PeriodicVestingAccount
- **Cliff:** 6 months
- **Linear vesting:** 18 months after cliff (monthly periods)
- **Total duration:** 24 months from TGE
- **Rationale:** Standard seed/strategic round vesting; shorter than team to reflect capital risk

```
Month:  0    6   12   18   24
        ├────┼────┼────┼────┤
Unlock  0%   0%  33%  67% 100%
         ←cliff→ ←── linear ──→
```

### 3.4 Ecosystem & Grants — 3,780,000 VTX (18%)

- **Type:** PeriodicVestingAccount (quarterly releases)
- **Cliff:** None
- **Release cadence:** Quarterly over 48 months
- **Control:** Grants committee multisig (separate from Foundation)
- **Rationale:** Developer and ecosystem adoption; front-loaded availability to bootstrap builders

### 3.5 Validator Incentives Pool — 2,520,000 VTX (12%)

- **Type:** ContinuousVestingAccount
- **Cliff:** None
- **Linear vesting:** 60 months (5 years) from genesis
- **Purpose:** Supplements fee revenue to fund validator rewards during bootstrap phase
- **Rationale:** Mirrors Bitcoin's declining block reward model — emissions taper as fee revenue grows
- **Bootstrap → mature transition:**

```
Phase         Validator Reward Source
────────────────────────────────────────────────────
Months 0–18   Pool emissions (dominant) + growing fees
Months 18–36  Declining pool + majority from fees
Month 36+     Pure fee-driven rewards (pool depleted ~month 60)
```

### 3.6 Public Liquidity — 1,680,000 VTX (8%)

- **Type:** ContinuousVestingAccount (partial)
- **At TGE:** 50% (840,000 VTX) immediately available for DEX liquidity provision
- **Remaining 50%:** 6-month linear vesting (market making operational reserve)
- **Rationale:** Bootstrap DEX trading pairs (VTX/USDC, VTX/ATOM on Osmosis) at mainnet

### 3.7 Community Airdrop — 1,470,000 VTX (7%)

- **Type:** BaseAccount (no vesting)
- **Cliff:** None
- **Claimable:** At genesis with a **6-month claim window**; unclaimed VTX returns to Foundation Treasury after window closes
- **Rationale:** Bootstraps Cosmos ecosystem community engagement at launch

---

## 4. Value Accrual Model

VTX is simultaneously a **work token**, **yield token**, and **deflationary asset**.

### 4.1 Work Token

Participation in Vertix's core infrastructure requires bonding VTX:

| Role | Requirement |
|---|---|
| Validator (oracle feeder) | Bond VTX via `x/staking` to enter active set; run oracle sidecar |
| RWA Issuer | Bond min 10,000 VTX per asset class (governance-adjustable) |

Misbehavior results in slashing:

| Event | Slash |
|---|---|
| Oracle miss rate > 5% per window | 0.5% of bonded stake |
| Oracle outlier submission | 1.0% of bonded stake |
| Double-sign | 5.0% of bonded stake (standard SDK) |
| Downtime (jailing) | 0.01% + jailed (standard SDK) |
| RWA fraud / dispute | Issuer bond slashed (governance vote required above threshold) |

### 4.2 Yield Token

All network fees flow to VTX stakers:

```
All collected fees
(gas fees + RWA mint/settle fees + oracle data subscriptions)
        ↓
    x/fees EndBlock
        ├── 40% burned permanently (x/bank.BurnCoins)
        └── 60% distributed to VTX stakers (x/distribution)
                    proportional to bonded stake
```

**Fee sources:**
- Gas fees: all transactions on the Vertix chain
- RWA mint fee: 0.1% of asset notional value at mint
- RWA settle fee: 0.1% of asset notional value at settlement
- Future: oracle data subscription fees from external protocols

### 4.3 Deflationary Asset

| Mechanism | Effect |
|---|---|
| 40% fee burn | Permanent VTX removal from 21M supply |
| RWA issuer bonds | VTX locked per active asset (illiquid) |
| Staker unbonding period | 21-day unbonding removes VTX from liquid supply |
| Validator bonds | Active set VTX continually locked in staking |

As RWA and oracle usage grows, more VTX is burned and locked. On a fixed 21M supply, this creates compounding scarcity.

---

## 5. Genesis Implementation

### 5.1 Denomination Metadata (genesis bank)

```json
{
  "description": "The native token of the Vertix blockchain",
  "denom_units": [
    { "denom": "uvtx", "exponent": 0, "aliases": ["microvtx"] },
    { "denom": "mvtx", "exponent": 3, "aliases": ["millivtx"] },
    { "denom": "vtx",  "exponent": 6, "aliases": [] }
  ],
  "base": "uvtx",
  "display": "vtx",
  "name": "Vertix",
  "symbol": "VTX"
}
```

### 5.2 Vesting Account Implementation

**Option A — Ignite `config.yml` genesis overrides (recommended for devnet/testnet):**

```yaml
genesis:
  app_state:
    bank:
      balances:
        - address: vtx1team...
          coins:
            - denom: uvtx
              amount: "4620000000000"  # 4,620,000 VTX
    auth:
      accounts:
        - "@type": "/cosmos.vesting.v1beta1.PeriodicVestingAccount"
          base_vesting_account:
            base_account:
              address: vtx1team...
            original_vesting:
              - denom: uvtx
                amount: "4620000000000"
            end_time: "0"
          start_time: "1749427200"  # TGE Unix timestamp
          vesting_periods:
            - length: "31536000"   # 12 months cliff (in seconds)
              amount:
                - denom: uvtx
                  amount: "0"      # Nothing unlocks at cliff end itself
            # then monthly periods for 36 months
            - length: "2628000"    # 1 month
              amount:
                - denom: uvtx
                  amount: "128333333"  # ~1/36 of remaining 4,620,000 VTX
            # ... repeat 35 more monthly periods
```

**Option B — CLI for simple continuous vesting:**

```bash
vertixd genesis add-genesis-account vtx1investor... 2730000000000uvtx \
  --vesting-amount 2730000000000uvtx \
  --vesting-start-time 1749427200 \
  --vesting-end-time   1811827200   # 24 months after TGE
```

**Option C — Direct `genesis.json` edit** for complex periodic schedules:

```bash
vertixd init vertix --chain-id vertix-1
# edit ~/.vertix/config/genesis.json to add vesting accounts
vertixd genesis validate-genesis
```

### 5.3 Airdrop Implementation

```bash
# BaseAccount — no vesting, claimable immediately
vertixd genesis add-genesis-account vtx1airdrop... 1470000000000uvtx
```

---

## 6. Governance Parameters

All fee and RWA parameters are adjustable via on-chain governance:

| Parameter | Module | Default | Description |
|---|---|---|---|
| `BurnRatio` | x/fees | 40% | Portion of fees burned |
| `DistributionRatio` | x/fees | 60% | Portion to stakers |
| `MinIssuerBond` | x/rwa | 10,000 VTX | Minimum bond to register RWA asset |
| `MintFeeRate` | x/rwa | 0.10% | Fee on RWA token minting |
| `SettleFeeRate` | x/rwa | 0.10% | Fee on RWA settlement |
| `VoteWindow` | x/oracle | 10 blocks | Oracle feed aggregation window |
| `MissThreshold` | x/oracle | 5% | Miss rate before oracle slash |
| `MissSlashRate` | x/oracle | 0.5% | Slash for missed oracle feeds |
| `OutlierSlashRate` | x/oracle | 1.0% | Slash for outlier submissions |
| `UnbondingTime` | x/staking | 21 days | Staker unbonding period |
| `MinCommission` | x/staking | 5% | Minimum validator commission |

---

## 7. Validation Checklist

Before mainnet genesis, verify:

- [ ] Sum of all allocations equals 21,000,000 VTX exactly
- [ ] Each vesting account's period amounts sum to `original_vesting`
- [ ] All addresses valid bech32 with `vtx` prefix
- [ ] TGE Unix timestamp is correct and agreed upon
- [ ] `vertixd genesis validate-genesis` passes with zero errors
- [ ] Bank balances and auth accounts cross-reference correctly
- [ ] Validator Incentives Pool is a ContinuousVestingAccount (not free)
- [ ] Airdrop accounts are BaseAccounts (no vesting)

**Verification queries:**

```bash
# Check balance of any genesis account
vertixd q bank balances vtx1team... --node <rpc>

# Inspect vesting schedule of an account
vertixd q auth account vtx1team... -o json | jq '.base_vesting_account'

# Verify total supply
vertixd q bank total --node <rpc>

# Confirm supply = 21,000,000,000,000 uvtx
vertixd q bank total -o json | jq '.supply[] | select(.denom=="uvtx")'
```

---

## 8. Transparency & Governance

- Allocation categories and vesting schedules published with genesis
- Any changes to allocations (pre-mainnet) require public communication and community discussion
- Post-mainnet: all token releases from Foundation and Ecosystem multisigs are published on-chain as transactions with memo/governance reference
- Audit trail: governance proposals for any parameter changes are publicly visible and queryable

---

## 9. Tokenomics Summary Card

```
┌─────────────────────────────────────────────────────┐
│                VERTIX (VTX) TOKENOMICS               │
├─────────────────────────────────────────────────────┤
│  Max Supply:        21,000,000 VTX  (hard cap)       │
│  Inflation:         None                             │
│  Decimals:          6  (1 VTX = 1,000,000 uvtx)     │
├─────────────────────────────────────────────────────┤
│  ALLOCATION                                          │
│  ├── Team              22%   4,620,000 VTX           │
│  ├── Foundation        20%   4,200,000 VTX           │
│  ├── Ecosystem         18%   3,780,000 VTX           │
│  ├── Investors         13%   2,730,000 VTX           │
│  ├── Validators        12%   2,520,000 VTX           │
│  ├── Liquidity          8%   1,680,000 VTX           │
│  └── Airdrop            7%   1,470,000 VTX           │
├─────────────────────────────────────────────────────┤
│  FEE MODEL                                           │
│  ├── 40% of all fees burned (permanent)              │
│  └── 60% of all fees to VTX stakers                 │
├─────────────────────────────────────────────────────┤
│  VALUE ACCRUAL                                       │
│  ├── Work token: bond to validate + issue RWA        │
│  ├── Yield token: stake to earn fee revenue          │
│  └── Deflationary: burn on fixed 21M supply          │
└─────────────────────────────────────────────────────┘
```
