package wallet

import (
	"crypto/ecdsa"
	"encoding/asn1"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/pkg/errors"
	"math/big"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/wallet/address"
)

type Account struct {
	key           *secp256k1.PrivateKey
	codeHash      types.Hash
	defaultScript bool
}

type ECDSASignature struct {
	R, S *big.Int
}

// Address returns an address.Participant with the public key belonging to this account and the default payment and
// unlock script hashes (secp256k1_blake160_sighash_all).
func (a Account) Address() wallet.Address {
	if a.defaultScript {
		addr, err := address.NewDefaultParticipant(a.key.PubKey())
		if err != nil {
			return &address.Participant{PubKey: a.key.PubKey()}
		}
		return addr
	}
	addr, _, err := address.NewEthereumParticipantFromPublicKey(a.key.PubKey(), a.codeHash)
	if err != nil {
		return &address.Participant{PubKey: a.key.PubKey()}
	}
	return addr
}

// func (a Account) SignDataOld(data []byte) ([]byte, error) {
// 	hash := PrefixedHash(data)
// 	return PadDEREncodedSignature(decredECDSA.Sign(a.key, hash[:]).Serialize())
// }

func (a Account) SignData(data []byte) ([]byte, error) {
	phash := PrefixedHash(data)

	// Convert Decred key to standard *ecdsa.PrivateKey for go-ethereum usage
	ecdsaKey := ConvertDecredKeyToECDSA(a.key)

	// Sign using go-ethereum crypto.Sign, produces 65-byte [R||S||V]
	sig, err := crypto.Sign(phash, ecdsaKey)
	if err != nil {
		return nil, errors.Wrap(err, "sign hash failed")
	}

	// Extract R and S to marshal DER signature (drop recovery id V)
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:64])

	derEncoded, err := asn1.Marshal(ECDSASignature{R: r, S: s})
	if err != nil {
		return nil, errors.Wrap(err, "DER marshal failed")
	}

	return PadDEREncodedSignature(derEncoded)
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

// ConvertDecredKeyToECDSA converts a decred secp256k1 PrivateKey to an ecdsa.PrivateKey compatible with go-ethereum.
func ConvertDecredKeyToECDSA(decredKey *secp256k1.PrivateKey) *ecdsa.PrivateKey {
	ecPriv := new(ecdsa.PrivateKey)
	ecPriv.PublicKey.Curve = secp256k1.S256()
	ecPriv.D = new(big.Int).SetBytes(decredKey.Serialize())

	pub := decredKey.PubKey()
	ecPriv.PublicKey.X = pub.X()
	ecPriv.PublicKey.Y = pub.Y()
	return ecPriv
}
