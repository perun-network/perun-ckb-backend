package backend

import "github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"

// CKBOutput groups a CKB output cell and its data.
type CKBOutput struct {
	Output molecule.CellOutput
	Data   molecule.Bytes
}

type CKBOutputs []CKBOutput

func MkCKBOutputs(outputs ...CKBOutput) CKBOutputs {
	return outputs
}

// AsOutputAndData encodes the CKBOutputs into molecule vectors to be used in
// transactions.
func (os CKBOutputs) AsOutputAndData() (molecule.CellOutputVec, molecule.BytesVec) {
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
