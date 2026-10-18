"""Convert offckb artifacts to disposable backend and regression fixtures."""
import hashlib
import json
import re
import struct
import sys
from pathlib import Path

run_dir = Path(sys.argv[1]).resolve()
repo = Path(__file__).resolve().parents[2]


def read_json(file):
    text = file.read_text()
    return json.loads(text[text.index("{"):])


def write_json(file, value):
    file.parent.mkdir(parents=True, exist_ok=True)
    file.write_text(json.dumps(value, indent=2) + "\n")


def digest(data):
    return hashlib.blake2b(data, digest_size=32, person=b"ckb-default-hash").digest()


def sequence(fields):
    offset = 4 * (1 + len(fields))
    offsets = []
    for field in fields:
        offsets.append(offset)
        offset += len(field)
    return struct.pack("<I", offset) + b"".join(struct.pack("<I", n) for n in offsets) + b"".join(fields)


def script(entry, args="0x", hash_type=None):
    return {"code_hash": entry["codeHash"], "hash_type": hash_type or entry["hashType"], "args": args}


def script_bytes(value):
    args = bytes.fromhex(value["args"][2:])
    hash_types = {"data": 0, "type": 1, "data1": 2, "data2": 4}
    return sequence([bytes.fromhex(value["code_hash"][2:]), bytes([hash_types[value["hash_type"]]]), struct.pack("<I", len(args)) + args])


def dependency(entry):
    value = entry["cellDep"]
    point = value["outPoint"]
    return {"out_point": {"tx_hash": point["txHash"], "index": hex(point["index"])}, "dep_type": "dep_group" if value["depType"] == "depGroup" else value["depType"]}


system = read_json(run_dir / "system-scripts.json")
contracts = read_json(run_dir / "deployment/scripts.json")["devnet"]
extension = read_json(run_dir / "extension-deployment/scripts.json")["devnet"]["channel-extension"]
accounts = (run_dir / "devnet-accounts.txt").read_text()
keys = re.findall(r"^privkey:\s*(?:0x)?([a-f0-9]{64})", accounts, re.M)
locks = re.findall(r"^lock_arg:\s*(0x[a-f0-9]+)", accounts, re.M)
assert len(keys) >= 9 and len(locks) == len(keys), "Unexpected offckb accounts format"

names = {"pcts": "perun-channel-typescript", "pcls": "perun-channel-lockscript", "pfls": "perun-funds-lockscript", "vcts": "perun-vchannel-typescript", "vcls": "perun-vchannel-lockscript"}
deployment = {}
for prefix, name in names.items():
    entry = contracts[name]
    deployment[prefix + "_dep"] = dependency(entry["cellDeps"][0])
    deployment[prefix + "_code_hash"] = entry["codeHash"]
    deployment[prefix + "_hash_type"] = "data1"
deployment.update({
    "pfls_min_capacity": 4100000032,
    "default_lock_script": script(system["Secp256k1Blake160"]),
    "default_lock_script_dep": dependency(system["Secp256k1Blake160"]["cellDeps"][0]),
    "omni_lock_script": script(system["OmniLock"]),
    "omni_lock_script_dep": [dependency(item) for item in system["OmniLock"]["cellDeps"]],
})
owner = digest(script_bytes(script(system["Secp256k1Blake160"], locks[8])))
ext_script = script(extension, "0x2a", "data1")
vector = sequence([script_bytes(ext_script)])
tokens = []
for mode in range(3):
    args = owner + struct.pack("<I", mode)
    if mode == 1:
        args += vector
    elif mode == 2:
        args += digest(vector)[:20]
    token = {"script": script(system["XUdt"], "0x" + args.hex()), "cell_deps": [dependency(item) for item in system["XUdt"]["cellDeps"]]}
    if mode:
        token["extension_scripts"] = [ext_script]
        token["cell_deps"].append(dependency(extension["cellDeps"][0]))
    tokens.append(token)
fixture = {"RPC": "http://127.0.0.1:8114", "Deployment": deployment, "Keys": keys[:2], "IssuerKey": keys[8], "Tokens": tokens}
write_json(run_dir / "xudt-fixture.json", fixture)
(run_dir / "xudt-fixture.json").chmod(0o600)

# Separate wallets keep the original suite independent of xUDT test balances.
reg_owner = "0x" + digest(script_bytes(script(system["Secp256k1Blake160"], locks[6]))).hex()
regression = dict(fixture, Keys=keys[6:8], Tokens=[], RegressionKeys=keys[3:6], RegressionToken={"script": script(contracts["sample-udt"], reg_owner, "data1"), "cell_deps": [dependency(contracts["sample-udt"]["cellDeps"][0])]})
write_json(run_dir / "regression-fixture.json", regression)
(run_dir / "regression-fixture.json").chmod(0o600)

# Validate all existing account identities before modifying any legacy fixtures.
for name, key in zip(["alice", "bob", "ingrid"], keys[3:6]):
    target = repo / "devnet/accounts" / (name + ".pk")
    if target.exists() and target.read_text().strip() != key:
        raise RuntimeError(f"Refusing to replace an existing devnet account: {target}")

# The original tests load these ignored files at fixed repository paths.
devnet = repo / "devnet"
write_json(devnet / "system_scripts/default_scripts.json", system)
account_dir = devnet / "accounts"
account_dir.mkdir(exist_ok=True)
for name, key in zip(["alice", "bob", "ingrid"], keys[3:6]):
    target = account_dir / (name + ".pk")
    if target.exists() and target.read_text().strip() != key:
        raise RuntimeError(f"Refusing to replace an existing devnet account: {target}")
    target.write_text(key + "\n")
    target.chmod(0o600)
(account_dir / "sudt-owner-lock-hash.txt").write_text(reg_owner + "\n")
for directory, scripts in [
    ("migrations_0", ["sample-udt", names["pcls"], names["pfls"]]),
    ("migrations_1", [names["pcts"]]),
    ("migrations_vc", [names["vcts"], names["vcls"]]),
]:
    recipes = []
    short_names = {value: key for key, value in names.items()}
    short_names["sample-udt"] = "sudt"
    for name in scripts:
        files = list((run_dir / "deployment/devnet" / name / "migrations").glob("*.json"))
        assert len(files) == 1, f"Expected one migration for {name}"
        recipe = read_json(files[0])["cell_recipes"][0]
        recipe["name"] = short_names[name]
        recipes.append(recipe)
    write_json(devnet / "contract" / directory / "dev/fixture.json", {"cell_recipes": recipes, "dep_group_recipes": []})
print(f"Prepared xUDT and regression fixtures in {run_dir}")
