package test

import (
	"math/big"
	"math/rand"

	"perun.network/perun-ckb-backend/encoding"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
)

func NewRandomBalances(rng *rand.Rand) *molecule.Balances {
	dist, err := encoding.PackCKByteDistribution([2]*big.Int{big.NewInt(0).SetUint64(rng.Uint64()), big.NewInt(0).SetUint64(rng.Uint64())})
	if err != nil {
		panic(err)
	}
	dist_union := molecule.AnyBalancesUnionFromCKByteDistribution(dist)
	b := molecule.NewBalancesBuilder().Assets(molecule.NewAllocationBuilder().Push(molecule.NewAnyBalancesBuilder().Set(dist_union).Build()).Build()).Build()
	return &b
}
