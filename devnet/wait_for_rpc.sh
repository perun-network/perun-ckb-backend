#!/bin/bash

set -eu

RPC_URL=${RPC_URL:-http://127.0.0.1:8114/}
TIMEOUT_SECS=${TIMEOUT_SECS:-120}
INTERVAL_SECS=${INTERVAL_SECS:-2}

start_ts=$(date +%s)
while true; do
  if curl -s -m 2 -H "Content-Type: application/json" \
    --data '{"id":1,"jsonrpc":"2.0","method":"get_blockchain_info","params":[]}' \
    "$RPC_URL" >/dev/null; then
    echo "RPC ready at $RPC_URL"
    exit 0
  fi
  now_ts=$(date +%s)
  if [ $((now_ts - start_ts)) -ge "$TIMEOUT_SECS" ]; then
    echo "RPC not ready after $TIMEOUT_SECS seconds: $RPC_URL" >&2
    exit 1
  fi
  sleep "$INTERVAL_SECS"
done
