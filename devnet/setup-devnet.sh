#!/bin/bash

set -eu
[ -n "${DEBUG:-}" ] && set -x || true

# This script sets up the devnet for CKB.
# Part of the setup are a miner, three accounts Alice, Bob, and Ingrid, as well as the
# registration of two accounts governing the genesis cells.

ACCOUNTS_DIR="accounts"
PERUN_CONTRACTS_DIR="contract"

mkdir -p $ACCOUNTS_DIR

rm -rf ~/.ckb-cli ~/.ckb/keystore/

if [ -d $ACCOUNTS_DIR ]; then
  rm -rf $ACCOUNTS_DIR/miner.pk $ACCOUNTS_DIR/miner.txt $ACCOUNTS_DIR/genesis-1.pk $ACCOUNTS_DIR/genesis-1.txt $ACCOUNTS_DIR/genesis-2.pk $ACCOUNTS_DIR/genesis-2.txt $ACCOUNTS_DIR/sudt-owner-lock-hash1.txt $ACCOUNTS_DIR/sudt-owner-lock-hash2.txt
fi

if [ -d "data" ]; then
  rm -rf "data"
fi

if [ -d "specs" ]; then
  rm -rf "specs"
fi

if [ -f "ckb-miner.toml" ]; then
  rm "ckb-miner.toml"
fi

if [ -f "ckb.toml" ]; then
  rm "ckb.toml"
fi

if [ -f "default.db-options" ]; then
  rm "default.db-options"
fi

# Drop any stale readiness sentinel from a previous run.
rm -f .devnet-ready

# Patch offckb's bundled ckb-miner.toml to 1s blocks (one-time bootstrap).
# `offckb node` runs its own miner with this toml; the default 5s block time is
# too slow for go-perun's dispute tests (see CI commit 5e6c0db). The local
# ckb-miner.toml created by `ckb init` below is patched separately for pane 2.
OFFCKB_DEVNET_DIR="$HOME/.local/share/offckb-nodejs/devnet"
OFFCKB_MINER_TOML="$OFFCKB_DEVNET_DIR/ckb-miner.toml"
if [ ! -f "$OFFCKB_MINER_TOML" ] || ! grep -q '^value = 1000' "$OFFCKB_MINER_TOML"; then
  echo "Bootstrapping offckb to create+patch its ckb-miner.toml..."
  ( nohup offckb node >/tmp/offckb-bootstrap.log 2>&1 & )
  for i in $(seq 1 30); do
    [ -f "$OFFCKB_MINER_TOML" ] && break
    sleep 1
  done
  # Graceful shutdown first (SIGINT lets ckb release the rocksdb lock), then SIGKILL fallback.
  pkill -INT -f 'offckb-nodejs/bins.*/ckb' 2>/dev/null || true
  pkill -INT -f 'offckb node' 2>/dev/null || true
  sleep 5
  pkill -KILL -f 'offckb-nodejs/bins.*/ckb' 2>/dev/null || true
  pkill -KILL -f 'offckb node' 2>/dev/null || true
  sleep 2
  if [ -f "$OFFCKB_MINER_TOML" ]; then
    sed -i 's/^value = 5000/value = 1000/' "$OFFCKB_MINER_TOML"
    echo "Patched $OFFCKB_MINER_TOML to 1s blocks"
  else
    echo "offckb miner toml still missing; continuing without patch" >&2
  fi
fi

# Always wipe offckb's chain data so pane 1's `offckb node` starts from a fresh
# rocksdb (no stale LOCK from a prior crash/kill). Keeps the patched toml.
if [ -d "$OFFCKB_DEVNET_DIR/data" ]; then
  rm -rf "$OFFCKB_DEVNET_DIR/data"
  echo "Wiped $OFFCKB_DEVNET_DIR/data for clean start"
fi

# Build all required contract for Perun.
DEVNET=$(pwd)
cd $PERUN_CONTRACTS_DIR
source ./setup_env.sh build && make build
cd $DEVNET
offckb accounts > accounts.txt

MinerPK=$(grep -A 4 '"#": 17' accounts.txt | grep 'privkey' | awk '{print $2}')
GenCellOnePK=$(grep -A 4 '"#": 18' accounts.txt | grep 'privkey' | awk '{print $2}')
GenCellTwoPK=$(grep -A 4 '"#": 19' accounts.txt | grep 'privkey' | awk '{print $2}')

GenCellOnePubKey=$(grep -A 4 '"#": 18' accounts.txt | grep 'pubkey' | awk '{print $2}')
GenCellTwoPubKey=$(grep -A 4 '"#": 19' accounts.txt | grep 'pubkey' | awk '{print $2}')
# Similarly, extract the addresses
MinerAddress=$(grep -A 1 '"#": 17' accounts.txt | grep 'address' | awk '{print $2}')
GenCellOneAddress=$(grep -A 1 '"#": 18' accounts.txt | grep 'address' | awk '{print $2}')
GenCellTwoAddress=$(grep -A 1 '"#": 19' accounts.txt | grep 'address' | awk '{print $2}')

# Extract lock_args
MinerLockArg=$(grep -A 6 '"#": 17' accounts.txt | grep 'lock_arg' | awk '{print $2}')
GenCellOneLockArg=$(grep -A 6 '"#": 18' accounts.txt | grep 'lock_arg' | awk '{print $2}')
GenCellTwoLockArg=$(grep -A 6 '"#": 19' accounts.txt | grep 'lock_arg' | awk '{print $2}')

# Extract fields for account #18 (GenCellOne)
GenCellOneCodeHash=$(awk '/"#": 18/,/^$/' accounts.txt | grep 'codeHash:' | awk '{print $2}')
GenCellOneHashType=$(awk '/"#": 18/,/^$/' accounts.txt | grep 'hashType:' | awk '{print $2}')

# Extract fields for account #19 (GenCellTwo)
GenCellTwoCodeHash=$(awk '/"#": 19/,/^$/' accounts.txt | grep 'codeHash:' | awk '{print $2}')
GenCellTwoHashType=$(awk '/"#": 19/,/^$/' accounts.txt | grep 'hashType:' | awk '{print $2}')


echo "getting lock hash $GenCellOneCodeHash type $GenCellOneHashType lockarg $GenCellOneLockArg"
ONE_LOCK_HASH=$(ckb-cli util key-info --pubkey "$GenCellOnePubKey" | grep 'lock_hash:' | awk '{print $2}')
TWO_LOCK_HASH=$(ckb-cli util key-info --pubkey "$GenCellTwoPubKey" | grep 'lock_hash:' | awk '{print $2}')
echo "Lock hash one: $ONE_LOCK_HASH"
echo "Lock hash two: $TWO_LOCK_HASH"

# Create accounts for genesis cells.
echo -e "lock_hash: $ONE_LOCK_HASH\nlock_arg: $GenCellOneLockArg\nckb_address: $GenCellOneAddress" > "$ACCOUNTS_DIR/genesis-1.txt"
echo -e "lock_hash: $TWO_LOCK_HASH\nlock_arg: $GenCellTwoLockArg\nckb_address: $GenCellTwoAddress" > "$ACCOUNTS_DIR/genesis-2.txt"
echo -e "eth_address: $MinerAddress\nlock_arg: $MinerLockArg\nckb_address: $MinerAddress" > "$ACCOUNTS_DIR/miner.txt"

# Also save private keys
echo "$GenCellOnePK" > "$ACCOUNTS_DIR/genesis-1.pk"
echo "$GenCellTwoPK" > "$ACCOUNTS_DIR/genesis-2.pk"
echo "$MinerPK" > "$ACCOUNTS_DIR/miner.pk"
echo -e "\n\n" | ckb-cli account import --privkey-path $ACCOUNTS_DIR/genesis-1.pk >/dev/null 2>&1 || true
echo -e "\n\n" | ckb-cli account import --privkey-path $ACCOUNTS_DIR/genesis-2.pk >/dev/null 2>&1 || true
echo -e "\n\n" | ckb-cli account import --privkey-path $ACCOUNTS_DIR/miner.pk >/dev/null 2>&1 || true
MINER_LOCK_ARG=$MinerLockArg

rm -rf accounts.txt

ckb init --chain dev --ba-arg $MINER_LOCK_ARG --ba-message "0x" --force

# Make the scripts owned by the miner.
sed -i "s/args =.*$/args = \"$MINER_LOCK_ARG\"/" $PERUN_CONTRACTS_DIR/deployment/dev/deployment_0.toml
# Use the debug versions of the contract.
# sed -i "s/release/debug/" $PERUN_CONTRACTS_DIR/deployment/dev/deployment.toml

# Adjust miner config to process blocks faster.
sed -i 's/value = 5000/value = 1000/' ckb-miner.toml

# Fast mining config
sed -i '/\[mining\]/a always_submit_block = true' ckb.toml

# Reduce epoch length to 10 blocks.
sed -i 's/genesis_epoch_length = 1000/genesis_epoch_length = 10/' specs/dev.toml
sed -i '/\[params\]/a\
max_block_bytes = 100_000_000' specs/dev.toml

# Enable the indexer.
sed -i '/"Debug"]/ s/"Debug"]/"Debug", "Indexer"]/' ckb.toml
sed -i '/filter = "info"/ s/filter = "info"/filter = "debug"/' ckb.toml
sed -i 's/max_tx_verify_cycles = 70_000_000/max_tx_verify_cycles = 100_000_000/' ckb.toml
# Increase max_request_body_size to allow for debug contracts (large in size)
# to be deployed.
sed -i 's/max_request_body_size =.*$/max_request_body_size = 104857600/' ckb.toml
