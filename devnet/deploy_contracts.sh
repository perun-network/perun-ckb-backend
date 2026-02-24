#!/bin/bash

set -eu
[ -n "${DEBUG:-}" ] && set -x || true

ACCOUNTS_DIR="accounts"
PERUN_CONTRACTS_DIR="contract"
SYSTEM_SCRIPTS_DIR="system_scripts"
DEVNET_DIR="$PWD"
DEPLOYMENT_INFO_0="info_0"
DEPLOYMENT_INFO_1="info_1"
DEPLOYMENT_INFO_VC="info_vc"
MIGRATION_0="migrations_0/dev"
MIGRATION_1="migrations_1/dev"
MIGRATION_VC="migrations_vc/dev"
genesis=$(awk '/^ckb_address:/ {print $2}' "$ACCOUNTS_DIR/genesis-2.txt")

if [ -f "$DEPLOYMENT_INFO_0.json" ]; then
  rm "$DEPLOYMENT_INFO_0.json"
fi

if [ -f "$DEPLOYMENT_INFO_1.json" ]; then
  rm "$DEPLOYMENT_INFO_1.json"
fi

cd $PERUN_CONTRACTS_DIR

if [ -d "./$MIGRATION_0" ]; then
  rm -f ./$MIGRATION_0/*.json
else
  mkdir -p "./$MIGRATION_0"
fi

if [ -d "./$MIGRATION_1" ]; then
  rm -f ./$MIGRATION_1/*.json
else
  mkdir -p "./$MIGRATION_1"
fi

if [ -d "./$MIGRATION_VC" ]; then
  rm -f ./$MIGRATION_VC/*.json
else
  mkdir -p "./$MIGRATION_VC"
fi

# rm  ./$MIGRATION_VC/*.json

echo "Deploying normal contracts part 1..."
expect << EOF
spawn ckb-cli deploy gen-txs --deployment-config ./deployment/dev/deployment_0.toml --migration-dir ./$MIGRATION_0  --from-address $genesis  --sign-now  --info-file $DEPLOYMENT_INFO_0.json --output-format json
expect "Password:"
send "\r"
expect eof

spawn ckb-cli deploy sign-txs --from-account $genesis --add-signatures --info-file $DEPLOYMENT_INFO_0.json
expect "Password:"
send "\r"
expect eof

spawn ckb-cli deploy apply-txs --migration-dir ./$MIGRATION_0 --info-file $DEPLOYMENT_INFO_0.json
expect eof
EOF

echo "Deploying normal contracts part 1 done."
echo "Waiting for 10 seconds before deploying part 2 contracts..."
sleep 10.0
echo "Deploying normal contracts part 1..."
expect << EOF
spawn ckb-cli deploy gen-txs --deployment-config ./deployment/dev/deployment_1.toml --migration-dir ./$MIGRATION_1  --from-address $genesis  --sign-now  --info-file $DEPLOYMENT_INFO_1.json --output-format json
expect "Password:"
send "\r"
expect eof

spawn ckb-cli deploy sign-txs --from-account $genesis --add-signatures --info-file $DEPLOYMENT_INFO_1.json
expect "Password:"
send "\r"
expect eof

spawn ckb-cli deploy apply-txs --migration-dir ./$MIGRATION_1 --info-file $DEPLOYMENT_INFO_1.json
expect eof
EOF

echo "Deploying normal contracts part 2 done."
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

mv ./$DEPLOYMENT_INFO_0.json ./..
mv ./$DEPLOYMENT_INFO_1.json ./..
mv ./$DEPLOYMENT_INFO_VC.json ./..

echo "Deploying contracts done."
# Fetch default contracts:
cd $DEVNET_DIR
echo "Fetching default contracts..."
if [ -d "$SYSTEM_SCRIPTS_DIR" ]; then
  rm -rf "$SYSTEM_SCRIPTS_DIR"
fi

mkdir -p "$SYSTEM_SCRIPTS_DIR"
offckb system-scripts --export-style ccc | tail -n +2 > "$SYSTEM_SCRIPTS_DIR/default_scripts.json"

echo "Fetching default contracts done."

cd $DEVNET_DIR
echo "Fetching genesis cell..."
timestamp=$(date '+%Y-%m-%d-%H%M%S')

rm ./$DEPLOYMENT_INFO_0.json
rm ./$DEPLOYMENT_INFO_1.json
rm ./$DEPLOYMENT_INFO_VC.json

SUDT_TX_HASH=$(cat ./contract/migrations_0/dev/*.json | jq .cell_recipes[0].tx_hash)
SUDT_TX_INDEX=$(cat ./contract/migrations_0/dev/*.json | jq .cell_recipes[0].index)
SUDT_DATA_HASH=$(cat ./contract/migrations_0/dev/*.json | jq .cell_recipes[0].data_hash)
echo "Fetching genesis cell done."
# TODO: This only works as long as the tx index is 0-9.
jq ".items.sudt.script_id.code_hash = $SUDT_DATA_HASH | .items.sudt.cell_dep.out_point.tx_hash = $SUDT_TX_HASH | .items.sudt.cell_dep.out_point.index = \"0x$SUDT_TX_INDEX\"" ./sudt-celldep-template.json > $SYSTEM_SCRIPTS_DIR/sudt-celldep.json