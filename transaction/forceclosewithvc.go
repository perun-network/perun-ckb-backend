package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
)

type ForceCloseWithVCInfo struct {
	ChannelCell types.OutPoint
	VCCell      types.OutPoint
	LCStatus    molecule.ChannelStatus
	VCStatus    molecule.VirtualChannelStatus
	State       *channel.State
	Params      *channel.Params
	Header      types.Hash
	AssetInputs []types.CellInput

	firstForceClose bool
}

func NewForceCloseWithVCInfo(
	channelCell *types.OutPoint,
	vcCell *types.OutPoint,
	state *channel.State,
	params *channel.Params,
	header types.Hash,
	assetInputs []types.CellInput,
	firstForceClose bool,
) *ForceCloseWithVCInfo {
	return &ForceCloseWithVCInfo{
		ChannelCell:     *channelCell,
		VCCell:          *vcCell,
		State:           state,
		Params:          params,
		Header:          header,
		AssetInputs:     assetInputs,
		firstForceClose: firstForceClose,
	}
}
