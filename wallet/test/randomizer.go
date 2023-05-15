package test

import (
	"math/rand"
	"perun.network/go-perun/wallet"
	"perun.network/go-perun/wallet/test"
	ckbwallet "perun.network/perun-ckb-backend/wallet"
)

type Randomizer struct {
	Account *ckbwallet.Account
}

// NewRandomAddress implements test.Randomizer
func (r *Randomizer) NewRandomAddress(rng *rand.Rand) wallet.Address {
	return NewRandomParticipant(rng)
}

// NewWallet implements test.Randomizer
func (*Randomizer) NewWallet() test.Wallet {
	panic("unimplemented")
}

// RandomWallet implements test.Randomizer
func (*Randomizer) RandomWallet() test.Wallet {
	panic("unimplemented")
}

var _ test.Randomizer = (*Randomizer)(nil)
