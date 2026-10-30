package transaction

import (
	"encoding/binary"
	"testing"

	"github.com/nervosnetwork/ckb-sdk-go/v2/collector/builder"
	ckbtx "github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
	"perun.network/perun-ckb-backend/xudt"
)

func TestXUDTWitnessPlacementAndPreservation(t *testing.T) {
	for _, outputOnly := range []bool{false, true} {
		cfg := xudt.Config{Script: types.Script{HashType: types.HashTypeData1, Args: make([]byte, 36)}, CellDeps: []types.CellDep{{OutPoint: &types.OutPoint{}, DepType: types.DepTypeCode}}}
		binary.LittleEndian.PutUint32(cfg.Script.Args[32:], 1)
		cfg.Script.Args = append(cfg.Script.Args, xudt.EncodeScriptVec(nil)...)
		hash := cfg.Script.Hash()
		group := &ckbtx.ScriptGroup{Script: &cfg.Script, GroupType: types.ScriptTypeType, InputIndices: []uint32{1}, OutputIndices: []uint32{2}}
		index := 1
		args := types.WitnessArgs{Lock: []byte("lock signature"), OutputType: []byte("other protocol")}
		if outputOnly {
			group.InputIndices = nil
			index = 2
			args.InputType = args.OutputType
			args.OutputType = nil
		}
		b := &PerunTransactionBuilder{SimpleTransactionBuilder: &builder.SimpleTransactionBuilder{}, xudts: map[types.Hash]xudt.Config{hash: cfg}, scriptGroups: []*ckbtx.ScriptGroup{group}}
		b.Witnesses = make([][]byte, index+1)
		b.Witnesses[index] = args.Serialize()
		require.NoError(t, b.prepareXUDTWitnesses())
		got, err := types.DeserializeWitnessArgs(b.Witnesses[index])
		require.NoError(t, err)
		require.Equal(t, args.Lock, got.Lock)
		want, err := cfg.EncodeWitness(nil)
		require.NoError(t, err)
		if outputOnly {
			require.Equal(t, args.InputType, got.InputType)
			require.Equal(t, want, got.OutputType)
		} else {
			require.Equal(t, args.OutputType, got.OutputType)
			require.Equal(t, want, got.InputType)
		}
		// Repeated application is harmless; a conflicting field is an error.
		require.NoError(t, b.prepareXUDTWitnesses())
		if outputOnly {
			got.OutputType = []byte("collision")
		} else {
			got.InputType = []byte("collision")
		}
		b.Witnesses[index] = got.Serialize()
		require.ErrorContains(t, b.prepareXUDTWitnesses(), "already occupied")
	}
}

func TestXUDTProofSeesAllTokenDependencies(t *testing.T) {
	b := &PerunTransactionBuilder{SimpleTransactionBuilder: &builder.SimpleTransactionBuilder{}, xudts: make(map[types.Hash]xudt.Config)}
	calls := 0
	for i := byte(0); i < 2; i++ {
		args := make([]byte, 36)
		args[0] = i
		binary.LittleEndian.PutUint32(args[32:], 1)
		cfg := xudt.Config{
			Script:   types.Script{HashType: types.HashTypeData1, Args: append(args, xudt.EncodeScriptVec(nil)...)},
			CellDeps: []types.CellDep{{OutPoint: &types.OutPoint{Index: uint32(i)}, DepType: types.DepTypeCode}},
			WitnessData: func(tx *types.Transaction, inputs, outputs []uint32) ([][]byte, error) {
				calls++
				require.Len(t, tx.CellDeps, 2, "proof must see dependencies of every token group")
				return nil, nil
			},
		}
		b.xudts[cfg.Script.Hash()] = cfg
		b.scriptGroups = append(b.scriptGroups, &ckbtx.ScriptGroup{Script: &cfg.Script, GroupType: types.ScriptTypeType, InputIndices: []uint32{uint32(i)}})
	}
	require.NoError(t, b.prepareXUDTWitnesses())
	require.Equal(t, 2, calls)
}
