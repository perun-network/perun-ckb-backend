package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/go-perun/channel"
)

type AbortInfo struct {
	ChannelInput    types.CellInput
	AssetInputs     []types.CellInput
	InitialState    *channel.State // State with version number 0!
	Params          *channel.Params
	Headers         []types.Hash
	ChannelCapacity uint64
}

func NewAbortInfo(channelInput types.CellInput, assetInputs []types.CellInput, initialState *channel.State, params *channel.Params, headers []types.Hash, channelCapacity uint64) *AbortInfo {
	return &AbortInfo{
		ChannelInput:    channelInput,
		AssetInputs:     assetInputs,
		InitialState:    initialState,
		Params:          params,
		Headers:         headers,
		ChannelCapacity: channelCapacity,
	}
}
