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
	ParentHashA *types.Script
	ParentHashB *types.Script
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
	parentSigA molecule.Bytes,
	parentSigB molecule.Bytes,
	parentHashA *types.Script,
	parentHashB *types.Script,
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
		ParentSigA:  parentSigA,
		ParentSigB:  parentSigB,
		ParentHashA: parentHashA,
		ParentHashB: parentHashB,
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

	var parentHashes [2]molecule.Byte32
	parentHashes[0] = *molecule2.PackByte32(di.ParentHashA.Hash())
	parentHashes[1] = *molecule2.PackByte32(di.ParentHashB.Hash())

	parentVec := molecule.NewParentsVecBuilder()
	for i := 0; i < 2; i++ {
		parentVec.Push(molecule.NewParentDataBuilder().IdxMap(*di.IndexMap).PctsHash(parentHashes[i]).Build())
	}

	return molecule.NewVirtualChannelStatusBuilder().
		Vcstate(packedState).
		Parents(parentVec.Build()).
		FirstForceClose(encoding.False).
		Build()
}

func (di *VcDisputeInfo) updateDisputed() *VcDisputeInfo {
	builder := di.LCStatus.AsBuilder()
	newStatus := builder.Disputed(encoding.True).VcDisputed(encoding.True).Build()
	di.LCStatus = &newStatus
	return di
}
