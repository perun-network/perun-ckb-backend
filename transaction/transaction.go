package transaction

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math/big"
	"sort"

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
		if len(psh.omniLockScriptDep) < 2 {
			return nil, fmt.Errorf("OmniLock requires both lock and secp256k1 dependencies")
		}
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

	if omni {
		for i := range psh.omniLockScriptDep {
			b.AddCellDep(&psh.omniLockScriptDep[i])
		}
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
		if len(deployment.OmniLockScriptDep) < 2 {
			return nil, fmt.Errorf("OmniLock requires both lock and secp256k1 dependencies")
		}
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

	if omni {
		for i := range deployment.OmniLockScriptDep {
			b.AddCellDep(&deployment.OmniLockScriptDep[i])
		}
	}
	if deployment.FeeShannon != 0 {
		b.feeShannon = deployment.FeeShannon
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
	seen := make(map[types.OutPoint]bool, len(ptb.Inputs))
	for _, input := range ptb.Inputs {
		if input == nil || input.PreviousOutput == nil {
			return nil, fmt.Errorf("missing input outpoint")
		}
		if seen[*input.PreviousOutput] {
			return nil, fmt.Errorf("duplicate input outpoint")
		}
		seen[*input.PreviousOutput] = true
	}
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
	minimum := ckbChangeCell.OccupiedCapacity(ptb.OutputsData[ptb.ckbChangeCellIndex])
	if ckbChangeCell.Capacity < minimum {
		return fmt.Errorf("insufficient CKB change cell capacity: %d < %d", ckbChangeCell.Capacity, minimum)
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
		if inputCell == nil || inputCell.Status != "live" || inputCell.Cell == nil || inputCell.Cell.Output == nil {
			return fmt.Errorf("input cell is not live")
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

// AssetInformation accumulates amounts without truncating the 128-bit UDT value.
// Capacity remains limited to uint64; token totals remain limited to uint128.
type AssetInformation struct {
	assetAmounts map[types.Hash]*big.Int
	knownUDTs    map[types.Hash]types.Script
}

func NewAssetInformation(knownUDTs map[types.Hash]types.Script) *AssetInformation {
	return &AssetInformation{assetAmounts: make(map[types.Hash]*big.Int), knownUDTs: knownUDTs}
}

func (ai AssetInformation) Clone() AssetInformation {
	c := NewAssetInformation(ai.knownUDTs)
	for hash, amount := range ai.assetAmounts {
		c.assetAmounts[hash] = new(big.Int).Set(amount)
	}
	return *c
}

func (ai *AssetInformation) MergeWithAssetInformation(other AssetInformation) error {
	next := ai.Clone()
	for hash, amount := range other.assetAmounts {
		if err := next.addAmount(hash, amount); err != nil {
			return err
		}
	}
	ai.assetAmounts = next.assetAmounts
	return nil
}

func (ai *AssetInformation) AddAssetAmount(hash types.Hash, amount uint64) error {
	return ai.addAmount(hash, new(big.Int).SetUint64(amount))
}

func (ai *AssetInformation) addAmount(hash types.Hash, amount *big.Int) error {
	if amount == nil || amount.Sign() < 0 {
		return fmt.Errorf("negative or nil asset amount")
	}
	sum := new(big.Int).Add(ai.AssetAmount(hash), amount)
	bits := 128
	if hash == zeroHash {
		bits = 64
	}
	if sum.BitLen() > bits {
		return fmt.Errorf("asset %s total exceeds uint%d", hash, bits)
	}
	ai.assetAmounts[hash] = sum
	return nil
}

func (ai AssetInformation) EqualAssets(other AssetInformation) bool {
	for hash, amount := range ai.assetAmounts {
		if amount.Cmp(other.AssetAmount(hash)) != 0 {
			return false
		}
	}
	for hash, amount := range other.assetAmounts {
		if amount.Cmp(ai.AssetAmount(hash)) != 0 {
			return false
		}
	}
	return true
}

// AssetAmount returns an independent copy; missing assets have a zero balance.
func (ai AssetInformation) AssetAmount(hash types.Hash) *big.Int {
	if amount := ai.assetAmounts[hash]; amount != nil {
		return new(big.Int).Set(amount)
	}
	return new(big.Int)
}

func (ai AssetInformation) CKBAmount() uint64 { return ai.AssetAmount(zeroHash).Uint64() }

func (ai *AssetInformation) AddValuesFromOutput(output *types.CellOutput, data []byte) error {
	if output == nil {
		return fmt.Errorf("missing cell output")
	}
	next := ai.Clone()
	if output.Type != nil {
		if _, known := ai.knownUDTs[output.Type.Hash()]; known {
			if len(data) < 16 {
				return fmt.Errorf("UDT data must contain a 16-byte amount, got %d bytes", len(data))
			}
			// Cell data may contain xUDT extension data after the amount.
			be := make([]byte, 16)
			for i := range be {
				be[15-i] = data[i]
			}
			if err := next.addAmount(output.Type.Hash(), new(big.Int).SetBytes(be)); err != nil {
				return err
			}
		}
	}
	if err := next.AddAssetAmount(zeroHash, output.Capacity); err != nil {
		return err
	}
	ai.assetAmounts = next.assetAmounts
	return nil
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
	provided := NewAssetInformation(ptb.knownUDTs)
	required := NewAssetInformation(ptb.knownUDTs)
	if len(ptb.Outputs) != len(ptb.OutputsData) {
		return fmt.Errorf("output/data count mismatch")
	}
	if err := required.AddAssetAmount(zeroHash, ptb.feeShannon); err != nil {
		return err
	}
	for i, output := range ptb.Outputs {
		if err := required.AddValuesFromOutput(output, ptb.OutputsData[i]); err != nil {
			return fmt.Errorf("output %d: %w", i, err)
		}
	}
	for i, input := range ptb.Inputs {
		output, data, err := ptb.resolveInputCell(input)
		if err != nil {
			return err
		}
		if err := provided.AddValuesFromOutput(output, data); err != nil {
			return fmt.Errorf("input %d: %w", i, err)
		}
	}
	// Include input-only assets as well: excess tokens must be returned, never burned.
	hashes := make([]types.Hash, 0, len(ptb.knownUDTs))
	for hash := range ptb.knownUDTs {
		hashes = append(hashes, hash)
	}
	sort.Slice(hashes, func(i, j int) bool { return bytes.Compare(hashes[i][:], hashes[j][:]) < 0 })
	for _, hash := range hashes {
		need := required.AssetAmount(hash)
		have := provided.AssetAmount(hash)
		if have.Cmp(need) < 0 {
			added, err := ptb.addInputsForFunding(ptb.iterators[hash], hash, new(big.Int).Sub(need, have))
			if err != nil {
				return err
			}
			if err := provided.MergeWithAssetInformation(added); err != nil {
				return err
			}
		}
		change := new(big.Int).Sub(provided.AssetAmount(hash), need)
		if change.Sign() > 0 {
			if err := ptb.addOrUpdateUDTChangeCell(hash, change); err != nil {
				return err
			}
			if err := required.AddAssetAmount(zeroHash, ptb.Outputs[ptb.udtChangeCellIndices[hash]].Capacity); err != nil {
				return err
			}
		}
	}
	return ptb.completeCKBCapacity(required, provided)
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

func (ptb *PerunTransactionBuilder) addOrUpdateUDTChangeCell(assetHash types.Hash, amount *big.Int) error {
	encodedAmount, err := molecule2.PackUint128(amount)
	if err != nil {
		return fmt.Errorf("encoding amount for %x as Uint128: %w", assetHash, err)
	}
	data := encodedAmount.AsSlice()

	idx, exists := ptb.udtChangeCellIndices[assetHash]
	if exists && idx != -1 {
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

	outputCell.Capacity = outputCell.OccupiedCapacity(data)

	ptb.Outputs = append(ptb.Outputs, outputCell)
	ptb.OutputsData = append(ptb.OutputsData, data)

	changeIndex := len(ptb.Outputs) - 1
	ptb.initializeScript(&udtScript, types.ScriptTypeType, -1, noneDir)
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

func (ptb *PerunTransactionBuilder) completeCKBCapacity(required, provided *AssetInformation) error {
	need := required.AssetAmount(zeroHash)
	have := provided.AssetAmount(zeroHash)
	if have.Cmp(need) < 0 {
		added, err := ptb.addInputsForFunding(ptb.iterators[zeroHash], zeroHash, new(big.Int).Sub(need, have))
		if err != nil {
			return err
		}
		if err := provided.MergeWithAssetInformation(added); err != nil {
			return err
		}
		have = provided.AssetAmount(zeroHash)
	}
	if have.Cmp(need) == 0 {
		return nil
	}
	// A positive remainder below minimum capacity needs additional CKB, not
	// unsigned subtraction of the already funded capacity from the deficit.
	target := new(big.Int).Add(need, new(big.Int).SetUint64(ptb.ckbChangeCellCapacity()))
	if !target.IsUint64() {
		return fmt.Errorf("CKB capacity including change exceeds uint64")
	}
	if have.Cmp(target) < 0 {
		added, err := ptb.addInputsForFunding(ptb.iterators[zeroHash], zeroHash, new(big.Int).Sub(target, have))
		if err != nil {
			return err
		}
		if err := provided.MergeWithAssetInformation(added); err != nil {
			return err
		}
	}
	change := new(big.Int).Sub(provided.AssetAmount(zeroHash), need)
	return ptb.addOrUpdateCKBChangeCell(change.Uint64())
}

func (ptb *PerunTransactionBuilder) addInputsForFunding(iter collector.CellIterator, assetHash types.Hash, requiredAmount *big.Int) (AssetInformation, error) {
	funded := NewAssetInformation(ptb.knownUDTs)
	if requiredAmount.Sign() == 0 {
		return *funded, nil
	}
	if iter == nil {
		return *funded, fmt.Errorf("no iterator for asset %s", assetHash)
	}
	used := make(map[types.OutPoint]bool, len(ptb.Inputs))
	for _, input := range ptb.Inputs {
		if input == nil || input.PreviousOutput == nil {
			return *funded, fmt.Errorf("missing input outpoint")
		}
		if used[*input.PreviousOutput] {
			return *funded, fmt.Errorf("duplicate input outpoint")
		}
		used[*input.PreviousOutput] = true
	}
	for iter.HasNext() {
		input := iter.Next()
		if input == nil || input.OutPoint == nil || input.Output == nil || input.Output.Lock == nil {
			return *funded, fmt.Errorf("invalid iterator cell")
		}
		if used[*input.OutPoint] {
			continue
		}
		if (assetHash == zeroHash && input.Output.Type != nil) ||
			(assetHash != zeroHash && (input.Output.Type == nil || input.Output.Type.Hash() != assetHash)) {
			return *funded, fmt.Errorf("iterator returned a cell for a different asset")
		}
		if err := funded.AddValuesFromOutput(input.Output, input.OutputData); err != nil {
			return *funded, err
		}
		used[*input.OutPoint] = true
		index := len(ptb.Inputs)
		ptb.Inputs = append(ptb.Inputs, &types.CellInput{PreviousOutput: input.OutPoint})
		for len(ptb.Witnesses) < len(ptb.Inputs) {
			ptb.Witnesses = append(ptb.Witnesses, []byte{})
		}
		ptb.initializeScript(input.Output.Lock, types.ScriptTypeLock, index, inputDir)
		ptb.initializeScript(input.Output.Type, types.ScriptTypeType, index, inputDir)
		if funded.AssetAmount(assetHash).Cmp(requiredAmount) >= 0 {
			return *funded, nil
		}
	}
	return *funded, fmt.Errorf("insufficient funds for asset %s: collected %s, need %s", assetHash, funded.AssetAmount(assetHash), requiredAmount)
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
	if cell == nil || cell.Status != "live" || cell.Cell == nil || cell.Cell.Output == nil || cell.Cell.Data == nil {
		return nil, nil, fmt.Errorf("input cell is not live or has no data")
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
	output, _, err := ptb.resolveInputCell(input)
	if err != nil {
		return err
	}
	return ptb.processLockScript(output, groups, contexts...)
}

func (ptb *PerunTransactionBuilder) processInputTypeScript(input *types.CellInput, groups []*ckbtransaction.ScriptGroup, contexts ...interface{}) error {
	output, _, err := ptb.resolveInputCell(input)
	if err != nil {
		return err
	}
	return ptb.processOutputTypeScript(output, groups, contexts...)
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
