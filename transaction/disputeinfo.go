package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
)

type DisputeInfo struct {
	ChannelCell types.OutPoint
	Status      molecule.ChannelStatus
	Params      *channel.Params
	Header      types.Hash
	PCTS        *types.Script
	SigA        molecule.Bytes
	SigB        molecule.Bytes
}

func NewDisputeInfo(channelCell types.OutPoint, status molecule.ChannelStatus, params *channel.Params, header types.Hash, pcts *types.Script, sigA molecule.Bytes, sigB molecule.Bytes) *DisputeInfo {
	return &DisputeInfo{
		ChannelCell: channelCell,
		Status:      status,
		Params:      params,
		Header:      header,
		PCTS:        pcts,
		SigA:        sigA,
		SigB:        sigB,
	}
}
