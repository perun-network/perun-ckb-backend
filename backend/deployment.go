package backend

import (
	"context"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

// Deployment contains all information about the deployed scripts necessary to
// use the go-perun framework. This includes the Perun scripts on their
// respective networks.
type Deployment struct {
	Network types.Network `json:"network"`

	PCTSDep types.CellDep `json:"pcts_dep"`
	PCLSDep types.CellDep `json:"pcls_dep"`
	PFLSDep types.CellDep `json:"pfls_dep"`
	VCLSDep types.CellDep `json:"vcls_dep"`
	VCTSDep types.CellDep `json:"vcts_dep"`

	PCTSCodeHash types.Hash           `json:"pcts_code_hash"`
	PCTSHashType types.ScriptHashType `json:"pcts_hash_type"`

	PCLSCodeHash types.Hash           `json:"pcls_code_hash"`
	PCLSHashType types.ScriptHashType `json:"pcls_hash_type"`

	PFLSCodeHash    types.Hash           `json:"pfls_code_hash"`
	PFLSHashType    types.ScriptHashType `json:"pfls_hash_type"`
	PFLSMinCapacity uint64               `json:"pfls_min_capacity"`

	DefaultLockScript    types.Script         `json:"default_lock_script"`
	DefaultLockScriptDep types.CellDep        `json:"default_lock_script_dep"`
	VCTSCodeHash         types.Hash           `json:"vcts_code_hash"`
	VCTSHashType         types.ScriptHashType `json:"vcts_hash_type"`

	VCLSCodeHash types.Hash           `json:"vcls_code_hash"`
	VCLSHashType types.ScriptHashType `json:"vcls_hash_type"`

	SUDTs    map[types.Hash]types.Script  `json:"sudts"`
	SUDTDeps map[types.Hash]types.CellDep `json:"sudt_deps"`
}

type DeploymentConfig struct {
	DefaultLockScript types.Script
}

// MkDeployment deploys the Perun scripts on the given network using the given
// deployment configuration.
func MkDeployment(ctx context.Context, client Transactor, network types.Network, config DeploymentConfig) (*Deployment, error) {
	panic("not implemented")
}
