package backend

import "github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"

type Token struct {
	Outpoint molecule.OutPoint
	Token    molecule.ChannelToken
}

func (t Token) AsCellInput() molecule.CellInput {
	return molecule.NewCellInputBuilder().PreviousOutput(t.Outpoint).Build()
}
