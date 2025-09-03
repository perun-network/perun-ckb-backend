package wallet

import (
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/wallet/address"
)

type Account struct {
	key           *secp256k1.PrivateKey
	codeHash      types.Hash
	defaultScript bool
	ethAddr       molecule.EthAddress
}

// Address returns an address.Participant with the public key belonging to this account and the default payment and
// unlock script hashes (secp256k1_blake160_sighash_all).
func (a Account) Address() wallet.Address {
	if a.defaultScript {
		addr, err := address.NewDefaultParticipant(a.key.PubKey(), a.ethAddr)
		if err != nil {
			return &address.Participant{PubKey: a.key.PubKey(), EthAddress: a.ethAddr}
		}
		return addr
	}
	addr, _, err := address.NewEthereumParticipantFromPublicKey(a.key.PubKey(), a.codeHash)
	if err != nil {
		return &address.Participant{PubKey: a.key.PubKey(), EthAddress: a.ethAddr}
	}
	return addr
}

func (a Account) SignData(data []byte) ([]byte, error) {
	hash := PrefixedHash(data)
	return PadDEREncodedSignature(ecdsa.Sign(a.key, hash[:]).Serialize())
}

func NewAccount() (*Account, error) {
	key, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	return &Account{key: key, defaultScript: true}, nil
}

func NewAccountFromPrivateKey(key *secp256k1.PrivateKey, codeHash types.Hash, defaultScript bool) *Account {
	return &Account{key: key, codeHash: codeHash, defaultScript: defaultScript}
}
