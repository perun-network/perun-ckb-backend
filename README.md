<h1 align="center"><br>
    <a href="https://perun.network/"><img src=".assets/go-perun.png" alt="Perun" width="196"></a>
<br></h1>

<h2 align="center">Perun CKB Backend</h2>

<p align="center">
  <a href="https://www.apache.org/licenses/LICENSE-2.0.txt"><img src="https://img.shields.io/badge/license-Apache%202-blue" alt="License: Apache 2.0"></a>
  <a href="https://github.com/perun-network/perun-ckb-backend/actions/workflows/go.yml"><img src="https://github.com/perun-network/perun-ckb-backend/actions/workflows/go.yml/badge.svg?branch=main" alt="CI status"></a>
</p>

# [Perun](https://perun.network/) CKB backend

This repository contains the Nervos/CKB backend for the [go-perun](https://github.com/perun-network/go-perun) channel library.

Learn how to use go-perun backends in the documentation of the go-perun core library. It supposed to work in tandem with the [perun-ckb-contract](https://github.com/perun-network/perun-ckb-contract)

## Project structure
* `backend/`: Backend interface implementations.
* `channel/`: Channel interface implementations.
* `client/`: Client bindings with tests.
* `wallet/`: Wallet interface implementations.
* `encoding/`: On/off-chain serialization.
* `testnet/`: Local testnet deployment.
* `transaction`: Contract bindings. 

## Development

1. Clone the repository.
```sh
git clone https://github.com/perun-network/perun-ckb-backend
cd perun-ckb-backend
```

2. Initialize the submodule.
```sh
git submodule update --init --recursive
```

3. Start the testnet on a separate terminal.
```sh
cd testnet/devnet

make dev
```

4. Run the tests. This step needs a working [Go distribution](https://golang.org), see [go.mod](go.mod) for the required version.

```sh
go test ./...
```

## Security Disclaimer

The authors take no responsibility for any loss of digital assets or other damage caused by the use of this software.

## Copyright

Copyright 2025 PolyCrypt GmbH.  
Use of the source code is governed by the Apache 2.0 license that can be found in the [LICENSE file](LICENSE).

<!--- Links -->