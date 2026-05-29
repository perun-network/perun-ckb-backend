package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
)

// TODO: Add Headers of consumed cells
type FundInfo struct {
	ChannelCell types.OutPoint
	Params      *channel.Params
	State       *channel.State
	PCTS        *types.Script
	Status      molecule.ChannelStatus
	Header      types.Hash

	// InputChannelCapacity is the actual capacity of the channel cell being consumed. The
	// rebuilt channel cell preserves it so the (party-0-funded) reserve is never refunded to
	// the funding party (see encoding.LockedSubAllocReserve).
	InputChannelCapacity uint64
}

func NewFundInfo(channelCell types.OutPoint, params *channel.Params, state *channel.State, pcts *types.Script, status molecule.ChannelStatus, header types.Hash) *FundInfo {
	return &FundInfo{
		ChannelCell: channelCell,
		Params:      params,
		State:       state,
		PCTS:        pcts,
		Status:      status,
		Header:      header,
	}
}
