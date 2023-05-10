package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/encoding"
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
