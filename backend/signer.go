package backend

import (
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nervosnetwork/ckb-sdk-go/v2/address"
	ckbsecp "github.com/nervosnetwork/ckb-sdk-go/v2/crypto/secp256k1"
	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction/signer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction/signer/omnilock"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"log"
)

type Signer interface {
	// SignTransaction signs the transaction and returns the signed transaction or an error.
	SignTransaction(tx *transaction.TransactionWithScriptGroups) (*types.Transaction, error)
	// Address returns the address of the signer.
	Address() address.Address
	// PublicKey returns the public key of the signer.
	PublicKey() *secp256k1.PublicKey
	// Contexts returns the contexts for signing the transaction.
	Contexts() *signer.OmnilockConfiguration
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

func (s LocalSigner) SignTransaction(tx *transaction.TransactionWithScriptGroups) (*types.Transaction, error) {
	log.Println("Signing transaction with local signer...", tx)
	for g := range tx.TxView.Witnesses {
		log.Println("Script group: ", g, tx.TxView.Witnesses[g])
	}
	for input := range tx.TxView.Inputs {
		log.Println("Input: ", input, tx.TxView.Inputs[input].PreviousOutput.TxHash)
	}
	for output := range tx.TxView.Outputs {
		log.Println("Output: ", output, tx.TxView.Outputs[output].Capacity, tx.TxView.Outputs[output].Type, tx.TxView.Outputs[output].Lock)
		if tx.TxView.Outputs[output].Lock != nil {
			log.Println("Output lock: ", tx.TxView.Outputs[output].Lock.CodeHash)
		}
	}
	log.Println("Signing transaction: ", tx.TxView)
	hash := tx.TxView.ComputeHash()
	log.Println("Transaction hash: ", hash)
	_, err := s.TxSigner.SignTransactionByPrivateKeys(tx, s.key.Key.String())
	return tx.TxView, err
}

func (s LocalSigner) Address() address.Address {
	return s.Addr
}

func (s LocalSigner) PublicKey() *secp256k1.PublicKey {
	return s.key.PubKey()
}

func (s LocalSigner) Contexts() *signer.OmnilockConfiguration {
	return nil
}

// EVMSigner is the signer used by the backend implementation.
type EVMSigner struct {
	key      secp256k1.PrivateKey
	contexts *signer.OmnilockConfiguration
	Addr     address.Address
	TxSigner signer.TransactionSigner
}

func NewEVMSignerInstance(addr address.Address, key secp256k1.PrivateKey, network types.Network, authContent [20]byte) *EVMSigner {
	config := &signer.OmnilockConfiguration{
		Args: &omnilock.OmnilockArgs{
			Authentication: &omnilock.Authentication{
				Flag:        omnilock.AuthFlagEVM,
				AuthContent: authContent,
			},
			OmniConfig: &omnilock.OmniConfig{},
		},
		Mode: signer.OmnolockModeAuth,
	}
	return &EVMSigner{
		key:      key,
		contexts: config,
		Addr:     addr,
		TxSigner: *signer.GetTransactionSignerInstance(network),
	}
}

func (s EVMSigner) SignTransaction(tx *transaction.TransactionWithScriptGroups) (*types.Transaction, error) {
	log.Println("Signing transaction with evm signer...", tx)
	for g := range tx.TxView.Witnesses {
		log.Println("Script group: ", g, tx.TxView.Witnesses[g])
	}
	for input := range tx.TxView.Inputs {
		log.Println("Input: ", input, tx.TxView.Inputs[input].PreviousOutput.TxHash)
	}
	for output := range tx.TxView.Outputs {
		log.Println("Output: ", output, tx.TxView.Outputs[output].Capacity, tx.TxView.Outputs[output].Type, tx.TxView.Outputs[output].Lock)
		if tx.TxView.Outputs[output].Lock != nil {
			log.Println("Output lock: ", tx.TxView.Outputs[output].Lock.CodeHash)
		}
	}

	ctx := &transaction.Context{
		Key:     &ckbsecp.Secp256k1Key{PrivateKey: s.key.ToECDSA()},
		Payload: s.contexts,
	}
	log.Println("Signing transaction: ", tx.TxView)
	hash := tx.TxView.ComputeHash()
	log.Println("Transaction hash: ", hash)
	_, err := s.TxSigner.SignTransaction(tx, ctx)
	return tx.TxView, err
}

func (s EVMSigner) Address() address.Address {
	return s.Addr
}

func (s EVMSigner) PublicKey() *secp256k1.PublicKey {
	return s.key.PubKey()
}

func (s EVMSigner) Signer() signer.TransactionSigner {
	return s.TxSigner
}

func (s EVMSigner) Contexts() *signer.OmnilockConfiguration {
	return s.contexts
}
