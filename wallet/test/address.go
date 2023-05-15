package test

import (
	"fmt"
	"math/rand"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	btest "perun.network/perun-ckb-backend/backend/test"
	"perun.network/perun-ckb-backend/wallet/address"
)

func NewRandomParticipant(rng *rand.Rand) *address.Participant {
	acc, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		panic(fmt.Sprintf("Generating private keys for participant: %v", err, err))
	}
	paymentScript := btest.NewRandomScript(rng)
	return &address.Participant{
		PubKey:             acc.PubKey(),
		PaymentScriptHash:  paymentScript.Hash(),
		UnlockScriptHash:   btest.NewRandomScript(rng).Hash(),
		PaymentMinCapacity: paymentScript.OccupiedCapacity(),
	}
}
