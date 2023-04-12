package encoding

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/perun-ckb-backend/wallet"
)

func NewSEC1EncodedPubKey(address wallet.Address) molecule.SEC1EncodedPubKey {
	sec1 := address.GetUncompressedSEC1()
	var bytes [65]molecule.Byte
	for i, b := range sec1 {
		bytes[i] = molecule.NewByte(b)
	}
	return molecule.NewSEC1EncodedPubKeyBuilder().Set(bytes).Build()
}
