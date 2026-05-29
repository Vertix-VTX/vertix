# Vertix load test harness (`tm-load-test`)

Operator-run throughput baseline for the Phase 6 Docker devnet. Phase 7 is **baseline capture only** — no pass/fail TPS gate (see [`docs/load-test.md`](../../docs/load-test.md)).

## Prerequisites

1. **Devnet running** — from the repo root:

   ```bash
   make localnet-up
   ```

   Wait until blocks advance (`curl -s http://localhost:26657/status | jq -r '.result.sync_info.latest_block_height'`). Full stack details: [`docs/devnet.md`](../../docs/devnet.md).

2. **`tm-load-test` installed** on your `PATH`:

   ```bash
   go install github.com/informalsystems/tm-load-test/cmd/tm-load-test@latest
   ```

   Upstream has moved to [cometbft/cometbft-load-test](https://github.com/cometbft/cometbft-load-test); the `informalsystems/tm-load-test` module remains the documented install path for this harness.

3. **Host resources** — enough CPU/RAM for the devnet containers plus the load generator (see devnet runbook).

## Configuration

Default parameters live in [`loadtest.toml`](./loadtest.toml) and mirror the Makefile `LOADTEST_*` variables:

| Parameter | Makefile variable | Default |
|-----------|-------------------|---------|
| Endpoint | `LOADTEST_ENDPOINT` | `ws://localhost:26657/websocket` |
| Duration (s) | `LOADTEST_DURATION` | `60` |
| Rate (tx/s) | `LOADTEST_RATE` | `200` |
| Connections | `LOADTEST_CONNS` | `4` |

Override at invoke time, for example:

```bash
make load-test LOADTEST_RATE=100 LOADTEST_DURATION=120
```

## Run

```bash
make load-test
```

Optional CSV aggregate stats (if your `tm-load-test` build supports `--stats-output`):

```bash
mkdir -p infra/loadtest/results
tm-load-test -c 4 -T 60 -r 200 -s 250 \
  --broadcast-tx-method sync \
  --endpoints ws://localhost:26657/websocket \
  --stats-output infra/loadtest/results/stats.csv
```

After a successful run, copy sustained TPS, latency percentiles, and observed block time into [`docs/load-test.md`](../../docs/load-test.md) (hardware/topology section included).

## Known limitation (Vertix transaction format)

`tm-load-test` defaults to the **kvstore** ABCI client. Vertix is a Cosmos SDK application (`vertix-1`); submissions use that client unless you implement a [custom loadtest client](https://github.com/informalsystems/tm-load-test/tree/master/pkg/loadtest). Expect many rejects or non-representative behavior until a Vertix-specific client exists.

Phase 7 still delivers:

- A repeatable **RPC load harness** (`make load-test` + documented parameters).
- A **documented baseline slot** in `docs/load-test.md` filled by the operator after a real run.

For module-realistic mixes (bank send, oracle feed, RWA ops), use a custom client or a `vertixd tx` flood script in a follow-up (Phase 9).

## Teardown

```bash
make localnet-down
```
