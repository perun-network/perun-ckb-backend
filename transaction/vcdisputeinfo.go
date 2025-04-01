package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/encoding"

	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
)

type VcDisputeInfo struct {
	ChannelCell *types.OutPoint
	VCCell      *types.OutPoint
	LCStatus    *molecule.ChannelStatus
	VCStatus    *molecule.VirtualChannelStatus
	State       *channel.State
	Params      *channel.Params
	Header      types.Hash
	PCTS        *types.Script
	VCTS        *types.Script
	ParentSigA  molecule.Bytes
	ParentSigB  molecule.Bytes
	VCDispute   *molecule.VCDispute
	IndexMap    *molecule.IndexMap
	first       bool
}

func NewVCDisputeInfo(
	channelCell *types.OutPoint,
	vcCell *types.OutPoint,
	lcstatus *molecule.ChannelStatus,
	vcStatus *molecule.VirtualChannelStatus,
	state *channel.State,
	params *channel.Params,
	header types.Hash,
	pcts *types.Script,
	vcts *types.Script,
	sigA molecule.Bytes,
	sigB molecule.Bytes,
	vcDispute *molecule.VCDispute,
	indexMap *molecule.IndexMap,
	first bool,
) *VcDisputeInfo {
	return &VcDisputeInfo{
		ChannelCell: channelCell,
		VCCell:      vcCell,
		LCStatus:    lcstatus,
		VCStatus:    vcStatus,
		State:       state,
		Params:      params,
		Header:      header,
		PCTS:        pcts,
		VCTS:        vcts,
		ParentSigA:  sigA,
		ParentSigB:  sigB,
		VCDispute:   vcDispute,
		IndexMap:    indexMap,
		first:       first,
	}
}

func (di *VcDisputeInfo) mkInitialVirtualChannelCell(vcLockScript, vcTypeScript types.Script) (types.CellOutput, []byte) {
	di.VCTS = &vcTypeScript

	vcStatus := di.mkInitialVirtualChannelStatus()
	vcOutput := types.CellOutput{
		Capacity: 0,
		Lock:     &vcLockScript,
		Type:     &vcTypeScript,
	}
	capacity := vcOutput.OccupiedCapacity(vcStatus.AsSlice())
	vcOutput.Capacity = capacity
	return vcOutput, vcStatus.AsSlice()
}

func (di *VcDisputeInfo) mkInitialVirtualChannelStatus() molecule.VirtualChannelStatus {
	packedState, err := encoding.PackChannelState(di.State)
	if err != nil {
		panic(err)
	}

	parentsHashes := di.Params.Aux
	parentVec := molecule.NewParentsVecBuilder()
	for i := 0; i < 2; i++ {
		var hash [32]byte
		startIndex := i * 32
		endIndex := 32 * (i + 1)
		copy(hash[:], parentsHashes[startIndex:endIndex])
		parentVec.Push(molecule.NewParentDataBuilder().IdxMap(*di.IndexMap).PctsHash(*molecule2.PackByte32(hash)).Build())
	}

	return molecule.NewVirtualChannelStatusBuilder().
		Vcstate(packedState).
		Parents(parentVec.Build()).
		FirstForceClose(encoding.False).
		Build()
}

func (di *VcDisputeInfo) updateDisputed() *VcDisputeInfo {
	builder := di.LCStatus.AsBuilder()
	newStatus := builder.VcDisputed(encoding.True).Build()
	di.LCStatus = &newStatus
	return di
}
