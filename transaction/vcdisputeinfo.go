package transaction

import (
	"encoding/hex"
	"log"

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
	owner       *address.Participant
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
	}
}

func (di *VcDisputeInfo) mkInitialVirtualChannelCell(owner address.Participant, vcLockScript, vcTypeScript types.Script) (types.CellOutput, []byte) {
	di.VCTS = &vcTypeScript

	vcStatus := di.mkInitialVirtualChannelStatus(owner)
	log.Println("mkInitialVirtualChannelStatus: ", "0x"+hex.EncodeToString(vcStatus.AsSlice()))
	vcOutput := types.CellOutput{
		Capacity: 0,
		Lock:     &vcLockScript,
		Type:     &vcTypeScript,
	}
	capacity := vcOutput.OccupiedCapacity(vcStatus.AsSlice())
	log.Println("Capacity: ", capacity)
	vcOutput.Capacity = capacity
	return vcOutput, vcStatus.AsSlice()
}

func (di *VcDisputeInfo) mkInitialVirtualChannelStatus(owner address.Participant) molecule.VirtualChannelStatus {
	packedState, err := encoding.PackChannelState(di.VcState)
	if err != nil {
		panic(err)
	}

	ownerPacked, err := owner.PackOnChainParticipant()
	if err != nil {
		panic(err)
	}

	return molecule.NewVirtualChannelStatusBuilder().
		Vcstate(packedState).
		Parents(*di.ParentsVec).
		FirstForceClose(encoding.False).
		Owner(ownerPacked).
		Build()
}

func (di *VcDisputeInfo) update(vcts *types.Script) *VcDisputeInfo {
	builder := di.LCStatus.AsBuilder()
	newState, err := encoding.PackChannelState(di.ParentState)
	if err != nil {
		panic(err)
	}
	newStatus := builder.State(newState).Disputed(encoding.True).VcDisputed(encoding.True).VctsHash(*molecule2.PackByte32(vcts.Hash())).Build()
	di.LCStatus = &newStatus
	return di
}

func (di *VcDisputeInfo) updateVCStatus() *VcDisputeInfo {
	builder := di.VCStatus.AsBuilder()
	newVCState, err := encoding.PackChannelState(di.VcState)
	if err != nil {
		panic(err)
	}
	newVCStatus := builder.Vcstate(newVCState).Build()
	di.VCStatus = &newVCStatus
	return di
}
