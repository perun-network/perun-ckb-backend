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
	// The SDK deserializes ScriptHashTypes in a wrong way.
	// types/molecule.go:81:UnpackScript():
	//	s.HashType = ScriptHashType(v.HashType().AsSlice())
	//
	// ^ This interprets the encoded HashType as a byte array, which is correct.
	// However the result is then interpreted as a `ScriptHashType` which in turn
	// is a `string` resulting in a wrong value. "0x00|0x01|0x02" instead of
	// "data|type|data1".
	//
	// Building a transaction using this output thus fails.
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
