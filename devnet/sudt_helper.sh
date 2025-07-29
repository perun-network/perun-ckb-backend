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

check_files "$ACCOUNTS_DIR/alice.txt" "$ACCOUNTS_DIR/bob.txt" "$ACCOUNTS_DIR/ingrid.txt" "$ACCOUNTS_DIR/genesis-2.txt" "$SYSTEM_SCRIPTS_DIR/sudt-celldep.json"

ALICE=$(awk '/^ckb_address:/ {print $2}' "$ACCOUNTS_DIR/alice.txt")
BOB=$(awk '/^ckb_address:/ {print $2}' "$ACCOUNTS_DIR/bob.txt")
INGRID=$(awk '/^ckb_address:/ {print $2}' "$ACCOUNTS_DIR/ingrid.txt")
GENESIS=$(awk '/^ckb_address:/ {print $2}' "$ACCOUNTS_DIR/genesis-2.txt")

fund_genesis() {
  echo "Funding accounts for Alice, Bob and Ingrid with SUDT tokens"
  SUDT_AMOUNT=400000000

  expect << EOF
  spawn ckb-cli sudt issue --owner $GENESIS --udt-to $GENESIS:$SUDT_AMOUNT --cell-deps $SYSTEM_SCRIPTS_DIR/sudt-celldep.json
  expect "owner Password:"
  send "\r"
  expect eof
EOF
}

list_accounts_balances() {
  echo "Listing SUDT account balances for Alice, Bob and Ingrid"
  echo "ALICE: ========================================"
  ckb-cli sudt get-amount --owner $GENESIS --cell-deps $SYSTEM_SCRIPTS_DIR/sudt-celldep.json --address $ALICE
  echo "BOB: ========================================"
  ckb-cli sudt get-amount --owner $GENESIS --cell-deps $SYSTEM_SCRIPTS_DIR/sudt-celldep.json --address $BOB
  echo "INGRID: ========================================"
  ckb-cli sudt get-amount --owner $GENESIS --cell-deps $SYSTEM_SCRIPTS_DIR/sudt-celldep.json --address $INGRID
  echo "GENESIS: ========================================"
    ckb-cli sudt get-amount --owner $GENESIS --cell-deps $SYSTEM_SCRIPTS_DIR/sudt-celldep.json --address $GENESIS
  echo "============================================="
}

if [ $# -eq 0 ]; then
  echo "No arguments provided. Please provide one of the following:"
  echo "  balances: list SUDT account balances for Alice, Bob and Ingrid"
  echo "  fund: fund Alice, Bob and Ingrid with SUDT tokens"
  exit 1
fi

for arg in "$@"; do
  case $arg in
  balances)
    list_accounts_balances
    ;;
  fund)
    fund_genesis
    ;;
  *)
    echo "Unknown argument: $arg"
    echo "Usage: $0 [balances|fund]"
    exit 1
    ;;
  esac
done
