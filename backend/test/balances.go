package test

import (
	"math/rand"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
)

func NewRandomBalances(rng *rand.Rand) *molecule.Balances {
	bs := molecule.NewBalancesBuilder()
	bs.Nth0(*types.PackUint64(rng.Uint64()))
	bs.Nth1(*types.PackUint64(rng.Uint64()))
	b := bs.Build()
	return &b
}
