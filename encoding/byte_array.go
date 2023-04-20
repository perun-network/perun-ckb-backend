package encoding

import "github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"

func PackByte32(b [32]byte) *molecule.Byte32 {
	return molecule.Byte32FromSliceUnchecked(b[:])
}
