package address

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nervosnetwork/ckb-sdk-go/v2/systemscript"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/wallet"
)

const (
	UncompressedPublicKeyLength = 65
	Uint64Length                = 8
	SerializedAddressLength     = UncompressedPublicKeyLength + types.HashLength + types.HashLength + Uint64Length
)

type Address struct {
	PubKey             *secp256k1.PublicKey
	PaymentScriptHash  types.Hash
	UnlockScriptHash   types.Hash
	PaymentMinCapacity uint64
}

func NewDefaultAddress(pubKey *secp256k1.PublicKey) (*Address, error) {
	if pubKey == nil {
		return nil, errors.New("public key is nil")
	}
	script, err := systemscript.Secp256K1Blake160SignhashAllByPublicKey(pubKey.SerializeCompressed())
	if err != nil {
		return nil, err
	}
	hash := script.Hash()
	return &Address{
		PubKey:             pubKey,
		PaymentScriptHash:  hash,
		UnlockScriptHash:   hash,
		PaymentMinCapacity: script.OccupiedCapacity(),
	}, nil
}

func NewAddress(pubKey *secp256k1.PublicKey, paymentScript, unlockScript *types.Script) *Address {
	return &Address{
		PubKey:             pubKey,
		PaymentScriptHash:  paymentScript.Hash(),
		UnlockScriptHash:   unlockScript.Hash(),
		PaymentMinCapacity: paymentScript.OccupiedCapacity(),
	}
}

func (a Address) MarshalBinary() (data []byte, err error) {
	if a.PubKey == nil {
		return nil, errors.New("public key is nil")
	}
	data = make([]byte, SerializedAddressLength)
	copy(data[:UncompressedPublicKeyLength], a.PubKey.SerializeUncompressed())
	copy(data[UncompressedPublicKeyLength:UncompressedPublicKeyLength+types.HashLength], a.PaymentScriptHash[:])
	copy(data[UncompressedPublicKeyLength+types.HashLength:UncompressedPublicKeyLength+types.HashLength+types.HashLength], a.UnlockScriptHash[:])
	binary.LittleEndian.PutUint64(data[UncompressedPublicKeyLength+types.HashLength+types.HashLength:], a.PaymentMinCapacity)
	return data, nil
}

func (a *Address) UnmarshalBinary(data []byte) error {
	if len(data) != SerializedAddressLength {
		return errors.New("invalid address length")
	}
	pubKey, err := secp256k1.ParsePubKey(data[:UncompressedPublicKeyLength])
	if err != nil {
		return err
	}
	a.PubKey = pubKey
	copy(a.PaymentScriptHash[:], data[UncompressedPublicKeyLength:UncompressedPublicKeyLength+types.HashLength])
	copy(a.UnlockScriptHash[:], data[UncompressedPublicKeyLength+types.HashLength:UncompressedPublicKeyLength+types.HashLength+types.HashLength])
	a.PaymentMinCapacity = binary.LittleEndian.Uint64(data[UncompressedPublicKeyLength+types.HashLength+types.HashLength:])
	return nil
}

func (a Address) String() string {
	return hex.EncodeToString(a.PubKey.SerializeUncompressed())
}

func (a Address) Equal(address wallet.Address) bool {
	addr, ok := address.(*Address)
	if !ok {
		return false
	}
	return a.PubKey.IsEqual(addr.PubKey)
}

func (a Address) GetUncompressedSEC1() [65]byte {
	var sec1 [65]byte
	copy(sec1[:], a.PubKey.SerializeUncompressed())
	return sec1
}

func GetZeroAddress() *Address {
	return &Address{PubKey: secp256k1.NewPublicKey(new(secp256k1.FieldVal), new(secp256k1.FieldVal))}
}

func (a Address) Pack() (molecule.Participant, error) {
	if a.PubKey == nil {
		return molecule.Participant{}, errors.New("public key is nil")
	}
	pubKey := PackSEC1EncodedPubKey(a.PubKey)
	party := molecule.NewParticipantBuilder().
		PubKey(pubKey).
		PaymentScriptHash(*a.PaymentScriptHash.Pack()).
		UnlockScriptHash(*a.UnlockScriptHash.Pack()).
		PaymentMinCapacity(*types.PackUint64(a.PaymentMinCapacity)).
		Build()

	return party, nil
}

func PackSEC1EncodedPubKey(key *secp256k1.PublicKey) molecule.SEC1EncodedPubKey {
	sec1 := key.SerializeUncompressed()
	var bytes [65]molecule.Byte
	for i, b := range sec1 {
		bytes[i] = molecule.NewByte(b)
	}
	return molecule.NewSEC1EncodedPubKeyBuilder().Set(bytes).Build()
}
