package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
)

type VcMergeInfo struct {
	VCCell0    types.OutPoint
	VCCell1    types.OutPoint
	VCStatus0  molecule.VirtualChannelStatus
	VCStatus1  molecule.VirtualChannelStatus
	BlockNum0  uint64
	BlockNum1  uint64
	Header     types.Hash
	VCTS       *types.Script
	ParentSigA molecule.Bytes
	ParentSigB molecule.Bytes
	VCDispute  *molecule.VCDispute
}

func NewVCMergeInfo(
	vcCell0 *types.OutPoint,
	vcCell1 *types.OutPoint,
	vcstatus0 *molecule.VirtualChannelStatus,
	vcStatus1 *molecule.VirtualChannelStatus,
	blockNum0 uint64,
	blockNum1 uint64,
	header types.Hash,
	vcts *types.Script,
	sigA molecule.Bytes,
	sigB molecule.Bytes,
	vcDispute *molecule.VCDispute,
) *VcMergeInfo {
	return &VcMergeInfo{
		VCCell0:    *vcCell0,
		VCCell1:    *vcCell1,
		VCStatus0:  *vcstatus0,
		VCStatus1:  *vcStatus1,
		BlockNum0:  blockNum0,
		BlockNum1:  blockNum1,
		Header:     header,
		VCTS:       vcts,
		ParentSigA: sigA,
		ParentSigB: sigB,
		VCDispute:  vcDispute,
	}
}
