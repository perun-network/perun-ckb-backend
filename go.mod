module perun.network/perun-ckb-backend

go 1.24.0

toolchain go1.24.2

require perun.network/go-perun v0.14.1

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.0
	github.com/nervosnetwork/ckb-sdk-go/v2 v2.2.0
	golang.org/x/crypto v0.45.0
)

require (
	github.com/Pilatuz/bigz v1.2.2
	github.com/stretchr/testify v1.11.1
	polycry.pt/poly-go v0.0.0-20220301085937-fb9d71b45a37
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProjectZKM/Ziren/crates/go-runtime/zkvm_runtime v0.0.0-20251119083800-2aa1d4cc79d7 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/deckarep/golang-set/v2 v2.8.0 // indirect
	github.com/ethereum/go-ethereum v1.16.7 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/nervosnetwork/ckb-sdk-go/v2 v2.2.0 => github.com/perun-network/ckb-sdk-go/v2 v2.2.1-0.20230601140721-2bf596fddd80
