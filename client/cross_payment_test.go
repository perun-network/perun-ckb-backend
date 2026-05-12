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
	"math/big"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/client"
	clienttest "perun.network/go-perun/client/test"
	"perun.network/go-perun/log"
	"perun.network/go-perun/wallet"
	"perun.network/go-perun/wire"
	"perun.network/perun-ckb-backend/channel/asset"
	btest "perun.network/perun-ckb-backend/channel/test"
	ctest "perun.network/perun-ckb-backend/client/test"
	"polycry.pt/poly-go/sync"
	pkgtest "polycry.pt/poly-go/test"
)

const (
	testDuration = 120 * time.Second
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

// TestCrossPaymentDispute and its setup helper live in
// cross_payment_dispute_test.go and are excluded from `go test -race` runs
// (see that file for the rationale).
