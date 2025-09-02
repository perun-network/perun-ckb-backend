package client_test

import (
	"math/big"
	"os"
	"testing"

	"perun.network/go-perun/client"
	clienttest "perun.network/go-perun/client/test"
	"perun.network/go-perun/log"
	"perun.network/go-perun/wire"
	"perun.network/perun-ckb-backend/channel/asset"
	btest "perun.network/perun-ckb-backend/channel/test"
	ctest "perun.network/perun-ckb-backend/client/test"
	"polycry.pt/poly-go/sync"
	pkgtest "polycry.pt/poly-go/test"
)

// TestPaymentHappy tests the happy path of the payment channel.
// It creates a payment channel between Alice and Bob, and then performs a series of payments.
// The test checks if the final balances are as expected and if the channel state is updated correctly.
// The test also checks if the payment channel is closed correctly.
func TestTestnetPaymentHappy(t *testing.T) {
	if !(os.Getenv("TESTNET") == "true") {
		t.Skip("Skipping testnet test")
	}

	log.Info("Starting happy test")
	rng := pkgtest.Prng(t)

	const A, B = 0, 1 // Indices of Alice and Bob
	var (
		name = [2]string{"Alice", "Bob"}
		role [2]clienttest.Executer
	)

	s := btest.NewTestnetLedgerChannelSetup(t, rng)
	setup := ctest.MakeRoleSetups(rng, s, name[:], true)

	role[A] = clienttest.NewAlice(t, setup[A])
	role[B] = clienttest.NewBob(t, setup[B])

	// enable stages synchronization
	stages := role[A].EnableStages()
	role[B].SetStages(stages)

	execConfig := &clienttest.AliceBobExecConfig{
		BaseExecConfig: clienttest.MakeBaseExecConfig(
			[2]wire.Address{setup[A].Identity.Address(), setup[B].Identity.Address()},
			s.Asset,
			[2]*big.Int{asset.CKByteToShannon(big.NewFloat(100)), asset.CKByteToShannon(big.NewFloat(100))},
			client.WithoutApp(),
		),
		NumPayments: [2]int{2, 2},
		TxAmounts:   [2]*big.Int{asset.CKByteToShannon(big.NewFloat(1)), asset.CKByteToShannon(big.NewFloat(1))},
	}

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			defer wg.Done()
			log.Infof("Starting %s.Execute", name[i])
			role[i].Execute(execConfig)
		}(i)
	}

	wg.Wait()

	log.Info("Happy test done")
}
