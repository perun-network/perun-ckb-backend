package transaction

import (
	"context"
	"encoding/binary"
	"fmt"
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
}

type LiveCellFetcher interface {
	// GetLiveCell returns the information about a cell by out_point if it is live.
	// If second with_data argument set to true, will return cell data and data_hash if it is live.
	GetLiveCell(ctx context.Context, outPoint *types.OutPoint, withData bool) (*types.CellWithStatus, error)
}

func NewPerunTransactionBuilder(client LiveCellFetcher, iterators map[types.Hash]collector.CellIterator, knownUDTs map[types.Hash]types.Script, psh *PerunScriptHandler) (*PerunTransactionBuilder, error) {
	b := &PerunTransactionBuilder{
		SimpleTransactionBuilder: NewSimpleTransactionBuilder(psh.defaultLockScript.CodeHash, psh.defaultLockScriptDep),
		psh:                      psh,
		iterators:                iterators,
		knownUDTs:                knownUDTs,
		cl:                       client,
		scriptGroups:             make([]*ckbtransaction.ScriptGroup, 0, 10),
		scriptGroupMap:           make(map[types.Hash]int),
	}

	return b, nil
}

func NewPerunTransactionBuilderWithDeployment(
	client LiveCellFetcher,
	deployment backend.Deployment,
	iterators map[types.Hash]collector.CellIterator,
) (*PerunTransactionBuilder, error) {
	psh := NewPerunScriptHandlerWithDeployment(deployment)
	simpleBuilder := NewSimpleTransactionBuilder(deployment.DefaultLockScript.CodeHash, deployment.DefaultLockScriptDep)

	b := &PerunTransactionBuilder{
		SimpleTransactionBuilder: simpleBuilder,
		psh:                      psh,
		iterators:                iterators,
		knownUDTs:                deployment.SUDTs,
		cl:                       client,
		scriptGroups:             make([]*ckbtransaction.ScriptGroup, 0, 10),
		scriptGroupMap:           make(map[types.Hash]int),
	}

	return b, nil
}

func NewSimpleTransactionBuilder(lockScriptCodeHash types.Hash, lockScriptDep types.CellDep) *builder.SimpleTransactionBuilder {
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

func (ptb *PerunTransactionBuilder) Build(contexts ...interface{}) (*ckbtransaction.TransactionWithScriptGroups, error) {
	// Initialize all required script groups that we expect when building
	// transactions for Perun channels.
	if err := ptb.initializeScriptGroups(); err != nil {
		return nil, fmt.Errorf("initializing script groups: %w", err)
	}

	// We balance the transaction in an initial step, because SUDTs required for
	// funding might not be accounted for in the inputs which might break
	// handlers adhering to the SUDT specification.
	if err := ptb.balanceTransaction(); err != nil {
		return nil, fmt.Errorf("balancing transaction: %w", err)
	}

	// We process outputs first, since the semantics of the
	// PerunTransactionBuilder is that a user only has to specify what he wants
	// the output and inputs to look like, at the very least. Everything else is
	// supposed to be autocompleted by the builder. This means that the necessary
	// inputs to fund the required outputs will be completed by the builder, as
	// well es the cell containing the change.
	if err := ptb.processOutputs(contexts...); err != nil {
		return nil, fmt.Errorf("processing outputs: %w", err)
	}

	// By the time we process the inputs, all custom requirements for the outputs
	// were handled, s.t. the inputs now contain the required funding. What might
	// be missing are requirements for the lock- & type-scripts of the inputs,
	// e.g. WitnessArgs set for sighash_all to unlock inputs.
	if err := ptb.processInputs(contexts...); err != nil {
		return nil, fmt.Errorf("processing inputs: %w", err)
	}

	tx := ptb.BuildTransaction()
	tx.ScriptGroups = ptb.scriptGroups
	return tx, nil
}

func (ptb *PerunTransactionBuilder) initializeScriptGroups() error {
	// Create all script groups that are required by the outputs specified by the
	// user of this transaction.
	for _, output := range ptb.SimpleTransactionBuilder.Outputs {
		ptb.initializeScript(output.Lock, types.ScriptTypeLock)
		ptb.initializeScript(output.Type, types.ScriptTypeType)
	}

	// Create all script groups that are required by the inputs specified by the
	// user of this transaction.
	for _, input := range ptb.SimpleTransactionBuilder.Inputs {
		inputCell, err := ptb.cl.GetLiveCell(context.Background(), input.PreviousOutput, false)
		if err != nil {
			return fmt.Errorf("getting live cell when resolving script groups: %w", err)
		}
		ptb.initializeScript(inputCell.Cell.Output.Lock, types.ScriptTypeLock)
		ptb.initializeScript(inputCell.Cell.Output.Type, types.ScriptTypeType)
	}

	// Make sure we have script the minimum set of script groups defined for
	// Perun transactions.
	defaultLockScript := ptb.defaultLockScript()
	ptb.initializeScript(defaultLockScript, types.ScriptTypeLock)
	return nil
}

func (ptb *PerunTransactionBuilder) initializeScript(script *types.Script, scriptType types.ScriptType) {
	if script == nil {
		return
	}
	if _, ok := ptb.scriptGroupMap[script.Hash()]; ok {
		// Return if we already added this script group.
		return
	}
	idx := ptb.AddScriptGroup(&ckbtransaction.ScriptGroup{
		Script:    script,
		GroupType: scriptType,
	})
	ptb.scriptGroupMap[script.Hash()] = idx
}

func (ptb *PerunTransactionBuilder) AddScriptGroup(group *ckbtransaction.ScriptGroup) int {
	ptb.scriptGroups = append(ptb.scriptGroups, group)
	return len(ptb.scriptGroups) - 1
}

func (ptb *PerunTransactionBuilder) processOutputs(contexts ...interface{}) error {
	groups := ptb.getScriptGroups()

	for _, output := range ptb.SimpleTransactionBuilder.Outputs {
		if err := ptb.processOutputLockScript(output, groups, contexts...); err != nil {
			return fmt.Errorf("processing output lock script: %w", err)
		}

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

func (ai AssetInformation) UDTAmount(hash types.Hash) uint64 {
	return ai.assetAmounts[hash]
}

func (ai AssetInformation) CKBAmount() uint64 {
	return ai.assetAmounts[types.Hash{}]
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

func (ptb *PerunTransactionBuilder) processInputs(contexts ...interface{}) error {
	groups := ptb.getScriptGroups()
	// Go over all inputs of the Perun transaction and call the contexts with
	// the appropriate script groups.
	for _, input := range ptb.SimpleTransactionBuilder.Inputs {
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
func (ptb *PerunTransactionBuilder) balanceTransaction() error {
	alreadyProvidedFunding := NewAssetInformation(ptb.knownUDTs)
	requiredFunding := NewAssetInformation(ptb.knownUDTs)

	// First go over the outputs and accumulate the required funding expected by
	// a user making some channel action.
	for idx, outputCell := range ptb.SimpleTransactionBuilder.Outputs {
		outputData := ptb.SimpleTransactionBuilder.OutputsData[idx]
		requiredFunding.AddValuesFromOutput(outputCell, outputData)
	}

	// We know the required funding, now we need to check and see what is already
	// provided.
	for _, input := range ptb.SimpleTransactionBuilder.Inputs {
		inputCell, inputCellData, err := ptb.resolveInputCell(input)
		if err != nil {
			return fmt.Errorf("resolving input cell: %w", err)
		}
		alreadyProvidedFunding.AddValuesFromOutput(inputCell, inputCellData)
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
		if alreadyProvidedAmount == requiredAmount {
			// The inputs already account for the required amount.
			continue
		}

		if alreadyProvidedAmount < requiredAmount {
			// We need more inputs to fund the required amount for the given UDT.
			// This might require a change cell for the UDT modifying the
			// provided/required amount of CKB capacity.
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
			if err := ptb.addChangeAndDeductCellCapacity(assetHash, change, alreadyProvidedFunding); err != nil {
				return fmt.Errorf("adding change cell for UDT %x: %w", assetHash, err)
			}
			continue
		}
	}

	// Do the final balancing.
	if err := ptb.completeCKBCapacity(alreadyProvidedFunding.assetAmounts[zeroHash] - requiredFunding.assetAmounts[zeroHash]); err != nil {
		return fmt.Errorf("final balancing of CKB capacity: %w", err)
	}

	return nil
}

// addChangeCell will add a change cell to the output of the given transaction
// using the preconfigured change address
func (ptb *PerunTransactionBuilder) addChangeCell(assetHash types.Hash, amount uint64) error {
	dataForCell := func(cell *types.CellOutput) ([]byte, error) {
		// []byte{} for CKBytes, encoded amount as Uint128 for UDTs.
		if cell.Type == nil {
			return []byte{}, nil
		}

		encodedAmount, err := molecule2.PackUint128(big.NewInt(0).SetUint64(amount))
		if err != nil {
			return nil, fmt.Errorf("encoding amount for %x as Uint128: %w", assetHash, err)
		}
		return encodedAmount.AsSlice(), nil
	}

	outputCell := &types.CellOutput{
		Capacity: 0,
		Lock:     ptb.defaultLockScript(),
		Type:     nil,
	}

	if assetHash != zeroHash {
		udtScript, ok := ptb.knownUDTs[assetHash]
		if !ok {
			return fmt.Errorf("unknown UDT %x", assetHash)
		}
		outputCell.Type = &udtScript
	}

	ptb.SimpleTransactionBuilder.Outputs = append(ptb.SimpleTransactionBuilder.Outputs, outputCell)

	data, err := dataForCell(outputCell)
	if err != nil {
		return err
	}
	ptb.SimpleTransactionBuilder.OutputsData = append(ptb.SimpleTransactionBuilder.OutputsData, data)

	// Make sure to update the ScriptGroup:
	lockScriptGroup, typeScriptGroup := ptb.scriptGroupsForHash(assetHash)
	if lockScriptGroup == nil || (assetHash != zeroHash && typeScriptGroup == nil) {
		return fmt.Errorf("no script group for asset %x", assetHash)
	}
	appendOutputToGroups(lockScriptGroup, typeScriptGroup, uint32(len(ptb.SimpleTransactionBuilder.Outputs)-1))
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

func (ptb *PerunTransactionBuilder) completeCKBCapacity(requiredAmount uint64) error {
	// We need to add inputs and potentially change cells to fund the required
	// amount.
	if err := ptb.addInputsAndChangeForFunding(types.Hash{}, requiredAmount, nil, nil); err != nil {
		return fmt.Errorf("adding inputs and change for funding: %w", err)
	}
	return nil
}

// addInputsAndChangeForFunding adds inputs and potentially change cells to the
// transaction to fund the required amount.
//
// NOTE: Depending on whether the addition
// might result in a change cell or not, the requiredFunding or
// alreadyProvidedFunding might be modified for CKB capacity.
func (ptb *PerunTransactionBuilder) addInputsAndChangeForFunding(assetHash types.Hash, requiredAmount uint64, requiredFunding, alreadyProvidedFunding *AssetInformation) error {
	iter := ptb.iterators[assetHash]

	fundedAmount, err := ptb.addInputsForFunding(iter, assetHash, requiredAmount)
	if err != nil {
		return fmt.Errorf("adding inputs for funding: %w", err)
	}

	actualAmount := fundedAmount.UDTAmount(assetHash)
	if actualAmount < requiredAmount {
		return fmt.Errorf("not enough UDT %x cells with proper amount available for funding: want: %v; got: %v", assetHash, requiredAmount, actualAmount)
	}

	// We have at least the required amount of funds for the UDT available, check
	// if we need to add a change cell.
	if fundedAmount.UDTAmount(assetHash) == requiredAmount {
		// Perfect funding, no change cell needed.
		return nil
	}

	change := fundedAmount.UDTAmount(assetHash) - requiredAmount
	if err := ptb.addChangeAndDeductCellCapacity(assetHash, change, alreadyProvidedFunding); err != nil {
		return fmt.Errorf("adding change cell for UDT %x: %w", assetHash, err)
	}

	return nil
}

func (ptb *PerunTransactionBuilder) addInputsForFunding(iter collector.CellIterator, assetHash types.Hash, requiredAmount uint64) (AssetInformation, error) {
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

		ptb.SimpleTransactionBuilder.Inputs = append(ptb.SimpleTransactionBuilder.Inputs, &types.CellInput{
			Since:          0,
			PreviousOutput: input.OutPoint,
		})
		fundedAmount.AddValuesFromOutput(input.Output, input.OutputData)

		// Make sure to update the according script group.
		appendInputToGroups(lockScriptGroup, typeScriptGroup, uint32(len(ptb.SimpleTransactionBuilder.Inputs)-1))
		if fundedAmount.UDTAmount(assetHash) >= requiredAmount {
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

func (ptb *PerunTransactionBuilder) addChangeAndDeductCellCapacity(assetHash types.Hash, change uint64, alreadyProvidedFunding *AssetInformation) error {
	// Adding the change cell will subtract the required capacity for the UDT
	// from the already provided amount since we add everything to the provided
	// amount by default.
	udtCapacity := ptb.requiredCapacity(assetHash)
	alreadyProvidedFunding.assetAmounts[assetHash] -= udtCapacity

	return ptb.addChangeCell(assetHash, change)
}

// requiredCapacity returns the required amount of CKBytes to accomodate the
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
	return calculateCellCapacity(cell)
}

func calculateCellCapacity(cell types.CellOutput) uint64 {
	if cell.Type == nil {
		return cell.OccupiedCapacity([]byte{})
	} else {
		uint128, err := molecule2.PackUint128(big.NewInt(0))
		if err != nil {
			panic("packing 0 as uint128")
		}
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

func (ptb *PerunTransactionBuilder) processOutputLockScript(output *types.CellOutput, groups []*ckbtransaction.ScriptGroup, contexts ...interface{}) error {
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
	return ptb.processOutputLockScript(cell.Cell.Output, groups, contexts...)
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
		return nil, fmt.Errorf("no script group for script %x", hash)
	}
	return ptb.scriptGroups[groupIdx], nil
}

// Returns the default lock script by using the registered change address to
// fill in the lock script args field.
func (ptb *PerunTransactionBuilder) defaultLockScript() *types.Script {
	return ptb.changeAddress.Script
}
