package test

import "perun.network/go-perun/channel/test"

func init() {
	test.SetRandomizer(&Randomizer{})
}
