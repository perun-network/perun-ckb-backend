package ckblp

import (
	ckbbuilder "github.com/nervosnetwork/ckb-sdk-go/v2/collector/builder"
	ckbtransaction "github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

func addInputLockScriptGroup(builder *ckbbuilder.SimpleTransactionBuilder, lockScript *types.Script, inputIndices ...uint32) {
	if lockScript == nil || len(inputIndices) == 0 {
		return
	}
	group := &ckbtransaction.ScriptGroup{
		Script:       lockScript,
		GroupType:    types.ScriptTypeLock,
		InputIndices: inputIndices,
	}
	builder.AddScriptGroup(group)
}

func initLockWitnessPlaceholder(builder *ckbbuilder.SimpleTransactionBuilder, index uint) {
	_ = builder.SetWitness(index, types.WitnessTypeLock, make([]byte, 65))
}
