#!/bin/bash

set -eu

# Poll ckb-cli sudt get-amount for a non-zero balance on the given account.
# Usage: wait_for_sudt.sh <account-name>
#   e.g. wait_for_sudt.sh alice   reads accounts/alice.txt and waits until
#        Alice's SUDT amount (owner = genesis-2) becomes > 0.

ACCOUNT_NAME="${1:?usage: wait_for_sudt.sh <account-name> [timeout_secs]}"
TIMEOUT_SECS="${2:-${TIMEOUT_SECS:-180}}"
INTERVAL_SECS="${INTERVAL_SECS:-3}"

ACCOUNT_FILE="accounts/${ACCOUNT_NAME}.txt"
SUDT_CELLDEP="system_scripts/sudt-celldep.json"

if [ ! -f "$ACCOUNT_FILE" ]; then
  echo "wait_for_sudt: missing $ACCOUNT_FILE" >&2
  exit 1
fi
if [ ! -f "$SUDT_CELLDEP" ]; then
  echo "wait_for_sudt: missing $SUDT_CELLDEP (run deploy_contracts.sh first)" >&2
  exit 1
fi

GENESIS=$(awk '/^ckb_address:/ {print $2}' accounts/genesis-2.txt)
ADDR=$(awk '/^ckb_address:/ {print $2}' "$ACCOUNT_FILE")

if [ -z "$GENESIS" ] || [ -z "$ADDR" ]; then
  echo "wait_for_sudt: could not extract addresses (GENESIS='$GENESIS' ADDR='$ADDR')" >&2
  exit 1
fi

start_ts=$(date +%s)
while :; do
  out=$(ckb-cli sudt get-amount \
          --owner "$GENESIS" \
          --cell-deps "$SUDT_CELLDEP" \
          --address "$ADDR" 2>/dev/null || true)
  # ckb-cli output has a top-level `total_amount: "<number>"` line; per-cell
  # `amount:` entries are nested under `cells:` and start with whitespace.
  amt=$(echo "$out" | awk '/^total_amount:/ {gsub(/"/, "", $2); print $2; exit}')
  case "$amt" in
    ''|0|0.0)
      ;;
    *)
      echo "wait_for_sudt: $ACCOUNT_NAME has total_amount=$amt"
      exit 0
      ;;
  esac
  now_ts=$(date +%s)
  if [ $((now_ts - start_ts)) -ge "$TIMEOUT_SECS" ]; then
    echo "wait_for_sudt: timeout after ${TIMEOUT_SECS}s waiting for $ACCOUNT_NAME (last amount='$amt')" >&2
    exit 1
  fi
  sleep "$INTERVAL_SECS"
done
