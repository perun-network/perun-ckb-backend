package channel_test

import (
	"math/big"
	"math/rand"
	gpchannel "perun.network/go-perun/channel"
	gptest "perun.network/go-perun/channel/test"
	gpwallet "perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/channel"
	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/wallet"
	pkgtest "polycry.pt/poly-go/test"
	"testing"
)

func setup(rng *rand.Rand) *gptest.Setup {
	getRandomAddress := func() gpwallet.Address {
		acc, err := wallet.NewAccount()
		if err != nil {
			panic(err)
		}
		return acc.Address()
	}
	newParamsAndState := func(opts ...gptest.RandomOpt) (*gpchannel.Params, *gpchannel.State) {
		return gptest.NewRandomParamsAndState(
			rng,
			gptest.WithoutApp().
				Append(gptest.WithParts(getRandomAddress(), getRandomAddress())).
				Append(gptest.WithLedgerChannel(true)).
				Append(gptest.WithVirtualChannel(false)).
				Append(gptest.WithAssets(asset.CKBAsset)).
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
