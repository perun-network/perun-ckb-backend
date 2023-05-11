package backend

import (
	"encoding/binary"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
)

// CKBOutput groups a CKB output cell and its data.
type CKBOutput struct {
	Output molecule.CellOutput
	Data   molecule.Bytes
}

func (o CKBOutput) AsOutputAndData() (types.CellOutput, []byte) {
	return types.CellOutput{
		Capacity: UnpackUint64(*o.Output.Capacity()),
		Lock:     types.UnpackScript(o.Output.Lock()),
		Type:     types.UnpackScriptOpt(o.Output.Type()),
	}, o.Data.AsSlice()
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

func UnpackUint64(u molecule.Uint64) uint64 {
	return binary.LittleEndian.Uint64(u.AsSlice())
}
