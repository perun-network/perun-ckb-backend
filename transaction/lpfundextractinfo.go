package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

// LPFundExtractInfo carries all the resolved state needed to assemble an LP
// FundChannelExtract transaction. The adapter resolves live cells and state
// transitions; the handler only stitches inputs, outputs, witness, and cell
// deps together.
//
// On-chain shape produced by the handler:
//
//	inputs:  [LPInput, OperatorInput]
//	outputs: [LPOutput (updated), proxy(OperatorLock, ChannelStatus{ChannelID}, cap=ExtractCKB), operator change]
//	witness on input 0: PoolWitness::FundChannelExtract
type LPFundExtractInfo struct {
	// LPInput is the LP cell being consumed (input index 0).
	LPInput types.CellInput
	// LPOutput is the updated LP cell layout (lock + type are reused from the
	// input by the adapter; capacity and data are updated to reflect the extract).
	LPOutput types.CellOutput
	// LPOutputData is EncodeLPCell(updatedLP).
	LPOutputData []byte

	// OperatorInput is an operator-locked cell that funds tx fees and carries
	// the lock-script group for the operator signature (input index 1).
	OperatorInput types.CellInput
	// OperatorLock is the operator's lock script. Used for the proxy cell and
	// the operator change cell.
	OperatorLock *types.Script
	// OperatorChangeCap is the capacity returned to the operator after deducting
	// the tx fee. The proxy cell consumes ExtractCKB of the operator's input
	// capacity-equivalent (the LP cell provides the actual extracted CKB).
	OperatorChangeCap uint64

	// ExtractCKB is the amount of CKB to extract from the LP cell into the proxy
	// channel cell.
	ExtractCKB uint64

	// ChannelID labels the extract (carried in PoolWitness and in proxy ChannelStatus data).
	ChannelID types.Hash
	// ContributionID labels the contribution (carried only in PoolWitness).
	ContributionID types.Hash

	// LPTypeScriptDep + LPLockScriptDep are added as cell deps by the handler.
	LPTypeScriptDep types.CellDep
	LPLockScriptDep types.CellDep
}

func NewLPFundExtractInfo(
	lpInput types.CellInput,
	lpOutput types.CellOutput,
	lpOutputData []byte,
	operatorInput types.CellInput,
	operatorLock *types.Script,
	operatorChangeCap uint64,
	extractCKB uint64,
	channelID types.Hash,
	contributionID types.Hash,
	lpTypeScriptDep types.CellDep,
	lpLockScriptDep types.CellDep,
) *LPFundExtractInfo {
	return &LPFundExtractInfo{
		LPInput:           lpInput,
		LPOutput:          lpOutput,
		LPOutputData:      lpOutputData,
		OperatorInput:     operatorInput,
		OperatorLock:      operatorLock,
		OperatorChangeCap: operatorChangeCap,
		ExtractCKB:        extractCKB,
		ChannelID:         channelID,
		ContributionID:    contributionID,
		LPTypeScriptDep:   lpTypeScriptDep,
		LPLockScriptDep:   lpLockScriptDep,
	}
}
