package transaction

import (
	"errors"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/channel/defaults"
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

	pctsCodeHash types.Hash
	pctsHashType types.ScriptHashType
	pclsCodeHash types.Hash
	pclsHashType types.ScriptHashType
	pflsCodeHash types.Hash
	pflsHashType types.ScriptHashType

	defaultLockScript types.Script
}

var _ collector.ScriptHandler = (*PerunScriptHandler)(nil)

// TODO: Create a helper function to easily build a PerunChannelTransaction,
// such that it uses the defaults for everything. This is especially important
// because we rely on default scripts with their respective handlers to be used
// in conjunction with this handler. Otherwise we might miss having required
// inputs/outputs autofilled by the sdk scaffolding.

func NewPerunScriptHandler(pctsDep, pclsDep, pflsDep types.CellDep) *PerunScriptHandler {
	return &PerunScriptHandler{
		pctsDep: pctsDep,
		pclsDep: pclsDep,
		pflsDep: pflsDep,
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
	case CloseInfo, *CloseInfo:
		var closeInfo *CloseInfo
		if closeInfo, ok = context.(*CloseInfo); !ok {
			v, _ := context.(CloseInfo)
			closeInfo = &v
		}
		return psh.buildCloseTransaction(builder, group, closeInfo)
	default:
	}
	return ok, nil
}

func (psh *PerunScriptHandler) buildOpenTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, openInfo *OpenInfo) (bool, error) {
	// Add required cell dependencies for Perun scripts.
	builder.AddCellDep(&psh.pctsDep)
	// Add channel token as input.
	channelToken := openInfo.ChannelToken.AsCellInput()
	builder.AddInput(&channelToken)

	/// Create outputs containing channel cell and channel funds cell.

	// Channel funds cell output.
	pcts := psh.mkChannelTypeScript(openInfo.Params, openInfo.ChannelToken)
	fundsLockScript := psh.mkFundsLockScript(pcts)
	channelFundsCell, fundsData := backend.CKBOutput{
		Output: molecule.NewCellOutputBuilder().
			Capacity(*types.PackUint64(openInfo.MinFunding())).
			Lock(*fundsLockScript.Pack()).
			Build(),
		Data: molecule.NewBytesBuilder().Build(),
	}.AsOutputAndData()
	builder.AddOutput(&channelFundsCell, fundsData)

	// Channel cell output.
	channelLockScript := psh.mkChannelLockScript()
	channelTypeScript := psh.mkChannelTypeScript(openInfo.Params, openInfo.ChannelToken)
	channelCell, channelData := openInfo.MkInitialChannelCell(*channelLockScript, *channelTypeScript).AsOutputAndData()
	builder.AddOutput(&channelCell, channelData)

	return true, nil
}

func (psh *PerunScriptHandler) buildCloseTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, closeInfo *CloseInfo) (bool, error) {
	// TODO: How do we make sure that we unlock the channel?
	// Add required cell dependencies for Perun scripts.
	builder.AddCellDep(&psh.pctsDep)
	builder.AddCellDep(&psh.pclsDep)
	builder.AddCellDep(&psh.pflsDep)
	// Add the live channel cell as input.
	builder.AddInput(&closeInfo.ChannelInput)
	// Add all the funds locked in this channel as inputs.
	for _, assetInput := range closeInfo.AssetInputs {
		builder.AddInput(&assetInput)
	}
	// Add the payment output for each participant.
	for i, addr := range closeInfo.Params.Parts {
		payoutMethod, err := defaults.KnownPayoutPreimage(addr)
		if err != nil {
			return false, err
		}
		participant, ok := addr.(*address.Participant)
		if !ok {
			return false, errors.New("invalid address type")
		}
		var payoutScript *types.Script
		switch payoutMethod {
		case defaults.Secp256k1Blake160SighashAll:
			payoutScript, err = participant.GetSecp256k1Blake160SighashAll()
			if err != nil {
				return false, err
			}
		default:
			return false, errors.New("unknown payout method for participant")
		}
		balance := closeInfo.State.Balance(channel.Index(i), asset.Asset)
		if !balance.IsUint64() {
			return false, errors.New("balance is not a uint64")
		}
		bal := balance.Uint64()
		// The capacity of the channel's live cell is added to the balance of the first party.
		if i == 0 {
			bal += closeInfo.ChannelCapacity
		}
		if bal >= participant.PaymentMinCapacity {
			paymentOutput := psh.mkPaymentOutput(payoutScript, bal)
			builder.AddOutput(paymentOutput, nil)
		}
	}
	// TODO: What index to use? We use 0 for now, because we add the channel input first.
	err := builder.SetWitness(0, types.WitnessTypeInputType, psh.mkWitnessClose(closeInfo.State, closeInfo.PaddedSignatures))
	if err != nil {
		return false, err
	}
	return true, nil
}

func (psh PerunScriptHandler) mkChannelLockScript() *types.Script {
	return &types.Script{
		CodeHash: psh.pclsCodeHash,
		HashType: psh.pclsHashType,
	}
}

func (psh PerunScriptHandler) mkChannelTypeScript(params *channel.Params, token backend.Token) *types.Script {
	channelConstants := psh.mkChannelConstants(params, token.Token)
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
	pflsMinCapacity := backend.MinCapacityForPFLS()

	return molecule.NewChannelConstantsBuilder().
		Params(chanParams).
		PclsCodeHash(*pclsCode).
		PclsHashType(*pclsHashType).
		PflsCodeHash(*pflsCode).
		PflsHashType(*pflsHashType).
		PflsMinCapacity(*types.PackUint64(pflsMinCapacity)).
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
	witnessClose := molecule.NewCloseBuilder().State(ps).SigA(*sigA).SigB(*sigB).Build()
	return witnessClose.AsSlice()
}
