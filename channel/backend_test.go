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
				Append(gptest.WithApp(channel.NewDefaultTempApp())).
				Append(gptest.WithAssets(asset.NewCKBytesAsset())).
				Append(gptest.WithBackend(channel.CKBBackendID)).
				Append(gptest.WithBackendIDs([]gpwallet.BackendID{channel.CKBBackendID})).
				Append(gptest.WithBalancesInRange(big.NewInt(0).Mul(big.NewInt(100), big.NewInt(100_000_000)), big.NewInt(0).Mul(big.NewInt(10_000), big.NewInt(100_000_000)))).
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
