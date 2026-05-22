package ckblp

import (
	"bytes"
	"encoding/binary"

	"github.com/Pilatuz/bigz/uint128"
)

const (
	lpCellSize = 185

	// lpCellMinOccupiedShannons is the minimum CKB capacity required by CKB's
	// occupied-capacity rule: 8 B (capacity field) + data + lock script + type script.
	// Lock and type scripts each carry a 32-byte code_hash, 1-byte hash_type, and
	// 32-byte args (type-script hash and pool-id respectively) = 65 bytes each.
	lpCellMinOccupiedShannons = (8 + lpCellSize + (32 + 1 + 32) + (32 + 1 + 32)) * 100_000_000
)

var lpMagic = []byte{'L', 'P', 'L', 'C'}

const (
	lpOffsetPoolID           = 4
	lpOffsetOwnerLockHash    = lpOffsetPoolID + 32
	lpOffsetOperatorLockHash = lpOffsetOwnerLockHash + 32
	lpOffsetAvailableCKB     = lpOffsetOperatorLockHash + 32
	lpOffsetReservedCKB      = lpOffsetAvailableCKB + 8
	lpOffsetCumulativeFees   = lpOffsetReservedCKB + 8
	lpOffsetMaxTradingVolume = lpOffsetCumulativeFees + 8
	lpOffsetFeeRateBps       = lpOffsetMaxTradingVolume + 8
	lpOffsetPolicyFlags      = lpOffsetFeeRateBps + 4
	lpOffsetPolicyVersion    = lpOffsetPolicyFlags + 4
	lpOffsetSafePriceMinX64  = lpOffsetPolicyVersion + 4
	lpOffsetSafePriceMaxX64  = lpOffsetSafePriceMinX64 + 16
	lpOffsetNonce            = lpOffsetSafePriceMaxX64 + 16
	lpOffsetActive           = lpOffsetNonce + 8

	lpEndPoolID           = lpOffsetPoolID + 32
	lpEndOwnerLockHash    = lpOffsetOwnerLockHash + 32
	lpEndOperatorLockHash = lpOffsetOperatorLockHash + 32
	lpEndAvailableCKB     = lpOffsetAvailableCKB + 8
	lpEndReservedCKB      = lpOffsetReservedCKB + 8
	lpEndCumulativeFees   = lpOffsetCumulativeFees + 8
	lpEndMaxTradingVolume = lpOffsetMaxTradingVolume + 8
	lpEndFeeRateBps       = lpOffsetFeeRateBps + 4
	lpEndPolicyFlags      = lpOffsetPolicyFlags + 4
	lpEndPolicyVersion    = lpOffsetPolicyVersion + 4
	lpEndSafePriceMinX64  = lpOffsetSafePriceMinX64 + 16
	lpEndSafePriceMaxX64  = lpOffsetSafePriceMaxX64 + 16
	lpEndNonce            = lpOffsetNonce + 8
	lpEndActive           = lpOffsetActive + 1
)

// EncodeLPCell encodes LP cell data using the raw fixed layout from pool.rs.
func EncodeLPCell(cell LPCell) ([]byte, error) {
	buf := make([]byte, 0, lpCellSize)
	buf = append(buf, lpMagic...)
	buf = append(buf, cell.PoolID[:]...)
	buf = append(buf, cell.OwnerLockHash[:]...)
	buf = append(buf, cell.OperatorLockHash[:]...)

	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], cell.AvailableCKB)
	buf = append(buf, tmp[:]...)
	binary.LittleEndian.PutUint64(tmp[:], cell.ReservedCKB)
	buf = append(buf, tmp[:]...)
	binary.LittleEndian.PutUint64(tmp[:], cell.CumulativeFeesEarnedCKB)
	buf = append(buf, tmp[:]...)
	binary.LittleEndian.PutUint64(tmp[:], cell.Policy.MaxTradingVolume)
	buf = append(buf, tmp[:]...)

	var tmp32 [4]byte
	binary.LittleEndian.PutUint32(tmp32[:], cell.Policy.FeeRateBps)
	buf = append(buf, tmp32[:]...)
	binary.LittleEndian.PutUint32(tmp32[:], cell.Policy.PolicyFlags)
	buf = append(buf, tmp32[:]...)
	binary.LittleEndian.PutUint32(tmp32[:], cell.Policy.PolicyVersion)
	buf = append(buf, tmp32[:]...)

	var tmp128 [16]byte
	uint128.StoreLittleEndian(tmp128[:], cell.Policy.SafePriceMinX64)
	buf = append(buf, tmp128[:]...)
	uint128.StoreLittleEndian(tmp128[:], cell.Policy.SafePriceMaxX64)
	buf = append(buf, tmp128[:]...)

	binary.LittleEndian.PutUint64(tmp[:], cell.Nonce)
	buf = append(buf, tmp[:]...)

	if cell.Active {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}

	if len(buf) != lpCellSize {
		return nil, ErrInvalidLPCell
	}
	return buf, nil
}

// DecodeLPCell decodes LP cell data using the raw fixed layout from pool.rs.
func DecodeLPCell(data []byte) (LPCell, error) {
	if len(data) != lpCellSize {
		return LPCell{}, ErrInvalidLPCell
	}
	if !bytes.Equal(data[:len(lpMagic)], lpMagic) {
		return LPCell{}, ErrInvalidLPCell
	}

	var cell LPCell
	copy(cell.PoolID[:], data[lpOffsetPoolID:lpEndPoolID])
	copy(cell.OwnerLockHash[:], data[lpOffsetOwnerLockHash:lpEndOwnerLockHash])
	copy(cell.OperatorLockHash[:], data[lpOffsetOperatorLockHash:lpEndOperatorLockHash])

	cell.AvailableCKB = binary.LittleEndian.Uint64(data[lpOffsetAvailableCKB:lpEndAvailableCKB])
	cell.ReservedCKB = binary.LittleEndian.Uint64(data[lpOffsetReservedCKB:lpEndReservedCKB])
	cell.CumulativeFeesEarnedCKB = binary.LittleEndian.Uint64(data[lpOffsetCumulativeFees:lpEndCumulativeFees])
	cell.Policy.MaxTradingVolume = binary.LittleEndian.Uint64(data[lpOffsetMaxTradingVolume:lpEndMaxTradingVolume])
	cell.Policy.FeeRateBps = binary.LittleEndian.Uint32(data[lpOffsetFeeRateBps:lpEndFeeRateBps])
	cell.Policy.PolicyFlags = binary.LittleEndian.Uint32(data[lpOffsetPolicyFlags:lpEndPolicyFlags])
	cell.Policy.PolicyVersion = binary.LittleEndian.Uint32(data[lpOffsetPolicyVersion:lpEndPolicyVersion])

	cell.Policy.SafePriceMinX64 = uint128.LoadLittleEndian(data[lpOffsetSafePriceMinX64:lpEndSafePriceMinX64])
	cell.Policy.SafePriceMaxX64 = uint128.LoadLittleEndian(data[lpOffsetSafePriceMaxX64:lpEndSafePriceMaxX64])

	cell.Nonce = binary.LittleEndian.Uint64(data[lpOffsetNonce:lpEndNonce])
	cell.Active = data[lpOffsetActive] != 0

	return cell, nil
}

// IsLPCell checks the magic prefix for a potential LP cell payload.
func IsLPCell(data []byte) bool {
	return len(data) >= len(lpMagic) && bytes.Equal(data[:len(lpMagic)], lpMagic)
}
