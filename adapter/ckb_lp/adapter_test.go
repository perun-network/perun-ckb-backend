package ckblp

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/stretchr/testify/require"
)

func TestBuildSettleChannelInsertTxRejectsZeroPrice(t *testing.T) {
	adapter := &Adapter{}

	err := adapter.BuildSettleChannelInsertTx(
		context.Background(),
		"",
		"",
		"",
		0,
		0,
		uint128.Uint128{},
	)

	require.ErrorIs(t, err, ErrZeroPrice)
	require.True(t, IsDeterministic(err))
}

func TestBuildFundChannelTxRejectsZeroChannelID(t *testing.T) {
	adapter := &Adapter{}
	zeroHash := "0x" + strings.Repeat("00", 32)

	err := adapter.BuildFundChannelTx(
		context.Background(),
		zeroHash,
		"0x"+strings.Repeat("11", 32)+":0",
		1,
	)

	require.ErrorIs(t, err, ErrInvalidChannelID)
	require.True(t, IsDeterministic(err))
}

func TestBuildSettleChannelInsertTxRejectsZeroContributionID(t *testing.T) {
	adapter := &Adapter{}
	channelID := "0x" + strings.Repeat("11", 32)
	zeroContribution := "0x" + strings.Repeat("00", 32)

	err := adapter.BuildSettleChannelInsertTx(
		context.Background(),
		channelID,
		zeroContribution,
		"0x"+strings.Repeat("22", 32)+":0",
		1,
		0,
		uint128.FromBig(bigOne()),
	)

	require.ErrorIs(t, err, ErrInvalidContributionID)
	require.True(t, IsDeterministic(err))
}

func bigOne() *big.Int {
	return big.NewInt(1)
}
