package ckblp

import (
	"context"
	"fmt"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
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

// BuildLPDepositTx builds and submits an LP deposit (creation/top-up) transaction.
// The signer is both owner and operator of the resulting LP cell.
func (a *Adapter) BuildLPDepositTx(ctx context.Context, lpCell LPCell) (string, error) {
	return a.BuildLPDepositTxWithOperator(ctx, lpCell, a.signer.Address().Script.Hash())
}

// BuildLPDepositTxWithOperator builds and submits an LP deposit transaction with
// a caller-supplied operator lock hash. The signer pays for the cell and becomes
// the owner; only the in-cell operator_lock_hash field diverges from the signer.
// Used when an LP wants to delegate fund-extract/settle-insert to a separate
// operator account.
func (a *Adapter) BuildLPDepositTxWithOperator(ctx context.Context, lpCell LPCell, operatorLockHash types.Hash) (string, error) {
	if lpCell.AvailableCKB == 0 {
		return "", Deterministic(ErrInvalidLPCellArg)
	}
	operatorCell, err := a.selectLargestOperatorCell(ctx)
	if err != nil {
		return "", err
	}
	if operatorCell.Output == nil {
		return "", Deterministic(ErrInvalidLPCell)
	}

	signerHash := a.signer.Address().Script.Hash()
	lpCell.OwnerLockHash = signerHash
	lpCell.OperatorLockHash = operatorLockHash
	lpCell.ReservedCKB = 0
	lpCell.CumulativeFeesEarnedCKB = 0
	lpCell.Nonce = 0
	lpCell.Active = true

	data, err := EncodeLPCell(lpCell)
	if err != nil {
		return "", Deterministic(ErrInvalidLPCell)
	}

	fee := transaction.DefaultFeeShannon
	if lpCell.AvailableCKB < lpCellMinOccupiedShannons {
		return "", Deterministic(ErrInvalidLPCellArg)
	}
	if operatorCell.Output.Capacity <= lpCell.AvailableCKB+fee {
		return "", Deterministic(ErrInsufficientOperatorFunds)
	}
	change := operatorCell.Output.Capacity - lpCell.AvailableCKB - fee

	lockScript, typeScript := buildLPScriptsFromDeployment(a.lpDeployment, lpCell.PoolID)

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

	builder.AddInput(&types.CellInput{PreviousOutput: operatorCell.OutPoint})
	initLockWitnessPlaceholder(builder, 0)
	if err := builder.SetWitness(0, types.WitnessTypeInputType, EncodeLPDepositWitness()); err != nil {
		return "", Deterministic(ErrInvalidWitness)
	}

	builder.AddOutput(&types.CellOutput{
		Capacity: lpCell.AvailableCKB,
		Lock:     &lockScript,
		Type:     &typeScript,
	}, data)

	builder.AddOutput(&types.CellOutput{
		Capacity: change,
		Lock:     operatorCell.Output.Lock,
		Type:     nil,
	}, []byte{})

	addInputLockScriptGroup(builder, 0, operatorCell.Output.Lock)

	tx, err := builder.Build()
	if err != nil {
		return "", Deterministic(err)
	}

	txHash, err := a.transactor.SubmitTransaction(ctx, tx)
	if err != nil {
		return "", Retriable(err)
	}
	return fmt.Sprintf("%s:%d", txHash.String(), 0), nil
}

// BuildLPWithdrawTx builds and submits an LP withdraw transaction.
func (a *Adapter) BuildLPWithdrawTx(ctx context.Context, lpCellID string, ckbOut uint64) (types.Hash, error) {
	if ckbOut == 0 {
		return types.Hash{}, Deterministic(ErrInvalidLPCellArg)
	}
	outPoint, err := parseOutPoint(lpCellID)
	if err != nil {
		return types.Hash{}, Deterministic(err)
	}
	cell, err := a.rpcClient.GetLiveCell(ctx, outPoint, true)
	if err != nil {
		return types.Hash{}, Retriable(err)
	}
	if cell == nil || cell.Cell == nil || cell.Cell.Output == nil || cell.Cell.Data == nil {
		return types.Hash{}, Deterministic(ErrInvalidLPCell)
	}
	inputLP, err := DecodeLPCell(cell.Cell.Data.Content)
	if err != nil {
		return types.Hash{}, Deterministic(ErrInvalidLPCell)
	}

	fee := transaction.DefaultFeeShannon
	if ckbOut+fee > inputLP.AvailableCKB {
		return types.Hash{}, Deterministic(ErrInvalidLPCellArg)
	}

	updatedLP := inputLP
	updatedLP.AvailableCKB -= ckbOut + fee
	updatedLP.Nonce += 1
	updatedLPData, err := EncodeLPCell(updatedLP)
	if err != nil {
		return types.Hash{}, Deterministic(ErrInvalidLPCell)
	}

	ownerCell, err := a.selectLargestOperatorCell(ctx)
	if err != nil {
		return types.Hash{}, err
	}
	if ownerCell.Output == nil {
		return types.Hash{}, Deterministic(ErrInvalidLPCell)
	}
	change := ownerCell.Output.Capacity + ckbOut

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
		{PreviousOutput: ownerCell.OutPoint},
	}
	for _, input := range inputs {
		builder.AddInput(input)
	}
	initLockWitnessPlaceholder(builder, 1)
	if err := builder.SetWitness(0, types.WitnessTypeInputType, EncodeLPWithdrawWitness(ckbOut)); err != nil {
		return types.Hash{}, Deterministic(ErrInvalidWitness)
	}

	builder.AddOutput(&types.CellOutput{
		Capacity: cell.Cell.Output.Capacity - ckbOut - fee,
		Lock:     cell.Cell.Output.Lock,
		Type:     cell.Cell.Output.Type,
	}, updatedLPData)

	builder.AddOutput(&types.CellOutput{
		Capacity: change,
		Lock:     ownerCell.Output.Lock,
		Type:     nil,
	}, []byte{})

	addInputLockScriptGroup(builder, 1, ownerCell.Output.Lock)

	tx, err := builder.Build()
	if err != nil {
		return types.Hash{}, Deterministic(err)
	}
	txHash, err := a.transactor.SubmitTransaction(ctx, tx)
	if err != nil {
		return types.Hash{}, Retriable(err)
	}
	return txHash, nil
}

// BuildFundChannelTx builds a FundChannelExtract transaction via the
// PerunTransactionBuilder. It creates a proxy channel output (operator lock,
// no type script) with capacity equal to extract_ckb. The real Perun channel
// cell is NOT consumed — only the LP cell and an operator fee cell are inputs
// — so the perun-channel-lockscript never runs and no participant signature is
// required. The LP typescript validates capacity accounting against the proxy
// channel output's data and capacity.
func (a *Adapter) BuildFundChannelTx(
	ctx context.Context,
	channelID string,
	lpCellID string,
	amount uint64,
	contributionID string,
) (types.Hash, error) {
	if amount == 0 {
		return types.Hash{}, Deterministic(ErrInvalidWitness)
	}
	channelHash, err := parseHash32(channelID)
	if err != nil {
		return types.Hash{}, Deterministic(err)
	}
	if isZeroHash(channelHash) {
		return types.Hash{}, Deterministic(ErrInvalidChannelID)
	}
	contribHash := channelHash
	if contributionID != "" {
		contribHash, err = parseHash32(contributionID)
		if err != nil {
			return types.Hash{}, Deterministic(err)
		}
		if isZeroHash(contribHash) {
			return types.Hash{}, Deterministic(ErrInvalidContributionID)
		}
	}
	outPoint, err := parseOutPoint(lpCellID)
	if err != nil {
		return types.Hash{}, Deterministic(err)
	}
	cell, err := a.rpcClient.GetLiveCell(ctx, outPoint, true)
	if err != nil {
		return types.Hash{}, Retriable(err)
	}
	if cell == nil || cell.Cell == nil || cell.Cell.Output == nil || cell.Cell.Data == nil {
		return types.Hash{}, Deterministic(ErrInvalidLPCell)
	}
	inputLP, err := DecodeLPCell(cell.Cell.Data.Content)
	if err != nil {
		return types.Hash{}, Deterministic(ErrInvalidLPCell)
	}
	operatorLockHash := a.signer.Address().Script.Hash()
	if inputLP.OperatorLockHash != operatorLockHash {
		return types.Hash{}, Deterministic(ErrScriptHashMismatch)
	}
	if amount > inputLP.AvailableCKB {
		return types.Hash{}, Deterministic(ErrInvalidLPCellArg)
	}

	operatorCell, err := a.selectLargestOperatorCell(ctx)
	if err != nil {
		return types.Hash{}, err
	}

	updatedLP := inputLP
	updatedLP.AvailableCKB -= amount
	updatedLP.ReservedCKB += amount
	updatedLP.Nonce += 1
	updatedLPData, err := EncodeLPCell(updatedLP)
	if err != nil {
		return types.Hash{}, Deterministic(ErrInvalidLPCell)
	}

	fee := transaction.DefaultFeeShannon
	if operatorCell.Output.Capacity <= fee {
		return types.Hash{}, Deterministic(ErrInsufficientOperatorFunds)
	}
	operatorChange := operatorCell.Output.Capacity - fee
	if cell.Cell.Output.Capacity < amount {
		return types.Hash{}, Deterministic(ErrInvalidLPCellArg)
	}
	updatedLPCap := cell.Cell.Output.Capacity - amount

	builder, err := a.newPerunTxBuilder()
	if err != nil {
		return types.Hash{}, Deterministic(err)
	}

	fi := &transaction.LPFundExtractInfo{
		LPInput: types.CellInput{PreviousOutput: outPoint},
		LPOutput: types.CellOutput{
			Capacity: updatedLPCap,
			Lock:     cell.Cell.Output.Lock,
			Type:     cell.Cell.Output.Type,
		},
		LPOutputData:      updatedLPData,
		OperatorInput:     types.CellInput{PreviousOutput: operatorCell.OutPoint},
		OperatorLock:      operatorCell.Output.Lock,
		OperatorChangeCap: operatorChange,
		ExtractCKB:        amount,
		ChannelID:         channelHash,
		ContributionID:    contribHash,
		LPTypeScriptDep:   a.lpDeployment.TypeScriptDep,
		LPLockScriptDep:   a.lpDeployment.LockScriptDep,
	}
	if err := builder.FundExtractLP(fi); err != nil {
		return types.Hash{}, Deterministic(err)
	}
	tx, err := builder.Build(a.signer.Contexts())
	if err != nil {
		return types.Hash{}, Deterministic(err)
	}
	txHash, err := a.transactor.SubmitTransaction(ctx, tx)
	if err != nil {
		return types.Hash{}, Retriable(err)
	}
	return txHash, nil
}

// BuildSettleChannelInsertTx builds a SettleChannelInsert transaction via the
// PerunTransactionBuilder. The referenced channel must NOT appear in any input
// or output cell — settle-insert always runs after the channel has been
// consumed elsewhere. The operator funds the principal + fee return from their
// own cells.
func (a *Adapter) BuildSettleChannelInsertTx(
	ctx context.Context,
	channelID string,
	contributionID string,
	lpCellID string,
	principal uint64,
	feeCKB uint64,
	priceX64 uint128.Uint128,
) (types.Hash, error) {
	if priceX64 == (uint128.Uint128{}) {
		return types.Hash{}, Deterministic(ErrZeroPrice)
	}
	channelHash, err := parseHash32(channelID)
	if err != nil {
		return types.Hash{}, Deterministic(err)
	}
	if isZeroHash(channelHash) {
		return types.Hash{}, Deterministic(ErrInvalidChannelID)
	}
	contribHash := channelHash
	if contributionID != "" {
		contribHash, err = parseHash32(contributionID)
		if err != nil {
			return types.Hash{}, Deterministic(err)
		}
		if isZeroHash(contribHash) {
			return types.Hash{}, Deterministic(ErrInvalidContributionID)
		}
	}
	outPoint, err := parseOutPoint(lpCellID)
	if err != nil {
		return types.Hash{}, Deterministic(err)
	}
	cell, err := a.rpcClient.GetLiveCell(ctx, outPoint, true)
	if err != nil {
		return types.Hash{}, Retriable(err)
	}
	if cell == nil || cell.Cell == nil || cell.Cell.Output == nil || cell.Cell.Data == nil {
		return types.Hash{}, Deterministic(ErrInvalidLPCell)
	}
	inputLPData := cell.Cell.Data.Content
	inputLP, err := DecodeLPCell(inputLPData)
	if err != nil {
		return types.Hash{}, Deterministic(ErrInvalidLPCell)
	}
	operatorLockHash := a.signer.Address().Script.Hash()
	if inputLP.OperatorLockHash != operatorLockHash {
		return types.Hash{}, Deterministic(ErrScriptHashMismatch)
	}
	if principal > inputLP.ReservedCKB {
		return types.Hash{}, Deterministic(ErrInvalidLPCellArg)
	}
	if (inputLP.Policy.PolicyFlags&policyFlagRequirePrice) != 0 && priceX64 == (uint128.Uint128{}) {
		return types.Hash{}, Deterministic(ErrZeroPrice)
	}
	if (inputLP.Policy.PolicyFlags & policyFlagSafePrice) != 0 {
		if priceX64.Cmp(inputLP.Policy.SafePriceMinX64) < 0 || priceX64.Cmp(inputLP.Policy.SafePriceMaxX64) > 0 {
			return types.Hash{}, Deterministic(ErrInvalidLPCellArg)
		}
	}

	operatorCell, err := a.selectLargestOperatorCell(ctx)
	if err != nil {
		return types.Hash{}, err
	}
	updatedLP := inputLP
	updatedLP.AvailableCKB += principal + feeCKB
	updatedLP.ReservedCKB -= principal
	updatedLP.CumulativeFeesEarnedCKB += feeCKB
	updatedLP.Nonce += 1
	updatedLPData, err := EncodeLPCell(updatedLP)
	if err != nil {
		return types.Hash{}, Deterministic(ErrInvalidLPCell)
	}

	fee := transaction.DefaultFeeShannon
	totalReturn := principal + feeCKB
	if operatorCell.Output.Capacity <= totalReturn+fee {
		return types.Hash{}, Deterministic(ErrInsufficientOperatorFunds)
	}
	operatorChange := operatorCell.Output.Capacity - totalReturn - fee
	updatedLPCap := cell.Cell.Output.Capacity + totalReturn

	builder, err := a.newPerunTxBuilder()
	if err != nil {
		return types.Hash{}, Deterministic(err)
	}

	si := &transaction.LPSettleInsertInfo{
		LPInput: types.CellInput{PreviousOutput: outPoint},
		LPOutput: types.CellOutput{
			Capacity: updatedLPCap,
			Lock:     cell.Cell.Output.Lock,
			Type:     cell.Cell.Output.Type,
		},
		LPOutputData:      updatedLPData,
		OperatorInput:     types.CellInput{PreviousOutput: operatorCell.OutPoint},
		OperatorLock:      operatorCell.Output.Lock,
		OperatorChangeCap: operatorChange,
		Principal:         principal,
		FeeCKB:            feeCKB,
		PriceX64:          priceX64,
		ChannelID:         channelHash,
		ContributionID:    contribHash,
		LPTypeScriptDep:   a.lpDeployment.TypeScriptDep,
		LPLockScriptDep:   a.lpDeployment.LockScriptDep,
	}
	if err := builder.SettleInsertLP(si); err != nil {
		return types.Hash{}, Deterministic(err)
	}
	tx, err := builder.Build(a.signer.Contexts())
	if err != nil {
		return types.Hash{}, Deterministic(err)
	}
	txHash, err := a.transactor.SubmitTransaction(ctx, tx)
	if err != nil {
		return types.Hash{}, Retriable(err)
	}
	return txHash, nil
}

// ReclaimProxyCell spends an operator-locked proxy cell created by a prior
// BuildFundChannelTx, sweeping its capacity (minus tx fee) into a fresh
// operator-locked plain cell. This is the third leg of the
// fund-extract / settle-insert / reclaim sequence: settle-insert cannot
// consume the proxy in the same tx because the LP typescript forbids the
// channel_id from appearing in inputs (liquidity-pool-typescript/src/main.rs:
// 578-582), so the operator must reclaim it separately. The tx invokes no LP
// scripts; it is authorized by the operator's signature alone.
func (a *Adapter) ReclaimProxyCell(ctx context.Context, proxyOutpoint string) (types.Hash, error) {
	outPoint, err := parseOutPoint(proxyOutpoint)
	if err != nil {
		return types.Hash{}, Deterministic(err)
	}
	cell, err := a.rpcClient.GetLiveCell(ctx, outPoint, true)
	if err != nil {
		return types.Hash{}, Retriable(err)
	}
	if cell == nil || cell.Cell == nil || cell.Cell.Output == nil {
		return types.Hash{}, Deterministic(ErrInvalidLPCell)
	}
	if cell.Cell.Output.Type != nil {
		// Refuses to spend cells with a type script (e.g. an LP cell passed by mistake).
		return types.Hash{}, Deterministic(ErrInvalidLPCellArg)
	}
	operatorLock := cell.Cell.Output.Lock
	if operatorLock == nil || operatorLock.Hash() != a.signer.Address().Script.Hash() {
		return types.Hash{}, Deterministic(ErrScriptHashMismatch)
	}

	fee := transaction.DefaultFeeShannon
	if cell.Cell.Output.Capacity <= fee {
		return types.Hash{}, Deterministic(ErrInsufficientOperatorFunds)
	}
	reclaimed := cell.Cell.Output.Capacity - fee

	builder, err := a.newPerunTxBuilder()
	if err != nil {
		return types.Hash{}, Deterministic(err)
	}
	// Reclaim bypasses the LP scripts entirely (no LP cell in/out, no LP
	// witness), so PerunScriptHandler.BuildTransaction is never invoked and
	// won't auto-add the operator's lock cell-dep. Add it explicitly here:
	// for omni-lock operators the proxy's lock-script verification needs the
	// omni-lock code cell on chain, otherwise CKB rejects with ScriptNotFound.
	isOmni := a.signer.Address().Script.CodeHash != a.deployment.DefaultLockScript.CodeHash
	if isOmni && len(a.deployment.OmniLockScriptDep) >= 2 {
		builder.AddCellDep(&a.deployment.OmniLockScriptDep[0])
		builder.AddCellDep(&a.deployment.OmniLockScriptDep[1])
	} else {
		builder.AddCellDep(&a.deployment.DefaultLockScriptDep)
	}
	builder.AddInput(&types.CellInput{PreviousOutput: outPoint})
	builder.AddOutput(&types.CellOutput{
		Capacity: reclaimed,
		Lock:     operatorLock,
		Type:     nil,
	}, nil)

	tx, err := builder.Build(a.signer.Contexts())
	if err != nil {
		return types.Hash{}, Deterministic(err)
	}
	txHash, err := a.transactor.SubmitTransaction(ctx, tx)
	if err != nil {
		return types.Hash{}, Retriable(err)
	}
	return txHash, nil
}

// newPerunTxBuilder constructs a PerunTransactionBuilder configured for the
// adapter's signer (sighash or omnilock). It is the single entry point for
// LP transactions that go through the Perun transaction pipeline.
func (a *Adapter) newPerunTxBuilder() (*transaction.PerunTransactionBuilder, error) {
	isOmni := a.signer.Address().Script.CodeHash != a.deployment.DefaultLockScript.CodeHash
	return transaction.NewPerunTransactionBuilderWithDeployment(
		a.rpcClient,
		a.deployment,
		nil,
		a.signer.Address(),
		isOmni,
	)
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

func buildLPScriptsFromDeployment(lpDeployment LPDeployment, poolID [32]byte) (types.Script, types.Script) {
	typeScript := types.Script{
		CodeHash: lpDeployment.TypeScriptCodeHash,
		HashType: lpDeployment.TypeScriptHashType,
		Args:     poolID[:],
	}
	tsHash := typeScript.Hash()
	lockScript := types.Script{
		CodeHash: lpDeployment.LockScriptCodeHash,
		HashType: lpDeployment.LockScriptHashType,
		Args:     tsHash[:],
	}
	return lockScript, typeScript
}
