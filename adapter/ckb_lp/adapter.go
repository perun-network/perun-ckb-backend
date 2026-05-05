package ckblp

import (
	"context"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/client"
	"perun.network/perun-ckb-backend/transaction"
)

// Adapter builds LP transactions and witnesses for the hub.
type Adapter struct {
	rpcClient    rpc.Client
	signer       backend.Signer
	transactor   backend.Transactor
	deployment   backend.Deployment
	lpDeployment LPDeployment
}

// NewAdapter creates a new LP adapter with the provided dependencies.
func NewAdapter(
	rpcClient rpc.Client,
	signer backend.Signer,
	transactor backend.Transactor,
	deployment backend.Deployment,
	lpDeployment LPDeployment,
) *Adapter {
	return &Adapter{
		rpcClient:    rpcClient,
		signer:       signer,
		transactor:   transactor,
		deployment:   deployment,
		lpDeployment: lpDeployment,
	}
}

// BuildFundChannelTx builds a FundChannelExtract transaction (not implemented yet).
func (a *Adapter) BuildFundChannelTx(
	ctx context.Context,
	channelID string,
	lpCellID string,
	amount uint64,
) error {
	if amount == 0 {
		return Deterministic(ErrInvalidWitness)
	}
	channelHash, err := parseHash32(channelID)
	if err != nil {
		return Deterministic(err)
	}
	if isZeroHash(channelHash) {
		return Deterministic(ErrInvalidChannelID)
	}
	outPoint, err := parseOutPoint(lpCellID)
	if err != nil {
		return Deterministic(err)
	}
	cell, err := a.rpcClient.GetLiveCell(ctx, outPoint, true)
	if err != nil {
		return Retriable(err)
	}
	if cell == nil || cell.Cell == nil || cell.Cell.Output == nil || cell.Cell.Data == nil {
		return Deterministic(ErrInvalidLPCell)
	}
	inputLPData := cell.Cell.Data.Content
	inputLP, err := DecodeLPCell(inputLPData)
	if err != nil {
		return Deterministic(ErrInvalidLPCell)
	}
	operatorLockHash := a.signer.Address().Script.Hash()
	if inputLP.OperatorLockHash != operatorLockHash {
		return Deterministic(ErrScriptHashMismatch)
	}
	if amount > inputLP.AvailableCKB {
		return Deterministic(ErrInvalidLPCellArg)
	}

	channelCell, err := a.findChannelCellByID(ctx, channelHash)
	if err != nil {
		return err
	}
	operatorCell, err := a.selectLargestOperatorCell(ctx)
	if err != nil {
		return err
	}

	updatedLP := inputLP
	updatedLP.AvailableCKB -= amount
	updatedLP.ReservedCKB += amount
	updatedLP.Nonce += 1
	updatedLPData, err := EncodeLPCell(updatedLP)
	if err != nil {
		return Deterministic(ErrInvalidLPCell)
	}

	fee := transaction.DefaultFeeShannon
	if operatorCell.Output.Capacity <= fee {
		return Deterministic(ErrInsufficientOperatorFunds)
	}
	operatorChange := operatorCell.Output.Capacity - fee
	if cell.Cell.Output.Capacity < amount {
		return Deterministic(ErrInvalidLPCellArg)
	}
	updatedLPCap := cell.Cell.Output.Capacity - amount
	updatedChannelCap := channelCell.Output.Capacity + amount

	builder := transaction.NewSimpleTransactionBuilder(a.signer.Address().Script.CodeHash, a.deployment.DefaultLockScriptDep, false)
	if a.signer.Address().Script.CodeHash != a.deployment.DefaultLockScript.CodeHash && len(a.deployment.OmniLockScriptDep) >= 2 {
		builder = transaction.NewSimpleTransactionBuilder(a.deployment.OmniLockScript.CodeHash, a.deployment.OmniLockScriptDep[1], true)
		builder.AddCellDep(&a.deployment.OmniLockScriptDep[0])
		builder.AddCellDep(&a.deployment.OmniLockScriptDep[1])
	} else {
		builder.AddCellDep(&a.deployment.DefaultLockScriptDep)
	}
	builder.AddCellDep(&a.lpDeployment.TypeScriptDep)
	builder.AddCellDep(&a.lpDeployment.LockScriptDep)

	inputs := []*types.CellInput{
		{PreviousOutput: outPoint},
		{PreviousOutput: channelCell.OutPoint},
		{PreviousOutput: operatorCell.OutPoint},
	}
	for _, input := range inputs {
		builder.AddInput(input)
		builder.Witnesses = append(builder.Witnesses, []byte{})
	}

	updatedLPOutput := &types.CellOutput{
		Capacity: updatedLPCap,
		Lock:     cell.Cell.Output.Lock,
		Type:     cell.Cell.Output.Type,
	}
	updatedChannelOutput := &types.CellOutput{
		Capacity: updatedChannelCap,
		Lock:     channelCell.Output.Lock,
		Type:     channelCell.Output.Type,
	}
	operatorChangeOutput := &types.CellOutput{
		Capacity: operatorChange,
		Lock:     operatorCell.Output.Lock,
		Type:     nil,
	}

	builder.AddOutput(updatedLPOutput, updatedLPData)
	builder.AddOutput(updatedChannelOutput, channelCell.OutputData)
	builder.AddOutput(operatorChangeOutput, []byte{})

	witness := EncodeFundChannelExtractWitness(FundChannelExtractWitness{
		ChannelID:      channelHash,
		ContributionID: channelHash,
		ExtractCKB:     amount,
	})
	if err := builder.SetWitness(0, types.WitnessTypeInputType, witness); err != nil {
		return Deterministic(ErrInvalidWitness)
	}

	tx, err := builder.Build()
	if err != nil {
		return Deterministic(err)
	}
	if _, err := a.transactor.SubmitTransaction(ctx, tx); err != nil {
		return Retriable(err)
	}
	return nil
}

// BuildSettleChannelInsertTx builds a SettleChannelInsert transaction (not implemented yet).
func (a *Adapter) BuildSettleChannelInsertTx(
	ctx context.Context,
	channelID string,
	contributionID string,
	lpCellID string,
	principal uint64,
	feeCKB uint64,
	priceX64 uint128.Uint128,
) error {
	if priceX64 == (uint128.Uint128{}) {
		return Deterministic(ErrZeroPrice)
	}
	channelHash, err := parseHash32(channelID)
	if err != nil {
		return Deterministic(err)
	}
	if isZeroHash(channelHash) {
		return Deterministic(ErrInvalidChannelID)
	}
	contribHash := channelHash
	if contributionID != "" {
		contribHash, err = parseHash32(contributionID)
		if err != nil {
			return Deterministic(err)
		}
		if isZeroHash(contribHash) {
			return Deterministic(ErrInvalidContributionID)
		}
	}
	outPoint, err := parseOutPoint(lpCellID)
	if err != nil {
		return Deterministic(err)
	}
	cell, err := a.rpcClient.GetLiveCell(ctx, outPoint, true)
	if err != nil {
		return Retriable(err)
	}
	if cell == nil || cell.Cell == nil || cell.Cell.Output == nil || cell.Cell.Data == nil {
		return Deterministic(ErrInvalidLPCell)
	}
	inputLPData := cell.Cell.Data.Content
	inputLP, err := DecodeLPCell(inputLPData)
	if err != nil {
		return Deterministic(ErrInvalidLPCell)
	}
	operatorLockHash := a.signer.Address().Script.Hash()
	if inputLP.OperatorLockHash != operatorLockHash {
		return Deterministic(ErrScriptHashMismatch)
	}
	if principal > inputLP.ReservedCKB {
		return Deterministic(ErrInvalidLPCellArg)
	}
	if (inputLP.Policy.PolicyFlags&policyFlagRequirePrice) != 0 && priceX64 == (uint128.Uint128{}) {
		return Deterministic(ErrZeroPrice)
	}
	if (inputLP.Policy.PolicyFlags & policyFlagSafePrice) != 0 {
		if priceX64.Cmp(inputLP.Policy.SafePriceMinX64) < 0 || priceX64.Cmp(inputLP.Policy.SafePriceMaxX64) > 0 {
			return Deterministic(ErrInvalidLPCellArg)
		}
	}

	operatorCell, err := a.selectLargestOperatorCell(ctx)
	if err != nil {
		return err
	}
	updatedLP := inputLP
	updatedLP.AvailableCKB += principal + feeCKB
	updatedLP.ReservedCKB -= principal
	updatedLP.CumulativeFeesEarnedCKB += feeCKB
	updatedLP.Nonce += 1
	updatedLPData, err := EncodeLPCell(updatedLP)
	if err != nil {
		return Deterministic(ErrInvalidLPCell)
	}

	fee := transaction.DefaultFeeShannon
	totalReturn := principal + feeCKB
	if operatorCell.Output.Capacity <= totalReturn+fee {
		return Deterministic(ErrInsufficientOperatorFunds)
	}
	operatorChange := operatorCell.Output.Capacity - totalReturn - fee
	updatedLPCap := cell.Cell.Output.Capacity + totalReturn

	builder := transaction.NewSimpleTransactionBuilder(a.signer.Address().Script.CodeHash, a.deployment.DefaultLockScriptDep, false)
	if a.signer.Address().Script.CodeHash != a.deployment.DefaultLockScript.CodeHash && len(a.deployment.OmniLockScriptDep) >= 2 {
		builder = transaction.NewSimpleTransactionBuilder(a.deployment.OmniLockScript.CodeHash, a.deployment.OmniLockScriptDep[1], true)
		builder.AddCellDep(&a.deployment.OmniLockScriptDep[0])
		builder.AddCellDep(&a.deployment.OmniLockScriptDep[1])
	} else {
		builder.AddCellDep(&a.deployment.DefaultLockScriptDep)
	}
	builder.AddCellDep(&a.lpDeployment.TypeScriptDep)
	builder.AddCellDep(&a.lpDeployment.LockScriptDep)

	inputs := []*types.CellInput{
		{PreviousOutput: outPoint},
		{PreviousOutput: operatorCell.OutPoint},
	}
	for _, input := range inputs {
		builder.AddInput(input)
		builder.Witnesses = append(builder.Witnesses, []byte{})
	}

	updatedLPOutput := &types.CellOutput{
		Capacity: updatedLPCap,
		Lock:     cell.Cell.Output.Lock,
		Type:     cell.Cell.Output.Type,
	}
	operatorChangeOutput := &types.CellOutput{
		Capacity: operatorChange,
		Lock:     operatorCell.Output.Lock,
		Type:     nil,
	}
	builder.AddOutput(updatedLPOutput, updatedLPData)
	builder.AddOutput(operatorChangeOutput, []byte{})

	witness := EncodeSettleChannelInsertWitness(SettleChannelInsertWitness{
		ChannelID:         channelHash,
		ContributionID:    contribHash,
		PrincipalReturned: principal,
		FeeCKB:            feeCKB,
		PriceX64:          priceX64,
	})
	if err := builder.SetWitness(0, types.WitnessTypeInputType, witness); err != nil {
		return Deterministic(ErrInvalidWitness)
	}

	tx, err := builder.Build()
	if err != nil {
		return Deterministic(err)
	}
	if _, err := a.transactor.SubmitTransaction(ctx, tx); err != nil {
		return Retriable(err)
	}
	return nil
}

// findChannelCellByID locates a channel cell by channel ID.
func (a *Adapter) findChannelCellByID(ctx context.Context, channelID types.Hash) (*indexer.LiveCell, error) {
	searchKey := &indexer.SearchKey{
		Script: &types.Script{
			CodeHash: a.deployment.PCTSCodeHash,
			HashType: a.deployment.PCTSHashType,
			Args:     []byte{},
		},
		ScriptType:       types.ScriptTypeType,
		ScriptSearchMode: types.ScriptSearchModePrefix,
		WithData:         true,
	}
	resp, err := a.rpcClient.GetCells(ctx, searchKey, indexer.SearchOrderDesc, client.SearchIndexerLimit, "")
	if err != nil {
		return nil, Retriable(err)
	}
	for _, cell := range resp.Objects {
		if cell.Output == nil {
			continue
		}
		status, err := molecule.ChannelStatusFromSlice(cell.OutputData, false)
		if err != nil {
			continue
		}
		if types.UnpackHash(status.State().ChannelId()) == channelID {
			return cell, nil
		}
	}
	return nil, Deterministic(ErrInvalidLPCellArg)
}

func (a *Adapter) selectLargestOperatorCell(ctx context.Context) (*indexer.LiveCell, error) {
	signerScript := a.signer.Address().Script
	searchKey := &indexer.SearchKey{
		Script:           signerScript,
		ScriptType:       types.ScriptTypeLock,
		ScriptSearchMode: types.ScriptSearchModeExact,
		WithData:         true,
	}
	resp, err := a.rpcClient.GetCells(ctx, searchKey, indexer.SearchOrderDesc, client.SearchIndexerLimit, "")
	if err != nil {
		return nil, Retriable(err)
	}
	var best *indexer.LiveCell
	for _, cell := range resp.Objects {
		if cell.Output == nil || cell.Output.Type != nil {
			continue
		}
		if IsLPCell(cell.OutputData) {
			continue
		}
		if best == nil || cell.Output.Capacity > best.Output.Capacity {
			best = cell
		}
	}
	if best == nil {
		return nil, Deterministic(ErrInsufficientOperatorFunds)
	}
	return best, nil
}
