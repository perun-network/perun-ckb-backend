package encoding

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	gpwallet "perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/wallet"
)

func NewDEREncodedSignatureFromPadded(paddedSignature []byte) (*molecule.Bytes, error) {
	sig, err := wallet.RemovePadding(paddedSignature)
	if err != nil {
		return nil, err
	}
	return types.PackBytes(sig), nil
}

func PackSignature(sig gpwallet.Sig) *molecule.Bytes {
	return types.PackBytes(sig)
}
