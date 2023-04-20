package wallet

import (
	"errors"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/blake2b"
	"io"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/wallet/address"
)

type backend struct {
}

var Backend = backend{}

func init() {
	wallet.SetBackend(Backend)
}

func (b backend) NewAddress() wallet.Address {
	return &address.Address{}
}

func (b backend) DecodeSig(reader io.Reader) (wallet.Sig, error) {
	sig := make([]byte, PaddedSignatureLength)
	if _, err := io.ReadFull(reader, sig); err != nil {
		return nil, err
	}
	return sig, nil
}

func (b backend) VerifySignature(msg []byte, sig wallet.Sig, a wallet.Address) (bool, error) {
	addr, ok := a.(*address.Address)
	if !ok {
		return false, errors.New("address is not of type Address")
	}
	hash := blake2b.Sum256(msg)
	sigWithoutPadding, err := RemovePadding(sig)
	if err != nil {
		return false, err
	}
	signature, err := ecdsa.ParseDERSignature(sigWithoutPadding)
	if err != nil {
		return false, err
	}
	return signature.Verify(hash[:], addr.PubKey), nil
}
