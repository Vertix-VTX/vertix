#!/usr/bin/env sh
# Single-node gaia genesis bootstrap. Funds the relayer account from a fixed mnemonic.
set -e
CHAIN_ID="${GAIA_CHAIN_ID:-gaia-devnet-1}"
HOME_DIR=/root/.gaia
RELAYER_MNEMONIC="$1"   # passed by compose from mnemonics.env (MNEMONIC_RELAYER)

if [ ! -f "$HOME_DIR/config/genesis.json" ]; then
  gaiad init gaia --chain-id "$CHAIN_ID" --home "$HOME_DIR"
  echo "$RELAYER_MNEMONIC" | gaiad keys add relayer --recover --keyring-backend test --home "$HOME_DIR"
  gaiad keys add val --keyring-backend test --home "$HOME_DIR"
  gaiad genesis add-genesis-account val 1000000000stake,1000000000uatom --keyring-backend test --home "$HOME_DIR"
  gaiad genesis add-genesis-account relayer 1000000000uatom --keyring-backend test --home "$HOME_DIR"
  gaiad genesis gentx val 700000000stake --chain-id "$CHAIN_ID" --keyring-backend test --home "$HOME_DIR"
  gaiad genesis collect-gentxs --home "$HOME_DIR"
  sed -i 's|^laddr = "tcp://127.0.0.1:26657"|laddr = "tcp://0.0.0.0:26657"|' "$HOME_DIR/config/config.toml"
  sed -i 's|^minimum-gas-prices = ""|minimum-gas-prices = "0.0uatom"|' "$HOME_DIR/config/app.toml"
fi
exec gaiad start --home "$HOME_DIR"
