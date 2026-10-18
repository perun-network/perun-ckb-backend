#!/usr/bin/env bash
# An isolated, reproducible local xUDT test network. No wallet home is modified.
set -euo pipefail
repo=$(cd "$(dirname "$0")/.." && pwd)
run_dir=${1:-"$repo/.xudt-devnet"}
mkdir -p "$run_dir"
run_dir=$(cd "$run_dir" && pwd)
export XDG_DATA_HOME="$run_dir/xdg/data"
export XDG_CONFIG_HOME="$run_dir/xdg/config"
export XDG_CACHE_HOME="$run_dir/xdg/cache"
export XDG_STATE_HOME="$run_dir/xdg/state"
export CARGO_HOME="$run_dir/cargo"
export GOMODCACHE="$run_dir/gomod"
export GOCACHE="$run_dir/gocache"
export RUSTUP_TOOLCHAIN=1.92.0
mkdir -p "$run_dir/tooling" "$run_dir/logs" "$run_dir/devnet-binaries" "$run_dir/extensions"
for command in go node npm curl python3 clang ld.lld llvm-ar llvm-objcopy rustup rg setsid; do command -v "$command" >/dev/null; done
# Fail before deploying anything if another local node occupies our ports.
python3 - <<'PORTS'
import socket
for port in (8114, 8115, 28114):
    with socket.socket() as s:
        s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        try:
            s.bind(("127.0.0.1", port))
        except OSError:
            raise SystemExit(f"Port {port} is already in use; stop that node first.")
PORTS
if [[ -d "$run_dir/deployment" || -d "$XDG_DATA_HOME/offckb-nodejs/devnet" ]]; then
  echo 'Use a fresh run directory; existing chain/deployment data is never replaced.' >&2
  exit 1
fi
rustup target list --installed | rg -q '^riscv64imac-unknown-none-elf$'
echo "Installing pinned devnet tooling..."
npm install --prefix "$run_dir/tooling" --cache "$run_dir/npm" @offckb/cli@0.4.13
ckb_dir="$run_dir/tooling/ckb_v0.201.0_x86_64-unknown-linux-gnu"
if [[ ! -x "$ckb_dir/ckb" ]]; then
  curl -fL https://github.com/nervosnetwork/ckb/releases/download/v0.201.0/ckb_v0.201.0_x86_64-unknown-linux-gnu.tar.gz -o "$run_dir/tooling/ckb.tar.gz"
  echo "ac55e2de14cea2c76c3f3a58d563aa97a9f96d5d6cc5c63d05939781979a3d0e  $run_dir/tooling/ckb.tar.gz" | sha256sum -c -
  tar -xzf "$run_dir/tooling/ckb.tar.gz" -C "$run_dir/tooling"
fi
offckb="$run_dir/tooling/node_modules/.bin/offckb"
mkdir -p "$XDG_DATA_HOME/offckb-nodejs/bins/0.201.0"
ln -sf "$ckb_dir/ckb" "$XDG_DATA_HOME/offckb-nodejs/bins/0.201.0/ckb"
"$offckb" config set ckb-version 0.201.0
cd "$repo"
git submodule update --init --recursive
# The pinned Rust toolchain supports this upstream contract's -a target flags.
# Use ckb-std's freestanding headers for blake2b-rs; never host libc headers.
(cd devnet/contract && cargo fetch --locked)
libc_headers=$(find "$CARGO_HOME/registry/src" -path '*/ckb-std-0.17.2/c/ckb-c-stdlib/libc' -type d | head -1)
test -n "$libc_headers"
echo "Building and testing the pinned Perun contracts..."
(cd devnet/contract && TARGET_CFLAGS="-DCKB_DECLARATION_ONLY -I$libc_headers" make build CLANG="$(command -v clang)") > "$run_dir/logs/contracts.log" 2>&1
(cd devnet/contract && cargo test --lib --manifest-path tests/Cargo.toml) > "$run_dir/logs/contract-tests.log" 2>&1
for name in perun-channel-lockscript perun-channel-typescript perun-funds-lockscript perun-vchannel-lockscript perun-vchannel-typescript sample-udt; do
  cp "devnet/contract/build/release/$name" "$run_dir/devnet-binaries/"
done
clang --target=riscv64 -march=rv64imc -mabi=lp64 -nostdlib -fPIC -fuse-ld=lld \
  -Wl,-shared,-z,separate-code -O2 devnet/xudt/channel-extension.c -o "$run_dir/extensions/channel-extension"
setsid "$offckb" node --binary-path "$ckb_dir/ckb" > "$run_dir/logs/node.log" 2>&1 &
node_pid=$!
miner_pid=
cleanup() {
  if [[ -n "$miner_pid" ]]; then kill "$miner_pid" 2>/dev/null || true; wait "$miner_pid" 2>/dev/null || true; fi
  kill -- "-$node_pid" 2>/dev/null || true
  wait "$node_pid" 2>/dev/null || true
  echo "Devnet stopped. Logs: $run_dir/logs"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
ready=false
for i in $(seq 1 60); do
  if curl -sf http://127.0.0.1:8114 -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","id":1,"method":"get_tip_block_number","params":[]}' | python3 -c 'import json,sys; assert "result" in json.load(sys.stdin)' 2>/dev/null; then ready=true; break; fi
  kill -0 "$node_pid"
  sleep 1
done
$ready || { echo 'Devnet failed to start' >&2; exit 1; }
mkdir -p "$run_dir/fast-miner"
cp "$XDG_DATA_HOME/offckb-nodejs/devnet/ckb-miner.toml" "$run_dir/fast-miner/ckb-miner.toml"
sed -i 's/^value = 5000/value = 200/' "$run_dir/fast-miner/ckb-miner.toml"
ln -sf "$XDG_DATA_HOME/offckb-nodejs/devnet/specs" "$run_dir/fast-miner/specs"
"$ckb_dir/ckb" miner -C "$run_dir/fast-miner" > "$run_dir/logs/fast-miner.log" 2>&1 &
miner_pid=$!
"$offckb" system-scripts --export-style ccc > "$run_dir/system-scripts.json"
"$offckb" accounts --show-private-keys > "$run_dir/devnet-accounts.txt"
chmod 600 "$run_dir/devnet-accounts.txt"
echo "Deploying test contracts..."
"$offckb" deploy --target "$run_dir/devnet-binaries" --output "$run_dir/deployment" --yes > "$run_dir/logs/deploy.log" 2>&1
"$offckb" deploy --target "$run_dir/extensions/channel-extension" --output "$run_dir/extension-deployment" --yes > "$run_dir/logs/deploy-extension.log" 2>&1
python3 devnet/xudt/prepare_fixture.py "$run_dir"
echo "Running xUDT lifecycle and rejection tests for both locks..."
export PERUN_XUDT_FIXTURE="$run_dir/xudt-fixture.json"
go test -tags xudt_integration ./client -timeout 15m -run '^TestXUDTSeed$' -count=1 -v > "$run_dir/logs/seed.log" 2>&1
go test -tags xudt_integration ./client -timeout 15m -run '^TestXUDTChannelLifecycle$' -count=1 -v > "$run_dir/logs/lifecycle.log" 2>&1
go test -tags xudt_integration ./client -timeout 15m -run '^TestXUDTMultiAssetTransfer$' -count=1 -v > "$run_dir/logs/multi-asset.log" 2>&1
PERUN_XUDT_FIXTURE="$run_dir/regression-fixture.json" go test -tags xudt_integration ./client -timeout 15m -run '^TestXUDTSeedRegression$' -count=1 -v > "$run_dir/logs/regression-seed.log" 2>&1
echo "Running original regression, race, vet and fuzz checks..."
go vet ./... > "$run_dir/logs/vet.log" 2>&1
go test ./xudt -run '^$' -fuzz FuzzValidateData -fuzztime 20s > "$run_dir/logs/fuzz.log" 2>&1
go test ./... -count=1 -timeout 15m > "$run_dir/logs/regression.log" 2>&1
go test -race ./... -count=1 -timeout 15m > "$run_dir/logs/race.log" 2>&1
printf 'xUDT lifecycle verification passed. Logs: %s/logs\n' "$run_dir"
