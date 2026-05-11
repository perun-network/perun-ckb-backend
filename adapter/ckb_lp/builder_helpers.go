package ckblp

import (
	ckbbuilder "github.com/nervosnetwork/ckb-sdk-go/v2/collector/builder"
	ckbtransaction "github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

func addInputLockScriptGroup(builder *ckbbuilder.SimpleTransactionBuilder, inputIndex uint32, lockScript *types.Script) {
	if lockScript == nil {
		return
	}
	group := &ckbtransaction.ScriptGroup{
		Script:       lockScript,
		GroupType:    types.ScriptTypeLock,
		InputIndices: []uint32{inputIndex},
	}
	builder.AddScriptGroup(group)
}

func initLockWitnessPlaceholder(builder *ckbbuilder.SimpleTransactionBuilder, index uint) {
	_ = builder.SetWitness(index, types.WitnessTypeLock, make([]byte, 65))
}
