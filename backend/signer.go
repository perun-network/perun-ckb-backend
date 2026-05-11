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
	"fmt"
	"github.com/nervosnetwork/ckb-sdk-go/v2/crypto/blake2b"
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
	log.Println("Signing transaction: ", tx.TxView)
	log.Printf("Signer pubkey hash=%x", blake2b.Blake160(s.PublicKey().SerializeCompressed()))
	// Debug: print script groups and witnesses before signing
	for i, sg := range tx.ScriptGroups {
		log.Printf("ScriptGroup[%d]: Type=%v, ScriptHash=%x, ScriptArgs=%x, InputIndices=%v, OutputIndices=%v", i, sg.GroupType, sg.Script.Hash(), sg.Script.Args, sg.InputIndices, sg.OutputIndices)
	}
	// Debug: print cell deps to verify TypeScript/Lock deps are present
	log.Printf("CellDeps count=%d", len(tx.TxView.CellDeps))
	for i, d := range tx.TxView.CellDeps {
		out := d.OutPoint
		var outStr string
		if out == nil {
			outStr = "<nil>"
		} else {
			outStr = out.TxHash.String() + ":" + fmt.Sprint(out.Index)
		}
		log.Printf("CellDep[%d]: DepType=%v OutPoint=%s", i, d.DepType, outStr)
	}
	log.Printf("Witnesses before sign: count=%d", len(tx.TxView.Witnesses))
	for i, w := range tx.TxView.Witnesses {
		log.Printf("Witness[%d] len=%d", i, len(w))
	}

	signedIndexes, err := s.TxSigner.SignTransactionByPrivateKeys(tx, s.key.Key.String())
	log.Printf("Signed script group indexes: %v", signedIndexes)

	// Debug: print witnesses after signing
	log.Printf("Witnesses after sign: count=%d", len(tx.TxView.Witnesses))
	for i, w := range tx.TxView.Witnesses {
		log.Printf("Witness[%d] len=%d; prefix_hex=%x", i, len(w), func(b []byte) []byte { if len(b) > 16 { return b[:16] }; return b }(w))
	}
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

	ctx := &transaction.Context{
		Key:     &ckbsecp.Secp256k1Key{PrivateKey: s.key.ToECDSA()},
		Payload: s.contexts,
	}
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
