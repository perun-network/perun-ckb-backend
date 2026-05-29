#!/bin/bash

set -eu

# Wait until every tx_hash recorded in any migrations_*/dev/*.json file is
# committed on-chain. Without this, sudt_helper.sh fund can race the in-flight
# deploy tx and get rejected with PoolRejectedRBF (the deploy tx still holds
# genesis-2's UTXO at the time SUDT issuance composes its tx).

TIMEOUT_SECS="${TIMEOUT_SECS:-180}"
INTERVAL_SECS="${INTERVAL_SECS:-2}"

TX_HASHES=$(jq -r '.. | objects | .tx_hash? // empty' \
    contract/migrations_0/dev/*.json \
    contract/migrations_1/dev/*.json \
    contract/migrations_vc/dev/*.json 2>/dev/null \
    | grep -E '^0x[a-f0-9]{64}$' \
    | sort -u)

if [ -z "$TX_HASHES" ]; then
  echo "wait_for_deploy_committed: no tx hashes found in migration jsons" >&2
  exit 1
fi

echo "wait_for_deploy_committed: tracking $(echo "$TX_HASHES" | wc -l) tx hashes"

start_ts=$(date +%s)
while :; do
  pending=0
  for h in $TX_HASHES; do
    st=$(ckb-cli rpc get_transaction --hash "$h" --output-format json 2>/dev/null \
          | jq -r 'try .tx_status.status catch empty')
    if [ "$st" != "committed" ]; then
      pending=$((pending + 1))
    fi
  done
  if [ "$pending" -eq 0 ]; then
    echo "wait_for_deploy_committed: all deploy txs committed"
    exit 0
  fi
  now_ts=$(date +%s)
  if [ $((now_ts - start_ts)) -ge "$TIMEOUT_SECS" ]; then
    echo "wait_for_deploy_committed: timeout after ${TIMEOUT_SECS}s ($pending tx still pending)" >&2
    exit 1
  fi
  sleep "$INTERVAL_SECS"
done
