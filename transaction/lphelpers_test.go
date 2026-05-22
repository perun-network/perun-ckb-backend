package transaction

import (
	"math/big"
	"testing"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
)

func filled32(b byte) types.Hash {
	var out types.Hash
	for i := range out {
		out[i] = b
	}
	return out
}

func TestEncodeFundChannelExtractWitness(t *testing.T) {
	channelID := filled32(0xAA)
	contribID := filled32(0xBB)
	witness := EncodeFundChannelExtractWitness(channelID, contribID, 42)

	require.Len(t, witness, witnessLenFundChannelExtract)
	require.Equal(t, opFundChannelExtract, witness[0])
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

	witness := EncodeSettleChannelInsertWitness(channelID, contribID, 100, 7, priceX64)

	require.Len(t, witness, witnessLenSettleChannelInsert)
	require.Equal(t, opSettleChannelInsert, witness[0])
	require.Equal(t, channelID[:], witness[1:33])
	require.Equal(t, contribID[:], witness[33:65])

	var expectedPrice [16]byte
	uint128.StoreLittleEndian(expectedPrice[:], priceX64)
	require.Equal(t, expectedPrice[:], witness[81:97])
}
