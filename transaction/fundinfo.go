package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/backend"
)

type FundInfo struct {
	Amount      uint64
	ChannelCell types.OutPoint
	Params      *channel.Params
	Token       backend.Token
	Status      molecule.ChannelStatus
	Header      types.Hash
}

func (fi FundInfo) AddFundingToStatus() molecule.ChannelStatus {
	oldBalances := fi.Status.Funding()
	newBalances := oldBalances.AsBuilder()
	newBalances.Nth1(*types.PackUint64(fi.Amount))
	updatedStatus := fi.Status.AsBuilder()
	return updatedStatus.Funding(newBalances.Build()).Build()
}

func (fi FundInfo) MkFundsCell(pfls *types.Script) *types.CellOutput {
	cellOutput := types.CellOutput{
		Capacity: fi.Amount,
		Lock:     pfls,
		Type:     nil,
	}
	cellOutput.Capacity = cellOutput.OccupiedCapacity([]byte{})
	return &cellOutput
}
