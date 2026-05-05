package ckblp

import (
	"math/big"
	"testing"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/stretchr/testify/require"
)

func TestEncodeFundChannelExtractWitness(t *testing.T) {
	channelID := filled32(0xAA)
	contribID := filled32(0xBB)
	witness := EncodeFundChannelExtractWitness(FundChannelExtractWitness{
		ChannelID:      channelID,
		ContributionID: contribID,
		ExtractCKB:     42,
	})

	require.Len(t, witness, witnessLenFundChannelExtract)
	require.Equal(t, byte(opFundChannelExtract), witness[0])
	require.Equal(t, channelID[:], witness[1:33])
	require.Equal(t, contribID[:], witness[33:65])
	require.Equal(t, byte(42), witness[65])
}

func TestEncodeSettleChannelInsertWitness(t *testing.T) {
	channelID := filled32(0x01)
	contribID := filled32(0x02)
	priceBig := new(big.Int).Lsh(big.NewInt(1), 65)
	priceBig.Add(priceBig, big.NewInt(5))
	priceX64 := uint128.FromBig(priceBig)

	witness := EncodeSettleChannelInsertWitness(SettleChannelInsertWitness{
		ChannelID:         channelID,
		ContributionID:    contribID,
		PrincipalReturned: 100,
		FeeCKB:            7,
		PriceX64:          priceX64,
	})

	require.Len(t, witness, witnessLenSettleChannelInsert)
	require.Equal(t, byte(opSettleChannelInsert), witness[0])
	require.Equal(t, channelID[:], witness[1:33])
	require.Equal(t, contribID[:], witness[33:65])

	var expectedPrice [16]byte
	uint128.StoreLittleEndian(expectedPrice[:], priceX64)
	require.Equal(t, expectedPrice[:], witness[81:97])
}
