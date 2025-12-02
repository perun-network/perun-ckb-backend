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
package test

import (
	"log"
	"math/rand"
	gpwallet "perun.network/go-perun/wallet"
	wallettest "perun.network/go-perun/wallet/test"
	"time"

	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"

	"perun.network/perun-ckb-backend/channel/test"

	gpwiretest "perun.network/go-perun/backend/sim/wire"
	clienttest "perun.network/go-perun/client/test"
	"perun.network/go-perun/watcher/local"
	"perun.network/go-perun/wire"
)

const (
	// DefaultTimeout is the default timeout for client tests.
	DefaultTimeout = 60 * time.Second
	// BlockInterval is the default block interval for the simulated chain.
	BlockInterval = 500 * time.Millisecond
	// challenge duration in blocks that is used by MakeRoleSetups.
	ChallengeDurationBlocks = 90
)

func MakeRoleSetups(rng *rand.Rand, s *test.Setup, names []string) []clienttest.RoleSetup {
	setups := make([]clienttest.RoleSetup, len(names))
	bus := wire.NewLocalBus()

	for i := 0; i < len(setups); i++ {
		watcher, err := local.NewWatcher(s.Adjs[i])
		if err != nil {
			panic("Error initializing watcher: " + err.Error())
		}

		balanceRPC, err := rpc.Dial(test.RpcNodeURL)
		if err != nil {
			panic("Error dialing RPC: " + err.Error())
		}

		errors := make(chan error)
		// Goroutine to listen for new errors and print them
		go func() {
			for err := range errors {
				if err != nil {
					log.Panicf("Error of %s: %s", names[i], err)
				}
			}
		}()

		setups[i] = clienttest.RoleSetup{
			Name:        names[i],
			Identity:    map[gpwallet.BackendID]wire.Account{3: gpwiretest.NewRandomAccount(rng)},
			Bus:         bus,
			Funder:      s.Funders[i],
			Adjudicator: s.Adjs[i],
			Watcher:     watcher,
			Wallet:      map[gpwallet.BackendID]wallettest.Wallet{3: s.EphemeralWallets[i]},
			Timeout:     DefaultTimeout,
			// Scaled due to simbackend automining progressing faster than real time.
			ChallengeDuration: ChallengeDurationBlocks * uint64(time.Second/BlockInterval),
			Errors:            errors,
			BalanceReader:     test.NewBalanceReader(balanceRPC, s.WalletAccs[i].Address()),
		}

	}

	return setups
}

func MakeRoleSetupsCross(rng *rand.Rand, s *test.SetupCross, names []string) []clienttest.RoleSetup {
	setups := make([]clienttest.RoleSetup, len(names))
	bus := wire.NewLocalBus()

	for i := 0; i < len(setups); i++ {
		watcher, err := local.NewWatcher(s.Adjs[i])
		if err != nil {
			panic("Error initializing watcher: " + err.Error())
		}

		balanceRPC, err := rpc.Dial(test.RpcNodeURL)
		if err != nil {
			panic("Error dialing RPC: " + err.Error())
		}

		errors := make(chan error)
		// Goroutine to listen for new errors and print them
		go func() {
			for err := range errors {
				if err != nil {
					log.Panicf("Error of %s: %s", names[i], err)
				}
			}
		}()

		setups[i] = clienttest.RoleSetup{
			Name:        names[i],
			Identity:    map[gpwallet.BackendID]wire.Account{3: gpwiretest.NewRandomAccount(rng)},
			Bus:         bus,
			Funder:      s.Funders[i],
			Adjudicator: s.Adjs[i],
			Watcher:     watcher,
			Wallet:      map[gpwallet.BackendID]wallettest.Wallet{3: s.EphemeralWallets[i]},
			Timeout:     DefaultTimeout,
			// Scaled due to simbackend automining progressing faster than real time.
			ChallengeDuration: ChallengeDurationBlocks * uint64(time.Second/BlockInterval),
			Errors:            errors,
			BalanceReader:     test.NewBalanceReader(balanceRPC, s.WalletAccs[i].Address()),
		}

	}

	return setups
}
