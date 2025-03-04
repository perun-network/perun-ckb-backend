package test

import "perun.network/go-perun/channel/test"

// CKBBackendID is the ID of the CKB backend.
const CKBBackendID = 3

func init() {
	test.SetRandomizer(&Randomizer{}, CKBBackendID)
}
