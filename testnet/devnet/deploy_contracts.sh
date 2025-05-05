#!/bin/bash

set -eu
[ -n "${DEBUG:-}" ] && set -x || true

ACCOUNTS_DIR="accounts"
PERUN_CONTRACTS_DIR="contracts"
SYSTEM_SCRIPTS_DIR="system_scripts"
DEVNET_DIR="$PWD"
DEPLOYMENT_INFO="info"
DEPLOYMENT_INFO_VC="info_vc"
MIGRATION="migrations/dev"
MIGRATION_VC="migrations_vc/dev"
genesis=$( cat ./accounts/genesis-2.txt | awk '/testnet/ {print$2}' | head -n 1)

if [ -f "$DEPLOYMENT_INFO.json"]; then
  rm "$DEPLOYMENT_INFO.json"
fi

cd $PERUN_CONTRACTS_DIR

if [ -d "./$MIGRATION" ]; then
  rm -f ./$MIGRATION/*.json
else
  mkdir -p "./$MIGRATION"
fi

if [ -d "./$MIGRATION_VC" ]; then
  rm -f ./$MIGRATION_VC/*.json
else
  mkdir -p "./$MIGRATION_VC"
fi

# rm  ./$MIGRATION_VC/*.json

echo "Deploying normal contracts..."
expect << EOF
spawn ckb-cli deploy gen-txs --deployment-config ./deployment/dev/deployment.toml --migration-dir ./$MIGRATION  --from-address $genesis  --sign-now  --info-file $DEPLOYMENT_INFO.json
expect "Password:"
send "\r"
expect eof

spawn ckb-cli deploy sign-txs --from-account $genesis --add-signatures --info-file $DEPLOYMENT_INFO.json
expect "Password:"
send "\r"
expect eof

spawn ckb-cli deploy apply-txs --migration-dir ./$MIGRATION --info-file $DEPLOYMENT_INFO.json
expect eof
EOF

echo "Deploying normal contracts done."
echo "Waiting for 10 seconds before deploying vc contracts..."
sleep 10.0
echo "Deplyoing vc contracts..."
expect << EOF
spawn ckb-cli deploy gen-txs --deployment-config ./deployment/dev/deployment_vc.toml --migration-dir ./$MIGRATION_VC  --from-address $genesis  --sign-now  --info-file $DEPLOYMENT_INFO_VC.json
expect "Password:"
send "\r"
expect eof

spawn ckb-cli deploy sign-txs --from-account $genesis --add-signatures --info-file $DEPLOYMENT_INFO_VC.json
expect "Password:"
send "\r"
expect eof

spawn ckb-cli deploy apply-txs --migration-dir ./$MIGRATION_VC --info-file $DEPLOYMENT_INFO_VC.json
expect eof
EOF

mv ./$DEPLOYMENT_INFO.json ./..
mv ./$DEPLOYMENT_INFO_VC.json ./..

echo "Deploying contracts done."
# Fetch default contracts:
cd $DEVNET_DIR
echo "Fetching default contracts..."
if [ -d "$SYSTEM_SCRIPTS_DIR" ]; then
  rm -rf "$SYSTEM_SCRIPTS_DIR"
fi

mkdir -p "$SYSTEM_SCRIPTS_DIR"
## jq will interpret the code_hash and tx_hash as numbers, so we need to wrap them in quotes.
## The index must also be a string value, but yaml does not support hex values as a top level block argument
## so we have to do that in a second pass...
ckb-cli util genesis-scripts --output-format json \
  | sed 's/code_hash: \(.*\)/code_hash: \"\1\"/; s/tx_hash: \(.*\)/tx_hash: \"\1\"/' \
  | sed 's/"index": \([0-9]\+\),/echo "\\"index\\": $(python3 -c "print(\\\"\\\\\\"{}\\\\\\"\\\".format(hex(\1)))"),";/e' \
  | jq . > "$SYSTEM_SCRIPTS_DIR/default_scripts.json"

echo "Fetching default contracts done."

cd $DEVNET_DIR
echo "Fetching genesis cell..."
timestamp=$(date '+%Y-%m-%d-%H%M%S')

rm ./$DEPLOYMENT_INFO.json
rm ./$DEPLOYMENT_INFO_VC.json

SUDT_TX_HASH=$(cat ./contracts/migrations/dev/*.json | jq .cell_recipes[0].tx_hash)
SUDT_TX_INDEX=$(cat ./contracts/migrations/dev/*.json | jq .cell_recipes[0].index)
SUDT_DATA_HASH=$(cat ./contracts/migrations/dev/*.json | jq .cell_recipes[0].data_hash)
echo "Fetching genesis cell done."
# TODO: This only works as long as the tx index is 0-9.
jq ".items.sudt.script_id.code_hash = $SUDT_DATA_HASH | .items.sudt.cell_dep.out_point.tx_hash = $SUDT_TX_HASH | .items.sudt.cell_dep.out_point.index = \"0x$SUDT_TX_INDEX\"" ./sudt-celldep-template.json > $SYSTEM_SCRIPTS_DIR/sudt-celldep.json
