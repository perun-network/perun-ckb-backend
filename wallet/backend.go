package wallet

import (
	"errors"
	"io"

	"github.com/ethereum/go-ethereum/crypto"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/wallet/address"
)

type backend struct{}

var Backend = backend{}

func init() {
	wallet.SetBackend(Backend, address.CKBBackendID)
}

// NewAddress returns an empty address.Participant to marshal into.
func (b backend) NewAddress() wallet.Address {
	return &address.Participant{}
}

// DecodeSig expects to read a DER encoded signature from the reader of length PaddedSignatureLength.
// The padding used is defined by PadDEREncodedSignature / RemovePadding.
// The signature is then returned (still padded, as VerifySignature also expects a padded signature).
func (b backend) DecodeSig(reader io.Reader) (wallet.Sig, error) {
	sig := make([]byte, 65)
	if _, err := io.ReadFull(reader, sig); err != nil {
		return nil, err
	}
	return sig, nil
}

// VerifySignature returns whether given signature is valid for given message and public key of the given address.
// It expects to receive the plain message, not the message hash.
// It expects a padded signature (see PadDEREncodedSignature / RemovePadding).
func (b backend) VerifySignature(msg []byte, sig wallet.Sig, a wallet.Address) (bool, error) {
	addr, ok := a.(*address.Participant)
	if !ok {
		return false, errors.New("address is not of type Participant")
	}
	hash := crypto.Keccak256(msg)
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	hash = crypto.Keccak256(prefix, hash)
	sigCopy := make([]byte, 65) //nolint:gomnd
	copy(sigCopy, sig)
	if len(sigCopy) == 65 && (sigCopy[65-1] >= 27) { //nolint:gomnd
		sigCopy[65-1] -= 27
	}
	pk, err := crypto.SigToPub(hash, sigCopy)
	if err != nil {
		return false, err
	}
	recovered := crypto.PubkeyToAddress(*pk)
	expected := crypto.PubkeyToAddress(*addr.PubKey.ToECDSA())
	return recovered == expected, nil
}
