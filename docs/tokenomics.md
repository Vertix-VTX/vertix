# Tokenomics & Unlock Schedule (PNS)

This document specifies total supply, allocations, and vesting schedules for PNS, and how to implement them in a Cosmos SDK genesis using Ignite CLI.

## 1) Denominations and Supply
- Base denom: `upns` (on-chain smallest unit)
- Display denom: `pns` (6 decimals) → `1 pns = 1_000_000 upns`
- Total supply (example): `1,000,000,000 pns` = `1_000_000_000 * 1_000_000 = 1_000_000_000_000_000 upns`

## 2) Allocations (Example)
Adjust to your project needs; ensure the sum equals total supply.
- Community & Ecosystem: 40% → 400,000,000 pns
- Foundation/DAO Treasury: 20% → 200,000,000 pns
- Team: 15% → 150,000,000 pns
- Investors: 15% → 150,000,000 pns
- Liquidity/Market Making: 5% → 50,000,000 pns
- Airdrop: 5% → 50,000,000 pns

## 3) Vesting Types (Cosmos SDK)
Cosmos SDK supports vesting via account types in `cosmos.vesting.v1beta1`:
- ContinuousVestingAccount: linearly vested between `start_time` and `end_time`.
- DelayedVestingAccount: funds locked until `end_time` then fully vested.
- PeriodicVestingAccount: custom periods with `length` and `amounts` arrays.

A vesting account wraps a base account; balances are assigned to the vesting account address.

## 4) Example Unlock Schedules
- Team (12-month cliff, 24 months linear): DelayedVesting for cliff seed + ContinuousVesting for linear, or PeriodicVesting with one long cliff period followed by monthly periods.
- Investors (6-month cliff, 18 months linear): PeriodicVesting with monthly periods.
- Community (no vesting): normal accounts.

## 5) Implementation Options

### A) Using Ignite `config.yml` Genesis Overrides
Add vesting accounts and bank balances under `genesis.app_state`. Example entry for a PeriodicVesting account (pseudocode YAML):
```yaml
bank:
  balances:
    - address: cosmos1teamxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
      coins:
        - denom: upns
          amount: "150000000000000000"  # 150,000,000 pns in upns
vesting:
  accounts:
    - "@type": "/cosmos.vesting.v1beta1.PeriodicVestingAccount"
      base_vesting_account:
        base_account:
          address: cosmos1teamxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
          pub_key: null
          account_number: "0"
          sequence: "0"
        original_vesting:
          - denom: upns
            amount: "150000000000000000"
        delegated_free: []
        delegated_vesting: []
        end_time: "0"
      start_time: "1735689600" # 2025-01-01T00:00:00Z
      vesting_periods:
        - length: "31536000" # 12 months cliff
          amount:
            - denom: upns
              amount: "75000000000000000" # 50% after cliff
        - length: "2628000" # 1 month
          amount:
            - denom: upns
              amount: "3125000000000000"  # spread rest monthly
        # ...repeat monthly periods to sum up original_vesting
```
Notes:
- `amount` values must sum to `original_vesting`.
- Addresses must be Bech32 for your chain prefix.
- Period lengths are in seconds.

### B) Using CLI to add genesis accounts
You can add a vesting account directly:
```bash
pnsd add-genesis-account cosmos1team... 150000000000000000upns \
  --vesting-amount 150000000000000000upns \
  --vesting-start-time 1735689600 \
  --vesting-end-time   1798857600
```
This creates a ContinuousVestingAccount from start to end time.

For more complex schedules (periodic), prefer editing genesis JSON/YAML.

### C) Direct genesis.json edit
After `ignite chain build` or initial `serve`, edit `~/.pns/config/genesis.json` (or the local dev path) adding `cosmos.vesting.v1beta1.*VestingAccount` entries. Then run `pnsd validate-genesis`.

## 6) Data Source and Template
Maintain allocations centrally and generate genesis entries. See `docs/tokenomics.yml` as a template.

## 7) Validation Checklist
- Sum of all allocations equals total supply
- Each vesting account’s periods sum to `original_vesting`
- All addresses valid and funded
- `pnsd validate-genesis` passes
- Queries:
  ```bash
  pnsd q bank balances cosmos1team...
  pnsd q auth account cosmos1team... -o json | jq .
  ```

## 8) Governance and Transparency
- Publish allocation categories and schedules
- Indicate cliffs, linear periods, and any lockups
- Document rationale and any changes via governance proposals

## 9) Next Steps
- Once finalized, wire allocations into `config.yml` for devnets/testnets
- Provide CSV/JSON of beneficiaries if many
- Automate generation with a script that converts `docs/tokenomics.yml` → `config.yml`/`genesis.json`
