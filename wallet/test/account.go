package test

import (
	"fmt"
	"perun.network/perun-ckb-backend/wallet"
)

func NewRandomAccount() *wallet.Account {
	acc, err := wallet.NewAccount()
	if err != nil {
		panic(fmt.Sprintf("generating secp256k1 private key: %v", err))
	}
	return acc
}
