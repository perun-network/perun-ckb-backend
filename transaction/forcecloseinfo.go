package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/go-perun/channel"
)

type ForceCloseInfo struct {
	ChannelInput    types.CellInput
	AssetInputs     []types.CellInput
	Headers         []types.Hash
	State           *channel.State
	Params          *channel.Params
	ChannelCapacity uint64
}
