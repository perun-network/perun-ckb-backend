package transaction

import (
	"fmt"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/encoding"
)

type DisputeInfo struct {
	ChannelCell types.OutPoint
	Status      molecule.ChannelStatus
	NewState    *channel.State
	Params      *channel.Params
	Header      types.Hash
	PCTS        *types.Script
	SigA        molecule.Bytes
	SigB        molecule.Bytes

	// InputChannelCapacity is the actual capacity of the channel cell being consumed. The
	// rebuilt channel cell preserves it so the reserved sub-alloc capacity is conserved
	// across the dispute (see encoding.LockedSubAllocReserve).
	InputChannelCapacity uint64
}

func NewDisputeInfo(
	channelCell types.OutPoint,
	status molecule.ChannelStatus,
	newState *channel.State,
	params *channel.Params,
	header types.Hash,
	ts *types.Script,
	sigA molecule.Bytes,
	sigB molecule.Bytes,
) *DisputeInfo {
	return &DisputeInfo{
		ChannelCell: channelCell,
		Status:      status,
		NewState:    newState,
		Params:      params,
		Header:      header,
		PCTS:        ts,
		SigA:        sigA,
		SigB:        sigB,
	}
}

func (di *DisputeInfo) update() (*DisputeInfo, error) {
	builder := di.Status.AsBuilder()
	newState, err := encoding.PackChannelState(di.NewState)
	if err != nil {
		return nil, fmt.Errorf("packing channel state for dispute: %w", err)
	}
	newStatus := builder.State(newState).Disputed(encoding.True).Build()
	di.Status = newStatus
	return di, nil
}
