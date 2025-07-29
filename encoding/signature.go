package encoding

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	gpwallet "perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/wallet"
)

// NewDEREncodedSignatureFromPadded converts a padded signature to a DER encoded signature.
func NewDEREncodedSignatureFromPadded(paddedSignature []byte) (*molecule.Bytes, error) {
	sig, err := wallet.RemovePadding(paddedSignature)
	if err != nil {
		return nil, err
	}
	return types.PackBytes(sig), nil
}

// PackSignature converts a perun signature to a molecule signature.
func PackSignature(sig gpwallet.Sig) *molecule.Bytes {
	return types.PackBytes(sig)
}

// PackVCDispute encodes the signatures needed for a VCDispute into the contract' witnesses.
func PackVCDispute(sigA, sigB, parentSigA, parentSigB *molecule.Bytes) molecule.VCDispute {
	vcdispute := molecule.NewVCDisputeBuilder().
		SigA(*sigA).
		SigB(*sigB).
		ParentStateSigs(molecule.NewDisputeBuilder().
			SigA(*parentSigA).
			SigB(*parentSigB).
			Build()).
		Build()
	return vcdispute
}
