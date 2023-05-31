package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/backend"
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

// TODO: Create a helper function to easily build a PerunChannelTransaction,
// such that it uses the defaults for everything. This is especially important
// because we rely on default scripts with their respective handlers to be used
// in conjunction with this handler. Otherwise we might miss having required
// inputs/outputs autofilled by the sdk scaffolding.

func NewPerunScriptHandler(
	pctsDep, pclsDep, pflsDep types.CellDep,
	pctsCodeHash types.Hash, pctsHashType types.ScriptHashType,
	pclsCodeHash types.Hash, pclsHashType types.ScriptHashType,
	pflsCodeHash types.Hash, pflsHashType types.ScriptHashType,
	pflsMinCapacity uint64,
	defaultLockScript types.Script,
	defaultLockScriptDep types.CellDep,
) *PerunScriptHandler {
	return &PerunScriptHandler{
		pctsDep:              pctsDep,
		pclsDep:              pclsDep,
		pflsDep:              pflsDep,
		pctsCodeHash:         pctsCodeHash,
		pctsHashType:         pctsHashType,
		pclsCodeHash:         pclsCodeHash,
		pclsHashType:         pclsHashType,
		pflsCodeHash:         pflsCodeHash,
		pflsHashType:         pflsHashType,
		pflsMinCapacity:      pflsMinCapacity,
		defaultLockScript:    defaultLockScript,
		defaultLockScriptDep: defaultLockScriptDep,
	}
}

func NewPerunScriptHandlerWithDeployment(deployment backend.Deployment) *PerunScriptHandler {
	return &PerunScriptHandler{
		pctsDep: deployment.PCTSDep,
		pclsDep: deployment.PCLSDep,
		pflsDep: deployment.PFLSDep,
		// set all fields using deployment
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
	// Add required cell dependencies for Perun scripts.
	builder.AddCellDep(&psh.defaultLockScriptDep)
	builder.AddCellDep(&psh.pctsDep)
	// Add channel token as input.
	channelToken := openInfo.ChannelToken.AsCellInput()
	builder.AddInput(&channelToken)

	/// Create outputs containing channel cell and channel funds cell.

	// Channel funds cell output.
	channelTypeScript := psh.mkChannelTypeScript(openInfo.Params, openInfo.ChannelToken.Token)
	fundsLockScript := psh.mkFundsLockScript(channelTypeScript)
	channelFundsCell, fundsData := backend.CKBOutput{
		Output: molecule.NewCellOutputBuilder().
			Capacity(*types.PackUint64(psh.pflsMinCapacity)).
			Lock(*fundsLockScript.Pack()).
			Build(),
		Data: molecule.NewBytesBuilder().Build(),
	}.AsOutputAndData()
	builder.AddOutput(&channelFundsCell, fundsData)

	// Channel cell output.
	channelLockScript := psh.mkChannelLockScript()
	channelCell, channelData := openInfo.MkInitialChannelCell(*channelLockScript, *channelTypeScript).AsOutputAndData()
	builder.AddOutput(&channelCell, channelData)

	return true, nil
}

func (psh *PerunScriptHandler) buildCloseTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, closeInfo *CloseInfo) (bool, error) {
	balA, balB, err := encoding.RestrictedBalances(closeInfo.State)
	if err != nil {
		return false, err
	}
	info := newSettleInfo(
		closeInfo.ChannelInput,
		closeInfo.AssetInputs,
		closeInfo.Headers,
		closeInfo.Params.Parts,
		[2]uint64{balA, balB},
		closeInfo.ChannelCapacity,
		psh.mkWitnessClose(closeInfo.State, closeInfo.PaddedSignatures),
	)
	return psh.buildSettleTransaction(builder, group, info)
}

func (psh *PerunScriptHandler) buildAbortTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, abortInfo *AbortInfo) (bool, error) {
	info := newSettleInfo(
		abortInfo.ChannelInput,
		abortInfo.AssetInputs,
		abortInfo.Headers,
		abortInfo.Params.Parts,
		abortInfo.FundingStatus,
		abortInfo.ChannelCapacity,
		psh.mkWitnessAbort(),
	)
	return psh.buildSettleTransaction(builder, group, info)
}

func (psh *PerunScriptHandler) buildForceCloseTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, forceCloseInfo *ForceCloseInfo) (bool, error) {
	balA, balB, err := encoding.RestrictedBalances(forceCloseInfo.State)
	if err != nil {
		return false, err
	}
	info := newSettleInfo(
		forceCloseInfo.ChannelInput,
		forceCloseInfo.AssetInputs,
		forceCloseInfo.Headers,
		forceCloseInfo.Params.Parts,
		[2]uint64{balA, balB},
		forceCloseInfo.ChannelCapacity,
		psh.mkWitnessForceClose(),
	)
	return psh.buildSettleTransaction(builder, group, info)
}

func (psh *PerunScriptHandler) buildFundTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, fundInfo *FundInfo) (bool, error) {
	// Dependencies.
	builder.AddCellDep(&psh.defaultLockScriptDep)
	builder.AddCellDep(&psh.pclsDep)
	builder.AddCellDep(&psh.pctsDep)
	builder.AddHeaderDep(fundInfo.Header)

	// Channel cell input.
	channelInputIndex := builder.AddInput(&types.CellInput{
		Since:          0,
		PreviousOutput: &fundInfo.ChannelCell,
	})
	builder.SetWitness(uint(channelInputIndex), types.WitnessTypeInputType, psh.mkWitnessFund())

	// Channel cell output.
	channelLockScript := psh.mkChannelLockScript()
	channelTypeScript := psh.mkChannelTypeScript(fundInfo.Params, fundInfo.Token.Token)
	channelData := fundInfo.AddFundingToStatus()
	channelCell := types.CellOutput{
		Capacity: 0,
		Lock:     channelLockScript,
		Type:     channelTypeScript,
	}
	channelCell.Capacity = channelCell.OccupiedCapacity(channelData.AsSlice())
	builder.AddOutput(&channelCell, channelData.AsSlice())

	// Channel funds cell output.
	fundsLockScript := psh.mkFundsLockScript(channelTypeScript)
	builder.AddOutput(fundInfo.MkFundsCell(fundsLockScript), []byte{})
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
	builder.SetWitness(uint(channelInputIndex), types.WitnessTypeInputType, psh.mkWitnessDispute(disputeInfo.SigA, disputeInfo.SigB))

	// Channel cell output.
	channelLockScript := psh.mkChannelLockScript()
	channelTypeScript := psh.mkChannelTypeScript(disputeInfo.Params, disputeInfo.Token)
	channelCell := types.CellOutput{
		Capacity: 0,
		Lock:     channelLockScript,
		Type:     channelTypeScript,
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
	channelArgs := types.PackBytes(channelConstants.AsSlice())
	return &types.Script{
		CodeHash: psh.pctsCodeHash,
		HashType: psh.pctsHashType,
		Args:     channelArgs.AsSlice(),
	}
}

func (psh PerunScriptHandler) mkFundsLockScript(pcts *types.Script) *types.Script {
	fundsArgs := types.PackBytes(pcts.Hash().Bytes())
	return &types.Script{
		CodeHash: psh.pflsCodeHash,
		HashType: psh.pflsHashType,
		Args:     fundsArgs.AsSlice(),
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

func (psh PerunScriptHandler) buildSettleTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, info *settleInfo) (bool, error) {
	// TODO: How do we make sure that we unlock the channel?

	builder.AddCellDep(&psh.pctsDep)
	builder.AddCellDep(&psh.pclsDep)
	builder.AddCellDep(&psh.pflsDep)

	idx := builder.AddInput(&info.channelInput)
	for _, assetInput := range info.assetInputs {
		builder.AddInput(&assetInput)
	}

	for _, h := range info.headers {
		builder.AddHeaderDep(h)
	}

	// Add the payment output for each participant.
	for i, addr := range info.parties {
		payoutScript := address.AsParticipant(addr).PaymentScript
		paymentMinCapacity := payoutScript.OccupiedCapacity()
		balance := info.payout[i]
		// The capacity of the channel's live cell is added to the balance of the first party.
		if i == 0 {
			balance += info.channelCapacity
		}
		if balance >= paymentMinCapacity {
			paymentOutput := psh.mkPaymentOutput(payoutScript, balance)
			builder.AddOutput(paymentOutput, nil)
		}
	}
	err := builder.SetWitness(uint(idx), types.WitnessTypeInputType, info.witness)
	if err != nil {
		return false, err
	}
	return true, nil
}

type settleInfo struct {
	channelInput    types.CellInput
	assetInputs     []types.CellInput
	headers         []types.Hash
	parties         []wallet.Address
	payout          [2]uint64
	channelCapacity uint64
	witness         []byte
}

func newSettleInfo(channelInput types.CellInput, assetInputs []types.CellInput, headers []types.Hash, parties []wallet.Address, payout [2]uint64, channelCapacity uint64, witness []byte) *settleInfo {
	return &settleInfo{
		channelInput:    channelInput,
		assetInputs:     assetInputs,
		headers:         headers,
		parties:         parties,
		payout:          payout,
		channelCapacity: channelCapacity,
		witness:         witness,
	}
}
