package ckblp

import (
	"math/big"
	"testing"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/stretchr/testify/require"
)

func sampleLPCell() LPCell {
	minPrice := uint128.FromBig(new(big.Int).SetUint64(10))
	maxPrice := uint128.FromBig(new(big.Int).SetUint64(20))
	return LPCell{
		PoolID:                  filled32(0x11),
		OwnerLockHash:           filled32(0x22),
		OperatorLockHash:        filled32(0x33),
		AvailableCKB:            123,
		ReservedCKB:             456,
		CumulativeFeesEarnedCKB: 789,
		Policy: LPPolicy{
			MaxTradingVolume: 1000,
			FeeRateBps:       30,
			PolicyFlags:      3,
			PolicyVersion:    1,
			SafePriceMinX64:  minPrice,
			SafePriceMaxX64:  maxPrice,
		},
		Nonce:          7,
		Active:         true,
		EthBeneficiary: filled20(0x44),
	}
}

func filled20(value byte) [20]byte {
	var out [20]byte
	for i := range out {
		out[i] = value
	}
	return out
}

func filled32(value byte) [32]byte {
	var out [32]byte
	for i := range out {
		out[i] = value
	}
	return out
}

func TestEncodeDecodeLPCellRoundTrip(t *testing.T) {
	cell := sampleLPCell()
	data, err := EncodeLPCell(cell)
	require.NoError(t, err)
	require.Len(t, data, lpCellSize)

	decoded, err := DecodeLPCell(data)
	require.NoError(t, err)
	require.Equal(t, cell, decoded)
}

func TestDecodeLPCellRejectsWrongLength(t *testing.T) {
	_, err := DecodeLPCell(make([]byte, lpCellSize-1))
	require.ErrorIs(t, err, ErrInvalidLPCell)
}

func TestDecodeLPCellRejectsBadMagic(t *testing.T) {
	cell := sampleLPCell()
	data, err := EncodeLPCell(cell)
	require.NoError(t, err)
	data[0] = 0x00

	_, err = DecodeLPCell(data)
	require.ErrorIs(t, err, ErrInvalidLPCell)
}
