package molecule

import (
	"encoding/binary"
	"errors"
	"github.com/Pilatuz/bigz/uint128"
	"math/big"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
)

func PackByte32(b [32]byte) *molecule.Byte32 {
	return molecule.Byte32FromSliceUnchecked(b[:])
}

func UnpackByte32(b *molecule.Byte32) ([32]byte, error) {
	var arr32 [32]byte
	source := b.AsSlice()
	for i := range arr32 {
		arr32[i] = source[i]
	}
	return arr32, nil
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

func UnpackSEC1EncodedPubKey(b *molecule.SEC1EncodedPubKey) (*secp256k1.PublicKey, error) {
	return secp256k1.ParsePubKey(b.AsSlice())
}

func PackUint128(x *big.Int) (*molecule.Uint128, error) {
	u128, err := ToUint128(x)
	if err != nil {
		return nil, err
	}
	bytes := make([]byte, 16)
	uint128.StoreLittleEndian(bytes[:], u128)
	return molecule.Uint128FromSliceUnchecked(bytes), nil
}

func UnpackUint128(x *molecule.Uint128) uint128.Uint128 {
	return uint128.LoadLittleEndian(x.AsSlice())
}

func ToUint128(x *big.Int) (uint128.Uint128, error) {
	if x.Cmp(uint128.Max().Big()) == 1 {
		return uint128.Uint128{}, errors.New("uint128 overflow")
	}
	if x.Sign() == -1 {
		return uint128.Uint128{}, errors.New("uint128 underflow")
	}
	return uint128.FromBig(x), nil
}
