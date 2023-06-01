module perun.network/perun-ckb-backend

go 1.16

require perun.network/go-perun v0.10.6

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.1.0
	github.com/nervosnetwork/ckb-sdk-go/v2 v2.2.0
	golang.org/x/crypto v0.1.0
)

require (
	github.com/Pilatuz/bigz v1.2.1
	github.com/stretchr/testify v1.7.0
	polycry.pt/poly-go v0.0.0-20220222131629-aa4bdbaab60b
)

replace github.com/nervosnetwork/ckb-sdk-go/v2 v2.2.0 => github.com/perun-network/ckb-sdk-go/v2 v2.2.1-0.20230601140721-2bf596fddd80
