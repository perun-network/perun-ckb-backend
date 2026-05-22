package ckblp

import (
	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

// LPDeployment holds deployed LP script metadata.
type LPDeployment struct {
	TypeScriptDep      types.CellDep
	LockScriptDep      types.CellDep
	TypeScriptCodeHash types.Hash
	TypeScriptHashType types.ScriptHashType
	LockScriptCodeHash types.Hash
	LockScriptHashType types.ScriptHashType
}

// LPPolicy mirrors perun-common pool policy fields.
type LPPolicy struct {
	MaxTradingVolume uint64
	FeeRateBps       uint32
	PolicyFlags      uint32
	PolicyVersion    uint32
	SafePriceMinX64  uint128.Uint128
	SafePriceMaxX64  uint128.Uint128
}

// LPCell mirrors perun-common LP cell data layout.
type LPCell struct {
	PoolID                  [32]byte
	OwnerLockHash           [32]byte
	OperatorLockHash        [32]byte
	AvailableCKB            uint64
	ReservedCKB             uint64
	CumulativeFeesEarnedCKB uint64
	Policy                  LPPolicy
	Nonce                   uint64
	Active                  bool
}

// LPCellInfo is a lightweight view of an LP cell returned by discovery.
type LPCellInfo struct {
	Cell        LPCell
	Capacity    uint64
	OutPointHex string
}

