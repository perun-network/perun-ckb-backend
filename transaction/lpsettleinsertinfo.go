package transaction

import (
	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

// LPSettleInsertInfo carries the resolved state needed to assemble an LP
// SettleChannelInsert transaction. The referenced channel_id MUST NOT appear
// in any input or output cell — settle-insert always runs in its own tx,
// after the corresponding channel has been consumed (closed or aborted).
//
// On-chain shape produced by the handler:
//
//	inputs:  [LPInput, OperatorInput]
//	outputs: [LPOutput (updated), operator change]
//	witness on input 0: PoolWitness::SettleChannelInsert
type LPSettleInsertInfo struct {
	// LPInput is the LP cell being consumed (input index 0).
	LPInput types.CellInput
	// LPOutput is the updated LP cell layout.
	LPOutput types.CellOutput
	// LPOutputData is EncodeLPCell(updatedLP).
	LPOutputData []byte

	// OperatorInput is the operator-locked cell that funds principal + fee back
	// into the LP cell plus the tx fee (input index 1).
	OperatorInput types.CellInput
	// OperatorLock is the operator's lock script. Used for the operator change cell.
	OperatorLock *types.Script
	// OperatorChangeCap = OperatorInput.cap - (Principal+FeeCKB) - tx_fee.
	OperatorChangeCap uint64

	// Principal is the CKB amount being returned to the LP cell from the
	// settled channel position.
	Principal uint64
	// FeeCKB is the operator's fee paid to the LP (increases LP.CumulativeFeesEarnedCKB).
	FeeCKB uint64
	// TradedCKB is the portion of the channel's extract that was sold to the
	// peer during the swap and does not return. Principal+TradedCKB together
	// release the channel's full reservation (decrease LP.ReservedCKB), and
	// the LP policy fee applies to TradedCKB.
	TradedCKB uint64
	// PriceX64 is the operator-declared price for this settlement, validated
	// against LP policy by the adapter before this struct is built.
	PriceX64 uint128.Uint128

	// ChannelID labels the settle (carried in PoolWitness).
	ChannelID types.Hash
	// ContributionID labels the contribution (carried in PoolWitness).
	ContributionID types.Hash

	// LPTypeScriptDep + LPLockScriptDep are added as cell deps by the handler.
	LPTypeScriptDep types.CellDep
	LPLockScriptDep types.CellDep
}

func NewLPSettleInsertInfo(
	lpInput types.CellInput,
	lpOutput types.CellOutput,
	lpOutputData []byte,
	operatorInput types.CellInput,
	operatorLock *types.Script,
	operatorChangeCap uint64,
	principal uint64,
	feeCKB uint64,
	tradedCKB uint64,
	priceX64 uint128.Uint128,
	channelID types.Hash,
	contributionID types.Hash,
	lpTypeScriptDep types.CellDep,
	lpLockScriptDep types.CellDep,
) *LPSettleInsertInfo {
	return &LPSettleInsertInfo{
		LPInput:           lpInput,
		LPOutput:          lpOutput,
		LPOutputData:      lpOutputData,
		OperatorInput:     operatorInput,
		OperatorLock:      operatorLock,
		OperatorChangeCap: operatorChangeCap,
		Principal:         principal,
		FeeCKB:            feeCKB,
		TradedCKB:         tradedCKB,
		PriceX64:          priceX64,
		ChannelID:         channelID,
		ContributionID:    contributionID,
		LPTypeScriptDep:   lpTypeScriptDep,
		LPLockScriptDep:   lpLockScriptDep,
	}
}
