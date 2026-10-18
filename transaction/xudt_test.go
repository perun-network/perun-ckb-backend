package transaction_test

import (
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/nervosnetwork/ckb-sdk-go/v2/address"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/crypto/blake2b"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/transaction"
	txtest "perun.network/perun-ckb-backend/transaction/test"
	"perun.network/perun-ckb-backend/xudt"
	ptest "polycry.pt/poly-go/test"
)

func TestXUDTBuild(t *testing.T) {
	for _, mode := range []uint32{0, 1, 2} {
		t.Run(string(rune('0'+mode)), func(t *testing.T) {
			rng := ptest.Prng(t)
			lock := &types.Script{HashType: types.HashTypeData, Args: make([]byte, 20)}
			cfg := xudt.Config{Script: types.Script{CodeHash: types.HexToHash("0x42"), HashType: types.HashTypeData1, Args: make([]byte, 36)}, CellDeps: []types.CellDep{{OutPoint: &types.OutPoint{TxHash: types.HexToHash("0x43")}, DepType: types.DepTypeCode}}}
			binary.LittleEndian.PutUint32(cfg.Script.Args[32:], mode)
			metadata := []byte{}
			if mode > 0 {
				cfg.ExtensionScripts = []types.Script{{HashType: types.HashTypeData1, Args: []byte{3}}}
				encoded := xudt.EncodeScriptVec(cfg.ExtensionScripts)
				if mode == 2 {
					encoded = blake2b.Blake160(encoded)
				}
				cfg.Script.Args = append(cfg.Script.Args, encoded...)
				metadata = xudt.EncodeData(nil, [][]byte{[]byte("metadata")})
				cfg.OutputData = func(output types.CellOutput, amount *big.Int) ([]byte, error) { return metadata, nil }
				cfg.WitnessData = func(tx *types.Transaction, in, out []uint32) ([][]byte, error) {
					require.Len(t, in, 1)
					require.Len(t, out, 2)
					require.GreaterOrEqual(t, len(tx.Outputs), 2)
					return [][]byte{[]byte("proof")}, nil
				}
			}
			hash := cfg.Script.Hash()
			cap := uint64(1000 * transaction.CKBYTE)
			tokens := txtest.NewMockIterator(txtest.WithLockScript(lock), txtest.WithTypeScript(&cfg.Script), txtest.WithCapacityAtLeast(cap))
			tokens.GenerateInput(rng, txtest.WithData(append(tokenData(t, big.NewInt(100)), metadata...)))
			ckb := txtest.NewMockIterator(txtest.WithLockScript(lock), txtest.WithTypeScript(nil), txtest.WithCapacityAtLeast(cap))
			ckb.GenerateInput(rng)
			cl := txtest.NewMockClient(txtest.WithMockLiveCells(liveCellMapFromIterators(tokens, ckb)))
			d := backend.Deployment{DefaultLockScript: *lock, DefaultLockScriptDep: types.CellDep{OutPoint: &types.OutPoint{}, DepType: types.DepTypeCode}, XUDTs: map[types.Hash]xudt.Config{hash: cfg}}
			b, err := transaction.NewPerunTransactionBuilderWithDeployment(cl, d, map[types.Hash]collector.CellIterator{hash: tokens, zeroHash: ckb}, address.Address{Script: lock}, false)
			require.NoError(t, err)
			b.ScriptHandlers = nil
			b.AddOutput(&types.CellOutput{Lock: lock, Type: &cfg.Script, Capacity: cap}, tokenData(t, big.NewInt(40)))
			tx, err := b.Build()
			require.NoError(t, err)
			require.Contains(t, tx.TxView.CellDeps, &cfg.CellDeps[0])
			require.NoError(t, checkSudtTransactionBalance(tx, ckb, tokens, cfg.Script, transaction.DefaultFeeShannon))
			for i, out := range tx.TxView.Outputs {
				require.GreaterOrEqual(t, out.Capacity, out.OccupiedCapacity(tx.TxView.OutputsData[i]))
				if out.Type != nil {
					require.Equal(t, metadata, tx.TxView.OutputsData[i][16:])
				}
			}
			if mode > 0 {
				want, err := cfg.EncodeWitness([][]byte{[]byte("proof")})
				require.NoError(t, err)
				w, err := types.DeserializeWitnessArgs(tx.TxView.Witnesses[0])
				require.NoError(t, err)
				require.Equal(t, want, w.InputType)
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
