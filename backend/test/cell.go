package test

import (
	"math/rand"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

func NewRandomCellDep(rng *rand.Rand) *types.CellDep {
	return &types.CellDep{
		OutPoint: NewRandomOutpoint(rng),
		DepType:  types.DepTypeCode,
	}
}
