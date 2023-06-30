package test

import (
	"math/rand"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/perun-ckb-backend/backend"
)

func NewRandomDeployment(rng *rand.Rand, opt ...DeploymentOpt) *backend.Deployment {
	d := &backend.Deployment{
		Network:              types.NetworkTest,
		PCTSDep:              *NewRandomCellDep(rng),
		PCLSDep:              *NewRandomCellDep(rng),
		PFLSDep:              *NewRandomCellDep(rng),
		PCTSCodeHash:         NewRandomHash(rng),
		PCTSHashType:         NewRandomHashType(rng),
		PCLSCodeHash:         NewRandomHash(rng),
		PCLSHashType:         NewRandomHashType(rng),
		PFLSCodeHash:         NewRandomHash(rng),
		PFLSHashType:         NewRandomHashType(rng),
		PFLSMinCapacity:      uint64(rng.Intn(10*10 ^ 8)),
		DefaultLockScript:    *NewRandomScript(rng),
		DefaultLockScriptDep: *NewRandomCellDep(rng),
	}
	for _, o := range opt {
		o(d)
	}
	return d
}

type DeploymentOpt func(*backend.Deployment)

func WithNetwork(network types.Network) DeploymentOpt {
	return func(d *backend.Deployment) {
		d.Network = network
	}
}

func WithPCTS(h types.Hash, dep types.CellDep, t types.ScriptHashType) DeploymentOpt {
	return func(d *backend.Deployment) {
		d.PCTSCodeHash = h
		d.PCTSDep = dep
		d.PCTSHashType = t
	}
}

func WithPCLS(h types.Hash, dep types.CellDep, t types.ScriptHashType) DeploymentOpt {
	return func(d *backend.Deployment) {
		d.PCLSCodeHash = h
		d.PCLSDep = dep
		d.PCLSHashType = t
	}
}

func WithPFLS(h types.Hash, dep types.CellDep, t types.ScriptHashType) DeploymentOpt {
	return func(d *backend.Deployment) {
		d.PFLSCodeHash = h
		d.PFLSDep = dep
		d.PFLSHashType = t
	}
}

func WithDefaultLockScript(s types.Script, dep types.CellDep) DeploymentOpt {
	return func(d *backend.Deployment) {
		d.DefaultLockScript = s
		d.DefaultLockScriptDep = dep
	}
}
