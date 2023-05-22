package test

import (
	"fmt"
	"math/rand"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/encoding"
)

type ChannelStatusOpt func(*molecule.ChannelStatusBuilder)

func NewRandomChannelStatus(rng *rand.Rand, opts ...ChannelStatusOpt) *molecule.ChannelStatus {
	s := molecule.NewChannelStatusBuilder()
	s.Funded(encoding.FromBool(rng.Intn(2) == 1))
	s.Disputed(encoding.FromBool(rng.Intn(2) == 1))
	s.Funding(*NewRandomBalances(rng))

	for _, opt := range opts {
		opt(s)
	}

	cs := s.Build()
	return &cs
}

func WithState(state *channel.State) ChannelStatusOpt {
	return func(cs *molecule.ChannelStatusBuilder) {
		encodedCs, err := encoding.PackChannelState(state)
		if err != nil {
			panic(fmt.Sprintf("packing channel state: %v", err))
		}
		cs.State(encodedCs)
	}
}

func WithFunding(fs molecule.Balances) ChannelStatusOpt {
	return func(cs *molecule.ChannelStatusBuilder) {
		cs.Funding(fs)
	}
}
