module perun.network/perun-ckb-backend

go 1.19

require perun.network/go-perun v0.10.7-0.20230808153546-74844191e56e

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.2.0
	github.com/nervosnetwork/ckb-sdk-go/v2 v2.2.0
	golang.org/x/crypto v0.18.0
)

require (
	github.com/Pilatuz/bigz v1.2.1
	github.com/stretchr/testify v1.8.4
	polycry.pt/poly-go v0.0.0-20220301085937-fb9d71b45a37
)

require (
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/btcsuite/btcd/btcec/v2 v2.3.2 // indirect
	github.com/btcsuite/btcd/chaincfg/chainhash v1.1.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/deckarep/golang-set/v2 v2.6.0 // indirect
	github.com/ethereum/go-ethereum v1.13.10 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/holiman/uint256 v1.2.4 // indirect
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/stretchr/objx v0.5.1 // indirect
	github.com/tklauser/go-sysconf v0.3.13 // indirect
	github.com/tklauser/numcpus v0.7.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	golang.org/x/exp v0.0.0-20240112132812-db7319d0e0e3 // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/net v0.20.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
	golang.org/x/tools v0.17.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/nervosnetwork/ckb-sdk-go/v2 v2.2.0 => github.com/perun-network/ckb-sdk-go/v2 v2.2.1-0.20241016165355-d1c9686fe018
