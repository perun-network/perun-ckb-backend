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
	CompressedPublicKeyLength   = 33
	Uint64Length                = 8
	SerializedParticipantLength = CompressedPublicKeyLength + types.HashLength + types.HashLength + Uint64Length
)

// Participant uniquely identifies a participant in a channel, encompassing all necessary on-chain information.
type Participant struct {
	// PubKey is the public key of the participant. It is used to verify signatures from the participant on channel
	// states.
	PubKey *secp256k1.PublicKey
	// PaymentScript hash is the script-hash of the payment script of the participant. Its preimage must be present
	// in an output of a transaction in order to address payments to this participant.
	PaymentScriptHash types.Hash
	// UnlockScriptHash is the script-hash of the unlock script of this participant. The participant uses it to authorize
	// itself to interact with a channel through an on-chain transaction.
	UnlockScriptHash types.Hash
	// PaymentMinCapacity is the minimum capacity required by the payment script of this participant to lock
	// CKBytes as payment to this participant.
	PaymentMinCapacity uint64
}

// NewDefaultParticipant creates a new participant with the script hash of the secp256k1_blake160_sighash_all script for
// the given public key as payment and unlock script hash.
func NewDefaultParticipant(pubKey *secp256k1.PublicKey) (*Participant, error) {
	if pubKey == nil {
		return nil, errors.New("public key is nil")
	}
	script, err := systemscript.Secp256K1Blake160SignhashAllByPublicKey(pubKey.SerializeCompressed())
	if err != nil {
		return nil, err
	}
	hash := script.Hash()
	return &Participant{
		PubKey:             pubKey,
		PaymentScriptHash:  hash,
		UnlockScriptHash:   hash,
		PaymentMinCapacity: script.OccupiedCapacity(),
	}, nil
}

func NewParticipant(pubKey *secp256k1.PublicKey, paymentScript, unlockScript *types.Script) *Participant {
	return &Participant{
		PubKey:             pubKey,
		PaymentScriptHash:  paymentScript.Hash(),
		UnlockScriptHash:   unlockScript.Hash(),
		PaymentMinCapacity: paymentScript.OccupiedCapacity(),
	}
}

// MarshalBinary encodes the participant into a binary representation.
// The encoding is as follows:
// <sec1 encoded public key: 33 bytes>
// | <payment script hash: 32 bytes>
// | <unlock script hash: 32 bytes>
// | <payment min capacity: 8 bytes (uint64 little endian)>
func (p Participant) MarshalBinary() (data []byte, err error) {
	if p.PubKey == nil {
		return nil, errors.New("public key is nil")
	}
	data = make([]byte, SerializedParticipantLength)
	copy(data[:CompressedPublicKeyLength], p.PubKey.SerializeCompressed())
	copy(data[CompressedPublicKeyLength:CompressedPublicKeyLength+types.HashLength], p.PaymentScriptHash[:])
	copy(data[CompressedPublicKeyLength+types.HashLength:CompressedPublicKeyLength+types.HashLength+types.HashLength], p.UnlockScriptHash[:])
	binary.LittleEndian.PutUint64(data[CompressedPublicKeyLength+types.HashLength+types.HashLength:], p.PaymentMinCapacity)
	return data, nil
}

// UnmarshalBinary decodes the participant from a binary representation. See MarshalBinary for the expected encoding.
func (p *Participant) UnmarshalBinary(data []byte) error {
	if len(data) != SerializedParticipantLength {
		return errors.New("invalid address length")
	}
	pubKey, err := secp256k1.ParsePubKey(data[:CompressedPublicKeyLength])
	if err != nil {
		return err
	}
	p.PubKey = pubKey
	copy(p.PaymentScriptHash[:], data[CompressedPublicKeyLength:CompressedPublicKeyLength+types.HashLength])
	copy(p.UnlockScriptHash[:], data[CompressedPublicKeyLength+types.HashLength:CompressedPublicKeyLength+types.HashLength+types.HashLength])
	p.PaymentMinCapacity = binary.LittleEndian.Uint64(data[CompressedPublicKeyLength+types.HashLength+types.HashLength:])
	return nil
}

func (p Participant) String() string {
	return hex.EncodeToString(p.PubKey.SerializeCompressed())
}

func (p Participant) Equal(address wallet.Address) bool {
	other, ok := address.(*Participant)
	if !ok {
		return false
	}
	if p.UnlockScriptHash != other.UnlockScriptHash ||
		p.PaymentScriptHash != other.PaymentScriptHash ||
		p.PaymentMinCapacity != other.PaymentMinCapacity {
		return false
	}
	return p.PubKey.IsEqual(other.PubKey)
}

// GetUncompressedSEC1 returns the uncompressed SEC1 encoded public key of the participant.
func (p Participant) GetUncompressedSEC1() [UncompressedPublicKeyLength]byte {
	var sec1 [UncompressedPublicKeyLength]byte
	copy(sec1[:], p.PubKey.SerializeUncompressed())
	return sec1
}

// GetCompressedSEC1 returns the compressed SEC1 encoded public key of the participant.
func (p Participant) GetCompressedSEC1() [CompressedPublicKeyLength]byte {
	var sec1 [CompressedPublicKeyLength]byte
	copy(sec1[:], p.PubKey.SerializeCompressed())
	return sec1
}

// GetZeroAddress returns the zero participant. Its public key is the zero public key.
func GetZeroAddress() *Participant {
	return &Participant{PubKey: secp256k1.NewPublicKey(new(secp256k1.FieldVal), new(secp256k1.FieldVal))}
}

// Pack packs the participant into a molecule participant (on-chain encoding of a participant).
func (p Participant) Pack() (molecule.Participant, error) {
	if p.PubKey == nil {
		return molecule.Participant{}, errors.New("public key is nil")
	}
	pubKey := PackSEC1EncodedPubKey(p.PubKey)
	party := molecule.NewParticipantBuilder().
		PubKey(pubKey).
		PaymentScriptHash(*p.PaymentScriptHash.Pack()).
		UnlockScriptHash(*p.UnlockScriptHash.Pack()).
		PaymentMinCapacity(*types.PackUint64(p.PaymentMinCapacity)).
		Build()

	return party, nil
}

// PackSEC1EncodedPubKey packs the given public key into a molecule SEC1 encoded public key (compressed).
func PackSEC1EncodedPubKey(key *secp256k1.PublicKey) molecule.SEC1EncodedPubKey {
	sec1 := key.SerializeCompressed()
	var bytes [CompressedPublicKeyLength]molecule.Byte
	for i, b := range sec1 {
		bytes[i] = molecule.NewByte(b)
	}
	return molecule.NewSEC1EncodedPubKeyBuilder().Set(bytes).Build()
}
