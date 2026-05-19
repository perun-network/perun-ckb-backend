# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Go implementation of the [go-perun](https://github.com/perun-network/go-perun) channel library backend for the Nervos CKB blockchain. Module: `perun.network/perun-ckb-backend`.

## Setup

Before building or testing, initialize the contract submodule:

```bash
git submodule update --init --recursive
```

## Build & Test

```bash
go build -v ./...
go test -v ./...
go test -v -race ./...
```

**Integration tests** require a running devnet and the `devnet` build tag:

```bash
RUN_DEVNET_TESTS=1 go test -v -tags devnet ./adapter/ckb_lp/...
```

Other build tags in use: `!testnet` (excludes testnet tests), `getaddress`, `fundsudt`.

## Devnet

```bash
cd devnet && make dev
```

The devnet setup involves CKB node startup, contract deployment, and account funding — it typically takes 3+ minutes and can be flaky. If tests fail unexpectedly, restart the devnet and wait for full initialization before re-running. The devnet expects a local CKB RPC at `http://localhost:8114`.

Tools required: CKB v0.201.0, ckb-cli v1.13.0, `@offckb/cli`, `jq`, `tmux`, `tmuxp`, `expect`. The smart contracts compile to RISC-V (`riscv64imac-unknown-none-elf`) using Rust/cargo; see `devnet/contract/setup_env.sh` for the required env vars.

## Linting

```bash
golangci-lint run
```

No custom `.golangci.yml` — uses defaults.

## Commits & PRs

Conventional commits (loose): `feat(scope):`, `fix(scope):`, `test(scope):`, `refactor(scope):`. PRs target `main`.
