package transaction_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	ckbtransaction "github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/numeric"
	"github.com/stretchr/testify/require"
	"perun.network/go-perun/channel/test"
	btest "perun.network/perun-ckb-backend/backend/test"
	"perun.network/perun-ckb-backend/transaction"
	txtest "perun.network/perun-ckb-backend/transaction/test"
	wtest "perun.network/perun-ckb-backend/wallet/test"
	ptest "polycry.pt/poly-go/test"
)

var zeroHash types.Hash = types.Hash{}

func TestScriptHandler(t *testing.T) {
	rng := ptest.Prng(t)
	sender := wtest.NewRandomParticipant(rng)
	senderCkbAddr := sender.ToCKBAddress(types.NetworkTest)
	defaultLock := btest.NewRandomScript(rng)
	defaultLockDep := btest.NewRandomCellDep(rng)
	pctsDep := btest.NewRandomCellDep(rng)
	pclsDep := btest.NewRandomCellDep(rng)
	pflsDep := btest.NewRandomCellDep(rng)
	deployment := btest.NewRandomDeployment(rng,
		btest.WithPCTS(types.Hash{}, *pctsDep, types.HashTypeData),
		btest.WithPCLS(types.Hash{}, *pclsDep, types.HashTypeData),
		btest.WithPFLS(types.Hash{}, *pflsDep, types.HashTypeData),
		btest.WithDefaultLockScript(*defaultLock, *defaultLockDep),
	)
	psh := transaction.NewPerunScriptHandlerWithDeployment(*deployment)

	mockHandler := txtest.NewMockHandler(defaultLock)
	funding := uint64(numeric.NewCapacityFromCKBytes(420_690))
	changeAmount := uint64(numeric.NewCapacityFromCKBytes(100_000))
	mkMockIterator := func() *txtest.MockIterator {
		mockIterator := txtest.NewMockIterator(
			txtest.WithLockScript(defaultLock),
			// required because the iterator is only concerned with cells that can be
			// used to satisfy CKB invariants about CKBytes in cells.
			txtest.WithTypeScript(nil),
			txtest.WithCapacityAtLeast(funding+changeAmount),
		)
		mockIterator.GenerateInput(rng)
		mockIterator.GenerateInput(rng)
		mockIterator.GenerateInput(rng)
		return mockIterator
	}

	t.Run("Balancing", func(t *testing.T) {
		mockIterator := mkMockIterator()
		iters := map[types.Hash]collector.CellIterator{
			zeroHash: mockIterator,
		}
		channelTokenOutpoint := btest.NewRandomOutpoint(rng)
		mockIterator.GenerateInput(rng, txtest.WithOutPoint(channelTokenOutpoint))
		liveCellMap := liveCellMapFromIterator(mockIterator)
		client := txtest.NewMockClient(txtest.WithMockLiveCells(liveCellMap))
		b, err := transaction.NewPerunTransactionBuilder(client, iters, make(map[types.Hash]types.Script), psh, senderCkbAddr)
		require.NoError(t, err, "creating perun transaction builder")
		b.Register(mockHandler)
		// Open
		state := test.NewRandomState(rng,
			test.WithNumParts(2),
			test.WithNumAssets(1),
			test.WithNumLocked(0),
			test.WithBalancesInRange(big.NewInt(0).Mul(big.NewInt(100), big.NewInt(100_000_000)), big.NewInt(0).Mul(big.NewInt(10_000), big.NewInt(100_000_000))),
		)
		params := test.NewRandomParams(rng,
			test.WithNumParts(2),
			test.WithLedgerChannel(true),
			test.WithVirtualChannel(false),
			test.WithoutApp())
		oi := transaction.NewOpenInfo([32]byte{}, btest.NewRandomToken(rng,
			btest.WithOutpoint(*channelTokenOutpoint)),
			params,
			state)
		require.NoError(t, b.Open(oi), "executing opening transaction handler")

		tx, err := b.Build()
		require.NoError(t, err)

		require.NoError(t, checkTransactionBalance(tx, mockIterator, transaction.DefaultFeeShannon), "transaction should be properly balanced")
	})

}

func checkTransactionBalance(
	tx *ckbtransaction.TransactionWithScriptGroups,
	mockIterator *txtest.MockIterator,
	expectedFeeShannon uint64) error {
	inputs := mockIterator.GetInputs()
	findInInputs := func(cellInput *types.CellInput) *types.TransactionInput {
		for _, input := range inputs {
			if *input.OutPoint == *cellInput.PreviousOutput {
				return input
			}
		}
		return nil
	}

	// Iterate over all inputs and fetch their corresponding amounts from the
	// mockiterator.
	inputCKBAmount := uint64(0)
	for _, input := range tx.TxView.Inputs {
		txInput := findInInputs(input)
		if txInput == nil {
			return fmt.Errorf("input %v not found in mock iterator", input)
		}
		inputCKBAmount += txInput.Output.Capacity
	}

	outputCKBAmount := uint64(0)
	for _, output := range tx.TxView.Outputs {
		outputCKBAmount += output.Capacity
	}

	if inputCKBAmount != (outputCKBAmount + expectedFeeShannon) {
		return fmt.Errorf("input and output amounts do not match: %d != %d", inputCKBAmount, outputCKBAmount)
	}

	return nil
}

type inputSource interface {
	GetInputs() []*types.TransactionInput
}

func liveCellMapFromIterator(is inputSource) map[string]*types.CellWithStatus {
	m := make(map[string]*types.CellWithStatus)
	for _, ti := range is.GetInputs() {
		key := txtest.MakeKeyFromOutpoint(ti.OutPoint)
		m[key] = &types.CellWithStatus{
			Cell: &types.CellInfo{
				Data: &types.CellData{
					Content: ti.OutputData,
					Hash:    zeroHash, // TODO: use real hash
				},
				Output: ti.Output,
			},
			Status: "live",
		}
	}
	return m
}
