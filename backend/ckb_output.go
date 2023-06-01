package backend

/*
// CKBOutput groups a CKB output cell and its data.
type CKBOutput struct {
	Output types.CellOutput
	Data   []byte
}

func (o CKBOutput) AsOutputAndData() (types.CellOutput, []byte) {
	return o.Output, o.Data
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
		outputs.Push(*o.Output.Pack())
		datas.Push(*types.PackBytes(o.Data))
	}
	return outputs.Build(), datas.Build()
}

func (os CKBOutputs) Append(o CKBOutput) CKBOutputs {
	return append(os, o)
}

func (os CKBOutputs) Extend(nos CKBOutputs) CKBOutputs {
	return append(os, nos...)
}

*/
