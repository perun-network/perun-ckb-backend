#!/bin/bash

set -e

ACCOUNTS_DIR="accounts"
FUND_AMOUNT_ALICE=${FUND_AMOUNT_ALICE:-100000}
FUND_AMOUNT_BOB=${FUND_AMOUNT_BOB:-100000}
FUND_AMOUNT_INGRID=${FUND_AMOUNT_INGRID:-50000}
FUND_AMOUNT_DEFAULT=${FUND_AMOUNT_DEFAULT:-20000}

# Extract CKB address from .txt file (line starting with ckb_address:)
extract_address() {
  local file="$1"
  grep '^ckb_address:' "$file" | awk '{print $2}'
}
# Fund a recipient using offckb transfer
fund_address() {
  local to_addr="$1"
  local amount="$2"
  local privkey_path="$ACCOUNTS_DIR/genesis-1.pk"
  local privkey=$(head -n 1 "$privkey_path")
  echo "${privkey}"

  echo "Sending ${amount} CKB to $to_addr"
  local output
    output=$(offckb transfer --privkey "$privkey" "$to_addr" "$amount" 2>&1)

    echo "$output"
  tx_hash=$(echo "$output" | grep -oE 'txHash: 0x[a-f0-9]{64}' | awk '{print $2}')

  # Wait for it to commit using ckb-cli (or poll via JSON-RPC if you prefer)
  if [ -n "$tx_hash" ]; then
    echo "⏳ Waiting for tx to commit: $tx_hash"
    while true; do
      response=$(ckb-cli rpc get_transaction --output-format json --hash "$tx_hash")
      status=$(echo "$response" | jq -r .tx_status.status)
      if [ "$status" == "committed" ]; then
        echo "✅ Tx committed: $tx_hash"
        break
      fi
      sleep 2
    done
  fi
}

# Extract addresses
alice=$(extract_address "$ACCOUNTS_DIR/alice.txt")
bob=$(extract_address "$ACCOUNTS_DIR/bob.txt")
ingrid=$(extract_address "$ACCOUNTS_DIR/ingrid.txt")
alice_def=$(extract_address "$ACCOUNTS_DIR/alice_default.txt")
bob_def=$(extract_address "$ACCOUNTS_DIR/bob_default.txt")
ingrid_def=$(extract_address "$ACCOUNTS_DIR/ingrid_default.txt")

# Fund each account
fund_address "$alice" "$FUND_AMOUNT_ALICE"
sleep 5
fund_address "$bob" "$FUND_AMOUNT_BOB"
sleep 5
fund_address "$ingrid" "$FUND_AMOUNT_INGRID"
sleep 5
fund_address "$alice_def" "$FUND_AMOUNT_DEFAULT"
sleep 5
fund_address "$bob_def" "$FUND_AMOUNT_DEFAULT"
sleep 5
fund_address "$ingrid_def" "$FUND_AMOUNT_DEFAULT"

echo "✅ All transfers completed."
