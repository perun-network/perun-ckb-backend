package encoding

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
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

func PackIndexMap(indexMap []channel.Index) molecule.IndexMap {
	indexMapBuilder := molecule.NewIndexMapBuilder()
	indexMapBuilder.Nth0(*types.PackByte(byte(indexMap[0])))
	indexMapBuilder.Nth1(*types.PackByte(byte(indexMap[1])))
	return indexMapBuilder.Build()
}
