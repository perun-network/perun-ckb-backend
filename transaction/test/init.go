package test

import (
	"perun.network/go-perun/wallet/test"
	_ "perun.network/perun-ckb-backend/channel"
	_ "perun.network/perun-ckb-backend/channel/asset/test"
	wtest "perun.network/perun-ckb-backend/wallet/test"
)

func init() {
	test.SetRandomizer(&wtest.Randomizer{})
}
