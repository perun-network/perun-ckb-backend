#!/bin/bash

# Pane 5 of devnet-session.yaml — the sole place where deploy + SUDT funding
# happens. Linearised under `set -e` so any failure aborts before the sentinel
# is written; otherwise downstream tests would race a half-initialised chain.

set -euo pipefail
[ -n "${DEBUG:-}" ] && set -x || true

./wait_for_rpc.sh
./deploy_contracts.sh
./wait_for_file.sh ./contract/migrations_vc/dev
./wait_for_deploy_committed.sh
./sudt_helper.sh fund
./wait_for_sudt.sh genesis-2
go run -tags fundsudt ../cmd/fund_sudt.go
./wait_for_sudt.sh alice
./sudt_helper.sh balances
touch .devnet-ready
echo "DEVNET READY"
