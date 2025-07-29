#!/bin/bash

set -e

ACCOUNTS_DIR="accounts"
FUND_AMOUNT=10000  # Amount in CKB

# Extract CKB address from .txt file (line starting with ckb_address:)
extract_address() {
  local file="$1"
  grep '^ckb_address:' "$file" | awk '{print $2}'
}
# Fund a recipient using offckb transfer
fund_address() {
  local to_addr="$1"
  local privkey_path="$ACCOUNTS_DIR/genesis-1.pk"
  local privkey=$(head -n 1 "$privkey_path")
  echo "${privkey}"

  echo "Sending ${FUND_AMOUNT} CKB to $to_addr"
  local output
    output=$(offckb transfer --privkey "$privkey" "$to_addr" "$FUND_AMOUNT" 2>&1)

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

# Fund each account
fund_address "$alice"
sleep 5
fund_address "$bob"
sleep 5
fund_address "$ingrid"
sleep 5
fund_address "$alice_def"
sleep 5
fund_address "$bob_def"

echo "✅ All transfers completed."
