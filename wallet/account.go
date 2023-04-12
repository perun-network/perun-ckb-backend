package wallet

import (
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/blake2b"
	"perun.network/go-perun/wallet"
)

type Account struct {
	key *secp256k1.PrivateKey
}

func (a Account) Address() wallet.Address {
	return &Address{pubKey: a.key.PubKey()}
}

func (a Account) SignData(data []byte) ([]byte, error) {
	hash := blake2b.Sum256(data)
	return PadDEREncodedSignature(ecdsa.Sign(a.key, hash[:]).Serialize())
}

func NewAccount() (*Account, error) {
	key, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	return &Account{key: key}, nil
}
