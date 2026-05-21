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
DEPLOYMENT_INFO_LP="info_lp"
MIGRATION_0="migrations_0/dev"
MIGRATION_1="migrations_1/dev"
MIGRATION_VC="migrations_vc/dev"
MIGRATION_LP="migrations_lp/dev"
genesis=$(awk '/^ckb_address:/ {print $2}' "$ACCOUNTS_DIR/genesis-2.txt")
GENESIS_PRIVKEY="$DEVNET_DIR/$ACCOUNTS_DIR/genesis-2.pk"
MINER_PRIVKEY="$DEVNET_DIR/$ACCOUNTS_DIR/miner.pk"

# Remove old info files in repo root if present
for f in "$DEPLOYMENT_INFO_0.json" "$DEPLOYMENT_INFO_1.json" "$DEPLOYMENT_INFO_VC.json" "$DEPLOYMENT_INFO_LP.json"; do
  [ -f "$f" ] && rm -f "$f"
done

cd "$PERUN_CONTRACTS_DIR"

# Ensure migration directories exist but DO NOT delete previous artifacts yet.
mkdir -p "$MIGRATION_0" "$MIGRATION_1" "$MIGRATION_VC"

# Helper: parse required capacity from gen-txs stderr
parse_required_capacity() {
  local text="$1"
  echo "$text" | grep -oE 'value=[0-9]+(\.[0-9]+)?' | head -n1 | sed 's/value=//'
}

# Run a deploy phase safely: generate txs into a tmp dir, sign non-interactively, apply, then move artifacts into place.
run_deploy_phase() {
  local name="$1"
  local deployment_toml="$2"
  local migration_dir="$3"  # relative path inside contract dir
  local info_file="$4"      # absolute path to write info file to (we use devnet root)
  local signer_privkey="$5" # absolute path to privkey used for signing
  local funder_privkey="$6" # absolute path to privkey used to fund genesis if needed

  echo "=== Deploy phase: $name ==="
  # remove old artifacts and logs for a clean run
  if [ -f "$DEVNET_DIR/deploy_${name}_gen-txs.log" ]; then
    echo "Removing old log $DEVNET_DIR/deploy_${name}_gen-txs.log"
    rm -f "$DEVNET_DIR/deploy_${name}_gen-txs.log" || true
  fi
  if [ -f "$info_file" ]; then
    echo "Removing old info file $info_file"
    rm -f "$info_file" || true
  fi
  if [ -d "$migration_dir" ]; then
    if compgen -G "$migration_dir/*.json" > /dev/null 2>&1; then
      echo "Removing old migration artifacts in $migration_dir"
      rm -f "$migration_dir"/*.json || true
    fi
  else
    mkdir -p "$migration_dir"
  fi
  # temp dir for this run
  TMP_MIG_DIR=$(mktemp -d "${migration_dir//\//_}.XXXX" 2>/dev/null || mktemp -d "/tmp/${name}_migration.XXXX")
  echo "Using temporary migration dir: $TMP_MIG_DIR"

  [ -f "$info_file" ] && rm -f "$info_file"

  local attempts=0
  local max_attempts=3
  while true; do
    attempts=$((attempts+1))
    echo "Running gen-txs (attempt $attempts)..."
    output=$(ckb-cli deploy gen-txs --deployment-config "$deployment_toml" --migration-dir "$TMP_MIG_DIR" --from-address "$genesis" --info-file "$info_file" --output-format json 2>&1) || rc=$?
    rc=${rc:-0}
    if [ $rc -eq 0 ]; then
      echo "gen-txs succeeded"
      break
    fi
    echo "gen-txs failed:" >&2
    echo "$output" >&2
    # Save output for debugging
    echo "$output" > "$DEVNET_DIR/deploy_${name}_gen-txs.log" || true

    # If gen-txs wrote migration JSONs or an info file, treat as success.
    if compgen -G "$TMP_MIG_DIR/*.json" > /dev/null 2>&1; then
      echo "gen-txs produced migration JSON files; treating as success"
      break
    fi
    if [ -f "$info_file" ] && [ -s "$info_file" ]; then
      echo "info-file $info_file present; treating gen-txs as success"
      break
    fi
    if echo "$output" | grep -qi "status: *success"; then
      echo "gen-txs reported success in output; treating as success"
      break
    fi

    required=$(parse_required_capacity "$output" || true)
    if [ -n "$required" ]; then
      echo "Detected required additional capacity: $required CKB"
      # Compute an amount to fund (ceil(required*1.05) + buffer)
      fund_amount=$(python3 - <<PY
import math
req=float($required)
amt=math.ceil(req*1.05)+1000
print(int(amt))
PY
)
      echo "Funding genesis with $fund_amount CKB from miner ($funder_privkey)"
      txjson=$(ckb-cli wallet transfer --privkey-path "$funder_privkey" --to-address "$genesis" --capacity "$fund_amount" --output-format json 2>&1) || true
      echo "$txjson"
      # Try parsing JSON responses first (some ckb-cli versions return a JSON object or a JSON string)
      tx_hash=$(echo "$txjson" | jq -r 'if type=="object" then (.tx_hash // .txHash // .hash) elif type=="string" then . else empty end' 2>/dev/null || true)
      # Fallback: extract hex string from any output
      if [ -z "$tx_hash" ]; then
        tx_hash=$(echo "$txjson" | grep -oE '0x[a-f0-9]{64}' | head -n1 || true)
      fi
      if [ -z "$tx_hash" ]; then
        echo "Failed to create funding tx; aborting phase $name" >&2
        rm -rf "$TMP_MIG_DIR"
        return 1
      fi
      echo "Waiting for funding tx to commit: $tx_hash"
      for i in $(seq 1 60); do
        status=$(ckb-cli rpc get_transaction --hash "$tx_hash" --output-format json 2>/dev/null || true)
        st=$(echo "$status" | jq -r 'try .tx_status.status catch empty')
        if [ "$st" = "committed" ]; then
          echo "Funding tx committed"
          break
        fi
        sleep 1
      done
      if [ $attempts -ge $max_attempts ]; then
        echo "Reached max gen-txs attempts; aborting phase $name" >&2
        rm -rf "$TMP_MIG_DIR"
        return 1
      fi
      # retry gen-txs after funding
    else
      echo "gen-txs failed for unknown reason; output above" >&2
      rm -rf "$TMP_MIG_DIR"
      return 1
    fi
  done

  echo "Signing transactions non-interactively..."
  ckb-cli deploy sign-txs --privkey-path "$signer_privkey" --add-signatures --info-file "$info_file" --output-format json

  echo "Applying transactions..."
  ckb-cli deploy apply-txs --migration-dir "$TMP_MIG_DIR" --info-file "$info_file"

  echo "Installing migration artifacts into $migration_dir"
  mkdir -p "$migration_dir"
  mv "$TMP_MIG_DIR"/*.json "$migration_dir"/ || true
  rmdir "$TMP_MIG_DIR" 2>/dev/null || true

  echo "Deploy phase $name done"
  return 0
}

# Run phases sequentially
run_deploy_phase "normal-0" "./deployment/dev/deployment_0.toml" "./$MIGRATION_0" "$DEVNET_DIR/$DEPLOYMENT_INFO_0.json" "$GENESIS_PRIVKEY" "$MINER_PRIVKEY" || { echo "phase normal-0 failed"; exit 1; }
sleep 25
run_deploy_phase "normal-1" "./deployment/dev/deployment_1.toml" "./$MIGRATION_1" "$DEVNET_DIR/$DEPLOYMENT_INFO_1.json" "$GENESIS_PRIVKEY" "$MINER_PRIVKEY" || { echo "phase normal-1 failed"; exit 1; }
sleep 25
run_deploy_phase "vc" "./deployment/dev/deployment_vc.toml" "./$MIGRATION_VC" "$DEVNET_DIR/$DEPLOYMENT_INFO_VC.json" "$GENESIS_PRIVKEY" "$MINER_PRIVKEY" || { echo "phase vc failed"; exit 1; }
sleep 25
run_deploy_phase "lp" "./deployment/dev/deployment_lp.toml" "./$MIGRATION_LP" "$DEVNET_DIR/$DEPLOYMENT_INFO_LP.json" "$GENESIS_PRIVKEY" "$MINER_PRIVKEY" || { echo "phase lp failed"; exit 1; }

ALICE_LOCK_HASH=$(awk '/^lock_hash:/ {print $2}' "$DEVNET_DIR/$ACCOUNTS_DIR/alice.txt")
BOB_LOCK_HASH=$(awk '/^lock_hash:/ {print $2}' "$DEVNET_DIR/$ACCOUNTS_DIR/bob.txt")
LP_POOL_ID=${LP_POOL_ID:-}
if [ -z "$LP_POOL_ID" ]; then
  LP_POOL_ID=$(python3 - <<'PY'
import os, secrets
print("0x" + secrets.token_hex(32))
PY
)
fi

if [ -n "$ALICE_LOCK_HASH" ] && [ -n "$BOB_LOCK_HASH" ]; then
  echo "Preparing LP migration spec (operator=alice, owner=bob)..."
  (cd "$DEVNET_DIR/$PERUN_CONTRACTS_DIR" && \
    ./scripts/lp_migration_prepare.sh \
      --pool-id "$LP_POOL_ID" \
      --owner-lock-hash "$BOB_LOCK_HASH" \
      --operator-lock-hash "$ALICE_LOCK_HASH" \
      --network dev \
      --out migrations_lp/lp_cell_spec.json)
else
  echo "Skipping LP migration spec: missing alice/bob lock hash"
fi

# Move info files to devnet root (they are already created there) — keep as artifacts
mv "$DEVNET_DIR/$DEPLOYMENT_INFO_0.json" "$DEVNET_DIR/" 2>/dev/null || true
mv "$DEVNET_DIR/$DEPLOYMENT_INFO_1.json" "$DEVNET_DIR/" 2>/dev/null || true
mv "$DEVNET_DIR/$DEPLOYMENT_INFO_VC.json" "$DEVNET_DIR/" 2>/dev/null || true
mv "$DEVNET_DIR/$DEPLOYMENT_INFO_LP.json" "$DEVNET_DIR/" 2>/dev/null || true

echo "Deploying contracts done."

cd "$DEVNET_DIR"
echo "Fetching default contracts..."
rm -rf "$SYSTEM_SCRIPTS_DIR"
mkdir -p "$SYSTEM_SCRIPTS_DIR"
offckb system-scripts --export-style ccc | sed -n '/^{/,$p' > "$SYSTEM_SCRIPTS_DIR/default_scripts.json"
echo "Fetching default contracts done."

echo "Preparing sudt celldep..."
SUDT_JSON=$(ls ./contract/$MIGRATION_0/*.json | head -n1 || true)
if [ -z "$SUDT_JSON" ]; then
  echo "No migration json found at ./contract/$MIGRATION_0/*.json" >&2
  exit 1
fi
SUDT_TX_HASH=$(jq -r '.cell_recipes[0].tx_hash' "$SUDT_JSON")
SUDT_TX_INDEX=$(jq -r '.cell_recipes[0].index' "$SUDT_JSON")
SUDT_DATA_HASH=$(jq -r '.cell_recipes[0].data_hash' "$SUDT_JSON")
# Normalize index to start with 0x
if [[ "$SUDT_TX_INDEX" != 0x* ]]; then
  SUDT_TX_INDEX="0x$SUDT_TX_INDEX"
fi
jq --arg code_hash "$SUDT_DATA_HASH" --arg tx_hash "$SUDT_TX_HASH" --arg tx_index "$SUDT_TX_INDEX" \
  '.items.sudt.script_id.code_hash = $code_hash | .items.sudt.cell_dep.out_point.tx_hash = $tx_hash | .items.sudt.cell_dep.out_point.index = $tx_index' \
  ./sudt-celldep-template.json > $SYSTEM_SCRIPTS_DIR/sudt-celldep.json

echo "Done."