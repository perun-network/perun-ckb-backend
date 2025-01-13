#!/bin/bash

ACCOUNTS_DIR="accounts"
SYSTEM_SCRIPTS_DIR="system_scripts"

# If any of the listed files is missing, exit with an error.
check_files() {
  for file in "$@"; do
    if [ ! -f "$file" ]; then
      echo "File $file not found. Please run $PWD/print_accounts.sh first and"
      echo "make sure $PWD/deploy_contracts.sh has been run successfully."
      exit 1
    fi
  done
}

check_files "$ACCOUNTS_DIR/alice.txt" "$ACCOUNTS_DIR/bob.txt" "$ACCOUNTS_DIR/genesis-2.txt" "$SYSTEM_SCRIPTS_DIR/sudt-celldep.json"

ALICE=$(cat $ACCOUNTS_DIR/alice.txt | awk '/testnet/ { count++; if (count == 1) print $2}')
BOB=$(cat $ACCOUNTS_DIR/bob.txt | awk '/testnet/ { count++; if (count == 1) print $2}')
genesis=$(cat $ACCOUNTS_DIR/genesis-2.txt | awk '/testnet/ { count++; if (count == 2) print $2}')
GENESIS=$(cat $ACCOUNTS_DIR/genesis-2.txt | awk '/testnet/ { count++; if (count == 1) print $2}')

fund_accounts() {
  echo "Funding accounts for Alice and Bob with SUDT tokens"
  SUDT_AMOUNT=100000000

  expect << EOF
  spawn ckb-cli sudt issue --owner $GENESIS --udt-to $ALICE:$SUDT_AMOUNT $BOB:$SUDT_AMOUNT --cell-deps $SYSTEM_SCRIPTS_DIR/sudt-celldep.json
  expect "owner Password:"
  send "\r"
  expect eof
EOF
}

list_accounts_balances() {
  echo "Listing SUDT account balances for Alice and Bob"
  echo "ALICE: ========================================"
  ckb-cli sudt get-amount --owner $GENESIS --cell-deps $SYSTEM_SCRIPTS_DIR/sudt-celldep.json --address $ALICE
  echo "BOB: ========================================"
  ckb-cli sudt get-amount --owner $GENESIS --cell-deps $SYSTEM_SCRIPTS_DIR/sudt-celldep.json --address $BOB
  echo "============================================="
}

if [ $# -eq 0 ]; then
  echo "No arguments provided. Please provide one of the following:"
  echo "  balances: list SUDT account balances for Alice and Bob"
  echo "  fund: fund Alice and Bob with SUDT tokens"
  exit 1
fi

for arg in "$@"; do
  case $arg in
  balances)
    list_accounts_balances
    ;;
  fund)
    fund_accounts
    ;;
  *)
    echo "Unknown argument: $arg"
    echo "Usage: $0 [balances|fund]"
    exit 1
    ;;
  esac
done
