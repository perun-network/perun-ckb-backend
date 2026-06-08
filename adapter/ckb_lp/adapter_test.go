package ckblp

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"github.com/stretchr/testify/require"
	ckbencoding "perun.network/perun-ckb-backend/encoding"
	"perun.network/perun-ckb-backend/transaction"
)

func TestBuildSettleChannelInsertTxRejectsZeroPrice(t *testing.T) {
	adapter := &Adapter{}

	_, err := adapter.BuildSettleChannelInsertTx(
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

	_, err := adapter.BuildFundChannelTx(
		context.Background(),
		zeroHash,
		"0x"+strings.Repeat("11", 32)+":0",
		1,
		"",
	)

	require.ErrorIs(t, err, ErrInvalidChannelID)
	require.True(t, IsDeterministic(err))
}

func TestBuildSettleChannelInsertTxRejectsZeroContributionID(t *testing.T) {
	adapter := &Adapter{}
	channelID := "0x" + strings.Repeat("11", 32)
	zeroContribution := "0x" + strings.Repeat("00", 32)

	_, err := adapter.BuildSettleChannelInsertTx(
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

func TestBuildLPDepositTxUnsignedRejectsZeroAvailable(t *testing.T) {
	adapter := &Adapter{}

	_, _, err := adapter.BuildLPDepositTxUnsigned(
		context.Background(),
		LPCell{AvailableCKB: 0},
		types.Hash{},
	)

	require.ErrorIs(t, err, ErrInvalidLPCellArg)
	require.True(t, IsDeterministic(err))
}

func TestBuildLPWithdrawTxUnsignedRejectsZeroCkbOut(t *testing.T) {
	adapter := &Adapter{}

	_, err := adapter.BuildLPWithdrawTxUnsigned(
		context.Background(),
		"0x"+strings.Repeat("11", 32)+":0",
		0,
	)

	require.ErrorIs(t, err, ErrInvalidLPCellArg)
	require.True(t, IsDeterministic(err))
}

func TestSubmitSignedTxRejectsNonRPCTransactor(t *testing.T) {
	adapter := &Adapter{}

	_, err := adapter.SubmitSignedTx(context.Background(), &types.Transaction{})

	require.ErrorIs(t, err, ErrUnsupportedTransactor)
	require.True(t, IsDeterministic(err))
}

func TestBuildProxyChannelDataEncodesChannelID(t *testing.T) {
	var channelHash types.Hash
	channelHash[0] = 0x12
	channelHash[31] = 0x34

	status, err := molecule.ChannelStatusFromSlice(transaction.BuildProxyChannelData(channelHash), false)
	require.NoError(t, err)
	require.Equal(t, channelHash, types.UnpackHash(status.State().ChannelId()))
	require.False(t, ckbencoding.ToBool(*status.Funded()))
	require.False(t, ckbencoding.ToBool(*status.Disputed()))
}

func bigOne() *big.Int {
	return big.NewInt(1)
}
