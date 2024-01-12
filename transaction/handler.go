package transaction

import (
	"errors"
	"fmt"
	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/encoding"
	"perun.network/perun-ckb-backend/wallet/address"
)

// PerunScriptHandler is responsible for building transactions utilizing Perun
// scripts. It is specialized to create transactions using a predeployed set
// of Perun scripts.
type PerunScriptHandler struct {
	pctsDep types.CellDep
	pclsDep types.CellDep
	pflsDep types.CellDep

	sudtDeps map[types.Hash]types.CellDep

	pctsCodeHash    types.Hash
	pctsHashType    types.ScriptHashType
	pclsCodeHash    types.Hash
	pclsHashType    types.ScriptHashType
	pflsCodeHash    types.Hash
	pflsHashType    types.ScriptHashType
	pflsMinCapacity uint64

	defaultLockScript    types.Script
	defaultLockScriptDep types.CellDep
}

var _ collector.ScriptHandler = (*PerunScriptHandler)(nil)

func NewPerunScriptHandlerWithDeployment(deployment backend.Deployment) *PerunScriptHandler {
	return &PerunScriptHandler{
		pctsDep:              deployment.PCTSDep,
		pclsDep:              deployment.PCLSDep,
		pflsDep:              deployment.PFLSDep,
		sudtDeps:             deployment.SUDTDeps,
		pctsCodeHash:         deployment.PCTSCodeHash,
		pctsHashType:         deployment.PCTSHashType,
		pclsCodeHash:         deployment.PCLSCodeHash,
		pclsHashType:         deployment.PCLSHashType,
		pflsCodeHash:         deployment.PFLSCodeHash,
		pflsHashType:         deployment.PFLSHashType,
		pflsMinCapacity:      deployment.PFLSMinCapacity,
		defaultLockScript:    deployment.DefaultLockScript,
		defaultLockScriptDep: deployment.DefaultLockScriptDep,
	}
}

// BuildTransaction implements collector.ScriptHandler.
func (psh *PerunScriptHandler) BuildTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, context interface{}) (bool, error) {
	ok := false
	switch context.(type) {
	case OpenInfo, *OpenInfo:
		var openInfo *OpenInfo
		if openInfo, ok = context.(*OpenInfo); !ok {
			v, _ := context.(OpenInfo)
			openInfo = &v
		}
		return psh.buildOpenTransaction(builder, group, openInfo)
	case AbortInfo, *AbortInfo:
		var abortInfo *AbortInfo
		if abortInfo, ok = context.(*AbortInfo); !ok {
			v, _ := context.(AbortInfo)
			abortInfo = &v
		}
		return psh.buildAbortTransaction(builder, group, abortInfo)
	case FundInfo, *FundInfo:
		var fundInfo *FundInfo
		if fundInfo, ok = context.(*FundInfo); !ok {
			v, _ := context.(FundInfo)
			fundInfo = &v
		}
		return psh.buildFundTransaction(builder, group, fundInfo)
	case DisputeInfo, *DisputeInfo:
		var disputeInfo *DisputeInfo
		if disputeInfo, ok = context.(*DisputeInfo); !ok {
			v, _ := context.(DisputeInfo)
			disputeInfo = &v
		}
		return psh.buildDisputeTransaction(builder, group, disputeInfo)
	case CloseInfo, *CloseInfo:
		var closeInfo *CloseInfo
		if closeInfo, ok = context.(*CloseInfo); !ok {
			v, _ := context.(CloseInfo)
			closeInfo = &v
		}
		return psh.buildCloseTransaction(builder, group, closeInfo)
	case ForceCloseInfo, *ForceCloseInfo:
		var forceCloseInfo *ForceCloseInfo
		if forceCloseInfo, ok = context.(*ForceCloseInfo); !ok {
			v, _ := context.(ForceCloseInfo)
			forceCloseInfo = &v
		}
		return psh.buildForceCloseTransaction(builder, group, forceCloseInfo)
	default:
	}
	return ok, nil
}

func (psh *PerunScriptHandler) buildOpenTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, openInfo *OpenInfo) (bool, error) {
	const partyIndex = 0
	// Add required cell dependencies for Perun scripts.
	builder.AddCellDep(&psh.defaultLockScriptDep)
	builder.AddCellDep(&psh.pctsDep)
	psh.AddSudtCellDeps(builder)
	// Add channel token as input.
	channelToken := openInfo.ChannelToken.AsCellInput()
	builder.AddInput(&channelToken)

	/// Create outputs containing channel cell and channel funds cell.

	// Channel funds cell output.
	channelTypeScript := psh.mkChannelTypeScript(openInfo.Params, openInfo.ChannelToken.Token)
	fundsLockScript := psh.mkFundsLockScript(channelTypeScript)
	balance, err := GetCKByteBalance(partyIndex, openInfo.State)
	if err != nil {
		return false, err
	}
	if balance >= psh.pflsMinCapacity {
		paymentOutput := psh.mkPaymentOutput(fundsLockScript, balance)
		builder.AddOutput(paymentOutput, nil)
	} else {
		return false, fmt.Errorf("balance %d is less than minimum capacity of the pfls %d", balance, psh.pflsMinCapacity)
	}

	err = psh.AddAssetsToOutputs(builder, openInfo.State, partyIndex, fundsLockScript, 0)
	if err != nil {
		return false, err
	}

	// Channel cell output.
	channelLockScript := psh.mkChannelLockScript()
	channelCell, channelData := openInfo.MkInitialChannelCell(*channelLockScript, *channelTypeScript)
	builder.AddOutput(&channelCell, channelData)

	return true, nil
}

func (psh *PerunScriptHandler) AddSudtCellDeps(builder collector.TransactionBuilder) {
	for _, d := range psh.sudtDeps {
		builder.AddCellDep(&d)
	}
}

func (psh *PerunScriptHandler) buildCloseTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, closeInfo *CloseInfo) (bool, error) {
	// TODO: How do we make sure that we unlock the channel?

	builder.AddCellDep(&psh.pctsDep)
	builder.AddCellDep(&psh.pclsDep)
	builder.AddCellDep(&psh.pflsDep)
	psh.AddSudtCellDeps(builder)

	idx := builder.AddInput(&closeInfo.ChannelInput)
	for idx := range closeInfo.AssetInputs {
		builder.AddInput(&closeInfo.AssetInputs[idx])
	}

	for _, h := range closeInfo.Headers {
		builder.AddHeaderDep(h)
	}

	// Add the payment output for each participant.
	for i, addr := range closeInfo.Params.Parts {
		payoutScript := address.AsParticipant(addr).PaymentScript
		paymentMinCapacity := payoutScript.OccupiedCapacity()
		// payout ckbytes
		balance, err := GetCKByteBalance(i, closeInfo.State)
		if err != nil {
			return false, err
		}
		// The capacity of the channel's live cell is added to the balance of the first party.
		if i == 0 {
			balance += closeInfo.ChannelCapacity
		}
		additionalBalance := uint64(0)
		if balance >= paymentMinCapacity {
			paymentOutput := psh.mkPaymentOutput(payoutScript, balance)
			builder.AddOutput(paymentOutput, nil)
		} else {
			additionalBalance = balance
		}
		err = psh.AddAssetsToOutputs(builder, closeInfo.State, i, payoutScript, additionalBalance)
		if err != nil {
			return false, err
		}
	}
	err := builder.SetWitness(uint(idx), types.WitnessTypeInputType, psh.mkWitnessClose(closeInfo.State, closeInfo.PaddedSignatures))
	if err != nil {
		return false, err
	}
	return true, nil
}

func (psh *PerunScriptHandler) buildAbortTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, abortInfo *AbortInfo) (bool, error) {
	const partyIdx = 0
	// TODO: How do we make sure that we unlock the channel?

	builder.AddCellDep(&psh.pctsDep)
	builder.AddCellDep(&psh.pclsDep)
	builder.AddCellDep(&psh.pflsDep)
	psh.AddSudtCellDeps(builder)

	idx := builder.AddInput(&abortInfo.ChannelInput)
	for _, assetInput := range abortInfo.AssetInputs {
		builder.AddInput(&assetInput)
	}

	for _, h := range abortInfo.Headers {
		builder.AddHeaderDep(h)
	}
	// To abort we only need to pay out the party with index 0.
	addr := abortInfo.Params.Parts[partyIdx]
	payoutScript := address.AsParticipant(addr).PaymentScript
	paymentMinCapacity := payoutScript.OccupiedCapacity()
	// payout ckbytes
	balance, err := GetCKByteBalance(partyIdx, abortInfo.InitialState)
	if err != nil {
		return false, err
	}
	// The capacity of the channel's live cell is added to the balance of the first party.
	balance += abortInfo.ChannelCapacity
	additionalBalance := uint64(0)
	if balance >= paymentMinCapacity {
		paymentOutput := psh.mkPaymentOutput(payoutScript, balance)
		builder.AddOutput(paymentOutput, nil)
	} else {
		additionalBalance = balance
	}
	err = psh.AddAssetsToOutputs(builder, abortInfo.InitialState, partyIdx, payoutScript, additionalBalance)
	if err != nil {
		return false, err
	}
	err = builder.SetWitness(uint(idx), types.WitnessTypeInputType, psh.mkWitnessAbort())
	if err != nil {
		return false, err
	}
	return true, nil
}

func (psh *PerunScriptHandler) buildForceCloseTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, forceCloseInfo *ForceCloseInfo) (bool, error) {
	// TODO: How do we make sure that we unlock the channel?

	builder.AddCellDep(&psh.pctsDep)
	builder.AddCellDep(&psh.pclsDep)
	builder.AddCellDep(&psh.pflsDep)
	psh.AddSudtCellDeps(builder)

	idx := builder.AddInput(&forceCloseInfo.ChannelInput)
	for _, assetInput := range forceCloseInfo.AssetInputs {
		builder.AddInput(&assetInput)
	}

	for _, h := range forceCloseInfo.Headers {
		builder.AddHeaderDep(h)
	}

	// Add the payment output for each participant.
	for i, addr := range forceCloseInfo.Params.Parts {
		payoutScript := address.AsParticipant(addr).PaymentScript
		paymentMinCapacity := payoutScript.OccupiedCapacity()
		// payout ckbytes
		balance, err := GetCKByteBalance(i, forceCloseInfo.State)
		if err != nil {
			return false, err
		}
		// The capacity of the channel's live cell is added to the balance of the first party.
		if i == 0 {
			balance += forceCloseInfo.ChannelCapacity
		}
		additionalBalance := uint64(0)
		if balance >= paymentMinCapacity {
			paymentOutput := psh.mkPaymentOutput(payoutScript, balance)
			builder.AddOutput(paymentOutput, nil)
		} else {
			additionalBalance = balance
		}

		err = psh.AddAssetsToOutputs(builder, forceCloseInfo.State, i, payoutScript, additionalBalance)
		if err != nil {
			return false, err
		}
	}
	err := builder.SetWitness(uint(idx), types.WitnessTypeInputType, psh.mkWitnessForceClose())
	if err != nil {
		return false, err
	}
	return true, nil
}

func (psh *PerunScriptHandler) buildFundTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, fundInfo *FundInfo) (bool, error) {
	const partyIndex = 1
	// Dependencies.
	builder.AddCellDep(&psh.defaultLockScriptDep)
	builder.AddCellDep(&psh.pclsDep)
	builder.AddCellDep(&psh.pctsDep)
	psh.AddSudtCellDeps(builder)
	builder.AddHeaderDep(fundInfo.Header)

	// Channel cell input.
	channelInputIndex := builder.AddInput(&types.CellInput{
		Since:          0,
		PreviousOutput: &fundInfo.ChannelCell,
	})
	err := builder.SetWitness(uint(channelInputIndex), types.WitnessTypeInputType, psh.mkWitnessFund())
	if err != nil {
		return false, err
	}

	// Channel cell output.
	channelLockScript := psh.mkChannelLockScript()
	channelStatus := encoding.ToFundedChannelStatus(fundInfo.Status)
	channelCell := types.CellOutput{
		Capacity: 0,
		Lock:     channelLockScript,
		Type:     fundInfo.PCTS,
	}
	channelCell.Capacity = channelCell.OccupiedCapacity(channelStatus.AsSlice())
	builder.AddOutput(&channelCell, channelStatus.AsSlice())

	// Channel funds cell output.
	fundsLockScript := psh.mkFundsLockScript(fundInfo.PCTS)
	balance, err := GetCKByteBalance(partyIndex, fundInfo.State)
	if err != nil {
		return false, err
	}
	if balance >= psh.pflsMinCapacity {
		paymentOutput := psh.mkPaymentOutput(fundsLockScript, balance)
		builder.AddOutput(paymentOutput, nil)
	} else {
		return false, fmt.Errorf("party balance %d is less than minimum capacity of the pfls %d", balance, psh.pflsMinCapacity)
	}

	err = psh.AddAssetsToOutputs(builder, fundInfo.State, partyIndex, fundsLockScript, 0)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (psh *PerunScriptHandler) mkWitnessFund() []byte {
	w := molecule.NewChannelWitnessBuilder().Set(molecule.ChannelWitnessUnionFromFund(molecule.FundDefault())).Build()
	return w.AsSlice()
}

func (psh *PerunScriptHandler) buildDisputeTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, disputeInfo *DisputeInfo) (bool, error) {
	builder.AddCellDep(&psh.pclsDep)
	builder.AddCellDep(&psh.pctsDep)
	builder.AddHeaderDep(disputeInfo.Header)

	// Channel cell input.
	channelInputIndex := builder.AddInput(&types.CellInput{
		Since:          0,
		PreviousOutput: &disputeInfo.ChannelCell,
	})
	err := builder.SetWitness(uint(channelInputIndex), types.WitnessTypeInputType, psh.mkWitnessDispute(disputeInfo.SigA, disputeInfo.SigB))
	if err != nil {
		return false, err
	}

	// Channel cell output.
	channelLockScript := psh.mkChannelLockScript()
	channelCell := types.CellOutput{
		Capacity: 0,
		Lock:     channelLockScript,
		Type:     disputeInfo.PCTS,
	}
	channelCell.Capacity = channelCell.OccupiedCapacity(disputeInfo.Status.AsSlice())
	builder.AddOutput(&channelCell, disputeInfo.Status.AsSlice())
	return true, nil
}

func (psh PerunScriptHandler) mkWitnessDispute(sigA, sigB molecule.Bytes) []byte {
	disputeRedeemer := molecule.NewDisputeBuilder().SigA(sigA).SigB(sigB).Build()
	witness := molecule.ChannelWitnessUnionFromDispute(disputeRedeemer)
	return witness.AsSlice()
}

func (psh PerunScriptHandler) mkChannelLockScript() *types.Script {
	return &types.Script{
		CodeHash: psh.pclsCodeHash,
		HashType: psh.pclsHashType,
	}
}

func (psh PerunScriptHandler) mkChannelTypeScript(params *channel.Params, token molecule.ChannelToken) *types.Script {
	channelConstants := psh.mkChannelConstants(params, token)
	return &types.Script{
		CodeHash: psh.pctsCodeHash,
		HashType: psh.pctsHashType,
		Args:     channelConstants.AsSlice(),
	}
}

func (psh PerunScriptHandler) mkFundsLockScript(pcts *types.Script) *types.Script {
	return &types.Script{
		CodeHash: psh.pflsCodeHash,
		HashType: psh.pflsHashType,
		Args:     pcts.Hash().Bytes(),
	}
}

func (psh PerunScriptHandler) mkChannelConstants(params *channel.Params, token molecule.ChannelToken) molecule.ChannelConstants {
	chanParams, err := encoding.PackChannelParameters(params)
	if err != nil {
		panic(err)
	}

	pclsCode := psh.pclsCodeHash.Pack()
	pclsHashType := psh.pclsHashType.Pack()
	pflsCode := psh.pflsCodeHash.Pack()
	pflsHashType := psh.pflsHashType.Pack()

	return molecule.NewChannelConstantsBuilder().
		Params(chanParams).
		PclsCodeHash(*pclsCode).
		PclsHashType(*pclsHashType).
		PflsCodeHash(*pflsCode).
		PflsHashType(*pflsHashType).
		PflsMinCapacity(*types.PackUint64(psh.pflsMinCapacity)).
		ThreadToken(token).
		Build()
}

func (psh PerunScriptHandler) mkPaymentOutput(lock *types.Script, bal uint64) *types.CellOutput {
	return &types.CellOutput{
		Capacity: bal,
		Lock:     lock,
		Type:     nil,
	}
}

func (psh PerunScriptHandler) mkAssetOutput(lock *types.Script, balances asset.SUDTBalances, index int, additionalBalance uint64) (*types.CellOutput, []byte) {
	data := make([]byte, 16)
	uint128.StoreLittleEndian(data[:], balances.Distribution[index])
	return &types.CellOutput{
		Capacity: balances.Asset.MaxCapacity + additionalBalance,
		Lock:     lock,
		Type:     &balances.Asset.TypeScript,
	}, data
}

func (psh PerunScriptHandler) mkWitnessAbort() []byte {
	w := molecule.NewChannelWitnessBuilder().Set(molecule.ChannelWitnessUnionFromAbort(molecule.AbortDefault())).Build()
	return w.AsSlice()
}

func (psh PerunScriptHandler) mkWitnessClose(state *channel.State, paddedSigs []wallet.Sig) []byte {
	ps, err := encoding.PackChannelState(state)
	if err != nil {
		panic(err)
	}
	sigA, err := encoding.NewDEREncodedSignatureFromPadded(paddedSigs[0])
	if err != nil {
		panic(err)
	}
	sigB, err := encoding.NewDEREncodedSignatureFromPadded(paddedSigs[1])
	if err != nil {
		panic(err)
	}
	c := molecule.NewCloseBuilder().State(ps).SigA(*sigA).SigB(*sigB).Build()
	witnessClose := molecule.NewChannelWitnessBuilder().Set(molecule.ChannelWitnessUnionFromClose(c)).Build()
	return witnessClose.AsSlice()
}

func (psh PerunScriptHandler) mkWitnessForceClose() []byte {
	w := molecule.NewChannelWitnessBuilder().Set(molecule.ChannelWitnessUnionFromForceClose(molecule.ForceCloseDefault())).Build()
	return w.AsSlice()
}

func GetCKByteBalance(index int, state *channel.State) (uint64, error) {
	assetIdx, ok := state.AssetIndex(asset.NewCKBytesAsset())
	if !ok {
		return 0, nil
	}
	bal := state.Balances[assetIdx][index]
	if !bal.IsUint64() {
		return 0, errors.New("balance is not uint64")
	}
	return bal.Uint64(), nil
}

func (psh PerunScriptHandler) AddAssetsToOutputs(builder collector.TransactionBuilder, state *channel.State, index int, lock *types.Script, additionalBalance uint64) error {
	sudtBalancesSlice, err := encoding.GetSUDTBalancesSlice(state)
	if err != nil {
		return err
	}
	for _, sudtBalances := range sudtBalancesSlice {
		if index >= len(sudtBalances.Distribution) || index < 0 {
			return errors.New("index out of range")
		}
		if sudtBalances.Distribution[index].IsZero() {
			paymentOutput := psh.mkPaymentOutput(lock, sudtBalances.Asset.MaxCapacity+additionalBalance)
			additionalBalance = 0
			builder.AddOutput(paymentOutput, []byte{})
		} else {
			paymentOutput, data := psh.mkAssetOutput(lock, sudtBalances, index, additionalBalance)
			additionalBalance = 0
			builder.AddOutput(paymentOutput, data)
		}

	}
	return nil
}
