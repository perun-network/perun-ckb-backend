package backend

import (
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nervosnetwork/ckb-sdk-go/v2/address"
	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction/signer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

// Signer is the signer used by the backend implementation.
type Signer struct {
	key      secp256k1.PrivateKey
	Address  address.Address
	TxSigner signer.TransactionSigner
}

func NewSigner(addr address.Address, key secp256k1.PrivateKey) *Signer {
	return &Signer{
		key:      key,
		Address:  addr,
		TxSigner: *signer.NewTransactionSigner(),
	}
}

func NewSignerInstance(addr address.Address, key secp256k1.PrivateKey, network types.Network) *Signer {
	return &Signer{
		key:      key,
		Address:  addr,
		TxSigner: *signer.GetTransactionSignerInstance(network),
	}
}

func (s Signer) SignTransaction(tx *transaction.TransactionWithScriptGroups) ([]int, error) {
	return s.TxSigner.SignTransactionByPrivateKeys(tx, s.key.Key.String())
}
