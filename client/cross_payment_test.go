// Copyright 2025 PolyCrypt GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package client_test

import (
	"context"
	"github.com/sirupsen/logrus"
	"math/big"
	"math/rand"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/client"
	"perun.network/go-perun/log"
	"perun.network/go-perun/wallet"
	"perun.network/go-perun/wire"
	"perun.network/perun-ckb-backend/channel/asset"
	btest "perun.network/perun-ckb-backend/channel/test"
	ctest "perun.network/perun-ckb-backend/client/test"
	"perun.network/perun-ckb-backend/transaction"
	"polycry.pt/poly-go/sync"
	pkgtest "polycry.pt/poly-go/test"
	"testing"
	"time"

	clienttest "perun.network/go-perun/client/test"
)

// TestCrossPaymentHappy tests the happy path of the payment channel.
// It creates a payment channel between Alice and Bob, and then performs a series of payments.
// The test checks if the final balances are as expected and if the channel state is updated correctly.
// The test also checks if the payment channel is closed correctly.
func TestCrossPaymentHappy(t *testing.T) {
	log.Info("Starting happy test")
	rng := pkgtest.Prng(t)

	const A, B = 0, 1 // Indices of Alice and Bob
	var (
		name = [2]string{"Alice", "Bob"}
		role [2]clienttest.Executer
	)

	s := btest.NewCrossSetup(t, rng, true)
	setup := ctest.MakeRoleSetupsCross(rng, s, name[:])

	role[A] = clienttest.NewAlice(t, setup[A])
	role[B] = clienttest.NewBob(t, setup[B])

	balAlice := setup[A].BalanceReader.Balance(s.CkbAsset)
	balBob := setup[B].BalanceReader.Balance(s.CkbAsset)

	logrus.Printf("Initial Balances - Alice: %s, Bob: %s", balAlice.String(), balBob.String())
	balAlice = setup[A].BalanceReader.Balance(s.SudtAsset)
	balBob = setup[B].BalanceReader.Balance(s.SudtAsset)

	logrus.Printf("Initial SUDT Balances - Alice: %s, Bob: %s", balAlice.String(), balBob.String())
	// enable stages synchronization
	stages := role[A].EnableStages()
	role[B].SetStages(stages)

	execConfig := &clienttest.AliceBobExecConfig{
		BaseExecConfig: clienttest.MakeBaseExecConfig(
			[2]map[wallet.BackendID]wire.Address{{3: setup[A].Identity[3].Address()}, {3: setup[B].Identity[3].Address()}},
			[]channel.Asset{s.CkbAsset, s.SudtAsset},
			[]wallet.BackendID{3, 3},
			[][2]*big.Int{
				{asset.CKByteToShannon(big.NewFloat(80)), asset.CKByteToShannon(big.NewFloat(100))},
				{new(big.Int).SetUint64(uint64(2)), new(big.Int).SetUint64(uint64(1))},
			},
			client.WithoutApp(),
		),
		NumPayments: [2]int{2, 2},
		TxAmounts:   [2]*big.Int{asset.CKByteToShannon(big.NewFloat(2)), asset.CKByteToShannon(big.NewFloat(2))},
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
	balAlice = setup[A].BalanceReader.Balance(s.CkbAsset)
	balBob = setup[B].BalanceReader.Balance(s.CkbAsset)

	logrus.Printf("Initial Balances - Alice: %s, Bob: %s", balAlice.String(), balBob.String())
	balAlice = setup[A].BalanceReader.Balance(s.SudtAsset)
	balBob = setup[B].BalanceReader.Balance(s.SudtAsset)
	logrus.Printf("Initial SUDT Balances - Alice: %s, Bob: %s", balAlice.String(), balBob.String())

	logrus.Info("Happy test done")
}

// TestCrossPaymentDispute tests the payment dispute scenario.
// It creates a payment channel between Alice and Bob, and then disputes the
// channel state. The test checks if the dispute is resolved correctly and the final balances
// are as expected.
func TestCrossPaymentDispute(t *testing.T) {
	log.Info("Starting payment dispute test")
	rng := pkgtest.Prng(t)

	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	setup := makeCrossPaymentChannelSetup(t, rng)
	clienttest.TestPaymentChannelDispute(ctx, t, setup)
	log.Info("Payment dispute test done")
}

func makeCrossPaymentChannelSetup(t *testing.T, rng *rand.Rand) clienttest.PaymentChannelSetup {
	t.Helper()
	name := [2]string{"Alice", "Bob"}
	setup := btest.NewCrossSetup(t, rng, false)

	roleSetup := ctest.MakeRoleSetupsCross(rng, setup, name[:])

	return clienttest.PaymentChannelSetup{
		Clients:           [2]clienttest.RoleSetup(roleSetup),
		ChallengeDuration: roleSetup[0].ChallengeDuration,
		Asset:             setup.CkbAsset,
		Balances: clienttest.PaymentChannelBalances{
			InitBalsAliceBob: []*big.Int{asset.CKByteToShannon(big.NewFloat(100)), asset.CKByteToShannon(big.NewFloat(100))},
			BalsUpdated:      []*big.Int{asset.CKByteToShannon(big.NewFloat(70)), asset.CKByteToShannon(big.NewFloat(130))},
			FinalBals:        []*big.Int{asset.CKByteToShannon(big.NewFloat(70)), asset.CKByteToShannon(big.NewFloat(130))},
		},
		BalanceDelta:       big.NewInt(int64(3 * transaction.DefaultFeeShannon)), // Max Fee: (Open + Dispute + Close) * 1 CKB
		Rng:                rng,
		WaitWatcherTimeout: 1 * time.Second,
		IsUTXO:             true,
	}
}
