package ckblp

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	ckbtransaction "github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
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
// a caller-supplied operator lock hash. The signer pays for the cell and — unless
// lpCell.OwnerLockHash is pre-set — becomes the owner; the in-cell
// operator_lock_hash always comes from the argument. Used when an LP wants to
// delegate fund-extract/settle-insert to a separate operator account.
func (a *Adapter) BuildLPDepositTxWithOperator(ctx context.Context, lpCell LPCell, operatorLockHash types.Hash) (string, error) {
	tx, lpOutpointID, err := a.BuildLPDepositTxUnsigned(ctx, lpCell, a.signer.Address().Script, operatorLockHash)
	if err != nil {
		return "", err
	}
	if _, err := a.transactor.SubmitTransaction(ctx, tx); err != nil {
		return "", Retriable(err)
	}
	return lpOutpointID, nil
}

// BuildLPDepositTxUnsigned builds an LP deposit transaction without signing or
// submitting it. The caller is responsible for signing (typically via the LP
// owner's external wallet) and broadcasting via SubmitSignedTx. lpOutpointID is
// the predetermined "<txhash>:0" of the resulting LP cell; the CKB tx hash is
// computed over the body excluding witnesses, so signing does not change it.
//
// ownerScript selects the funding UTXOs; its hash is also written to the cell's
// OwnerLockHash field unless the caller pre-set one (payer/owner decoupling).
// The adapter's configured signer is not consulted on this path — ws-backend
// can build deposits for any LP whose lock script it has on the wire, without
// holding their key. ownerScript must include code_hash + hash_type + args
// (the lock hash alone is insufficient because cells are indexed by script,
// not by hash, and the omnilock-vs-secp256k1 builder dispatch needs code_hash).
func (a *Adapter) BuildLPDepositTxUnsigned(ctx context.Context, lpCell LPCell, ownerScript *types.Script, operatorLockHash types.Hash) (*ckbtransaction.TransactionWithScriptGroups, string, error) {
	if lpCell.AvailableCKB == 0 {
		return nil, "", Deterministic(ErrInvalidLPCellArg)
	}
	if ownerScript == nil {
		return nil, "", Deterministic(ErrInvalidLPCellArg)
	}
	// ownerScript selects the funding UTXOs; by default its hash also becomes
	// the cell owner. A caller may pre-set OwnerLockHash to decouple payer from
	// owner (the devnet seeder pays from bob's default lock while the owner is
	// his omnilock identity — the hash LP position queries and withdrawals are
	// keyed by).
	if lpCell.OwnerLockHash == ([32]byte{}) {
		lpCell.OwnerLockHash = ownerScript.Hash()
	}
	lpCell.OperatorLockHash = operatorLockHash
	lpCell.ReservedCKB = 0
	lpCell.CumulativeFeesEarnedCKB = 0
	lpCell.Nonce = 0
	lpCell.Active = true
	// The verifier rejects cell creation with a zero eth_beneficiary
	// (LPMissingBeneficiary); fail fast here instead of on-chain.
	if lpCell.EthBeneficiary == ([20]byte{}) {
		return nil, "", Deterministic(ErrInvalidLPCellArg)
	}

	data, err := EncodeLPCell(lpCell)
	if err != nil {
		return nil, "", Deterministic(ErrInvalidLPCell)
	}

	fee := transaction.DefaultFeeShannon
	if lpCell.AvailableCKB < MinLPCellOccupiedShannons {
		return nil, "", Deterministic(ErrInvalidLPCellArg)
	}
	// The change output must itself satisfy CKB's occupied-capacity floor, so
	// the inputs have to cover deposit + fee + a valid change cell.
	minChange := types.CellOutput{Lock: ownerScript}.OccupiedCapacity([]byte{})
	fundingCells, fundingTotal, err := a.selectCellsByLockForAmount(ctx, ownerScript, lpCell.AvailableCKB+fee+minChange)
	if err != nil {
		return nil, "", err
	}
	change := fundingTotal - lpCell.AvailableCKB - fee

	lockScript, typeScript := buildLPScriptsFromDeployment(a.lpDeployment, lpCell.PoolID)

	builder := transaction.NewSimpleTransactionBuilder(ownerScript.CodeHash, a.deployment.DefaultLockScriptDep, false)
	if ownerScript.CodeHash != a.deployment.DefaultLockScript.CodeHash && len(a.deployment.OmniLockScriptDep) >= 2 {
		builder = transaction.NewSimpleTransactionBuilder(a.deployment.OmniLockScript.CodeHash, a.deployment.OmniLockScriptDep[1], true)
		builder.AddCellDep(&a.deployment.OmniLockScriptDep[0])
		builder.AddCellDep(&a.deployment.OmniLockScriptDep[1])
	} else {
		builder.AddCellDep(&a.deployment.DefaultLockScriptDep)
	}
	builder.AddCellDep(&a.lpDeployment.TypeScriptDep)
	builder.AddCellDep(&a.lpDeployment.LockScriptDep)

	inputIndices := make([]uint32, len(fundingCells))
	for i, fundingCell := range fundingCells {
		builder.AddInput(&types.CellInput{PreviousOutput: fundingCell.OutPoint})
		inputIndices[i] = uint32(i)
	}
	initLockWitnessPlaceholder(builder, 0)
	if err := builder.SetWitness(0, types.WitnessTypeInputType, EncodeLPDepositWitness()); err != nil {
		return nil, "", Deterministic(ErrInvalidWitness)
	}

	builder.AddOutput(&types.CellOutput{
		Capacity: lpCell.AvailableCKB,
		Lock:     &lockScript,
		Type:     &typeScript,
	}, data)

	builder.AddOutput(&types.CellOutput{
		Capacity: change,
		Lock:     fundingCells[0].Output.Lock,
		Type:     nil,
	}, []byte{})

	addInputLockScriptGroup(builder, fundingCells[0].Output.Lock, inputIndices...)

	tx, err := builder.Build()
	if err != nil {
		return nil, "", Deterministic(err)
	}
	return tx, fmt.Sprintf("%s:%d", tx.TxView.ComputeHash().String(), 0), nil
}

// BuildLPWithdrawTx builds and submits an LP withdraw transaction.
func (a *Adapter) BuildLPWithdrawTx(ctx context.Context, lpCellID string, ckbOut uint64) (types.Hash, error) {
	tx, err := a.BuildLPWithdrawTxUnsigned(ctx, lpCellID, ckbOut, a.signer.Address().Script)
	if err != nil {
		return types.Hash{}, err
	}
	txHash, err := a.transactor.SubmitTransaction(ctx, tx)
	if err != nil {
		return types.Hash{}, Retriable(err)
	}
	return txHash, nil
}

// BuildLPWithdrawTxUnsigned builds an LP withdraw transaction without signing
// or submitting it. The caller signs externally (LP owner's wallet) and
// broadcasts via SubmitSignedTx.
//
// ownerScript identifies the LP owner: it selects the funding/change UTXO and
// drives the omnilock-vs-secp256k1 builder dispatch. Its hash must equal the
// LP cell's existing OwnerLockHash for the LP typescript to accept the tx;
// this is enforced on-chain, not here, so a mismatch surfaces as a submit
// error rather than a build error.
func (a *Adapter) BuildLPWithdrawTxUnsigned(ctx context.Context, lpCellID string, ckbOut uint64, ownerScript *types.Script) (*ckbtransaction.TransactionWithScriptGroups, error) {
	if ckbOut == 0 {
		return nil, Deterministic(ErrInvalidLPCellArg)
	}
	if ownerScript == nil {
		return nil, Deterministic(ErrInvalidLPCellArg)
	}
	outPoint, err := parseOutPoint(lpCellID)
	if err != nil {
		return nil, Deterministic(err)
	}
	cell, err := a.rpcClient.GetLiveCell(ctx, outPoint, true)
	if err != nil {
		return nil, Retriable(err)
	}
	if cell == nil || cell.Cell == nil || cell.Cell.Output == nil || cell.Cell.Data == nil {
		return nil, Deterministic(ErrInvalidLPCell)
	}
	inputLP, err := DecodeLPCell(cell.Cell.Data.Content)
	if err != nil {
		return nil, Deterministic(ErrInvalidLPCell)
	}

	fee := transaction.DefaultFeeShannon
	if ckbOut+fee > inputLP.AvailableCKB {
		return nil, Deterministic(ErrInvalidLPCellArg)
	}
	updatedLPCap, err := updatedLPCellCapacity(cell.Cell.Output.Capacity, ckbOut+fee)
	if err != nil {
		return nil, err
	}

	updatedLP := inputLP
	updatedLP.AvailableCKB -= ckbOut + fee
	updatedLP.Nonce += 1
	updatedLPData, err := EncodeLPCell(updatedLP)
	if err != nil {
		return nil, Deterministic(ErrInvalidLPCell)
	}

	ownerCell, err := a.selectLargestCellByLock(ctx, ownerScript)
	if err != nil {
		return nil, err
	}
	if ownerCell.Output == nil {
		return nil, Deterministic(ErrInvalidLPCell)
	}
	change := ownerCell.Output.Capacity + ckbOut

	builder := transaction.NewSimpleTransactionBuilder(ownerScript.CodeHash, a.deployment.DefaultLockScriptDep, false)
	if ownerScript.CodeHash != a.deployment.DefaultLockScript.CodeHash && len(a.deployment.OmniLockScriptDep) >= 2 {
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
		return nil, Deterministic(ErrInvalidWitness)
	}

	builder.AddOutput(&types.CellOutput{
		Capacity: updatedLPCap,
		Lock:     cell.Cell.Output.Lock,
		Type:     cell.Cell.Output.Type,
	}, updatedLPData)

	builder.AddOutput(&types.CellOutput{
		Capacity: change,
		Lock:     ownerCell.Output.Lock,
		Type:     nil,
	}, []byte{})

	addInputLockScriptGroup(builder, ownerCell.Output.Lock, 1)

	tx, err := builder.Build()
	if err != nil {
		return nil, Deterministic(err)
	}
	return tx, nil
}

// BuildLPTopUpTx builds and submits an owner-signed top-up of an existing LP
// cell: one LP input, one LP output with AvailableCKB grown by ckbIn (funded
// from the adapter signer's own cells) and the nonce incremented; every other
// field — owner, operator, policy, eth_beneficiary — is carried over unchanged,
// as the LP typescript requires. The signer must be the cell's owner (the
// contract's top-up branch verifies the owner's signature). Returns the LP
// cell's new outpoint "<txhash>:0". Used by the hub to recycle CKB received
// from CKB→ETH swaps back into its operator-owned inventory cell.
func (a *Adapter) BuildLPTopUpTx(ctx context.Context, lpCellID string, ckbIn uint64) (string, error) {
	if ckbIn == 0 {
		return "", Deterministic(ErrInvalidLPCellArg)
	}
	ownerScript := a.signer.Address().Script
	outPoint, err := parseOutPoint(lpCellID)
	if err != nil {
		return "", Deterministic(err)
	}
	cell, err := a.rpcClient.GetLiveCell(ctx, outPoint, true)
	if err != nil {
		return "", Retriable(err)
	}
	if cell == nil || cell.Cell == nil || cell.Cell.Output == nil || cell.Cell.Data == nil {
		return "", Deterministic(ErrInvalidLPCell)
	}
	inputLP, err := DecodeLPCell(cell.Cell.Data.Content)
	if err != nil {
		return "", Deterministic(ErrInvalidLPCell)
	}
	if inputLP.OwnerLockHash != [32]byte(ownerScript.Hash()) {
		return "", Deterministic(ErrInvalidLPCellArg)
	}

	updatedLP := inputLP
	updatedLP.AvailableCKB += ckbIn
	if updatedLP.AvailableCKB < inputLP.AvailableCKB {
		return "", Deterministic(ErrInvalidLPCellArg)
	}
	updatedLP.Nonce += 1
	updatedLPData, err := EncodeLPCell(updatedLP)
	if err != nil {
		return "", Deterministic(ErrInvalidLPCell)
	}

	fee := transaction.DefaultFeeShannon
	minChange := types.CellOutput{Lock: ownerScript}.OccupiedCapacity([]byte{})
	fundingCells, fundingTotal, err := a.selectCellsByLockForAmount(ctx, ownerScript, ckbIn+fee+minChange)
	if err != nil {
		return "", err
	}
	change := fundingTotal - ckbIn - fee

	builder := transaction.NewSimpleTransactionBuilder(ownerScript.CodeHash, a.deployment.DefaultLockScriptDep, false)
	if ownerScript.CodeHash != a.deployment.DefaultLockScript.CodeHash && len(a.deployment.OmniLockScriptDep) >= 2 {
		builder = transaction.NewSimpleTransactionBuilder(a.deployment.OmniLockScript.CodeHash, a.deployment.OmniLockScriptDep[1], true)
		builder.AddCellDep(&a.deployment.OmniLockScriptDep[0])
		builder.AddCellDep(&a.deployment.OmniLockScriptDep[1])
	} else {
		builder.AddCellDep(&a.deployment.DefaultLockScriptDep)
	}
	builder.AddCellDep(&a.lpDeployment.TypeScriptDep)
	builder.AddCellDep(&a.lpDeployment.LockScriptDep)

	builder.AddInput(&types.CellInput{PreviousOutput: outPoint})
	fundingIndices := make([]uint32, len(fundingCells))
	for i, fundingCell := range fundingCells {
		builder.AddInput(&types.CellInput{PreviousOutput: fundingCell.OutPoint})
		fundingIndices[i] = uint32(i + 1)
	}
	initLockWitnessPlaceholder(builder, 1)
	if err := builder.SetWitness(0, types.WitnessTypeInputType, EncodeLPDepositWitness()); err != nil {
		return "", Deterministic(ErrInvalidWitness)
	}

	builder.AddOutput(&types.CellOutput{
		Capacity: cell.Cell.Output.Capacity + ckbIn,
		Lock:     cell.Cell.Output.Lock,
		Type:     cell.Cell.Output.Type,
	}, updatedLPData)

	builder.AddOutput(&types.CellOutput{
		Capacity: change,
		Lock:     ownerScript,
		Type:     nil,
	}, []byte{})

	addInputLockScriptGroup(builder, ownerScript, fundingIndices...)

	tx, err := builder.Build()
	if err != nil {
		return "", Deterministic(err)
	}
	if _, err := a.transactor.SubmitTransaction(ctx, tx); err != nil {
		return "", Retriable(err)
	}
	return fmt.Sprintf("%s:%d", tx.TxView.ComputeHash().String(), 0), nil
}

// SubmitSignedTx broadcasts a transaction that has already been signed (for
// example by the LP owner's external wallet) and waits for commitment. The
// adapter's internal transactor must be *backend.RPCTransactor; otherwise
// ErrUnsupportedTransactor is returned.
func (a *Adapter) SubmitSignedTx(ctx context.Context, signed *types.Transaction) (types.Hash, error) {
	rpcTx, ok := a.transactor.(*backend.RPCTransactor)
	if !ok {
		return types.Hash{}, Deterministic(ErrUnsupportedTransactor)
	}
	txHash, err := rpcTx.SubmitSignedTransaction(ctx, signed)
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
	updatedLPCap, err := updatedLPCellCapacity(cell.Cell.Output.Capacity, amount)
	if err != nil {
		return types.Hash{}, err
	}

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
	tradedCKB uint64,
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
	// Principal (the extract's returned remainder) and tradedCKB (the portion
	// sold to the peer) together release the channel's full reservation.
	if tradedCKB > math.MaxUint64-principal {
		return types.Hash{}, Deterministic(ErrInvalidLPCellArg)
	}
	reservedRelease := principal + tradedCKB
	if reservedRelease > inputLP.ReservedCKB {
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
	// Mirror the typescript's fee-policy rule: the policy fee is computed on
	// the traded notional, and Enforce{Max,Min}Fee bound feeCKB against it.
	if inputLP.Policy.FeeRateBps != 0 &&
		(inputLP.Policy.PolicyFlags&(policyFlagEnforceMaxFee|policyFlagEnforceMinFee)) != 0 {
		policyFee := uint128.From64(tradedCKB).Mul64(uint64(inputLP.Policy.FeeRateBps)).Div64(10_000)
		fee := uint128.From64(feeCKB)
		if (inputLP.Policy.PolicyFlags&policyFlagEnforceMaxFee) != 0 && fee.Cmp(policyFee) > 0 {
			return types.Hash{}, Deterministic(ErrInvalidLPCellArg)
		}
		if (inputLP.Policy.PolicyFlags&policyFlagEnforceMinFee) != 0 && fee.Cmp(policyFee) < 0 {
			return types.Hash{}, Deterministic(ErrInvalidLPCellArg)
		}
	}

	operatorCell, err := a.selectLargestOperatorCell(ctx)
	if err != nil {
		return types.Hash{}, err
	}
	updatedLP := inputLP
	updatedLP.AvailableCKB += principal + feeCKB
	updatedLP.ReservedCKB -= reservedRelease
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
		TradedCKB:         tradedCKB,
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
	return a.selectLargestCellByLock(ctx, a.signer.Address().Script)
}

// selectLargestCellByLock picks the largest plain (no type script, non-LP)
// UTXO locked by the given script. Used by the non-custodial unsigned-build
// path to fund the deposit/withdraw from any caller's account without
// requiring the adapter's configured signer to be the owner.
func (a *Adapter) selectLargestCellByLock(ctx context.Context, lockScript *types.Script) (*indexer.LiveCell, error) {
	candidates, err := a.plainCellsByLock(ctx, lockScript)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, Deterministic(ErrInsufficientOperatorFunds)
	}
	return candidates[0], nil
}

// selectCellsByLockForAmount gathers plain (no type script, non-LP) UTXOs
// locked by the given script, largest first, until their combined capacity
// reaches target. Accounts fragment into change cells over time, so funding
// must be able to span inputs rather than require one cell to cover it all.
func (a *Adapter) selectCellsByLockForAmount(ctx context.Context, lockScript *types.Script, target uint64) ([]*indexer.LiveCell, uint64, error) {
	candidates, err := a.plainCellsByLock(ctx, lockScript)
	if err != nil {
		return nil, 0, err
	}
	var selected []*indexer.LiveCell
	var total uint64
	for _, cell := range candidates {
		selected = append(selected, cell)
		total += cell.Output.Capacity
		if total >= target {
			return selected, total, nil
		}
	}
	return nil, 0, Deterministic(ErrInsufficientOperatorFunds)
}

// plainCellsByLock returns the live plain (no type script, non-LP) UTXOs
// locked by the given script, sorted by capacity descending.
func (a *Adapter) plainCellsByLock(ctx context.Context, lockScript *types.Script) ([]*indexer.LiveCell, error) {
	if lockScript == nil {
		return nil, Deterministic(ErrInvalidLPCellArg)
	}
	searchKey := &indexer.SearchKey{
		Script:           lockScript,
		ScriptType:       types.ScriptTypeLock,
		ScriptSearchMode: types.ScriptSearchModeExact,
		WithData:         true,
	}
	resp, err := a.rpcClient.GetCells(ctx, searchKey, indexer.SearchOrderDesc, client.SearchIndexerLimit, "")
	if err != nil {
		return nil, Retriable(err)
	}
	candidates := make([]*indexer.LiveCell, 0, len(resp.Objects))
	for _, cell := range resp.Objects {
		if cell.Output == nil || cell.Output.Type != nil {
			continue
		}
		if IsLPCell(cell.OutputData) {
			continue
		}
		candidates = append(candidates, cell)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Output.Capacity > candidates[j].Output.Capacity
	})
	return candidates, nil
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
