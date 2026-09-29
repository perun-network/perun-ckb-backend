package transaction

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math/big"

	"github.com/nervosnetwork/ckb-sdk-go/v2/address"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector/builder"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector/handler"
	ckbtransaction "github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/perun-ckb-backend/backend"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
)

var zeroHash types.Hash = types.Hash{}

const (
	DefaultFeeShannon uint64 = 0.0001 * CKBYTE
	CKBYTE                   = 1 * 100_000_000
	MinCKBFeeAmount   uint64 = 1_000
)

// PerunTransactionBuilder is a transaction builder specifically for Perun
// channels. It allows creating transactions corresponding to Perun on-chain
// interactions. Following invariants hold:
//
//  1. It uses its iterator to gather cells for funding each action.
//  2. It will use the default lock script to secure the change cell.
type PerunTransactionBuilder struct {
	*builder.SimpleTransactionBuilder
	psh *PerunScriptHandler

	changeAddress address.Address

	cl LiveCellFetcher

	// iterators associated by the hash of the TypeScript of the cells that are
	// provided by the iterator.
	//
	// 0x0..0 = zero hash for native CKBytes.
	iterators map[types.Hash]collector.CellIterator
	// knownUDTs is a set of all UDTs that are known to the builder.
	knownUDTs map[types.Hash]types.Script

	scriptGroups []*ckbtransaction.ScriptGroup
	// scriptGroupmap maps the hash of a script to the index within the
	// scriptGroup.
	scriptGroupMap map[types.Hash]int

	ckbChangeCellIndex   int
	udtChangeCellIndices map[types.Hash]int

	feeShannon uint64
}

type LiveCellFetcher interface {
	// GetLiveCell returns the information about a cell by out_point if it is live.
	// If second with_data argument set to true, will return cell data and data_hash if it is live.
	GetLiveCell(ctx context.Context, outPoint *types.OutPoint, withData bool) (*types.CellWithStatus, error)
}

func NewPerunTransactionBuilder(client LiveCellFetcher, iterators map[types.Hash]collector.CellIterator, knownUDTs map[types.Hash]types.Script, psh *PerunScriptHandler, changeAddress address.Address, omni bool) (*PerunTransactionBuilder, error) {
	udtChangeCellIndices := make(map[types.Hash]int)
	for hash := range knownUDTs {
		udtChangeCellIndices[hash] = -1
	}
	var tb *builder.SimpleTransactionBuilder
	if omni {
		tb = NewSimpleTransactionBuilder(psh.omniLockScript.CodeHash, psh.omniLockScriptDep[1], omni)
	} else {
		tb = NewSimpleTransactionBuilder(psh.defaultLockScript.CodeHash, psh.defaultLockScriptDep, omni)
	}
	b := &PerunTransactionBuilder{
		SimpleTransactionBuilder: tb,
		psh:                      psh,
		iterators:                iterators,
		knownUDTs:                knownUDTs,
		cl:                       client,
		scriptGroups:             make([]*ckbtransaction.ScriptGroup, 0, 10),
		scriptGroupMap:           make(map[types.Hash]int),
		ckbChangeCellIndex:       -1,
		udtChangeCellIndices:     udtChangeCellIndices,
		changeAddress:            changeAddress,
		feeShannon:               DefaultFeeShannon,
	}

	return b, nil
}

func NewPerunTransactionBuilderWithDeployment(
	client LiveCellFetcher,
	deployment backend.Deployment,
	iterators map[types.Hash]collector.CellIterator,
	changeAddress address.Address,
	omni bool,
) (*PerunTransactionBuilder, error) {
	log.Println("Creating PerunTransactionBuilder with omni?", omni)
	psh := NewPerunScriptHandlerWithDeployment(deployment)
	var simpleBuilder *builder.SimpleTransactionBuilder
	if omni {
		simpleBuilder = NewSimpleTransactionBuilder(deployment.OmniLockScript.CodeHash, deployment.OmniLockScriptDep[1], omni)
	} else {
		simpleBuilder = NewSimpleTransactionBuilder(deployment.DefaultLockScript.CodeHash, deployment.DefaultLockScriptDep, omni)
	}

	udtChangeCellIndices := make(map[types.Hash]int)
	for hash := range deployment.SUDTs {
		udtChangeCellIndices[hash] = -1
	}
	b := &PerunTransactionBuilder{
		SimpleTransactionBuilder: simpleBuilder,
		psh:                      psh,
		iterators:                iterators,
		knownUDTs:                deployment.SUDTs,
		ckbChangeCellIndex:       -1,
		udtChangeCellIndices:     udtChangeCellIndices,
		cl:                       client,
		changeAddress:            changeAddress,
		scriptGroups:             make([]*ckbtransaction.ScriptGroup, 0, 10),
		scriptGroupMap:           make(map[types.Hash]int),
		feeShannon:               DefaultFeeShannon,
	}

	return b, nil
}

func NewSimpleTransactionBuilder(lockScriptCodeHash types.Hash, lockScriptDep types.CellDep, omni bool) *builder.SimpleTransactionBuilder {
	if omni {
		omniLockScriptHandler := &handler.OmnilockScriptHandler{
			CellDep:  &lockScriptDep,
			CodeHash: lockScriptCodeHash,
		}
		sb := builder.SimpleTransactionBuilder{}

		// TODO: Register SUDT script handlers here.
		sb.Register(omniLockScriptHandler)

		return &sb
	}
	secp256k1Blake160SighashAllScriptHandler := &handler.Secp256k1Blake160SighashAllScriptHandler{
		CellDep:  &lockScriptDep,
		CodeHash: lockScriptCodeHash,
	}
	sb := builder.SimpleTransactionBuilder{}

	// TODO: Register SUDT script handlers here.
	sb.Register(secp256k1Blake160SighashAllScriptHandler)

	return &sb
}

// Start tells the builder to construct a channel start transaction, s.t. the
// appropriate handler can be invoked by the logic of CKB-SDK.
func (ptb *PerunTransactionBuilder) Open(oi *OpenInfo) error {
	_, err := ptb.psh.buildOpenTransaction(ptb, nil, oi)
	return err
}

func (ptb *PerunTransactionBuilder) Abort(ai *AbortInfo) error {
	_, err := ptb.psh.buildAbortTransaction(ptb, nil, ai)
	return err
}

func (ptb *PerunTransactionBuilder) Fund(fi *FundInfo) error {
	_, err := ptb.psh.buildFundTransaction(ptb, nil, fi)
	return err
}

// FundExtractLP builds a PoolWitness::FundChannelExtract transaction. The
// referenced channel is recorded only via the proxy cell's ChannelStatus data;
// the real perun-channel-typescript is intentionally not invoked.
func (ptb *PerunTransactionBuilder) FundExtractLP(fi *LPFundExtractInfo) error {
	_, err := ptb.psh.buildFundExtractLPTransaction(ptb, nil, fi)
	return err
}

// SettleInsertLP builds a PoolWitness::SettleChannelInsert transaction. The
// caller must ensure the referenced channel is absent from inputs and outputs
// (the LP typescript rejects the tx otherwise).
func (ptb *PerunTransactionBuilder) SettleInsertLP(si *LPSettleInsertInfo) error {
	_, err := ptb.psh.buildSettleInsertLPTransaction(ptb, nil, si)
	return err
}

func (ptb *PerunTransactionBuilder) Dispute(di *DisputeInfo) error {
	_, err := ptb.psh.buildDisputeTransaction(ptb, nil, di)

	return err
}

func (ptb *PerunTransactionBuilder) DisputeVC(di *VcDisputeInfo) error {
	var err error
	if di.first {
		_, err = ptb.psh.buildFirstVCDisputeTransaction(ptb, nil, di)
	} else {
		_, err = ptb.psh.buildVCDisputeProgressTransaction(ptb, nil, di)
	}
	return err
}

func (ptb *PerunTransactionBuilder) MergeVC(vmi *VcMergeInfo) error {
	var err error
	_, err = ptb.psh.buildVCMergeTransaction(ptb, nil, vmi)
	return err
}

func (ptb *PerunTransactionBuilder) Close(ci *CloseInfo) error {
	_, err := ptb.psh.buildCloseTransaction(ptb, nil, ci)
	return err
}

func (ptb *PerunTransactionBuilder) ForceClose(fci *ForceCloseInfo) error {
	_, err := ptb.psh.buildForceCloseTransaction(ptb, nil, fci)
	return err
}

func (ptb *PerunTransactionBuilder) ForceCloseWithVC(fcvi *ForceCloseWithVCInfo) error {
	var err error
	if !fcvi.firstForceClose {
		_, err = ptb.psh.buildFirstForceCloseWithVCTransaction(ptb, nil, fcvi)
	} else {
		minInput, _, err := ptb.prepareMinCKBInput()
		if err != nil {
			return fmt.Errorf("preparing min CKB input: %w", err)
		}
		fcvi.MinCKBInput = minInput
		_, err = ptb.psh.buildSecondForceCloseWithVCTransaction(ptb, nil, fcvi)
		if err != nil {
			return fmt.Errorf("building second force close with VC transaction: %w", err)
		}
	}
	return err
}

func (ptb *PerunTransactionBuilder) Build(contexts ...interface{}) (*ckbtransaction.TransactionWithScriptGroups, error) {
	// Initialize all required script groups that we expect when building
	// transactions for Perun channels.
	if err := ptb.InitializeScriptGroups(); err != nil {
		return nil, fmt.Errorf("initializing script groups: %w", err)
	}

	// We balance the transaction in an initial step, because SUDTs required for
	// funding might not be accounted for in the inputs which might break
	// handlers adhering to the SUDT specification.
	if err := ptb.balanceTransaction(); err != nil {
		return nil, fmt.Errorf("balancing transaction: %w", err)
	}

	// The CKB-SDK expects at least a single context to be passed, otherwise its
	// logic is not executed. This context can as well be nil, which is why we
	// extend them here if necessary.
	if len(contexts) == 0 {
		contexts = append(contexts, nil)
	}

	// We process outputs first, since the semantics of the
	// PerunTransactionBuilder is that a user only has to specify what he wants
	// the output and inputs to look like, at the very least. Everything else is
	// supposed to be autocompleted by the builder. This means that the necessary
	// inputs to fund the required outputs will be completed by the builder, as
	// well es the cell containing the change.
	if err := ptb.ProcessOutputs(contexts...); err != nil {
		return nil, fmt.Errorf("processing outputs: %w", err)
	}

	// By the time we process the inputs, all custom requirements for the outputs
	// were handled, s.t. the inputs now contain the required funding. What might
	// be missing are requirements for the lock- & type-scripts of the inputs,
	// e.g. WitnessArgs set for sighash_all to unlock inputs.
	if err := ptb.ProcessInputs(contexts...); err != nil {
		return nil, fmt.Errorf("processing inputs: %w", err)
	}

	if err := ptb.HandleCKBFee(); err != nil {
		return nil, fmt.Errorf("handling CKB fee: %w", err)
	}

	tx := ptb.BuildTransaction()
	tx.ScriptGroups = ptb.CopyValidScriptGroups()
	return tx, nil
}

func (ptb *PerunTransactionBuilder) HandleCKBFee() error {
	if ptb.ckbChangeCellIndex == -1 {
		// No change cell needed: inputs already balance outputs+fee exactly.
		// completeCKBCapacity skips creating a change cell in that case.
		return nil
	}

	ckbChangeCell := ptb.Outputs[ptb.ckbChangeCellIndex]
	if ckbChangeCell.Capacity <= ptb.feeShannon {
		// TODO: Handle proper change cell deletion/update.
		return fmt.Errorf("insufficient CKB change cell capacity: %d < %d", ckbChangeCell.Capacity, ptb.feeShannon)
	}

	return nil
}

// CopyValidScriptGroups only returns the script groups that are valid.
//
// Our builder just createes all script groups preemtively, but some of them
// might not be necessary: e.g. a ScriptGroup of type "Lock" which has only
// references to output cells does not have to be handled, because that
// lockscript group is not evaluated when verifying the transaction.
func (ptb *PerunTransactionBuilder) CopyValidScriptGroups() []*ckbtransaction.ScriptGroup {
	validScriptGroups := make([]*ckbtransaction.ScriptGroup, 0, len(ptb.scriptGroups))
	for _, sg := range ptb.scriptGroups {
		switch sg.GroupType {
		case types.ScriptTypeLock:
			if len(sg.InputIndices) == 0 {
				// Skip lock script groups which are not referenced in inputs.
				continue
			}
		case types.ScriptTypeType:
			if len(sg.OutputIndices)+len(sg.InputIndices) == 0 {
				// Skip type script groups which are not referenced in inputs or
				// outputs.
				continue
			}
		}
		validScriptGroups = append(validScriptGroups, sg)
	}
	return validScriptGroups
}

func (ptb *PerunTransactionBuilder) InitializeScriptGroups() error {
	// Create all script groups that are required by the outputs specified by the
	// user of this transaction.
	for idx, output := range ptb.Outputs {
		ptb.initializeScript(output.Lock, types.ScriptTypeLock, idx, outputDir)
		ptb.initializeScript(output.Type, types.ScriptTypeType, idx, outputDir)
	}

	// Create all script groups that are required by the inputs specified by the
	// user of this transaction.
	for idx, input := range ptb.Inputs {
		inputCell, err := ptb.cl.GetLiveCell(context.Background(), input.PreviousOutput, false)
		if err != nil {
			return fmt.Errorf("getting live cell when resolving script groups: %w", err)
		}
		ptb.initializeScript(inputCell.Cell.Output.Lock, types.ScriptTypeLock, idx, inputDir)
		ptb.initializeScript(inputCell.Cell.Output.Type, types.ScriptTypeType, idx, inputDir)
	}

	// Make sure we have the minimum set of script groups defined for Perun
	// transactions.
	defaultLockScript := ptb.defaultLockScript()
	ptb.initializeScript(defaultLockScript, types.ScriptTypeLock, -1, noneDir)
	return nil
}

type InputOrOutput string

var (
	inputDir  InputOrOutput = "input"
	outputDir InputOrOutput = "output"
	noneDir   InputOrOutput = "none"
)

// initializeScript initializes the script group for the given script.
func (ptb *PerunTransactionBuilder) initializeScript(script *types.Script, scriptType types.ScriptType, ioIdx int, d InputOrOutput) {
	if script == nil {
		return
	}
	appendToScriptGroup := func(group *ckbtransaction.ScriptGroup, ioidx int, d InputOrOutput) {
		switch d {
		case inputDir:
			group.InputIndices = append(group.InputIndices, uint32(ioidx))
		case outputDir:
			group.OutputIndices = append(group.OutputIndices, uint32(ioidx))
		default:
		}
	}
	if g, ok := ptb.scriptGroupMap[script.Hash()]; ok {
		// Only append the idx if the scriptgroup already existed.
		appendToScriptGroup(ptb.scriptGroups[g], ioIdx, d)
		return
	}
	idx := ptb.AddScriptGroup(&ckbtransaction.ScriptGroup{
		Script:    script,
		GroupType: scriptType,
	})
	ptb.scriptGroupMap[script.Hash()] = idx
	appendToScriptGroup(ptb.scriptGroups[idx], ioIdx, d)
}

func (ptb *PerunTransactionBuilder) AddScriptGroup(group *ckbtransaction.ScriptGroup) int {
	ptb.scriptGroups = append(ptb.scriptGroups, group)
	return len(ptb.scriptGroups) - 1
}

func (ptb *PerunTransactionBuilder) ProcessOutputs(contexts ...interface{}) error {
	groups := ptb.getScriptGroups()

	for _, output := range ptb.Outputs {
		// Only process type scripts, because lock scripts are not evaluated in
		// their outputs.
		if err := ptb.processOutputTypeScript(output, groups, contexts...); err != nil {
			return fmt.Errorf("processing output type script: %w", err)
		}
	}
	return nil
}

func (ptb *PerunTransactionBuilder) getScriptGroups() []*ckbtransaction.ScriptGroup {
	return ptb.scriptGroups
}

type AssetInformation struct {
	// AssetInformation maps the hash of a type script to its amount.
	// The zero hash is used for native CKBytes.
	assetAmounts map[types.Hash]uint64

	knownUDTs map[types.Hash]types.Script
}

func NewAssetInformation(knownUDTs map[types.Hash]types.Script) *AssetInformation {
	return &AssetInformation{
		assetAmounts: make(map[types.Hash]uint64),
		knownUDTs:    knownUDTs,
	}
}

func (ai AssetInformation) Clone() AssetInformation {
	clonedAssets := make(map[types.Hash]uint64)
	for hash, amount := range ai.assetAmounts {
		clonedAssets[hash] = amount
	}
	return AssetInformation{
		assetAmounts: clonedAssets,
		knownUDTs:    ai.knownUDTs,
	}
}

func (ai *AssetInformation) MergeWithAssetInformation(other AssetInformation) {
	for hash, amount := range other.assetAmounts {
		ai.AddAssetAmount(hash, amount)
	}
}

func (ai *AssetInformation) AddAssetAmount(hash types.Hash, amount uint64) {
	ai.assetAmounts[hash] += amount
}

func (ai AssetInformation) EqualAssets(other AssetInformation) bool {
	if len(ai.assetAmounts) != len(other.assetAmounts) {
		return false
	}

	for hash, amount := range ai.assetAmounts {
		if other.assetAmounts[hash] != amount {
			return false
		}
	}
	return true
}

func (ai AssetInformation) AssetAmount(hash types.Hash) uint64 {
	return ai.assetAmounts[hash]
}

func (ai AssetInformation) CKBAmount() uint64 {
	return ai.assetAmounts[zeroHash]
}

// AddValuesFromOutput adds the capacity of the given cell together with the
// potential UDT values, if the given output cell is a known UDT.
// It holds that:
//
//	isKnownUDT(output) --> data != nil && len(data) == default UDT value size.
func (ai *AssetInformation) AddValuesFromOutput(output *types.CellOutput, data []byte) {
	ai.assetAmounts[zeroHash] += output.Capacity
	if ai.isUDT(output.Type) {
		ai.assetAmounts[output.Type.Hash()] += binary.LittleEndian.Uint64(data)
	}
}

// isUDT checks if the given type script is a known UDT.
func (ai AssetInformation) isUDT(typeScript *types.Script) bool {
	if typeScript == nil {
		return false
	}

	_, ok := ai.knownUDTs[typeScript.Hash()]
	return ok
}

func (ptb *PerunTransactionBuilder) ProcessInputs(contexts ...interface{}) error {
	groups := ptb.getScriptGroups()
	// Go over all inputs of the Perun transaction and call the contexts with
	// the appropriate script groups.
	for _, input := range ptb.Inputs {
		if err := ptb.processInputLockScript(input, groups, contexts...); err != nil {
			return fmt.Errorf("processing input lock script: %w", err)
		}

		if err := ptb.processInputTypeScript(input, groups, contexts...); err != nil {
			return fmt.Errorf("processing input type script: %w", err)
		}
	}
	return nil
}

// balanceTransaction balances the CKBytes and UDTs cells for the given
// transaction.
// NOTE: This method is NOT idempotent. It should only be called once all
// user-defined inputs and outputs have been added.
func (ptb *PerunTransactionBuilder) balanceTransaction() error {
	alreadyProvidedFunding := NewAssetInformation(ptb.knownUDTs)
	requiredFunding := NewAssetInformation(ptb.knownUDTs)
	requiredFunding.AddAssetAmount(zeroHash, ptb.feeShannon)

	// First go over the outputs and accumulate the required funding expected by
	// a user making some channel action.
	ptb.accumulateOutputs(requiredFunding)

	// We know the required funding, now we need to check and see what is already
	// provided.
	if err := ptb.accumulateInputs(alreadyProvidedFunding); err != nil {
		return fmt.Errorf("accumulating inputs: %w", err)
	}

	if alreadyProvidedFunding.EqualAssets(*requiredFunding) {
		// Everything is in order, the user built a transaction that is already
		// balanced.
		return nil
	}

	// Go over all required amounts check if the inputs already account for them.
	for assetHash, requiredAmount := range requiredFunding.assetAmounts {
		if assetHash == zeroHash {
			// We skip the native CKB asset, because if SUDTs have to be added to
			// balance this transaction, we might be able to use the occupied CKB
			// capacity of SUDT cells to fund the required CKB capacity.
			continue
		}
		alreadyProvidedAmount := alreadyProvidedFunding.assetAmounts[assetHash]
		if alreadyProvidedAmount == requiredAmount && requiredAmount > 0 {
			// The inputs already account for the required amount.
			continue
		}

		if alreadyProvidedAmount < requiredAmount {
			// We need more inputs to fund the required amount for the given UDT.
			// This might require a change cell for the UDT modifying the
			// required amount of CKB capacity.
			log.Println("Adding inputs for UDT", assetHash, "required amount:", requiredAmount, "already provided amount:", alreadyProvidedAmount)
			if err := ptb.addInputsAndChangeForFunding(assetHash, requiredAmount-alreadyProvidedAmount, requiredFunding, alreadyProvidedFunding); err != nil {
				return fmt.Errorf("adding inputs and change for UDT %x funding: %w", assetHash, err)
			}
			continue
		}

		if alreadyProvidedAmount > requiredAmount {
			// We provide more than required, add a change cell for the difference.
			// This uses the CKBBytes of the original "input" cell containing the
			// SUDTs.
			change := alreadyProvidedAmount - requiredAmount
			if err := ptb.addChangeAndAdjustRequiredFunding(assetHash, change, requiredFunding); err != nil {
				return fmt.Errorf("adding change cell for UDT %x: %w", assetHash, err)
			}
			continue
		}
	}

	// Do the final balancing.
	if err := ptb.completeCKBCapacity(requiredFunding, alreadyProvidedFunding); err != nil {
		return fmt.Errorf("final balancing of CKB capacity: %w", err)
	}

	return nil
}

func (ptb *PerunTransactionBuilder) accumulateOutputs(requiredFunding *AssetInformation) {
	outputs := ptb.Outputs
	outputData := ptb.OutputsData
	for idx, outputCell := range outputs {
		outputData := outputData[idx]
		requiredFunding.AddValuesFromOutput(outputCell, outputData)
	}
}

func (ptb *PerunTransactionBuilder) accumulateInputs(providedFunding *AssetInformation) error {
	for _, input := range ptb.Inputs {
		inputCell, inputCellData, err := ptb.resolveInputCell(input)
		if err != nil {
			return fmt.Errorf("resolving input cell: %w", err)
		}
		providedFunding.AddValuesFromOutput(inputCell, inputCellData)
	}
	return nil
}

// addChangeCell will add a change cell to the output of the given transaction
// using the preconfigured change address.
func (ptb *PerunTransactionBuilder) addOrUpdateChangeCell(assetHash types.Hash, amount uint64) error {
	if assetHash == zeroHash {
		return ptb.addOrUpdateCKBChangeCell(amount)
	}

	return ptb.addOrUpdateUDTChangeCell(assetHash, amount)
}

func (ptb *PerunTransactionBuilder) addOrUpdateCKBChangeCell(amount uint64) error {
	idx := ptb.ckbChangeCellIndex

	if idx != -1 {
		// We already added the change cell to the script group, just update the
		// cell capacity.
		ptb.Outputs[idx].Capacity = amount
		return nil
	}

	outputCell := &types.CellOutput{
		Capacity: amount,
		Lock:     ptb.defaultLockScript(),
		Type:     nil,
	}
	ptb.Outputs = append(ptb.Outputs, outputCell)
	ptb.OutputsData = append(ptb.OutputsData, []byte{})
	log.Println("ckbChangeCellIndex: ", len(ptb.Outputs)-1)
	ptb.ckbChangeCellIndex = len(ptb.Outputs) - 1

	lockScriptGroup, _ := ptb.scriptGroupsForHash(zeroHash)
	appendOutputToGroups(lockScriptGroup, nil, uint32(len(ptb.Outputs)-1))

	return nil
}

func (ptb *PerunTransactionBuilder) addOrUpdateUDTChangeCell(assetHash types.Hash, amount uint64) error {
	encodedAmount, err := molecule2.PackUint128(big.NewInt(0).SetUint64(amount))
	if err != nil {
		return fmt.Errorf("encoding amount for %x as Uint128: %w", assetHash, err)
	}
	data := encodedAmount.AsSlice()

	idx := ptb.udtChangeCellIndices[assetHash]
	if idx != -1 {
		// We already constructed and added the udt change cell to its script
		// group, only the change amount has to be adjusted.
		ptb.OutputsData[idx] = data
		return nil
	}

	outputCell := &types.CellOutput{
		Capacity: 0,
		Lock:     ptb.defaultLockScript(),
		Type:     nil,
	}

	udtScript, ok := ptb.knownUDTs[assetHash]
	if !ok {
		return fmt.Errorf("unknown UDT %x", assetHash)
	}
	outputCell.Type = &udtScript

	requiredCapacity := ptb.requiredCapacity(assetHash)
	outputCell.Capacity = requiredCapacity

	ptb.Outputs = append(ptb.Outputs, outputCell)
	ptb.OutputsData = append(ptb.OutputsData, data)

	changeIndex := len(ptb.Outputs) - 1
	lockScriptGroup, typeScriptGroup := ptb.scriptGroupsForHash(assetHash)
	appendOutputToGroups(lockScriptGroup, typeScriptGroup, uint32(changeIndex))

	ptb.udtChangeCellIndices[assetHash] = changeIndex
	return nil
}

func (ptb *PerunTransactionBuilder) scriptGroupsForHash(assetHash types.Hash) (*ckbtransaction.ScriptGroup, *ckbtransaction.ScriptGroup) {
	findInGroup := func(hash types.Hash) *ckbtransaction.ScriptGroup {
		groupIdx, ok := ptb.scriptGroupMap[hash]
		if !ok {
			return nil
		}
		return ptb.scriptGroups[groupIdx]
	}

	lockScriptGroup := findInGroup(ptb.defaultLockScript().Hash())
	var typeScriptGroup *ckbtransaction.ScriptGroup
	if assetHash != zeroHash {
		typeScriptGroup = findInGroup(assetHash)
	}
	return lockScriptGroup, typeScriptGroup
}

func (ptb *PerunTransactionBuilder) completeCKBCapacity(requiredFunding, alreadyProvidedFunding *AssetInformation) error {
	alreadyProvidedCKBAmount := alreadyProvidedFunding.assetAmounts[zeroHash]
	requiredCKBAmount := requiredFunding.assetAmounts[zeroHash]
	if alreadyProvidedCKBAmount >= (requiredCKBAmount + ptb.ckbChangeCellCapacity()) {
		// We provided more funding than required AND we can accommodate a CKB
		// change cell, set the difference as change.
		return ptb.addOrUpdateChangeCell(zeroHash, alreadyProvidedCKBAmount-requiredCKBAmount)
	}

	if alreadyProvidedCKBAmount < (requiredCKBAmount + ptb.ckbChangeCellCapacity()) {
		// We are missing CKB, complete funding by adding inputs from the CKB
		// iterator.
		return ptb.addInputsAndChangeForCKB(alreadyProvidedCKBAmount, requiredCKBAmount, requiredFunding, alreadyProvidedFunding)
	}
	return nil
}

func (ptb *PerunTransactionBuilder) addInputsAndChangeForCKB(alreadyProvidedCKBAmount, requestedCKBAmount uint64, requiredFunding, alreadyProvidedFunding *AssetInformation) error {
	ckbIter := ptb.iterators[zeroHash]

	requiredCKBAmount := requestedCKBAmount - alreadyProvidedCKBAmount
	amountAddedByInputs, err := ptb.addInputsForFunding(ckbIter, zeroHash, requiredCKBAmount)
	if err != nil {
		return fmt.Errorf("adding inputs for ckb funding: %w", err)
	}
	alreadyProvidedFunding.MergeWithAssetInformation(amountAddedByInputs)

	alreadyProvidedCKBAmount = alreadyProvidedFunding.CKBAmount()
	requiredCKBAmount = requiredFunding.CKBAmount()

	// We need to make sure, that we have enough CKB to pay for the storage
	// required for a CKB change cell.
	if alreadyProvidedCKBAmount < (requiredCKBAmount + ptb.ckbChangeCellCapacity()) {
		requiredCKBAmount = (requiredCKBAmount + ptb.ckbChangeCellCapacity()) - alreadyProvidedCKBAmount
		amountAddedByInputs, err := ptb.addInputsForFunding(ckbIter, zeroHash, requiredCKBAmount)
		if err != nil {
			return fmt.Errorf("adding inputs for ckb change funding: %w", err)
		}
		alreadyProvidedFunding.MergeWithAssetInformation(amountAddedByInputs)
	}

	// We now definitely have enough CKB to pay for the storage of all change
	// cells.
	requiredCKBAmount = requiredFunding.CKBAmount()
	alreadyProvidedCKBAmount = alreadyProvidedFunding.CKBAmount()
	change := alreadyProvidedCKBAmount - requiredCKBAmount
	if err := ptb.addChangeAndAdjustRequiredFunding(zeroHash, change, requiredFunding); err != nil {
		return fmt.Errorf("adding change cell for CKB: %w", err)
	}
	return nil
}

// addInputsAndChangeForFunding adds inputs and potentially change cells to the
// transaction to fund the required amount.
func (ptb *PerunTransactionBuilder) addInputsAndChangeForFunding(assetHash types.Hash, requestedAmount uint64, requiredFunding, alreadyProvidedFunding *AssetInformation) error {
	iter := ptb.iterators[assetHash]

	alreadyProvidedAssetAmount := alreadyProvidedFunding.AssetAmount(assetHash)
	if alreadyProvidedAssetAmount < requestedAmount || requestedAmount == uint64(0) {
		requiredAmount := requestedAmount - alreadyProvidedAssetAmount
		amountAddedByInputs, err := ptb.addInputsForFunding(iter, assetHash, requiredAmount)
		if err != nil {
			return fmt.Errorf("adding inputs for funding: %w", err)
		}
		alreadyProvidedFunding.MergeWithAssetInformation(amountAddedByInputs)
	}

	fundedAmount := alreadyProvidedFunding.Clone()
	fundedAssetValue := fundedAmount.AssetAmount(assetHash)
	if fundedAssetValue < requestedAmount {
		log.Println("not enough funds for asset", assetHash, "funded amount:", fundedAssetValue, "requested amount:", requestedAmount)
		return fmt.Errorf("not enough funds for asset: %#x", assetHash.Bytes())
	}

	// We have at least the required amount of funds for the asset available,
	// check if we need to add a change cell.
	if fundedAmount.AssetAmount(assetHash) == requestedAmount {
		// Perfect funding, no change cell needed.
		return nil
	}

	// We need a change cell, this will increase the required capacity of this TX
	// by the minimum cell capacity for this asset.
	change := fundedAmount.AssetAmount(assetHash) - requestedAmount
	if err := ptb.addChangeAndAdjustRequiredFunding(assetHash, change, requiredFunding); err != nil {
		return fmt.Errorf("adding change cell for asset %#x: %w", assetHash.Bytes(), err)
	}

	return nil
}

func (ptb *PerunTransactionBuilder) addInputsForFunding(iter collector.CellIterator, assetHash types.Hash, requiredAmount uint64) (AssetInformation, error) {
	if iter == nil {
		return AssetInformation{}, fmt.Errorf("no iterator for asset %#x registered", assetHash.Bytes())
	}

	if !iter.HasNext() {
		return AssetInformation{}, fmt.Errorf("empty iterator for asset %#x", assetHash.Bytes())
	}

	fundedAmount := NewAssetInformation(ptb.knownUDTs)

	// Fetch inputs from the correct iterator until we fullfill our needs.
	for iter.HasNext() {
		input := iter.Next()

		// We need to fetch the groups for each output cell, because they might
		// contain different lock-scripts & type-scripts.
		lockScriptGroup, typeScriptGroup := ptb.scriptGroupsForHash(assetHash)
		if lockScriptGroup == nil || (assetHash != zeroHash && typeScriptGroup == nil) {
			// It is mandatory for cells to have at least a lock script.
			// If we are looking at a UDT cell, we will also expect to find a
			// typeScriptGroup.
			return AssetInformation{}, fmt.Errorf("no script group for asset %x", assetHash)
		}

		ptb.Inputs = append(ptb.Inputs, &types.CellInput{
			Since:          0,
			PreviousOutput: input.OutPoint,
		})
		fundedAmount.AddValuesFromOutput(input.Output, input.OutputData)
		// Make sure to add a dummy witness for each input.
		ptb.Witnesses = append(ptb.Witnesses, []byte{})

		// Make sure to update the according script group.
		appendInputToGroups(lockScriptGroup, typeScriptGroup, uint32(len(ptb.Inputs)-1))
		if fundedAmount.AssetAmount(assetHash) >= requiredAmount {
			break
		}
	}

	return *fundedAmount, nil
}

func appendInputToGroups(lockScriptGroup, typeScriptGroup *ckbtransaction.ScriptGroup, idx uint32) {
	lockScriptGroup.InputIndices = append(lockScriptGroup.InputIndices, uint32(idx))
	if typeScriptGroup != nil {
		typeScriptGroup.InputIndices = append(typeScriptGroup.InputIndices, uint32(idx))
	}
}

func appendOutputToGroups(lockScriptGroup, typeScriptGroup *ckbtransaction.ScriptGroup, idx uint32) {
	lockScriptGroup.OutputIndices = append(lockScriptGroup.OutputIndices, uint32(idx))
	if typeScriptGroup != nil {
		typeScriptGroup.OutputIndices = append(typeScriptGroup.OutputIndices, uint32(idx))
	}
}

func (ptb *PerunTransactionBuilder) addChangeAndAdjustRequiredFunding(assetHash types.Hash, change uint64, requiredFunding *AssetInformation) error {
	// Adding the change cell will subtract the required CKB capacity for the UDT
	// from the already provided amount since we add everything to the provided
	// amount by default.
	cellCapacity := ptb.requiredCapacity(assetHash)
	requiredFunding.assetAmounts[zeroHash] += cellCapacity

	return ptb.addOrUpdateChangeCell(assetHash, change)
}

// requiredCapacity returns the required amount of CKBytes to accommodate the
// storage requirement for the given assets hash.
func (ptb *PerunTransactionBuilder) requiredCapacity(assetHash types.Hash) uint64 {
	var typeScript *types.Script
	if assetHash != zeroHash {
		udtScript := ptb.knownUDTs[assetHash]
		typeScript = &udtScript
	}
	cell := types.CellOutput{
		Capacity: 0,
		Lock:     ptb.defaultLockScript(),
		Type:     typeScript,
	}
	return CalculateCellCapacity(cell)
}

func CalculateCellCapacity(cell types.CellOutput) uint64 {
	if cell.Type == nil {
		return cell.OccupiedCapacity([]byte{})
	} else {
		uint128, _ := molecule2.PackUint128(big.NewInt(0)) // packing 0 cannot fail
		return cell.OccupiedCapacity(uint128.AsSlice())
	}
}

// resolveInput returns the types.CellOutput and the data of the given input if
// they exist.
func (ptb *PerunTransactionBuilder) resolveInputCell(input *types.CellInput) (*types.CellOutput, []byte, error) {
	// TODO: Use an injectable context.
	cell, err := ptb.cl.GetLiveCell(context.Background(), input.PreviousOutput, true)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving input cell: %w", err)
	}
	return cell.Cell.Output, cell.Cell.Data.Content, nil
}

func (ptb *PerunTransactionBuilder) processLockScript(output *types.CellOutput, groups []*ckbtransaction.ScriptGroup, contexts ...interface{}) error {
	lockScriptGroup, err := ptb.groupForScript(output.Lock)
	if err != nil {
		return fmt.Errorf("getting lock script group: %w", err)
	}
	return ptb.callHandlers(lockScriptGroup, contexts...)
}

func (ptb *PerunTransactionBuilder) processOutputTypeScript(output *types.CellOutput, groups []*ckbtransaction.ScriptGroup, contexts ...interface{}) error {
	group, err := ptb.groupForScript(output.Type)
	if err != nil {
		return fmt.Errorf("getting type script group: %w", err)
	}
	return ptb.callHandlers(group, contexts...)
}

func (ptb *PerunTransactionBuilder) callHandlers(group *ckbtransaction.ScriptGroup, contexts ...interface{}) error {
	if group == nil {
		// Skip if there is no type script which has to be handled.
		return nil
	}
	for _, handler := range ptb.ScriptHandlers {
		for _, ctx := range contexts {
			if _, err := handler.BuildTransaction(ptb, group, ctx); err != nil {
				return fmt.Errorf("building transaction with handler for %s script %x: %w",
					group.GroupType, group.Script.Hash(), err)
			}
		}
	}
	return nil
}

func (ptb *PerunTransactionBuilder) processInputLockScript(input *types.CellInput, groups []*ckbtransaction.ScriptGroup, contexts ...interface{}) error {
	cell, err := ptb.cl.GetLiveCell(context.Background(), input.PreviousOutput, false)
	if err != nil {
		return fmt.Errorf("resolving input cell: %w", err)
	}
	return ptb.processLockScript(cell.Cell.Output, groups, contexts...)
}

func (ptb *PerunTransactionBuilder) processInputTypeScript(input *types.CellInput, groups []*ckbtransaction.ScriptGroup, contexts ...interface{}) error {
	cell, err := ptb.cl.GetLiveCell(context.Background(), input.PreviousOutput, false)
	if err != nil {
		return fmt.Errorf("resolving input cell: %w", err)
	}
	return ptb.processOutputTypeScript(cell.Cell.Output, groups, contexts...)
}

func (ptb *PerunTransactionBuilder) groupForScript(script *types.Script) (*ckbtransaction.ScriptGroup, error) {
	if script == nil {
		return nil, nil
	}

	hash := script.Hash()
	groupIdx, ok := ptb.scriptGroupMap[hash]
	if !ok {
		return nil, fmt.Errorf("no script group for script %s \n %x", hash, ptb.scriptGroupMap)
	}
	return ptb.scriptGroups[groupIdx], nil
}

// Returns the default lock script by using the registered change address to
// fill in the lock script args field.
func (ptb *PerunTransactionBuilder) defaultLockScript() *types.Script {
	return ptb.changeAddress.Script
}

// Returns the capacity required to accommodate the change cell only holding
// CKBytes using the default lock-script.
func (ptb *PerunTransactionBuilder) ckbChangeCellCapacity() uint64 {
	return ptb.requiredCapacity(zeroHash)
}

func (ptb *PerunTransactionBuilder) prepareMinCKBInput() (*types.OutPoint, uint64, error) {
	iterator := ptb.iterators[zeroHash]
	if iterator == nil {
		return nil, 0, fmt.Errorf("no iterator for CKB registered")
	}
	if !iterator.HasNext() {
		return nil, 0, fmt.Errorf("no CKB input available")
	}
	input := iterator.Next()
	input.Output.Capacity = MinCKBFeeAmount

	return input.OutPoint, input.Output.Capacity, nil
}
