package transaction

import (
	"fmt"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/encoding"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
	"perun.network/perun-ckb-backend/wallet/address"
)

type VcDisputeInfo struct {
	ChannelCell *types.OutPoint
	VCCell      *types.OutPoint
	LCStatus    *molecule.ChannelStatus
	VCStatus    *molecule.VirtualChannelStatus
	VcState     *channel.State
	ParentState *channel.State
	Params      *channel.Params
	Header      types.Hash
	PCTS        *types.Script
	VCTS        *types.Script
	ParentSigA  molecule.Bytes
	ParentSigB  molecule.Bytes
	VCDispute   *molecule.VCDispute
	ParentsVec  *molecule.ParentsVec
	first       bool
	Owner       *address.Participant

	// InputChannelCapacity is the actual capacity of the parent channel cell being consumed.
	// The rebuilt channel cell preserves it so materialising the VC's locked sub-alloc does
	// not grow the cell at register (see encoding.LockedSubAllocReserve).
	InputChannelCapacity uint64
}

func NewVCDisputeInfo(
	channelCell *types.OutPoint,
	vcCell *types.OutPoint,
	lcstatus *molecule.ChannelStatus,
	vcStatus *molecule.VirtualChannelStatus,
	vcstate *channel.State,
	parentState *channel.State,
	params *channel.Params,
	header types.Hash,
	pcts *types.Script,
	vcts *types.Script,
	parentSigA molecule.Bytes,
	parentSigB molecule.Bytes,
	vcDispute *molecule.VCDispute,
	parentVec *molecule.ParentsVec,
	first bool,
	owner *address.Participant,
) *VcDisputeInfo {
	return &VcDisputeInfo{
		ChannelCell: channelCell,
		VCCell:      vcCell,
		LCStatus:    lcstatus,
		VCStatus:    vcStatus,
		VcState:     vcstate,
		ParentState: parentState,
		Params:      params,
		Header:      header,
		PCTS:        pcts,
		VCTS:        vcts,
		ParentSigA:  parentSigA,
		ParentSigB:  parentSigB,
		VCDispute:   vcDispute,
		ParentsVec:  parentVec,
		first:       first,
		Owner:       owner,
	}
}

func (di *VcDisputeInfo) mkInitialVirtualChannelCell(vcLockScript, vcTypeScript types.Script) (types.CellOutput, []byte, error) {
	di.VCTS = &vcTypeScript

	vcStatus, err := di.mkInitialVirtualChannelStatus()
	if err != nil {
		return types.CellOutput{}, nil, err
	}
	vcOutput := types.CellOutput{
		Capacity: 0,
		Lock:     &vcLockScript,
		Type:     &vcTypeScript,
	}
	capacity := vcOutput.OccupiedCapacity(vcStatus.AsSlice())
	vcOutput.Capacity = capacity
	return vcOutput, vcStatus.AsSlice(), nil
}

func (di *VcDisputeInfo) mkInitialVirtualChannelStatus() (molecule.VirtualChannelStatus, error) {
	packedState, err := encoding.PackChannelState(di.VcState)
	if err != nil {
		return molecule.VirtualChannelStatus{}, fmt.Errorf("packing vc state: %w", err)
	}

	ownerPacked, err := di.Owner.PackOnChainParticipant()
	if err != nil {
		return molecule.VirtualChannelStatus{}, fmt.Errorf("packing owner: %w", err)
	}

	return molecule.NewVirtualChannelStatusBuilder().
		Vcstate(packedState).
		Parents(*di.ParentsVec).
		FirstForceClose(encoding.False).
		Owner(ownerPacked).
		Build(), nil
}

func (di *VcDisputeInfo) update(vcts *types.Script) (*VcDisputeInfo, error) {
	builder := di.LCStatus.AsBuilder()
	newState, err := encoding.PackChannelState(di.ParentState)
	if err != nil {
		return nil, fmt.Errorf("packing parent state: %w", err)
	}
	newStatus := builder.State(newState).Disputed(encoding.True).VcDisputed(encoding.True).VctsHash(*molecule2.PackByte32(vcts.Hash())).Build()
	di.LCStatus = &newStatus
	return di, nil
}

func (di *VcDisputeInfo) updateVCStatus() (*VcDisputeInfo, error) {
	builder := di.VCStatus.AsBuilder()
	newVCState, err := encoding.PackChannelState(di.VcState)
	if err != nil {
		return nil, fmt.Errorf("packing vc state: %w", err)
	}
	newVCStatus := builder.Vcstate(newVCState).Build()
	di.VCStatus = &newVCStatus
	return di, nil
}
