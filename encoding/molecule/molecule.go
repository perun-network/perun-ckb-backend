package molecule

import (
	"encoding/binary"
	"errors"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
)

func PackByte32(b [32]byte) *molecule.Byte32 {
	return molecule.Byte32FromSliceUnchecked(b[:])
}

func UnpackUint64(x *molecule.Uint64) uint64 {
	return binary.LittleEndian.Uint64(x.AsSlice())
}

func ToHashType(b *molecule.Byte) (types.ScriptHashType, error) {
	if b == nil {
		return "", errors.New("nil byte")
	}
	return types.DeserializeHashTypeByte(b[0])
}
