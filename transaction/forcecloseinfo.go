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

func NewForceCloseInfo(channelInput types.CellInput, assetInputs []types.CellInput, headers []types.Hash, state *channel.State, params *channel.Params, channelCapacity uint64) *ForceCloseInfo {
	return &ForceCloseInfo{
		ChannelInput:    channelInput,
		AssetInputs:     assetInputs,
		Headers:         headers,
		State:           state,
		Params:          params,
		ChannelCapacity: channelCapacity,
	}
}
