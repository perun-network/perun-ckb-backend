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
package channel_test

import (
	"math/big"
	"math/rand"
	"testing"

	gpchannel "perun.network/go-perun/channel"
	gptest "perun.network/go-perun/channel/test"
	gpwallet "perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/channel"
	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/wallet"
	pkgtest "polycry.pt/poly-go/test"
)

func setup(rng *rand.Rand) *gptest.Setup {
	getRandomAddress := func() map[gpwallet.BackendID]gpwallet.Address {
		acc, err := wallet.NewAccount()
		if err != nil {
			panic(err)
		}
		return map[gpwallet.BackendID]gpwallet.Address{channel.CKBBackendID: acc.Address()}
	}
	newParamsAndState := func(opts ...gptest.RandomOpt) (*gpchannel.Params, *gpchannel.State) {
		return gptest.NewRandomParamsAndState(
			rng,
			gptest.WithoutApp().
				Append(gptest.WithParts([]map[gpwallet.BackendID]gpwallet.Address{getRandomAddress(), getRandomAddress()})).
				Append(gptest.WithLedgerChannel(true)).
				Append(gptest.WithVirtualChannel(false)).
				Append(gptest.WithAssets(asset.NewCKBytesNervosAsset())).
				Append(gptest.WithBackend(channel.CKBBackendID)).
				Append(gptest.WithBackendIDs([]gpwallet.BackendID{channel.CKBBackendID})).
				Append(gptest.WithBalancesInRange(
					new(big.Int).SetUint64(0),
					channel.MaxBalance,
				)).
				Append(opts...),
		)
	}
	acc, err := wallet.NewAccount()
	if err != nil {
		panic(err)
	}

	p1, s1 := newParamsAndState()
	p2, s2 := newParamsAndState(gptest.WithIsFinal(!s1.IsFinal))
	return &gptest.Setup{
		Params:        p1,
		Params2:       p2,
		State:         s1,
		State2:        s2,
		Account:       acc,
		RandomAddress: getRandomAddress,
	}
}

func TestBackend(t *testing.T) {
	rng := pkgtest.Prng(t)
	gptest.GenericBackendTest(t, setup(rng), gptest.IgnoreApp, gptest.IgnoreAssets)
}
