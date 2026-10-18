# xUDT payment channels

The backend supports xUDT transfers using the token's exact CKB type script.
Register xUDTs in `backend.Deployment.XUDTs`; continue registering sUDTs in
`SUDTs` and `SUDTDeps`. An exact script hash cannot be registered in both maps.

```go
config := xudt.Config{
    Script: tokenScript, // code hash, hash type, owner hash and flags
    CellDeps: tokenDeps, // deployed xUDT code and any extension dependencies
}
deployment.XUDTs = map[types.Hash]xudt.Config{tokenScript.Hash(): config}
channelAsset, err := asset.NewXUDTAsset(tokenScript, maxCellCapacity)
```

The constructor uses the existing fungible asset encoding. Its historical
`SUDT` type name and wire discriminator are retained for compatibility with
Perun's on-chain contracts and signed channel states. The entire type script,
including extension flags and arguments, is part of the signed asset identity.
No new contract deployment is required solely to recognize the token standard.

## Extension scripts

RFC 0052 modes are supported:

- A 32-byte owner hash, or flags with extension mode 0: ordinary transfers.
- Extension mode 1: a Molecule `ScriptVec` embedded in the type arguments.
- Extension mode 2: a Blake160 commitment to that vector, whose preimage is
  supplied in the type-group witness.

For modes 1 and 2, configure `ExtensionScripts` in their original order. The
backend checks the vector or its hash against the type arguments. Add the
necessary extension code/proof dependencies to `CellDeps`.

Nonempty extension lists require an explicit `WitnessData` callback. It receives
the balanced transaction and the input/output indices for this token group and
returns exactly one witness-data entry per extension. It runs before signing.
Use `OutputData` to construct the bytes following the 16-byte token amount for
both channel outputs and wallet change (whose capacity is initially zero when
the callback runs, and is calculated from the resulting data). `xudt.EncodeData` encodes the optional
`XudtData` table; the builder checks its structure and extension count. An input
with metadata is rejected unless an output provider is configured, preventing
accidental loss of that metadata through an ordinary amount-only transfer.

Callbacks must be deterministic, concurrency-safe, and must not mutate their
arguments. They return errors when required proofs or output data cannot be
constructed. The backend places the transfer witness in `input_type` at the
first group input, or `output_type` at the first group output when no inputs
exist. It preserves other witness fields and rejects conflicting contents.

**Extension semantics remain part of the token's rules.** A freeze, allowlist,
third-party authorization, or time-dependent extension can prevent unilateral
settlement even when an ordinary transfer succeeds. Both channel participants
must establish that the chosen token policy permits funding, refunds, and all
settlement paths before locking funds. Supplying callbacks does not prove that
property. This backend does not grant permission to bypass extension rules.
Owner flags are retained; channel transactions conserve tokens and do not
construct owner authorization for minting.

## Capacity and amounts

Token amounts and accumulated totals are checked against the full unsigned
128-bit range. Capacity and fees are checked against the unsigned 64-bit range.
Malformed amounts and overflow return errors. Funding preserves input-only
assets, handles partial funding, skips previously selected inputs, and reports
insufficient funds before signing. Wallet change capacity includes extension
metadata. Choose an asset `MaxCapacity` sufficient for every participant's lock,
the complete type script and the largest metadata payload required by the
channel; undersized xUDT outputs return an error. The existing default fee is
10,000 shannons per transaction. Set `Deployment.FeeShannon` when large extension
witnesses or node fee policies require more; zero selects that default. Fees are
reserved during balancing before proofs and signatures are generated.

`transaction.AssetInformation.AssetAmount` now returns an independent
`*big.Int`. Its mutating accumulation methods return errors. Callers using these
exported helpers must handle those errors and must not convert token amounts to
`uint64`.

## Verification

Unit tests cover full-width balances, change, partially funded transactions,
input-only tokens, malformed amounts, overflow, witness placement and collision,
all extension encodings, and missing configuration. The metadata parser also
has a fuzz target:

```sh
go test -race ./transaction ./xudt ./backend/... ./channel/asset/... ./encoding/...
go test ./xudt -run '^$' -fuzz FuzzValidateData -fuzztime 20s
```

The integration harness builds the pinned Perun contracts, deploys them beside
CKB's real xUDT contract, and tests secp256k1 and Ethereum OmniLock participants.
Each participant funds `2^80` tokens and signs an off-chain transfer of `2^70`.
Every token mode verifies exact balances after cooperative close, abort/refund,
and dispute/force-close. The test issuer is independent of both participants,
so owner-mode authorization cannot bypass the extension checks. A dynamically
loaded extension checks both a witness proof and output metadata; deliberately
invalid proofs and metadata must be rejected by CKB.

On **Linux x86-64**, install Go (as required by `go.mod`), Node/npm, Python 3,
Clang/LLD/LLVM, curl, ripgrep and Rust 1.92.0 with the
`riscv64imac-unknown-none-elf` target, then run:

```sh
./devnet/setup-xudt.sh /absolute/path/to/a-fresh-run-directory
```

The script pins offckb 0.4.13 and CKB 0.201.0, verifies the downloaded CKB archive,
uses isolated caches/configuration and disposable devnet keys, and stops its node
on exit. Ports 8114, 8115 and 28114 must be free. It also runs the original Go
suite, the race-enabled suite, `go vet`, and a 20-second parser fuzz test. Logs
remain in the run directory. It writes the ignored `devnet/accounts`,
`devnet/system_scripts`, and contract migration fixtures required by the original
suite; it refuses to replace different existing account keys. Use a fresh
checkout if you already maintain other devnet fixtures there.

The original repository disables virtual-channel dispute coverage and excludes
one cross-payment dispute test under `-race`; these existing limitations remain.
The added xUDT integration tests cover bilateral channels, with unilateral exits
verified for all three token modes and both supported participant locks. A separate chain test transfers
all three xUDTs in one transaction to verify shared-lock and type-group witnesses.
Custom extension policies still require their own settlement/liveness analysis.

To rerun selected tests while the fixture's node is running:

```sh
PERUN_XUDT_FIXTURE=/absolute/path/to/run/xudt-fixture.json \
  go test -tags xudt_integration ./client -run '^TestXUDTChannelLifecycle$' -count=1
```

Reference: [CKB RFC 0052](https://github.com/nervosnetwork/rfcs/blob/master/rfcs/0052-extensible-udt/0052-extensible-udt.md).
