package test

import (
	"math/rand"

	"perun.network/perun-ckb-backend/wallet"

	perunwallet "perun.network/go-perun/wallet"
)

type TestEphemeralWallet struct {
	wallet.EphemeralWallet
	acc *wallet.Account
}

func NewTestEphemeralWallet(acc *wallet.Account) *TestEphemeralWallet {
	return &TestEphemeralWallet{
		*wallet.NewEphemeralWallet(),
		acc,
	}
}

func (w *TestEphemeralWallet) NewRandomAccount(rng *rand.Rand) perunwallet.Account {
	return w.acc
}
