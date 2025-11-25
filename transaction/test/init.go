package test

import (
	"perun.network/go-perun/wallet/test"
	_ "perun.network/perun-ckb-backend/channel"
	_ "perun.network/perun-ckb-backend/channel/asset/test"
	wtest "perun.network/perun-ckb-backend/wallet/test"
)

// CKBBackendID is the ID of the CKB backend.
const CKBBackendID = 3

func init() {
	test.SetRandomizer(&wtest.Randomizer{}, CKBBackendID)
}
