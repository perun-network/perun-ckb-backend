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

The integration tests reuse the existing devnet deployment, system-script
configuration and Alice/Bob accounts. The issuer is the separate genesis-2
account, so channel participants cannot bypass extension checks using owner mode.
The optional extension is maintained in the Rust contract submodule at
`test-contracts/xudt-extension`; the backend contains no C fixture or assembly.

Follow the repository's normal devnet setup. Enable the additional fixture in
its existing deployment step, then run the integration tests:

```sh
cd devnet
PERUN_XUDT_TESTS=1 ./deploy_contracts.sh
make test-xudt
```

`make test-xudt` mints disposable xUDTs into the existing wallets and runs the
lifecycle and multi-asset tests. Run it on a local test network only. There is no
separate node launcher, downloaded toolchain, fixture JSON or Python encoder.
The tests cover all three modes and both secp256k1/OmniLock participants, with
`2^80` funding, a `2^70` off-chain transfer, exact balances after close, abort
and dispute/force-close, and on-chain rejection of invalid proofs and metadata.
Hashed-extension lifecycle tests also exercise a 50,000-shannon fee override.
The multi-asset test transfers all three xUDTs in a shared lock group.

The existing virtual-channel dispute and cross-payment race exclusions remain.
Custom extension policies require their own settlement/liveness analysis.

Reference: [CKB RFC 0052](https://github.com/nervosnetwork/rfcs/blob/master/rfcs/0052-extensible-udt/0052-extensible-udt.md).
