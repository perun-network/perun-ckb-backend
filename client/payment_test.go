package client_test

import (
	"math/big"
	"testing"

	"perun.network/go-perun/client"
	"perun.network/go-perun/log"
	"perun.network/go-perun/wire"
	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/channel/test"
	ctest "perun.network/perun-ckb-backend/client/test"
	"polycry.pt/poly-go/sync"
	pkgtest "polycry.pt/poly-go/test"

	clienttest "perun.network/go-perun/client/test"
)

func TestPaymentHappy(t *testing.T) {
	log.Info("Starting happy test")
	rng := pkgtest.Prng(t)

	const A, B = 0, 1 // Indices of Alice and Bob
	var (
		name = [2]string{"Alice", "Bob"}
		role [2]clienttest.Executer
	)

	s := test.NewSetup(t, rng)
	setup := ctest.MakeRoleSetups(rng, s, name[:])

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
		TxAmounts:   [2]*big.Int{asset.CKByteToShannon(big.NewFloat(5)), asset.CKByteToShannon(big.NewFloat(5))},
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

func TestPaymentDispute(t *testing.T) {
	log.Info("Starting payment dispute test")
	rng := pkgtest.Prng(t)

	const A, B = 0, 1 // Indices of Mallory and Carol
	var (
		name = [2]string{"Mallory", "Carol"}
		role [2]clienttest.Executer
	)

	s := test.NewSetup(t, rng)
	setup := ctest.MakeRoleSetups(rng, s, name[:])

	role[A] = clienttest.NewMallory(t, setup[A])
	role[B] = clienttest.NewCarol(t, setup[B])

	execConfig := &clienttest.MalloryCarolExecConfig{
		BaseExecConfig: clienttest.MakeBaseExecConfig(
			[2]wire.Address{setup[A].Identity.Address(), setup[B].Identity.Address()},
			s.Asset,
			[2]*big.Int{asset.CKByteToShannon(big.NewFloat(1000)), asset.CKByteToShannon(big.NewFloat(100))},
			client.WithoutApp(),
		),
		NumPayments: [2]int{2, 2},
		TxAmounts:   [2]*big.Int{asset.CKByteToShannon(big.NewFloat(200)), asset.CKByteToShannon(big.NewFloat(50))},
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
	log.Info("Payment dispute test done")
}
