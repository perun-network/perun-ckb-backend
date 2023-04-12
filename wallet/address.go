package wallet

import (
	"encoding/hex"
	"errors"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"perun.network/go-perun/wallet"
)

const UncompressedPublicKeyLength = 65

type Address struct {
	pubKey *secp256k1.PublicKey
}

func (a Address) MarshalBinary() (data []byte, err error) {
	if a.pubKey == nil {
		return nil, errors.New("public key is nil")
	}

	return a.pubKey.SerializeUncompressed(), nil
}

func (a *Address) UnmarshalBinary(data []byte) error {
	if len(data) != UncompressedPublicKeyLength {
		return errors.New("invalid public key length")
	}
	pubKey, err := secp256k1.ParsePubKey(data)
	if err != nil {
		return err
	}
	a.pubKey = pubKey
	return nil
}

func (a Address) String() string {
	return hex.EncodeToString(a.pubKey.SerializeUncompressed())
}

func (a Address) Equal(address wallet.Address) bool {
	addr, ok := address.(*Address)
	if !ok {
		return false
	}
	return a.pubKey.IsEqual(addr.pubKey)
}

func (a Address) GetUncompressedSEC1() [65]byte {
	var sec1 [65]byte
	copy(sec1[:], a.pubKey.SerializeUncompressed())
	return sec1
}

func GetZeroAddress() *Address {
	return &Address{pubKey: secp256k1.NewPublicKey(new(secp256k1.FieldVal), new(secp256k1.FieldVal))}
}
