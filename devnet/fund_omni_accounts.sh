#!/bin/bash

set -e

ACCOUNTS_DIR="accounts"
FUND_AMOUNT=10000
DEPLOYER_AMOUNT=100000
MINER_PK="$ACCOUNTS_DIR/miner.pk"

extract_address() {
  awk '/^ckb_address:/{print $2}' "$1"
}

fund_address() {
  local to_addr="$1"
  local amount="${2:-$FUND_AMOUNT}"
  echo "Sending ${amount} CKB to $to_addr"
  local out tx_hash status
  out=$(ckb-cli wallet transfer \
    --privkey-path "$MINER_PK" \
    --to-address "$to_addr" \
    --capacity "$amount" \
    --skip-check-to-address 2>&1)
  tx_hash=$(echo "$out" | grep -m 1 -oE '0x[a-f0-9]{64}')
  if [ -z "$tx_hash" ]; then
    echo "Transfer failed: $out" >&2
    return 1
  fi
  echo "  tx $tx_hash — waiting for commit..."
  for _ in $(seq 1 30); do
    status=$(ckb-cli rpc get_transaction --output-format json --hash "$tx_hash" 2>/dev/null \
      | jq -r .tx_status.status 2>/dev/null)
    if [ "$status" = "committed" ]; then
      echo "  committed"
      return 0
    fi
    sleep 1
  done
  echo "Transfer $tx_hash did not commit in time (status=$status)" >&2
  return 1
}

for name in alice bob ingrid alice_default bob_default ingrid_default; do
  fund_address "$(extract_address "$ACCOUNTS_DIR/$name.txt")"
done

# genesis-2 is the deployment-tx funder used by deploy_contracts.sh.
# On a vanilla ckb dev chain it owns no genesis cells, so bootstrap from miner.
fund_address "$(extract_address "$ACCOUNTS_DIR/genesis-2.txt")" "$DEPLOYER_AMOUNT"

echo "All transfers completed."
