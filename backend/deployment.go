package backend

import (
	"context"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

// Deployment contains all information about the deployed scripts necessary to
// use the go-perun framework. This includes the Perun scripts on their
// respective networks.
type Deployment struct {
	Network types.Network

	PCTSDep types.CellDep
	PCLSDep types.CellDep
	PFLSDep types.CellDep

	PCTSCodeHash types.Hash
	PCTSHashType types.ScriptHashType

	PCLSCodeHash types.Hash
	PCLSHashType types.ScriptHashType

	PFLSCodeHash    types.Hash
	PFLSHashType    types.ScriptHashType
	PFLSMinCapacity uint64

	DefaultLockScript    types.Script
	DefaultLockScriptDep types.CellDep

	SUDTs map[types.Hash]types.Script
}

type DeploymentConfig struct {
	DefaultLockScript types.Script
}

// MkDeployment deploys the Perun scripts on the given network using the given
// deployment configuration.
func MkDeployment(ctx context.Context, client Transactor, network types.Network, config DeploymentConfig) (*Deployment, error) {
	panic("not implemented")
}
