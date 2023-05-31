package backend

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
)

// CKBOutput groups a CKB output cell and its data.
type CKBOutput struct {
	Output molecule.CellOutput
	Data   molecule.Bytes
}

func (o CKBOutput) AsOutputAndData() (types.CellOutput, []byte) {
	var optType *types.Script = nil
	if o.Output.Type().IsSome() {
		optType = types.UnpackScriptOpt(o.Output.Type())
	}
	op := types.CellOutput{
		Capacity: molecule2.UnpackUint64(o.Output.Capacity()),
		Lock:     types.UnpackScript(o.Output.Lock()),
		Type:     optType,
	}
	return op, o.Data.AsSlice()
}

type CKBOutputs []CKBOutput

func MkCKBOutputs(outputs ...CKBOutput) CKBOutputs {
	return outputs
}

// AsSerializedOutputAndData encodes the CKBOutputs into molecule vectors to be
// used in transactions.
func (os CKBOutputs) AsSerializedOutputAndData() (molecule.CellOutputVec, molecule.BytesVec) {
	datas := molecule.NewBytesVecBuilder()
	outputs := molecule.NewCellOutputVecBuilder()
	for _, o := range os {
		outputs.Push(o.Output)
		datas.Push(o.Data)
	}
	return outputs.Build(), datas.Build()
}

func (os CKBOutputs) Append(o CKBOutput) CKBOutputs {
	return append(os, o)
}

func (os CKBOutputs) Extend(nos CKBOutputs) CKBOutputs {
	return append(os, nos...)
}
