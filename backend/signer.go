package backend

import (
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nervosnetwork/ckb-sdk-go/v2/address"
	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction/signer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

type Signer interface {
	// SignTransaction signs the transaction and returns the signed transaction or an error.
	SignTransaction(tx *transaction.TransactionWithScriptGroups) (*transaction.TransactionWithScriptGroups, error)
	// Address returns the address of the signer.
	Address() address.Address
}

// LocalSigner is the signer used by the backend implementation.
type LocalSigner struct {
	key      secp256k1.PrivateKey
	Addr     address.Address
	TxSigner signer.TransactionSigner
}

func NewSigner(addr address.Address, key secp256k1.PrivateKey) *LocalSigner {
	return &LocalSigner{
		key:      key,
		Addr:     addr,
		TxSigner: *signer.NewTransactionSigner(),
	}
}

func NewSignerInstance(addr address.Address, key secp256k1.PrivateKey, network types.Network) *LocalSigner {
	return &LocalSigner{
		key:      key,
		Addr:     addr,
		TxSigner: *signer.GetTransactionSignerInstance(network),
	}
}

func (s LocalSigner) SignTransaction(tx *transaction.TransactionWithScriptGroups) (*transaction.TransactionWithScriptGroups, error) {
	_, err := s.TxSigner.SignTransactionByPrivateKeys(tx, s.key.Key.String())
	return tx, err
}

func (s LocalSigner) Address() address.Address {
	return s.Addr
}
