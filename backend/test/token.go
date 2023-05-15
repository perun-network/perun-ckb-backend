package test

import (
	"math/rand"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/perun-ckb-backend/backend"
)

func NewRandomToken(rng *rand.Rand) backend.Token {
	op := NewRandomOutpoint(rng)
	ct := molecule.NewChannelTokenBuilder()

	return backend.Token{
		Outpoint: *op.Pack(),
		Token:    ct.OutPoint(*op.Pack()).Build(),
	}
}

func NewRandomOutpoint(rng *rand.Rand) *types.OutPoint {
	return &types.OutPoint{
		TxHash: NewRandomHash(rng),
		Index:  rng.Uint32(),
	}
}

func NewRandomHash(rng *rand.Rand) types.Hash {
	bytes := make([]byte, 32)
	_, _ = rng.Read(bytes)
	return types.BytesToHash(bytes)
}
