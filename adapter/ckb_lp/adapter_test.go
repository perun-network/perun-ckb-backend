package ckblp

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"github.com/stretchr/testify/require"
	"perun.network/perun-ckb-backend/backend"
	clienttest "perun.network/perun-ckb-backend/client/test"
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

func TestBuildLPDepositTxUnsignedGathersMultipleFundingCells(t *testing.T) {
	const ckb = uint64(100_000_000)
	ownerScript := &types.Script{
		CodeHash: types.BytesToHash([]byte{0x01}),
		HashType: types.HashTypeType,
		Args:     make([]byte, 20),
	}
	// Fragmented owner account: no single cell covers the 4000 CKB deposit,
	// but the two largest together do.
	cells := []*indexer.LiveCell{
		fundingCell(0xaa, 3213*ckb, ownerScript),
		fundingCell(0xbb, 1288*ckb, ownerScript),
		fundingCell(0xcc, 300*ckb, ownerScript),
	}
	adapter := &Adapter{
		rpcClient:    mockRPCWithCells(cells),
		deployment: backend.Deployment{
			DefaultLockScript:    types.Script{CodeHash: ownerScript.CodeHash},
			DefaultLockScriptDep: types.CellDep{OutPoint: &types.OutPoint{TxHash: types.Hash{0x05}, Index: 0}, DepType: types.DepTypeCode},
		},
		lpDeployment: testLPDeployment(),
	}

	deposit := 4000 * ckb
	tx, expectedOutpoint, err := adapter.BuildLPDepositTxUnsigned(
		context.Background(),
		LPCell{AvailableCKB: deposit},
		ownerScript,
		types.Hash{0x02},
	)

	require.NoError(t, err)
	require.Len(t, tx.TxView.Inputs, 2)
	require.Len(t, tx.TxView.Outputs, 2)
	require.Equal(t, deposit, tx.TxView.Outputs[0].Capacity)
	inTotal := (3213 + 1288) * ckb
	require.Equal(t, inTotal-deposit-transaction.DefaultFeeShannon, tx.TxView.Outputs[1].Capacity)
	require.Len(t, tx.TxView.Witnesses, 2)
	require.Len(t, tx.ScriptGroups, 1)
	require.Equal(t, []uint32{0, 1}, tx.ScriptGroups[0].InputIndices)
	require.NotEmpty(t, expectedOutpoint)
}

func TestBuildLPDepositTxUnsignedRejectsWhenTotalFundsInsufficient(t *testing.T) {
	const ckb = uint64(100_000_000)
	ownerScript := &types.Script{
		CodeHash: types.BytesToHash([]byte{0x01}),
		HashType: types.HashTypeType,
		Args:     make([]byte, 20),
	}
	cells := []*indexer.LiveCell{
		fundingCell(0xaa, 300*ckb, ownerScript),
		fundingCell(0xbb, 200*ckb, ownerScript),
	}
	adapter := &Adapter{
		rpcClient:    mockRPCWithCells(cells),
		deployment: backend.Deployment{
			DefaultLockScript:    types.Script{CodeHash: ownerScript.CodeHash},
			DefaultLockScriptDep: types.CellDep{OutPoint: &types.OutPoint{TxHash: types.Hash{0x05}, Index: 0}, DepType: types.DepTypeCode},
		},
		lpDeployment: testLPDeployment(),
	}

	_, _, err := adapter.BuildLPDepositTxUnsigned(
		context.Background(),
		LPCell{AvailableCKB: 4000 * ckb},
		ownerScript,
		types.Hash{0x02},
	)

	require.ErrorIs(t, err, ErrInsufficientOperatorFunds)
	require.True(t, IsDeterministic(err))
}

func testLPDeployment() LPDeployment {
	return LPDeployment{
		TypeScriptDep:      types.CellDep{OutPoint: &types.OutPoint{TxHash: types.Hash{0x06}, Index: 0}, DepType: types.DepTypeCode},
		LockScriptDep:      types.CellDep{OutPoint: &types.OutPoint{TxHash: types.Hash{0x07}, Index: 0}, DepType: types.DepTypeCode},
		TypeScriptCodeHash: types.Hash{0x03},
		TypeScriptHashType: types.HashTypeType,
		LockScriptCodeHash: types.Hash{0x04},
		LockScriptHashType: types.HashTypeType,
	}
}

func fundingCell(txHashByte byte, capacity uint64, lock *types.Script) *indexer.LiveCell {
	return &indexer.LiveCell{
		OutPoint: &types.OutPoint{TxHash: types.Hash{txHashByte}, Index: 0},
		Output:   &types.CellOutput{Capacity: capacity, Lock: lock},
	}
}

func mockRPCWithCells(cells []*indexer.LiveCell) *clienttest.MockRPCClient {
	mock := clienttest.NewMockRPCClient()
	mock.SetGetCells(func(_ context.Context, _ *indexer.SearchKey, _ indexer.SearchOrder, _ uint64, _ string) (*indexer.LiveCells, error) {
		return &indexer.LiveCells{Objects: cells}, nil
	})
	return mock
}

func bigOne() *big.Int {
	return big.NewInt(1)
}
