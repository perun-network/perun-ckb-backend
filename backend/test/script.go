package test

import (
	"math/rand"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

func NewRandomScript(rng *rand.Rand) *types.Script {
	return NewRandomScriptWithArgs(rng, nil)
}

func NewRandomScriptWithArgs(rng *rand.Rand, args []byte) *types.Script {
	return &types.Script{
		CodeHash: NewRandomHash(rng),
		HashType: types.HashTypeData,
		Args:     args,
	}
}
