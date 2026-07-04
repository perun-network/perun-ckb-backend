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
		nil,
		types.Hash{},
	)

	require.ErrorIs(t, err, ErrInvalidLPCellArg)
	require.True(t, IsDeterministic(err))
}

func TestBuildLPDepositTxUnsignedRejectsNilOwnerScript(t *testing.T) {
	adapter := &Adapter{}

	_, _, err := adapter.BuildLPDepositTxUnsigned(
		context.Background(),
		LPCell{AvailableCKB: MinLPCellOccupiedShannons},
		nil,
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
		nil,
	)

	require.ErrorIs(t, err, ErrInvalidLPCellArg)
	require.True(t, IsDeterministic(err))
}

func TestBuildLPWithdrawTxUnsignedRejectsNilOwnerScript(t *testing.T) {
	adapter := &Adapter{}

	_, err := adapter.BuildLPWithdrawTxUnsigned(
		context.Background(),
		"0x"+strings.Repeat("11", 32)+":0",
		1,
		nil,
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

// TestUpdatedLPCellCapacity locks the occupied-capacity floor for the rebuilt
// LP cell in fund-extract and LP-withdraw. The numbers replay the fullcycle6
// failure: a 500-CKB cell cannot serve a 380-CKB extract (remainder 120 CKB <
// 323 CKB occupied), while a 4000-CKB cell can (remainder 3620 CKB).
func TestUpdatedLPCellCapacity(t *testing.T) {
	const ckb = uint64(100_000_000) // shannons per CKB

	// The dust cell from the bug report: rejected, deterministic.
	_, err := updatedLPCellCapacity(500*ckb, 380*ckb)
	require.ErrorIs(t, err, ErrInsufficientLPCellCapacity)
	require.True(t, IsDeterministic(err))

	// The 4000-CKB cell survives the same extract.
	got, err := updatedLPCellCapacity(4000*ckb, 380*ckb)
	require.NoError(t, err)
	require.Equal(t, 3620*ckb, got)

	// Exactly at the floor is allowed (remainder == occupied).
	got, err = updatedLPCellCapacity(380*ckb+MinLPCellOccupiedShannons, 380*ckb)
	require.NoError(t, err)
	require.Equal(t, uint64(MinLPCellOccupiedShannons), got)

	// One shannon below the floor is rejected.
	_, err = updatedLPCellCapacity(380*ckb+MinLPCellOccupiedShannons-1, 380*ckb)
	require.ErrorIs(t, err, ErrInsufficientLPCellCapacity)

	// Taking more than the cell holds is rejected, deterministic.
	_, err = updatedLPCellCapacity(100*ckb, 380*ckb)
	require.ErrorIs(t, err, ErrInvalidLPCellArg)
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
