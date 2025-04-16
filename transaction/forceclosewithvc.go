package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/encoding"
)

type ForceCloseWithVCInfo struct {
	ChannelCell types.OutPoint
	VCCell      types.OutPoint
	VCTS        *types.Script
	State       *channel.State
	VCStatus    *molecule.VirtualChannelStatus
	VCState     *channel.State
	Params      *channel.Params
	Headers     []types.Hash
	AssetInputs []types.CellInput

	SigA      molecule.Bytes
	SigB      molecule.Bytes
	VCDispute *molecule.VCDispute

	ChannelCapacity        uint64
	VirtualChannelCapacity uint64
	firstForceClose        bool

	IndexMap []channel.Index

	MinCKBInput *types.OutPoint
}

func NewForceCloseWithVCInfo(
	channelCell *types.OutPoint,
	vcCell *types.OutPoint,
	vcts *types.Script,
	state *channel.State,
	vcstate *channel.State,
	vcStatus *molecule.VirtualChannelStatus,
	sigA molecule.Bytes,
	sigB molecule.Bytes,
	vcDispute *molecule.VCDispute,
	params *channel.Params,
	headers []types.Hash,
	assetInputs []types.CellInput,
	channelCapacity uint64,
	virtualChannelCapacity uint64,
	firstForceClose bool,
	indexMap []channel.Index,
) *ForceCloseWithVCInfo {
	return &ForceCloseWithVCInfo{
		ChannelCell:            *channelCell,
		VCCell:                 *vcCell,
		VCTS:                   vcts,
		State:                  state,
		VCState:                vcstate,
		VCStatus:               vcStatus,
		SigA:                   sigA,
		SigB:                   sigB,
		VCDispute:              vcDispute,
		Params:                 params,
		Headers:                headers,
		AssetInputs:            assetInputs,
		ChannelCapacity:        channelCapacity,
		VirtualChannelCapacity: virtualChannelCapacity,
		firstForceClose:        firstForceClose,
		IndexMap:               indexMap,
	}
}

func (fcvi *ForceCloseWithVCInfo) updateFirstForceClose() *ForceCloseWithVCInfo {
	fcvi.firstForceClose = true
	builder := fcvi.VCStatus.AsBuilder()
	newStatus := builder.FirstForceClose(encoding.True).Build()
	fcvi.VCStatus = &newStatus
	return fcvi
}
