package transaction

import (
	"errors"
	"fmt"
	"log"

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
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
	"perun.network/perun-ckb-backend/wallet/address"
)

// PerunScriptHandler is responsible for building transactions utilizing Perun
// scripts. It is specialized to create transactions using a predeployed set
// of Perun scripts.
type PerunScriptHandler struct {
	pctsDep types.CellDep
	pclsDep types.CellDep
	pflsDep types.CellDep
	vclsDep types.CellDep
	vctsDep types.CellDep

	sudtDeps map[types.Hash]types.CellDep

	pctsCodeHash    types.Hash
	pctsHashType    types.ScriptHashType
	pclsCodeHash    types.Hash
	pclsHashType    types.ScriptHashType
	vctsCodeHash    types.Hash
	vctsHashType    types.ScriptHashType
	vclsCodeHash    types.Hash
	vclsHashType    types.ScriptHashType
	pflsCodeHash    types.Hash
	pflsHashType    types.ScriptHashType
	pflsMinCapacity uint64

	defaultLockScript    types.Script
	defaultLockScriptDep types.CellDep
	omniLockScript       types.Script
	omniLockScriptDep    []types.CellDep
}

var _ collector.ScriptHandler = (*PerunScriptHandler)(nil)

func NewPerunScriptHandlerWithDeployment(deployment backend.Deployment) *PerunScriptHandler {
	return &PerunScriptHandler{
		pctsDep:              deployment.PCTSDep,
		pclsDep:              deployment.PCLSDep,
		pflsDep:              deployment.PFLSDep,
		vclsDep:              deployment.VCLSDep,
		vctsDep:              deployment.VCTSDep,
		sudtDeps:             deployment.SUDTDeps,
		pctsCodeHash:         deployment.PCTSCodeHash,
		pctsHashType:         deployment.PCTSHashType,
		pclsCodeHash:         deployment.PCLSCodeHash,
		pclsHashType:         deployment.PCLSHashType,
		vctsCodeHash:         deployment.VCTSCodeHash,
		vctsHashType:         deployment.VCTSHashType,
		vclsCodeHash:         deployment.VCLSCodeHash,
		vclsHashType:         deployment.VCLSHashType,
		pflsCodeHash:         deployment.PFLSCodeHash,
		pflsHashType:         deployment.PFLSHashType,
		pflsMinCapacity:      deployment.PFLSMinCapacity,
		defaultLockScript:    deployment.DefaultLockScript,
		defaultLockScriptDep: deployment.DefaultLockScriptDep,
		omniLockScript:       deployment.OmniLockScript,
		omniLockScriptDep:    deployment.OmniLockScriptDep,
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
	if len(psh.omniLockScriptDep) != 0 {
		builder.AddCellDep(&psh.omniLockScriptDep[0])
		builder.AddCellDep(&psh.omniLockScriptDep[1])
	}
	builder.AddCellDep(&psh.pctsDep)
	psh.AddSudtCellDeps(builder)
	// Add channel token as input.
	channelToken := openInfo.ChannelToken.AsCellInput()
	log.Println("channelToken", channelToken.PreviousOutput.TxHash, channelToken.PreviousOutput.Index)
	builder.AddInput(&channelToken)

	/// Create outputs containing channel cell and channel funds cell.

	// Channel funds cell output.
	channelTypeScript, err := psh.mkChannelTypeScript(openInfo.Params, openInfo.ChannelToken.Token)
	if err != nil {
		return false, fmt.Errorf("building channel type script: %w", err)
	}
	fundsLockScript := psh.mkFundsLockScript(channelTypeScript)
	balance, err := GetCKByteBalance(partyIndex, openInfo.State)
	if err != nil {
		return false, err
	}
	if balance >= psh.pflsMinCapacity || balance == 0 {
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
	channelCell, channelData, err := openInfo.MkInitialChannelCell(*channelLockScript, *channelTypeScript)
	if err != nil {
		return false, fmt.Errorf("creating initial channel cell: %w", err)
	}
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
	if len(psh.omniLockScriptDep) != 0 {
		builder.AddCellDep(&psh.omniLockScriptDep[0])
		builder.AddCellDep(&psh.omniLockScriptDep[1])
	}
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
		payoutScript := address.AsParticipant(addr[address.CKBBackendID]).PaymentScript
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
	witnessData, err := psh.mkWitnessClose(closeInfo.State, closeInfo.PaddedSignatures)
	if err != nil {
		return false, fmt.Errorf("building close witness: %w", err)
	}
	err = builder.SetWitness(uint(idx), types.WitnessTypeInputType, witnessData)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (psh *PerunScriptHandler) buildAbortTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, abortInfo *AbortInfo) (bool, error) {
	const partyIdx = 0
	// TODO: How do we make sure that we unlock the channel?
	if len(psh.omniLockScriptDep) != 0 {
		builder.AddCellDep(&psh.omniLockScriptDep[0])
		builder.AddCellDep(&psh.omniLockScriptDep[1])
	}
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
	payoutScript := address.AsParticipant(addr[address.CKBBackendID]).PaymentScript
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
	if len(psh.omniLockScriptDep) != 0 {
		builder.AddCellDep(&psh.omniLockScriptDep[0])
		builder.AddCellDep(&psh.omniLockScriptDep[1])
	}
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
		payoutScript := address.AsParticipant(addr[address.CKBBackendID]).PaymentScript
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
	if len(psh.omniLockScriptDep) != 0 {
		builder.AddCellDep(&psh.omniLockScriptDep[0])
		builder.AddCellDep(&psh.omniLockScriptDep[1])
	}
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
	if balance >= psh.pflsMinCapacity || balance == 0 {
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
	if len(psh.omniLockScriptDep) != 0 {
		builder.AddCellDep(&psh.omniLockScriptDep[0])
		builder.AddCellDep(&psh.omniLockScriptDep[1])
	}
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
	if _, err := disputeInfo.update(); err != nil {
		return false, fmt.Errorf("updating dispute info: %w", err)
	}
	channelCell.Capacity = channelCell.OccupiedCapacity(disputeInfo.Status.AsSlice())
	builder.AddOutput(&channelCell, disputeInfo.Status.AsSlice())
	return true, nil
}

func (psh *PerunScriptHandler) buildFirstVCDisputeTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, disputeInfo *VcDisputeInfo) (bool, error) {
	log.Println("buildFirstVCDisputeTransaction")
	if len(psh.omniLockScriptDep) != 0 {
		builder.AddCellDep(&psh.omniLockScriptDep[0])
		builder.AddCellDep(&psh.omniLockScriptDep[1])
	}
	builder.AddCellDep(&psh.pclsDep)
	builder.AddCellDep(&psh.pctsDep)
	builder.AddCellDep(&psh.vclsDep)
	builder.AddCellDep(&psh.vctsDep)
	builder.AddHeaderDep(disputeInfo.Header)

	// Channel cell input.
	channelInputIndex := builder.AddInput(&types.CellInput{
		Since:          0,
		PreviousOutput: disputeInfo.ChannelCell,
	})
	err := builder.SetWitness(uint(channelInputIndex), types.WitnessTypeInputType, psh.mkWitnessVCDispute(disputeInfo.VCDispute))
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

	// VC Channel cell output.

	vcLockScript := psh.mkVirtualChannelLockScript()
	vcTypeScript, err := psh.mkVirtualChannelTypeScript(disputeInfo.Params, vcLockScript)
	if err != nil {
		return false, fmt.Errorf("building vc type script: %w", err)
	}
	vcChannelCell, vcChannelData, err := disputeInfo.mkInitialVirtualChannelCell(*vcLockScript, *vcTypeScript)
	if err != nil {
		return false, fmt.Errorf("creating initial vc cell: %w", err)
	}
	if _, err := disputeInfo.update(vcTypeScript); err != nil {
		return false, fmt.Errorf("updating vc dispute info: %w", err)
	}
	channelCell.Capacity = channelCell.OccupiedCapacity(disputeInfo.LCStatus.AsSlice())
	builder.AddOutput(&vcChannelCell, vcChannelData)
	builder.AddOutput(&channelCell, disputeInfo.LCStatus.AsSlice())

	return true, nil
}

func (psh *PerunScriptHandler) buildVCDisputeProgressTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, disputeInfo *VcDisputeInfo) (bool, error) {
	if len(psh.omniLockScriptDep) != 0 {
		builder.AddCellDep(&psh.omniLockScriptDep[0])
		builder.AddCellDep(&psh.omniLockScriptDep[1])
	}
	builder.AddCellDep(&psh.pclsDep)
	builder.AddCellDep(&psh.pctsDep)
	builder.AddCellDep(&psh.vclsDep)
	builder.AddCellDep(&psh.vctsDep)
	builder.AddHeaderDep(disputeInfo.Header)

	// Channel cell input.
	channelInputIndex := builder.AddInput(&types.CellInput{
		Since:          0,
		PreviousOutput: disputeInfo.ChannelCell,
	})
	err := builder.SetWitness(uint(channelInputIndex), types.WitnessTypeInputType, psh.mkWitnessDispute(
		disputeInfo.ParentSigA,
		disputeInfo.ParentSigB,
	))
	if err != nil {
		return false, err
	}

	// Virtual Channel cell input.
	vcInputIndex := builder.AddInput(&types.CellInput{
		Since:          0,
		PreviousOutput: disputeInfo.VCCell,
	})
	err = builder.SetWitness(uint(vcInputIndex), types.WitnessTypeInputType, psh.mkWitnessDispute(
		*disputeInfo.VCDispute.SigA(),
		*disputeInfo.VCDispute.SigB(),
	))
	if err != nil {
		return false, err
	}

	// Define the channel cell output.
	channelLockScript := psh.mkChannelLockScript()
	channelCell := types.CellOutput{
		Capacity: 0,
		Lock:     channelLockScript,
		Type:     disputeInfo.PCTS,
	}
	// Set disputed flag to true.
	if _, err := disputeInfo.update(disputeInfo.VCTS); err != nil {
		return false, fmt.Errorf("updating vc dispute info: %w", err)
	}
	channelCell.Capacity = channelCell.OccupiedCapacity(disputeInfo.LCStatus.AsSlice())
	builder.AddOutput(&channelCell, disputeInfo.LCStatus.AsSlice())

	// Define the virtual channel cell output.
	vcChannelLockscript := psh.mkVirtualChannelLockScript()
	vcCell := types.CellOutput{
		Capacity: 0,
		Lock:     vcChannelLockscript,
		Type:     disputeInfo.VCTS,
	}
	if _, err := disputeInfo.updateVCStatus(); err != nil {
		return false, fmt.Errorf("updating vc status: %w", err)
	}
	vcCell.Capacity = vcCell.OccupiedCapacity(disputeInfo.VCStatus.AsSlice())
	builder.AddOutput(&vcCell, disputeInfo.VCStatus.AsSlice())
	return true, nil
}

func (psh *PerunScriptHandler) buildVCMergeTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, disputeInfo *VcMergeInfo) (bool, error) {
	if len(psh.omniLockScriptDep) != 0 {
		builder.AddCellDep(&psh.omniLockScriptDep[0])
		builder.AddCellDep(&psh.omniLockScriptDep[1])
	}
	builder.AddCellDep(&psh.vctsDep)
	builder.AddCellDep(&psh.vclsDep)
	builder.AddCellDep(&psh.vctsDep)
	builder.AddCellDep(&psh.vclsDep)
	builder.AddHeaderDep(disputeInfo.Header)

	// Inputs
	vc0InputIndex := builder.AddInput(&types.CellInput{
		PreviousOutput: &disputeInfo.VCCell0,
	})
	err := builder.SetWitness(uint(vc0InputIndex), types.WitnessTypeInputType, psh.mkWitnessVCDispute(disputeInfo.VCDispute))
	if err != nil {
		return false, err
	}

	vc1InputIndex := builder.AddInput(&types.CellInput{
		PreviousOutput: &disputeInfo.VCCell1,
	})
	err = builder.SetWitness(uint(vc1InputIndex), types.WitnessTypeInputType, psh.mkWitnessVCDispute(disputeInfo.VCDispute))
	if err != nil {
		return false, err
	}

	// Choose the virtual channel cell with the lower block number.
	vcChannelLockscript := psh.mkVirtualChannelLockScript()
	vcCell := types.CellOutput{
		Capacity: 0,
		Lock:     vcChannelLockscript,
		Type:     disputeInfo.VCTS,
	}
	var status molecule.VirtualChannelStatus
	var occupiedCapacity uint64
	if disputeInfo.BlockNum0 < disputeInfo.BlockNum1 {
		vcCell.Capacity = vcCell.OccupiedCapacity(disputeInfo.VCStatus0.AsSlice())
		builder.AddOutput(&vcCell, disputeInfo.VCStatus0.AsSlice())
		status = disputeInfo.VCStatus1
		occupiedCapacity = disputeInfo.OccupiedCapacity1
	} else {
		vcCell.Capacity = vcCell.OccupiedCapacity(disputeInfo.VCStatus1.AsSlice())
		builder.AddOutput(&vcCell, disputeInfo.VCStatus1.AsSlice())
		status = disputeInfo.VCStatus0
		occupiedCapacity = disputeInfo.OccupiedCapacity0
	}

	// Add the occupied capacity of the virtual channel cell the participant, who created the virtual channel.
	var restoredParticipant address.Participant
	err = restoredParticipant.UnpackOnChainParticipant(status.Owner())
	if err != nil {
		return false, fmt.Errorf("failed to unpack on-chain participant: %w", err)
	}
	payoutScript := restoredParticipant.PaymentScript
	paymentOutput := psh.mkPaymentOutput(payoutScript, occupiedCapacity)
	builder.AddOutput(paymentOutput, nil)
	return true, nil
}

func (psh *PerunScriptHandler) buildFirstForceCloseWithVCTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, forceCloseWithVCInfo *ForceCloseWithVCInfo) (bool, error) {
	if len(psh.omniLockScriptDep) != 0 {
		builder.AddCellDep(&psh.omniLockScriptDep[0])
		builder.AddCellDep(&psh.omniLockScriptDep[1])
	}
	builder.AddCellDep(&psh.pctsDep)
	builder.AddCellDep(&psh.pclsDep)
	builder.AddCellDep(&psh.pflsDep)
	builder.AddCellDep(&psh.vctsDep)
	builder.AddCellDep(&psh.vclsDep)
	psh.AddSudtCellDeps(builder)
	for _, h := range forceCloseWithVCInfo.Headers {
		builder.AddHeaderDep(h)
	}

	// Channel cell input.
	channelInputIndex := builder.AddInput(&types.CellInput{
		PreviousOutput: &forceCloseWithVCInfo.ChannelCell,
	})
	err := builder.SetWitness(uint(channelInputIndex), types.WitnessTypeInputType, psh.mkWitnessForceClose())
	if err != nil {
		return false, err
	}

	// Virtual channel cell input.
_:
	builder.AddInput(&types.CellInput{
		PreviousOutput: &forceCloseWithVCInfo.VCCell,
	})

	for _, assetInput := range forceCloseWithVCInfo.AssetInputs {
		builder.AddInput(&assetInput)
	}

	// Outputs
	// Add the payment output for each participant.
	for i, addr := range forceCloseWithVCInfo.Params.Parts {
		payoutScript := address.AsParticipant(addr[3]).PaymentScript
		paymentMinCapacity := payoutScript.OccupiedCapacity()
		// payout ckbytes
		balance, err := GetCKByteBalance(i, forceCloseWithVCInfo.State)
		if err != nil {
			return false, err
		}

		// Extract payout from virtual channel.
		vcBalance, err := GetCKByteBalanceFromVirtualChannel(i, forceCloseWithVCInfo.VCState, forceCloseWithVCInfo.IndexMap)
		if err != nil {
			return false, err
		}
		// Add the payout back to the original balance
		balance += vcBalance

		// The capacity of the channel's live cell is added to the balance of the first party.
		if i == 0 {
			balance += forceCloseWithVCInfo.ChannelCapacity
		}

		additionalBalance := uint64(0)
		if balance >= paymentMinCapacity {
			paymentOutput := psh.mkPaymentOutput(payoutScript, balance)
			builder.AddOutput(paymentOutput, nil)
		} else {
			additionalBalance = balance
		}

		err = psh.AddAssetsToOutputsWithVirtualChannel(builder, forceCloseWithVCInfo.State, forceCloseWithVCInfo.VCState, i, payoutScript, additionalBalance, forceCloseWithVCInfo.IndexMap)
		if err != nil {
			return false, err
		}
	}

	// VC Output
	forceCloseWithVCInfo.updateFirstForceClose()
	vcChannelLockscript := psh.mkVirtualChannelLockScript()
	vcCell := types.CellOutput{
		Capacity: 0,
		Lock:     vcChannelLockscript,
		Type:     forceCloseWithVCInfo.VCTS,
	}
	vcCell.Capacity = vcCell.OccupiedCapacity(forceCloseWithVCInfo.VCStatus.AsSlice())
	builder.AddOutput(&vcCell, forceCloseWithVCInfo.VCStatus.AsSlice())
	return true, nil
}

func (psh *PerunScriptHandler) buildSecondForceCloseWithVCTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, forceCloseWithVCInfo *ForceCloseWithVCInfo) (bool, error) {
	if len(psh.omniLockScriptDep) != 0 {
		builder.AddCellDep(&psh.omniLockScriptDep[0])
		builder.AddCellDep(&psh.omniLockScriptDep[1])
	}
	builder.AddCellDep(&psh.pctsDep)
	builder.AddCellDep(&psh.pclsDep)
	builder.AddCellDep(&psh.pflsDep)
	builder.AddCellDep(&psh.vctsDep)
	builder.AddCellDep(&psh.vclsDep)
	psh.AddSudtCellDeps(builder)
	for _, h := range forceCloseWithVCInfo.Headers {
		builder.AddHeaderDep(h)
	}

	// Channel cell input.
	channelInputIndex := builder.AddInput(&types.CellInput{
		PreviousOutput: &forceCloseWithVCInfo.ChannelCell,
	})
	err := builder.SetWitness(uint(channelInputIndex), types.WitnessTypeInputType, psh.mkWitnessForceClose())
	if err != nil {
		return false, err
	}

	// Virtual channel cell input.
	_ = builder.AddInput(&types.CellInput{
		PreviousOutput: &forceCloseWithVCInfo.VCCell,
	})
	for _, assetInput := range forceCloseWithVCInfo.AssetInputs {
		builder.AddInput(&assetInput)
	}

	// Forcefully add an input cell to unlock the lock script.
	builder.AddInput(&types.CellInput{
		Since:          0,
		PreviousOutput: forceCloseWithVCInfo.MinCKBInput,
	})

	// Return the virtual channel cacpacity to its owner.
	var restoredParticipant address.Participant
	err = restoredParticipant.UnpackOnChainParticipant(forceCloseWithVCInfo.VCStatus.Owner())
	if err != nil {
		return false, fmt.Errorf("failed to unpack on-chain participant: %w", err)
	}
	restoredPayoutScript := restoredParticipant.PaymentScript
	returnedVCBalance := false

	// Outputs
	// Add the payment output for each participant.
	for i, addr := range forceCloseWithVCInfo.Params.Parts {
		payoutScript := address.AsParticipant(addr[3]).PaymentScript
		paymentMinCapacity := payoutScript.OccupiedCapacity()
		// payout ckbytes
		balance, err := GetCKByteBalance(i, forceCloseWithVCInfo.State)
		if err != nil {
			return false, err
		}
		// Extract payout from virtual channel.
		vcBalance, err := GetCKByteBalanceFromVirtualChannel(i, forceCloseWithVCInfo.VCState, forceCloseWithVCInfo.IndexMap)
		if err != nil {
			return false, err
		}
		// Add the payout back to the original balance
		balance += vcBalance

		// The capacity of the channel's live cell is added to the balance of the first party.
		if i == 0 {
			balance += forceCloseWithVCInfo.ChannelCapacity
		}

		if restoredPayoutScript.Equals(payoutScript) {
			// The restored participant receives the virtual channel capacity.
			balance += forceCloseWithVCInfo.VirtualChannelCapacity
			returnedVCBalance = true
		}

		additionalBalance := uint64(0)
		if balance >= paymentMinCapacity {
			paymentOutput := psh.mkPaymentOutput(payoutScript, balance)
			builder.AddOutput(paymentOutput, nil)
		} else {
			additionalBalance = balance
		}

		err = psh.AddAssetsToOutputsWithVirtualChannel(builder, forceCloseWithVCInfo.State, forceCloseWithVCInfo.VCState, i, payoutScript, additionalBalance, forceCloseWithVCInfo.IndexMap)
		if err != nil {
			return false, err
		}
	}
	if !returnedVCBalance {
		paymentOutput := psh.mkPaymentOutput(restoredPayoutScript, forceCloseWithVCInfo.VirtualChannelCapacity)
		builder.AddOutput(paymentOutput, nil)
	}
	return true, nil
}

func (psh PerunScriptHandler) mkWitnessDispute(sigA, sigB molecule.Bytes) []byte {
	disputeRedeemer := molecule.NewDisputeBuilder().SigA(sigA).SigB(sigB).Build()
	witness := molecule.NewChannelWitnessBuilder().Set(molecule.ChannelWitnessUnionFromDispute(disputeRedeemer)).Build()
	return witness.AsSlice()
}

func (psh PerunScriptHandler) mkWitnessVCDispute(vcDispute *molecule.VCDispute) []byte {
	w := molecule.NewChannelWitnessBuilder().Set(molecule.ChannelWitnessUnionFromVCDispute(*vcDispute)).Build()
	return w.AsSlice()
}

func (psh PerunScriptHandler) mkChannelLockScript() *types.Script {
	return &types.Script{
		CodeHash: psh.pclsCodeHash,
		HashType: psh.pclsHashType,
	}
}

func (psh PerunScriptHandler) mkVirtualChannelLockScript() *types.Script {
	return &types.Script{
		CodeHash: psh.vclsCodeHash,
		HashType: psh.vclsHashType,
	}
}

func (psh PerunScriptHandler) mkChannelTypeScript(params *channel.Params, token molecule.ChannelToken) (*types.Script, error) {
	channelConstants, err := psh.mkChannelConstants(params, token)
	if err != nil {
		return nil, err
	}
	return &types.Script{
		CodeHash: psh.pctsCodeHash,
		HashType: psh.pctsHashType,
		Args:     channelConstants.AsSlice(),
	}, nil
}

func (psh PerunScriptHandler) mkVirtualChannelTypeScript(params *channel.Params, vcLockScript *types.Script) (*types.Script, error) {
	vcChannelConstants, err := psh.mkVirtualChannelConstants(params, vcLockScript)
	if err != nil {
		return nil, err
	}
	return &types.Script{
		CodeHash: psh.vctsCodeHash,
		HashType: psh.vctsHashType,
		Args:     vcChannelConstants.AsSlice(),
	}, nil
}

func (psh PerunScriptHandler) mkFundsLockScript(pcts *types.Script) *types.Script {
	return &types.Script{
		CodeHash: psh.pflsCodeHash,
		HashType: psh.pflsHashType,
		Args:     pcts.Hash().Bytes(),
	}
}

func (psh PerunScriptHandler) mkChannelConstants(params *channel.Params, token molecule.ChannelToken) (molecule.ChannelConstants, error) {
	chanParams, err := encoding.PackChannelParameters(params)
	if err != nil {
		return molecule.ChannelConstants{}, fmt.Errorf("packing channel parameters: %w", err)
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
		Build(), nil
}

func (psh PerunScriptHandler) mkVirtualChannelConstants(params *channel.Params, vcLockScript *types.Script) (molecule.VCChannelConstants, error) {
	chanParams, err := encoding.PackChannelParameters(params)
	if err != nil {
		return molecule.VCChannelConstants{}, fmt.Errorf("packing vc channel parameters: %w", err)
	}

	vclsHashType := psh.vclsHashType.Pack()

	return molecule.NewVCChannelConstantsBuilder().
		Params(chanParams).
		VclsCodeHash(*molecule2.PackByte32(vcLockScript.Hash())).
		VclsHashType(*vclsHashType).
		Build(), nil
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

func (psh PerunScriptHandler) mkWitnessClose(state *channel.State, paddedSigs []wallet.Sig) ([]byte, error) {
	ps, err := encoding.PackChannelState(state)
	if err != nil {
		return nil, fmt.Errorf("packing channel state: %w", err)
	}
	sigA, err := encoding.NewMoleculeSignature(paddedSigs[0])
	if err != nil {
		return nil, fmt.Errorf("packing sig A: %w", err)
	}
	sigB, err := encoding.NewMoleculeSignature(paddedSigs[1])
	if err != nil {
		return nil, fmt.Errorf("packing sig B: %w", err)
	}
	c := molecule.NewCloseBuilder().State(ps).SigA(*sigA).SigB(*sigB).Build()
	witnessClose := molecule.NewChannelWitnessBuilder().Set(molecule.ChannelWitnessUnionFromClose(c)).Build()
	return witnessClose.AsSlice(), nil
}

func (psh PerunScriptHandler) mkWitnessForceClose() []byte {
	w := molecule.NewChannelWitnessBuilder().Set(molecule.ChannelWitnessUnionFromForceClose(molecule.ForceCloseDefault())).Build()
	return w.AsSlice()
}

func GetCKByteBalance(index int, state *channel.State) (uint64, error) {
	assetIdx, ok := state.AssetIndex(asset.NewCKBytesNervosAsset())
	if !ok {
		return 0, nil
	}
	bal := state.Balances[assetIdx][index]
	if !bal.IsUint64() {
		return 0, errors.New("balance is not uint64")
	}
	return bal.Uint64(), nil
}

func GetCKByteBalanceFromVirtualChannel(index int, vcstate *channel.State, indexMap []channel.Index) (uint64, error) {
	assetIdx, ok := vcstate.AssetIndex(asset.NewCKBytesNervosAsset())
	if !ok {
		return 0, nil
	}
	bal := vcstate.Balances[assetIdx][indexMap[index]]
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
		paymentOutput, data := psh.mkAssetOutput(lock, sudtBalances, index, additionalBalance)
		additionalBalance = 0
		builder.AddOutput(paymentOutput, data)
	}
	return nil
}

func (psh PerunScriptHandler) AddAssetsToOutputsWithVirtualChannel(
	builder collector.TransactionBuilder,
	state *channel.State,
	vcstate *channel.State,
	index int,
	lock *types.Script,
	additionalBalance uint64,
	indexMap []channel.Index,
) error {
	// Merge SUDT balances from both parent and virtual channel
	parentBalances, err := encoding.GetSUDTBalancesSlice(state)
	if err != nil {
		return err
	}
	virtualBalances, err := encoding.GetSUDTBalancesSlice(vcstate)
	if err != nil {
		return err
	}

	// Validate index mapping
	if index < 0 || index >= len(indexMap) {
		return errors.New("index out of range in indexMap")
	}
	mappedIndex := int(indexMap[index])

	// Assuming parentBalances and virtualBalances are aligned in order of assets
	for i := 0; i < len(parentBalances); i++ {
		pb := parentBalances[i]

		var vb asset.SUDTBalances
		if i < len(virtualBalances) {
			vb = virtualBalances[i]
		} else {
			// No virtual balance entry for this asset
			vb = asset.SUDTBalances{
				Asset:        pb.Asset,
				Distribution: [2]uint128.Uint128{},
			}
		}

		// Validate mapped index
		if mappedIndex < 0 || mappedIndex >= len(pb.Distribution) || mappedIndex >= len(vb.Distribution) {
			return errors.New("mapped index out of range in asset distribution")
		}

		// Merge balances from parent and virtual channel
		total := pb.Distribution[mappedIndex]
		total = total.Add(vb.Distribution[index])

		if total.IsZero() {
			paymentOutput := psh.mkPaymentOutput(lock, pb.Asset.MaxCapacity+additionalBalance)
			additionalBalance = 0
			builder.AddOutput(paymentOutput, []byte{})
		} else {
			// Temporarily inject merged distribution into a synthetic balance
			merged := asset.SUDTBalances{
				Asset:        pb.Asset,
				Distribution: [2]uint128.Uint128{},
			}
			merged.Distribution[mappedIndex] = total

			paymentOutput, data := psh.mkAssetOutput(lock, merged, mappedIndex, additionalBalance)
			additionalBalance = 0
			builder.AddOutput(paymentOutput, data)
		}
	}
	return nil
}
