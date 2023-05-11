package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/go-perun/channel"
)

type AbortInfo struct {
	ChannelInput    types.CellInput
	AssetInputs     []types.CellInput
	FundingStatus   [2]uint64
	Params          *channel.Params
	ChannelCapacity uint64
}
