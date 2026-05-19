package ckblp

import (
	"context"
	"fmt"
	"log"
	"sync"

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

	lpOrchestrationEnabled bool
	contribMu              sync.Mutex
	contribSeen            map[types.Hash]struct{}
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
		rpcClient:              rpcClient,
		signer:                 signer,
		transactor:             transactor,
		deployment:             deployment,
		lpDeployment:           lpDeployment,
		lpOrchestrationEnabled: isLPOrchestrationEnabled(),
		contribSeen:            make(map[types.Hash]struct{}),
	}
}

// BuildLPDepositTx builds and submits an LP deposit (creation/top-up) transaction.
func (a *Adapter) BuildLPDepositTx(ctx context.Context, lpCell LPCell) (string, error) {
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
	lpCell.OperatorLockHash = signerHash
	lpCell.ReservedCKB = 0
	lpCell.CumulativeFeesEarnedCKB = 0
	lpCell.Nonce = 0
	lpCell.Active = true

	data, err := EncodeLPCell(lpCell)
	if err != nil {
		return "", Deterministic(ErrInvalidLPCell)
	}

	fee := transaction.DefaultFeeShannon
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

// BuildFundChannelTx builds a FundChannelExtract transaction.
// It creates a proxy channel output (operator lock, no type script) with capacity
// equal to extract_ckb. The real Perun channel cell is NOT consumed — only the LP
// cell and an operator fee cell are inputs — so the perun-channel-lockscript never
// runs and no participant signature is required. The LP typescript validates all
// capacity accounting against the proxy channel output's data and capacity.
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
	inputLPData := cell.Cell.Data.Content
	inputLP, err := DecodeLPCell(inputLPData)
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

	// Input 0: LP cell; Input 1: operator fee cell.
	// The real Perun channel cell is intentionally excluded so the
	// perun-channel-lockscript never runs (it would require participant signatures).
	inputs := []*types.CellInput{
		{PreviousOutput: outPoint},
		{PreviousOutput: operatorCell.OutPoint},
	}
	for _, input := range inputs {
		builder.AddInput(input)
	}
	initLockWitnessPlaceholder(builder, 1)

	updatedLPOutput := &types.CellOutput{
		Capacity: updatedLPCap,
		Lock:     cell.Cell.Output.Lock,
		Type:     cell.Cell.Output.Type,
	}
	// Proxy channel: operator-locked plain cell carrying minimal ChannelStatus data.
	// The LP typescript reads channel_id from the data and verifies capacity delta.
	proxyChannelOutput := &types.CellOutput{
		Capacity: amount,
		Lock:     operatorCell.Output.Lock,
		Type:     nil,
	}
	operatorChangeOutput := &types.CellOutput{
		Capacity: operatorChange,
		Lock:     operatorCell.Output.Lock,
		Type:     nil,
	}

	builder.AddOutput(updatedLPOutput, updatedLPData)
	builder.AddOutput(proxyChannelOutput, buildProxyChannelData(channelHash))
	builder.AddOutput(operatorChangeOutput, []byte{})

	witness := EncodeFundChannelExtractWitness(FundChannelExtractWitness{
		ChannelID:      channelHash,
		ContributionID: contribHash,
		ExtractCKB:     amount,
	})
	if err := builder.SetWitness(0, types.WitnessTypeInputType, witness); err != nil {
		return types.Hash{}, Deterministic(ErrInvalidWitness)
	}

	addInputLockScriptGroup(builder, 1, operatorCell.Output.Lock)

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

// BuildSettleChannelInsertTx builds a SettleChannelInsert transaction (not implemented yet).
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
	}
	initLockWitnessPlaceholder(builder, 1)

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
		return types.Hash{}, Deterministic(ErrInvalidWitness)
	}

	addInputLockScriptGroup(builder, 1, operatorCell.Output.Lock)

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

func (a *Adapter) FundChannelWithLP(
	ctx context.Context,
	channelID string,
	lpCellID string,
	amount uint64,
) (types.Hash, error) {
	if !a.lpOrchestrationEnabled {
		return types.Hash{}, Deterministic(ErrFeatureDisabled)
	}
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

	contribHash := deriveContributionID("lp_fund", channelHash, amount, 0)
	if a.contributionSeen(contribHash) {
		log.Printf("lp_fund_noop channel_id=%s contribution_id=%s lp_cell_id=%s reason=duplicate", channelID, contribHash.String(), lpCellID)
		return types.Hash{}, Deterministic(ErrNoOp)
	}

	preLP, err := a.GetLPCell(ctx, lpCellID)
	if err != nil {
		return types.Hash{}, err
	}
	if preLP.Cell.ReservedCKB >= amount {
		log.Printf("lp_fund_noop channel_id=%s contribution_id=%s lp_cell_id=%s reason=reserved_already", channelID, contribHash.String(), lpCellID)
		return types.Hash{}, Deterministic(ErrNoOp)
	}

	if _, err := a.findChannelCellByID(ctx, channelHash); err != nil {
		if IsRetriable(err) {
			return types.Hash{}, err
		}
		return types.Hash{}, Deterministic(ErrChannelNotFound)
	}

	txHash, err := a.BuildFundChannelTx(ctx, channelID, lpCellID, amount, contribHash.String())
	if err != nil {
		return types.Hash{}, err
	}
	a.markContribution(contribHash)

	// The fund tx spends the old LP cell (input 0) and creates a new one at output 0.
	newLPCellID := fmt.Sprintf("%s:0", txHash.String())
	postLP, err := a.GetLPCell(ctx, newLPCellID)
	if err != nil {
		return txHash, err
	}
	if postLP.Cell.ReservedCKB != preLP.Cell.ReservedCKB+amount ||
		postLP.Cell.AvailableCKB != preLP.Cell.AvailableCKB-amount ||
		postLP.Cell.Nonce != preLP.Cell.Nonce+1 ||
		postLP.Capacity != preLP.Capacity-amount {
		return txHash, Retriable(ErrUnexpectedState)
	}

	log.Printf("lp_fund_orchestrated channel_id=%s contribution_id=%s lp_cell_id=%s funding_tx_hash=%s", channelID, contribHash.String(), lpCellID, txHash.String())
	return txHash, nil
}

func (a *Adapter) SettleChannelWithLP(
	ctx context.Context,
	channelID string,
	lpCellID string,
	principal uint64,
	feeCKB uint64,
	priceX64 uint128.Uint128,
) (types.Hash, error) {
	if !a.lpOrchestrationEnabled {
		return types.Hash{}, Deterministic(ErrFeatureDisabled)
	}
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

	channelCell, err := a.findChannelCellByID(ctx, channelHash)
	if err == nil && channelCell != nil {
		return types.Hash{}, Deterministic(ErrChannelStillLive)
	}
	if err != nil && IsRetriable(err) {
		return types.Hash{}, err
	}

	contribHash := deriveContributionID("lp_settle", channelHash, principal, feeCKB)
	if a.contributionSeen(contribHash) {
		log.Printf("lp_settle_noop channel_id=%s contribution_id=%s lp_cell_id=%s reason=duplicate", channelID, contribHash.String(), lpCellID)
		return types.Hash{}, Deterministic(ErrNoOp)
	}

	preLP, err := a.GetLPCell(ctx, lpCellID)
	if err != nil {
		return types.Hash{}, err
	}
	if preLP.Cell.ReservedCKB < principal {
		log.Printf("lp_settle_noop channel_id=%s contribution_id=%s lp_cell_id=%s reason=principal_already_returned", channelID, contribHash.String(), lpCellID)
		return types.Hash{}, Deterministic(ErrNoOp)
	}

	totalReturn := principal + feeCKB
	txHash, err := a.BuildSettleChannelInsertTx(ctx, channelID, contribHash.String(), lpCellID, principal, feeCKB, priceX64)
	if err != nil {
		return types.Hash{}, err
	}
	a.markContribution(contribHash)

	// The settle tx spends the old LP cell (input 0) and creates a new one at output 0.
	newLPCellID := fmt.Sprintf("%s:0", txHash.String())
	postLP, err := a.GetLPCell(ctx, newLPCellID)
	if err != nil {
		return txHash, err
	}
	if postLP.Cell.ReservedCKB != preLP.Cell.ReservedCKB-principal ||
		postLP.Cell.AvailableCKB != preLP.Cell.AvailableCKB+totalReturn ||
		postLP.Cell.CumulativeFeesEarnedCKB != preLP.Cell.CumulativeFeesEarnedCKB+feeCKB ||
		postLP.Cell.Nonce != preLP.Cell.Nonce+1 ||
		postLP.Capacity != preLP.Capacity+totalReturn {
		return txHash, Retriable(ErrUnexpectedState)
	}

	log.Printf("lp_settle_orchestrated channel_id=%s contribution_id=%s lp_cell_id=%s settle_tx_hash=%s", channelID, contribHash.String(), lpCellID, txHash.String())
	return txHash, nil
}

func (a *Adapter) contributionSeen(contribHash types.Hash) bool {
	a.contribMu.Lock()
	defer a.contribMu.Unlock()
	_, ok := a.contribSeen[contribHash]
	return ok
}

func (a *Adapter) markContribution(contribHash types.Hash) {
	a.contribMu.Lock()
	defer a.contribMu.Unlock()
	a.contribSeen[contribHash] = struct{}{}
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
	return nil, Deterministic(ErrChannelNotFound)
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
