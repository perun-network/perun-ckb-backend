#!/bin/bash

set -eu
[ -n "${DEBUG:-}" ] && set -x || true

# This script sets up the devnet for CKB.
# Part of the setup are a miner, two accounts Alice and Bob, as well as the
# registration of two accounts governing the genesis cells.

ACCOUNTS_DIR="accounts"
PERUN_CONTRACTS_DIR="contracts"

if [ -d $ACCOUNTS_DIR ]; then
  rm -rf $ACCOUNTS_DIR/*
fi
mkdir -p $ACCOUNTS_DIR

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

# Build all required contracts for Perun.
DEVNET=$(pwd)
cd $PERUN_CONTRACTS_DIR
capsule build --release
# If debug contracts are wanted:
# capsule build
cd $DEVNET

# Genesis cell #1
GenCellOnePK="0xd00c06bfd800d27397002dca6fb0993d5ba6399b4238b2f29ee9deb97593d2bc"
GenCellOneLockArg="0xc8328aabcd9b9e8e64fbc566c4385c3bdeb219d7"
GenCellOneAddress="ckt1qyqvsv5240xeh85wvnau2eky8pwrhh4jr8ts8vyj37"
# Genesis cell #2
GenCellTwoPK="0x63d86723e08f0f813a36ce6aa123bb2289d90680ae1e99d4de8cdb334553f24d"
GenCellTwoLockArg="0x470dcdc5e44064909650113a274b3b36aecb6dc7"
GenCellTwoAddress="ckt1qyqywrwdchjyqeysjegpzw38fvandtktdhrs0zaxl4"

create_account() {
  echo -e '\n\n' | ckb-cli account new  > $ACCOUNTS_DIR/$1.txt
}

# Create accounts for genesis cells.
touch privateKeyGenesisCells.txt
echo $GenCellOnePK > privateKeyGenesisCells.txt
echo -e '\n\n' | ckb-cli account import --privkey-path privateKeyGenesisCells.txt || true
ckb-cli account list | grep -B 5 -A 4 "$GenCellOneAddress" > $ACCOUNTS_DIR/genesis-1.txt
echo $GenCellTwoPK > privateKeyGenesisCells.txt
echo -e '\n\n' | ckb-cli account import --privkey-path privateKeyGenesisCells.txt || true
ckb-cli account list | grep -B 5 -A 4 "$GenCellTwoAddress" > $ACCOUNTS_DIR/genesis-2.txt
rm privateKeyGenesisCells.txt

echo -e '\n\n' |  ckb-cli account new > $ACCOUNTS_DIR/miner.txt
MINER_LOCK_ARG=$(cat $ACCOUNTS_DIR/miner.txt | awk '/lock_arg/ {print $2}')

create_account "alice"
create_account "bob"

ckb init --chain dev --ba-arg $MINER_LOCK_ARG --ba-message "0x" --force

# Make the scripts owned by the miner.
sed -i "s/args =.*$/args = \"$MINER_LOCK_ARG\"/" $PERUN_CONTRACTS_DIR/deployment/dev/deployment.toml
# Use the debug versions of the contracts.
# sed -i "s/release/debug/" $PERUN_CONTRACTS_DIR/deployment/dev/deployment.toml

# Adjust miner config to process blocks faster.
sed -i 's/value = 5000/value = 1000/' ckb-miner.toml

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
