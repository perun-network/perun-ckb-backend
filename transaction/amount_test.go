package transaction_test

import (
	"math/big"
	"testing"

	"github.com/nervosnetwork/ckb-sdk-go/v2/address"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
	"perun.network/perun-ckb-backend/backend"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
	"perun.network/perun-ckb-backend/transaction"
	txtest "perun.network/perun-ckb-backend/transaction/test"
	ptest "polycry.pt/poly-go/test"
)

func tokenData(t *testing.T, amount *big.Int) []byte {
	t.Helper()
	data, err := molecule2.PackUint128(amount)
	require.NoError(t, err)
	return data.AsSlice()
}

func TestTokenBalancingFullPrecision(t *testing.T) {
	high := new(big.Int).Lsh(big.NewInt(1), 96)
	for _, scenario := range []string{"partial", "exact", "input-only", "insufficient", "duplicate-iterator"} {
		t.Run(scenario, func(t *testing.T) {
			rng := ptest.Prng(t)
			lock := &types.Script{CodeHash: types.HexToHash("0x01"), HashType: types.HashTypeData, Args: make([]byte, 20)}
			token := &types.Script{CodeHash: types.HexToHash("0x02"), HashType: types.HashTypeData1, Args: make([]byte, 32)}
			hash := token.Hash()
			cap := uint64(1000 * transaction.CKBYTE)
			ckb := txtest.NewMockIterator(txtest.WithLockScript(lock), txtest.WithTypeScript(nil), txtest.WithCapacityAtLeast(cap))
			ckb.GenerateInput(rng)
			tokens := txtest.NewMockIterator(txtest.WithLockScript(lock), txtest.WithTypeScript(token), txtest.WithCapacityAtLeast(cap))
			tokens.GenerateInput(rng, txtest.WithData(tokenData(t, high)))
			tokens.GenerateInput(rng, txtest.WithData(tokenData(t, high)))
			cl := txtest.NewMockClient(txtest.WithMockLiveCells(liveCellMapFromIterators(ckb, tokens)))
			d := backend.Deployment{DefaultLockScript: *lock, DefaultLockScriptDep: types.CellDep{OutPoint: &types.OutPoint{}, DepType: types.DepTypeCode}, SUDTs: map[types.Hash]types.Script{hash: *token}}
			b, err := transaction.NewPerunTransactionBuilderWithDeployment(cl, d, map[types.Hash]collector.CellIterator{zeroHash: ckb, hash: tokens}, address.Address{Script: lock}, false)
			require.NoError(t, err)
			b.ScriptHandlers = nil // These tests verify balancing, not lock signatures.
			first := tokens.GetInputs()[0]
			b.AddInput(&types.CellInput{PreviousOutput: first.OutPoint})
			want := new(big.Int).Add(high, big.NewInt(7))
			switch scenario {
			case "exact":
				want.Set(high)
			case "input-only":
				want.SetInt64(0)
			case "insufficient":
				want.Mul(high, big.NewInt(3))
			case "duplicate-iterator": // Same input must not be counted twice.
				want.Add(high, big.NewInt(1))
			}
			if scenario != "input-only" {
				b.AddOutput(&types.CellOutput{Lock: lock, Type: token, Capacity: cap}, tokenData(t, want))
			}
			tx, err := b.Build()
			if scenario == "insufficient" {
				require.ErrorContains(t, err, "insufficient funds")
				return
			}
			require.NoError(t, err)
			require.NoError(t, checkSudtTransactionBalance(tx, ckb, tokens, *token, transaction.DefaultFeeShannon))
			total := new(big.Int)
			seen := map[types.OutPoint]bool{}
			for _, input := range tx.TxView.Inputs {
				require.False(t, seen[*input.PreviousOutput])
				seen[*input.PreviousOutput] = true
				for _, cell := range tokens.GetInputs() {
					if *cell.OutPoint == *input.PreviousOutput {
						total.Add(total, high)
					}
				}
			}
			outputTotal := new(big.Int)
			for i, o := range tx.TxView.Outputs {
				if o.Type != nil && o.Type.Hash() == hash {
					data := tx.TxView.OutputsData[i]
					require.Len(t, data, 16)
					be := make([]byte, 16)
					for j := range be {
						be[15-j] = data[j]
					}
					outputTotal.Add(outputTotal, new(big.Int).SetBytes(be))
				}
			}
			require.Equal(t, 0, total.Cmp(outputTotal), "all 128 bits must be conserved")
		})
	}
}

func TestAssetInformationRejectsOverflowAndMalformedData(t *testing.T) {
	token := types.Script{HashType: types.HashTypeData1, Args: make([]byte, 32)}
	hash := token.Hash()
	ai := transaction.NewAssetInformation(map[types.Hash]types.Script{hash: token})
	out := &types.CellOutput{Type: &token, Capacity: 1}
	for n := 0; n < 16; n++ {
		require.Error(t, ai.AddValuesFromOutput(out, make([]byte, n)))
	}
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	require.NoError(t, ai.AddValuesFromOutput(out, tokenData(t, max)))
	require.Error(t, ai.AddValuesFromOutput(out, tokenData(t, big.NewInt(1))))
	require.Equal(t, 0, max.Cmp(ai.AssetAmount(hash)))
	require.Equal(t, uint64(1), ai.CKBAmount(), "failed additions are atomic")
	ai.AssetAmount(hash).SetInt64(0)
	require.Equal(t, 0, max.Cmp(ai.AssetAmount(hash)))
	clone := ai.Clone()
	require.NoError(t, clone.AddAssetAmount(zeroHash, 1))
	require.Equal(t, uint64(1), ai.CKBAmount())
	require.Error(t, ai.AddAssetAmount(zeroHash, ^uint64(0)))
}

func TestCKBCapacityBoundaries(t *testing.T) {
	for _, scenario := range []string{"exact", "dust", "insufficient", "overflow", "duplicate", "custom-fee"} {
		t.Run(scenario, func(t *testing.T) {
			rng := ptest.Prng(t)
			lock := &types.Script{HashType: types.HashTypeData, Args: make([]byte, 20)}
			outCapacity := uint64(100 * transaction.CKBYTE)
			fee := transaction.DefaultFeeShannon
			if scenario == "custom-fee" {
				fee = 100 * transaction.CKBYTE
			}
			inputCapacity := outCapacity + fee
			if scenario == "custom-fee" {
				inputCapacity += 61 * transaction.CKBYTE
			}
			switch scenario {
			case "dust":
				inputCapacity++
			case "insufficient":
				inputCapacity--
			case "overflow":
				inputCapacity = ^uint64(0)
			}
			iter := txtest.NewMockIterator(txtest.WithLockScript(lock), txtest.WithTypeScript(nil), txtest.WithCapacityAtLeast(inputCapacity))
			iter.GenerateInput(rng)
			if scenario == "dust" {
				iter.GenerateInput(rng)
			}
			cl := txtest.NewMockClient(txtest.WithMockLiveCells(liveCellMapFromIterators(iter)))
			d := backend.Deployment{DefaultLockScript: *lock, DefaultLockScriptDep: types.CellDep{OutPoint: &types.OutPoint{}, DepType: types.DepTypeCode}}
			if scenario == "custom-fee" {
				d.FeeShannon = fee
			}
			b, err := transaction.NewPerunTransactionBuilderWithDeployment(cl, d, map[types.Hash]collector.CellIterator{zeroHash: iter}, address.Address{Script: lock}, false)
			require.NoError(t, err)
			b.ScriptHandlers = nil
			if scenario == "duplicate" {
				cell := iter.GetInputs()[0]
				b.AddInput(&types.CellInput{PreviousOutput: cell.OutPoint})
				b.AddInput(&types.CellInput{PreviousOutput: cell.OutPoint})
			}
			if scenario == "overflow" {
				outCapacity = ^uint64(0)
			}
			b.AddOutput(&types.CellOutput{Lock: lock, Capacity: outCapacity}, nil)
			tx, err := b.Build()
			if scenario == "insufficient" || scenario == "overflow" || scenario == "duplicate" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NoError(t, checkTransactionBalance(tx, iter, fee))
			if scenario == "exact" {
				require.Len(t, tx.TxView.Outputs, 1)
			}
			if scenario == "dust" {
				require.Len(t, tx.TxView.Inputs, 2)
				require.Len(t, tx.TxView.Outputs, 2)
			}
		})
	}
}

func TestOmniLockBuilderDependencies(t *testing.T) {
	lock := types.Script{HashType: types.HashTypeType}
	deps := []types.CellDep{
		{OutPoint: &types.OutPoint{Index: 0}, DepType: types.DepTypeCode},
		{OutPoint: &types.OutPoint{Index: 1}, DepType: types.DepTypeDepGroup},
	}
	for n := 0; n <= len(deps); n++ {
		d := backend.Deployment{OmniLockScript: lock, OmniLockScriptDep: deps[:n]}
		b, err := transaction.NewPerunTransactionBuilderWithDeployment(nil, d, nil, address.Address{Script: &lock}, true)
		if n < 2 {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Len(t, b.CellDeps, 2)
		for i := range deps {
			require.Contains(t, b.CellDeps, &deps[i])
		}
	}
}
