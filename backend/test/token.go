package test

import (
	"math/rand"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/perun-ckb-backend/backend"
)

func NewRandomToken(rng *rand.Rand, opts ...TokenOpt) backend.Token {
	op := NewRandomOutpoint(rng)
	ct := molecule.NewChannelTokenBuilder()

	bt := &backend.Token{
		Outpoint: *op.Pack(),
		Token:    ct.OutPoint(*op.Pack()).Build(),
	}

	for _, opt := range opts {
		opt(bt)
	}

	return *bt
}

type TokenOpt func(*backend.Token)

func WithOutpoint(op types.OutPoint) TokenOpt {
	return func(t *backend.Token) {
		ct := molecule.NewChannelTokenBuilder()
		t.Idx = op.Index
		t.Outpoint = *op.Pack()
		t.Token = ct.OutPoint(*op.Pack()).Build()
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

func NewRandomHashType(rng *rand.Rand) types.ScriptHashType {
	types := []types.ScriptHashType{
		types.HashTypeData,
		types.HashTypeType,
		types.HashTypeData1,
	}

	return types[rng.Intn(len(types))]
}
