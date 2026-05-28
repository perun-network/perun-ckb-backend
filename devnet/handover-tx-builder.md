# Handover: TX Builder Updates for Perun CKB Contracts

## Scope
This note targets the off-chain developer who builds the CKB transactions for Perun channel contracts. The on-chain schema and ABI encoding have changed, and the TX builder must stay consistent to avoid signature and validation failures.

## Summary of Changes
- Locked balances schema changed: `SubBalances` is now a flat `vector<Uint128>` (no nested struct per asset).
- `SubAlloc` now includes an `IndexMap` (per asset, mapping from VC index to LC index).
- `convert_ckb_state` now serializes locked `SubAlloc` entries (the ABI is considered canonical and should not be reverted).
- Signature tests were updated to sign the ABI-encoded `StateSol` output using prehash signing.

## What You Must Update in the TX Builder
### 1) Locked Balances Encoding (SubBalances)
Old behavior likely encoded sub-balances as a struct per asset. New behavior is:
- One flat list of `Uint128` values.
- The list order follows assets order; it should align with the `balances` matrix rows.
- Each `SubAlloc` uses the same flat vector shape.

**Action:** When constructing `LockedBalances` and each `SubAlloc`, build `balances: vector<Uint128>` directly. Do not wrap balances in per-asset containers.

### 2) Index Map in SubAlloc
`SubAlloc` now includes an `IndexMap` to align VC participant indices with LC participants.

**Action:** Populate `index_map` for each `SubAlloc` so it matches the intended mapping between VC and LC. Ensure its length matches the number of participants in the VC and that it is used consistently for balance attribution.

### 3) convert_ckb_state ABI Compatibility
`convert_ckb_state` is now the source of truth for ABI encoding and signature hashing.

**Action:** Any off-chain signature generation must:
- Use the ABI encoding of `StateSol` as produced by `convert_ckb_state`.
- Hash with `ethereum_message_hash` (Keccak + Ethereum prefix).
- Sign the 32-byte hash as a prehash (ECDSA prehash signing).

### 4) Signature Verification Compatibility
The on-chain verifier uses prehash verification.

**Action:** Ensure your signing path uses prehash signing, not double-hashing. This matches the on-chain `verify_signature` behavior.

## Consistency Checklist
- Allocation assets order matches all balances (including locked sub-balances).
- `LockedBalances` uses flat `vector<Uint128>` for all sub-allocs.
- `SubAlloc.index_map` length and values are correct for the VC/LC mapping.
- All signatures are produced over ABI-encoded `StateSol` (not legacy encoding).
- Message hash is `ethereum_message_hash(abi_encoded_state)` and is signed as a prehash.

## Suggested Regression Tests
- Construct a state with at least one `SubAlloc` and multiple assets; verify that:
  - ABI encoding matches `convert_ckb_state` output.
  - Off-chain signatures verify on-chain.
- Test a VC update where the index map is non-trivial (permutation).

## Reference Implementation (In-Repo)
- `convert_ckb_state` (ABI encoding): crates/perun-common/src/sol.rs
- Signature hashing and verification: crates/perun-common/src/sig.rs
- Test signing path: tests/src/tests.rs

If you need a concrete sample state or expected encoded bytes, use the `test_signature` and `test_cross_signature` tests as templates.