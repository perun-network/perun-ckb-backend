package backend

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
)

type Token struct {
	Idx      uint32
	Outpoint molecule.OutPoint
	Token    molecule.ChannelToken
}

func (t Token) AsSerializedCellInput() molecule.CellInput {
	return molecule.NewCellInputBuilder().PreviousOutput(t.Outpoint).Build()
}

func (t Token) AsCellInput() types.CellInput {
	return types.CellInput{
		Since: 0,
		PreviousOutput: &types.OutPoint{
			TxHash: types.BytesToHash(t.Outpoint.TxHash().AsSlice()),
			Index:  t.Idx,
		},
	}
}
