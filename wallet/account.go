package wallet

import (
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/blake2b"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/wallet/address"
)

type Account struct {
	key *secp256k1.PrivateKey
}

// Address returns an address.Participant with the public key belonging to this account and the default payment and
// unlock script hashes (secp256k1_blake160_sighash_all).
func (a Account) Address() wallet.Address {
	addr, err := address.NewDefaultParticipant(a.key.PubKey())
	if err != nil {
		return &address.Participant{PubKey: a.key.PubKey()}
	}
	return addr
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
