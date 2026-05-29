package address

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/nervosnetwork/ckb-sdk-go/v2/address"
	"github.com/nervosnetwork/ckb-sdk-go/v2/systemscript"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/wallet"
)

const (
	UncompressedPublicKeyLength = 65
	CompressedPublicKeyLength   = 33
	CKBBackendID                = 3
)

var _ wallet.Address = (*Participant)(nil)

// Participant uniquely identifies a participant in a channel, encompassing all necessary on-chain information.
type Participant struct {
	// PubKey is the public key of the participant. It is used to verify signatures from the participant on channel
	// states.
	PubKey *secp256k1.PublicKey
	// PaymentScript hash is the script-hash of the payment script of the participant. Its preimage must be present
	// in an output of a transaction in order to address payments to this participant.
	PaymentScript *types.Script
	// UnlockScriptHash is the script-hash of the unlock script of this participant. The participant uses it to authorize
	// itself to interact with a channel through an on-chain transaction.
	UnlockScript *types.Script
}

func (p Participant) BackendID() wallet.BackendID {
	return CKBBackendID
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
	return &Participant{
		PubKey:        pubKey,
		PaymentScript: script,
		UnlockScript:  script,
	}, nil
}

func NewEthereumParticipantFromPublicKey(key *secp256k1.PublicKey, omniCodeHash types.Hash) (*Participant, [20]byte, error) {
	var ethAddr [20]byte
	if omniCodeHash == (types.Hash{}) {
		return nil, ethAddr, fmt.Errorf("omni-lock code hash must be provided")
	}

	ecdsaPubKey := key.ToECDSA()
	pubBytes := crypto.FromECDSAPub(ecdsaPubKey)[1:]

	// Keccak256 hash of pubkey, then take last 20 bytes (Ethereum address)
	ethAddr = [20]byte(crypto.Keccak256(pubBytes)[12:])

	args := append([]byte{0x12}, ethAddr[:]...)
	args = append(args, 0x00)

	script := &types.Script{
		CodeHash: omniCodeHash,
		HashType: types.HashTypeType, // Omni-lock is typically deployed with `type`
		Args:     args,
	}

	return &Participant{
		PubKey:        key,
		PaymentScript: script,
		UnlockScript:  script, // same unless separated
	}, ethAddr, nil
}

func NewCrossChainParticipantFromPublicKeys(CKBL1Key *secp256k1.PublicKey, EthL1Key *secp256k1.PublicKey, omniCodeHash types.Hash) (*Participant, [20]byte, error) {
	var ethAddr [20]byte
	if omniCodeHash == (types.Hash{}) {
		return nil, ethAddr, fmt.Errorf("omni-lock code hash must be provided")
	}

	ecdsaEthPubKey := EthL1Key.ToECDSA()
	pubBytesEth := crypto.FromECDSAPub(ecdsaEthPubKey)[1:]

	// Keccak256 hash of pubkey, then take last 20 bytes (Ethereum address)
	ethAddr = [20]byte(crypto.Keccak256(pubBytesEth)[12:])

	args := append([]byte{0x12}, ethAddr[:]...)
	args = append(args, 0x00)

	script := &types.Script{
		CodeHash: omniCodeHash,
		HashType: types.HashTypeType, // Omni-lock is typically deployed with `type`
		Args:     args,
	}

	return &Participant{
		PubKey:        CKBL1Key,
		PaymentScript: script,
		UnlockScript:  script, // same unless separated
	}, ethAddr, nil
}

func NewParticipant(pubKey *secp256k1.PublicKey, paymentScript, unlockScript *types.Script) *Participant {
	return &Participant{
		PubKey:        pubKey,
		PaymentScript: paymentScript,
		UnlockScript:  unlockScript,
	}
}

// MarshalBinary encodes the participant into a binary representation as a molecule.OffChainParticipant.
func (p Participant) MarshalBinary() ([]byte, error) {
	offChainParticipant, err := p.PackOnChainParticipant()
	return offChainParticipant.AsSlice(), err
}

// UnmarshalBinary decodes the participant from a molecule.OffChainParticipant.
func (p *Participant) UnmarshalBinary(data []byte) error {
	onChainParticipant, err := molecule.ParticipantFromSlice(data, true)
	if err != nil {
		return err
	}
	return p.UnpackOnChainParticipant(onChainParticipant)
}

func (p Participant) String() string {
	return hex.EncodeToString(p.PubKey.SerializeCompressed())
}

// Equal returns true, iff the given address is a participant with the same public key, payment script and unlock script.
func (p Participant) Equal(address wallet.Address) bool {
	other, ok := address.(*Participant)
	if !ok {
		return false
	}
	return p.UnlockScript.Equals(other.UnlockScript) &&
		p.PubKey.IsEqual(other.PubKey) &&
		p.PaymentScript.Equals(other.PaymentScript)
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
	return &Participant{
		PubKey:        secp256k1.NewPublicKey(new(secp256k1.FieldVal), new(secp256k1.FieldVal)),
		PaymentScript: &types.Script{HashType: types.HashTypeData},
		UnlockScript:  &types.Script{HashType: types.HashTypeData},
	}
}

// PackOffChainParticipant packs the participant into a molecule.OffChainParticipant
// (off-chain encoding of a participant).
func (p Participant) PackOffChainParticipant() (molecule.OffChainParticipant, error) {
	key, err := PackSEC1EncodedPubKey(p.PubKey)
	if err != nil {
		return molecule.OffChainParticipant{}, err
	}
	return molecule.NewOffChainParticipantBuilder().
		PubKey(key).
		PaymentScript(*p.PaymentScript.Pack()).
		UnlockScript(*p.UnlockScript.Pack()).
		Build(), nil
}

// PackOnChainParticipant packs the participant into a molecule.Participant (on-chain encoding of a participant).
func (p Participant) PackOnChainParticipant() (molecule.Participant, error) {
	pubKey, err := PackSEC1EncodedPubKey(p.PubKey)
	if err != nil {
		return molecule.Participant{}, err
	}
	psh := p.PaymentScript.Hash()
	ush := p.UnlockScript.Hash()
	party := molecule.NewParticipantBuilder().
		PubKey(pubKey).
		PaymentScriptHash(*psh.Pack()).
		UnlockScriptHash(*ush.Pack()).
		Build()

	return party, nil
}

// PackSEC1EncodedPubKey packs the given public key into a molecule SEC1 encoded public key (compressed).
func PackSEC1EncodedPubKey(key *secp256k1.PublicKey) (molecule.SEC1EncodedPubKey, error) {
	if key == nil {
		return molecule.SEC1EncodedPubKey{}, errors.New("public key is nil")
	}
	sec1 := key.SerializeCompressed()
	var bytes [CompressedPublicKeyLength]molecule.Byte
	for i, b := range sec1 {
		bytes[i] = molecule.NewByte(b)
	}
	return molecule.NewSEC1EncodedPubKeyBuilder().Set(bytes).Build(), nil
}

// UnpackSEC1EncodedPubKey unpacks the given molecule SEC1 encoded public key (compressed) into a public key.
func UnpackSEC1EncodedPubKey(key *molecule.SEC1EncodedPubKey) (*secp256k1.PublicKey, error) {
	if key == nil {
		return nil, errors.New("public key is nil")
	}
	sec1 := key.AsSlice()
	if len(sec1) != CompressedPublicKeyLength {
		return nil, errors.New("invalid public key length")
	}
	return secp256k1.ParsePubKey(sec1)
}

func GetSecp256k1Blake160SighashAll(key *secp256k1.PublicKey) (*types.Script, error) {
	if key == nil {
		return nil, errors.New("public key is nil")
	}
	return systemscript.Secp256K1Blake160SignhashAllByPublicKey(key.SerializeCompressed())
}

// AsParticipant returns the participant from the given address.
// It panics if the address is not a participant.
func AsParticipant(address wallet.Address) *Participant {
	p, ok := address.(*Participant)
	if !ok {
		panic("address is not participant")
	}
	return p
}

// IsParticipant returns the participant from the given address.
// It returns an error if the address is not a participant.
func IsParticipant(address wallet.Address) (*Participant, error) {
	p, ok := address.(*Participant)
	if !ok {
		return nil, errors.New("address is not participant")
	}
	return p, nil
}

// UnpackOffChainParticipant unpacks the given molecule.OffChainParticipant into the participant.
func (p *Participant) UnpackOffChainParticipant(op *molecule.OffChainParticipant) error {
	key, err := UnpackSEC1EncodedPubKey(op.PubKey())
	if err != nil {
		return err
	}
	p.PubKey = key
	p.PaymentScript = types.UnpackScript(op.PaymentScript())
	p.UnlockScript = types.UnpackScript(op.UnlockScript())
	return nil
}

// RecoverOnChainPaymentScript reconstructs the full payment script of an on-chain
// participant. The on-chain encoding (PackOnChainParticipant) stores only the payment
// script *hash*, so the script cannot be read back directly. We rebuild candidate scripts
// from the stored public key — the default secp256k1-blake160 sighash script and the
// omni-lock script — and return the one whose hash matches the stored PaymentScriptHash.
// This recovers the correct script for omni-lock / EVM participants instead of defaulting
// to a sighash address they do not control.
func RecoverOnChainPaymentScript(op *molecule.Participant, omniCodeHash types.Hash) (*types.Script, error) {
	key, err := UnpackSEC1EncodedPubKey(op.PubKey())
	if err != nil {
		return nil, err
	}
	want := op.PaymentScriptHash().AsSlice()

	defaultParticipant, err := NewDefaultParticipant(key)
	if err != nil {
		return nil, err
	}
	if h := defaultParticipant.PaymentScript.Hash(); bytes.Equal(h[:], want) {
		return defaultParticipant.PaymentScript, nil
	}

	if omniCodeHash != (types.Hash{}) {
		omniParticipant, _, err := NewEthereumParticipantFromPublicKey(key, omniCodeHash)
		if err != nil {
			return nil, err
		}
		if h := omniParticipant.PaymentScript.Hash(); bytes.Equal(h[:], want) {
			return omniParticipant.PaymentScript, nil
		}
	}

	return nil, fmt.Errorf("could not recover on-chain payment script: no candidate matches stored hash %x", want)
}

func (p *Participant) UnpackOnChainParticipant(op *molecule.Participant) error {
	key, err := UnpackSEC1EncodedPubKey(op.PubKey())
	if err != nil {
		return err
	}

	script, err := systemscript.Secp256K1Blake160SignhashAllByPublicKey(key.SerializeCompressed())
	if err != nil {
		return err
	}
	p.PubKey = key
	p.PaymentScript = script
	p.UnlockScript = script
	return nil
}

// ToCKBAddress allows to convert the given participant to a CKB address which can be used
// with the CKB SDK. The script is set to the PaymentScript of the participant.
func (p Participant) ToCKBAddress(network types.Network) address.Address {
	return address.Address{
		Script:  p.PaymentScript,
		Network: network,
	}
}
