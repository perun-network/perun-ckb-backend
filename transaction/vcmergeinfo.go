package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
)

// VcMergeInfo contains the information needed to merge two virtual channels.
type VcMergeInfo struct {
	VCCell0           types.OutPoint
	VCCell1           types.OutPoint
	VCStatus0         molecule.VirtualChannelStatus
	VCStatus1         molecule.VirtualChannelStatus
	OccupiedCapacity0 uint64
	OccupiedCapacity1 uint64
	ParentParams0     *channel.Params
	ParentParams1     *channel.Params
	BlockNum0         uint64
	BlockNum1         uint64
	Header            types.Hash
	VCTS              *types.Script
	VCDispute         *molecule.VCDispute

	// RestoredOwnerScript0 / RestoredOwnerScript1 are the real payment scripts of the
	// owners of VCStatus0 / VCStatus1, resolved from the on-chain owner records (see
	// address.RecoverOnChainPaymentScript). The dropped cell's capacity is returned to the
	// matching owner script.
	RestoredOwnerScript0 *types.Script
	RestoredOwnerScript1 *types.Script
}

// NewVcMergeInfo creates a new VcMergeInfo instance with all the required fields.
func NewVCMergeInfo(
	vcCell0 *types.OutPoint,
	vcCell1 *types.OutPoint,
	vcstatus0 *molecule.VirtualChannelStatus,
	vcStatus1 *molecule.VirtualChannelStatus,
	occupiedCapacity0 uint64,
	occupiedCapacity1 uint64,
	blockNum0 uint64,
	blockNum1 uint64,
	header types.Hash,
	vcts *types.Script,
	vcDispute *molecule.VCDispute,
	restoredOwnerScript0 *types.Script,
	restoredOwnerScript1 *types.Script,
) *VcMergeInfo {
	return &VcMergeInfo{
		VCCell0:              *vcCell0,
		VCCell1:              *vcCell1,
		VCStatus0:            *vcstatus0,
		VCStatus1:            *vcStatus1,
		OccupiedCapacity0:    occupiedCapacity0,
		OccupiedCapacity1:    occupiedCapacity1,
		BlockNum0:            blockNum0,
		BlockNum1:            blockNum1,
		Header:               header,
		VCTS:                 vcts,
		VCDispute:            vcDispute,
		RestoredOwnerScript0: restoredOwnerScript0,
		RestoredOwnerScript1: restoredOwnerScript1,
	}
}
