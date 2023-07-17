package transaction_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	ckbtransaction "github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/numeric"
	"github.com/stretchr/testify/require"
	"perun.network/go-perun/channel/test"
	btest "perun.network/perun-ckb-backend/backend/test"
	"perun.network/perun-ckb-backend/channel/asset"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
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
	mkMockIterator := func(opts ...txtest.TransactionInputOpt) *txtest.MockIterator {
		opts = append(opts, txtest.WithLockScript(defaultLock))
		mockIterator := txtest.NewMockIterator(opts...)
		mockIterator.GenerateInput(rng)
		mockIterator.GenerateInput(rng)
		mockIterator.GenerateInput(rng)
		return mockIterator
	}

	t.Run("Balancing CKBytes", func(t *testing.T) {
		mockIterator := mkMockIterator(txtest.WithTypeScript(nil), txtest.WithCapacityAtLeast(funding+changeAmount))
		iters := map[types.Hash]collector.CellIterator{
			zeroHash: mockIterator,
		}
		channelTokenOutpoint := btest.NewRandomOutpoint(rng)
		mockIterator.GenerateInput(rng, txtest.WithOutPoint(channelTokenOutpoint))
		liveCellMap := liveCellMapFromIterators(mockIterator)
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

	t.Run("Balancing SUDT", func(t *testing.T) {
		sudtTypeScript := btest.NewRandomScript(rng)
		mockIterator := mkMockIterator(txtest.WithTypeScript(nil), txtest.WithCapacityAtLeast(funding+changeAmount))
		sudtMockIterator := mkMockIterator(
			txtest.WithTypeScript(sudtTypeScript),
			txtest.WithDataGenerator(func() []byte {
				d, err := molecule2.PackUint128(big.NewInt(rng.Int63n(math.MaxInt64)))
				if err != nil {
					panic(err)
				}
				return d.AsSlice()
			}),
		)
		iters := map[types.Hash]collector.CellIterator{
			zeroHash:              mockIterator,
			sudtTypeScript.Hash(): sudtMockIterator,
		}
		channelTokenOutpoint := btest.NewRandomOutpoint(rng)
		mockIterator.GenerateInput(rng, txtest.WithOutPoint(channelTokenOutpoint))
		liveCellMap := liveCellMapFromIterators(mockIterator, sudtMockIterator)
		client := txtest.NewMockClient(txtest.WithMockLiveCells(liveCellMap))
		b, err := transaction.NewPerunTransactionBuilder(client, iters, map[types.Hash]types.Script{sudtTypeScript.Hash(): *sudtTypeScript}, psh, senderCkbAddr)
		require.NoError(t, err, "creating perun transaction builder")
		b.Register(mockHandler)
		maxSUDTCellCapacity := transaction.CalculateCellCapacity(types.CellOutput{
			Capacity: 0,
			Lock:     defaultLock,
			Type:     sudtTypeScript,
		})
		// Open
		state := test.NewRandomState(rng,
			test.WithNumParts(2),
			test.WithAssets(asset.CKBAsset, &asset.SUDTAsset{
				TypeScript:  *sudtTypeScript,
				MaxCapacity: maxSUDTCellCapacity,
			}),
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

		require.NoError(t, checkSudtTransactionBalance(tx,
			mockIterator,
			sudtMockIterator,
			*sudtTypeScript,
			transaction.DefaultFeeShannon),
			"transaction should be properly balanced")
	})

	t.Run("BalancingFundingEqualsFundsOfCell", func(t *testing.T) {
		ckbInFundingCells := funding + changeAmount
		sudtTypeScript := btest.NewRandomScript(rng)
		mockIterator := mkMockIterator(txtest.WithTypeScript(nil), txtest.WithCapacityAtLeast(ckbInFundingCells))
		sudtMockIterator := mkMockIterator(
			txtest.WithTypeScript(sudtTypeScript),
			txtest.WithDataGenerator(func() []byte {
				d, err := molecule2.PackUint128(big.NewInt(rng.Int63n(math.MaxInt64)))
				if err != nil {
					panic(err)
				}
				return d.AsSlice()
			}),
		)
		iters := map[types.Hash]collector.CellIterator{
			zeroHash:              mockIterator,
			sudtTypeScript.Hash(): sudtMockIterator,
		}
		channelTokenOutpoint := btest.NewRandomOutpoint(rng)
		mockIterator.GenerateInput(rng, txtest.WithOutPoint(channelTokenOutpoint))
		liveCellMap := liveCellMapFromIterators(mockIterator, sudtMockIterator)
		client := txtest.NewMockClient(txtest.WithMockLiveCells(liveCellMap))
		b, err := transaction.NewPerunTransactionBuilder(client, iters, map[types.Hash]types.Script{sudtTypeScript.Hash(): *sudtTypeScript}, psh, senderCkbAddr)
		require.NoError(t, err, "creating perun transaction builder")
		b.Register(mockHandler)
		maxSUDTCellCapacity := transaction.CalculateCellCapacity(types.CellOutput{
			Capacity: 0,
			Lock:     defaultLock,
			Type:     sudtTypeScript,
		})
		// Open
		state := test.NewRandomState(rng,
			test.WithNumParts(2),
			test.WithAssets(asset.CKBAsset, &asset.SUDTAsset{
				TypeScript:  *sudtTypeScript,
				MaxCapacity: maxSUDTCellCapacity,
			}),
			test.WithNumLocked(0),
			test.WithBalances([]*big.Int{big.NewInt(0).SetUint64(ckbInFundingCells), big.NewInt(100)},
				[]*big.Int{big.NewInt(0).SetUint64(ckbInFundingCells), big.NewInt(100)}),
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

		require.NoError(t, checkSudtTransactionBalance(tx,
			mockIterator,
			sudtMockIterator,
			*sudtTypeScript,
			transaction.DefaultFeeShannon),
			"transaction should be properly balanced")
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

func checkSudtTransactionBalance(
	tx *ckbtransaction.TransactionWithScriptGroups,
	mockIterator *txtest.MockIterator,
	sudtMockIterator *txtest.MockIterator,
	sudtScript types.Script,
	expectedFeeShannon uint64) error {
	ckbInputs := mockIterator.GetInputs()
	sudtInputs := sudtMockIterator.GetInputs()
	findInCKBInputs := func(cellInput *types.CellInput) *types.TransactionInput {
		for _, input := range ckbInputs {
			if *input.OutPoint == *cellInput.PreviousOutput {
				return input
			}
		}
		return nil
	}
	findInSUDTInputs := func(cellInput *types.CellInput) *types.TransactionInput {
		for _, input := range sudtInputs {
			if *input.OutPoint == *cellInput.PreviousOutput {
				return input
			}
		}
		return nil
	}
	requireSUDTAmount := func(sudtData []byte) uint64 {
		amount := molecule2.UnpackUint128(molecule.Uint128FromSliceUnchecked(sudtData))
		return amount.Big().Uint64()
	}

	inputCKBAmount := uint64(0)
	inputSUDTAmount := uint64(0)
	for _, input := range tx.TxView.Inputs {
		txInput := findInCKBInputs(input)
		if txInput != nil {
			inputCKBAmount += txInput.Output.Capacity
		}

		txInput = findInSUDTInputs(input)
		if txInput != nil {
			inputCKBAmount += txInput.Output.Capacity
			if txInput.Output.Type != nil && txInput.Output.Type.Hash() == sudtScript.Hash() {
				inputSUDTAmount += requireSUDTAmount(txInput.OutputData)
			}
		}
	}

	outputCKBAmount := uint64(0)
	outputSUDTAmount := uint64(0)
	for idx, output := range tx.TxView.Outputs {
		outputCKBAmount += output.Capacity

		if output.Type != nil && output.Type.Hash() == sudtScript.Hash() {
			outputSUDTAmount += requireSUDTAmount(tx.TxView.OutputsData[idx])
		}
	}

	if inputCKBAmount != (outputCKBAmount + expectedFeeShannon) {
		return fmt.Errorf("input and output CKB amounts do not match: %d != %d", inputCKBAmount, outputCKBAmount)
	}

	if inputSUDTAmount != outputSUDTAmount {
		return fmt.Errorf("input and output SUDT amounts do not match: %d != %d", inputSUDTAmount, outputSUDTAmount)
	}

	return nil
}

type inputSource interface {
	GetInputs() []*types.TransactionInput
}

func liveCellMapFromIterators(is ...inputSource) map[string]*types.CellWithStatus {
	m := make(map[string]*types.CellWithStatus)
	for _, it := range is {
		for _, ti := range it.GetInputs() {
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
	}
	return m
}
