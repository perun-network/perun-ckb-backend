package test

import (
	"math/rand"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/test"
	"perun.network/perun-ckb-backend/channel/asset"
)

type Randomizer struct{}

func (*Randomizer) NewRandomAsset(*rand.Rand) channel.Asset {
	// TODO: Also allow to generate random SUDT asset.
	return asset.CKBAsset
}

var _ test.Randomizer = (*Randomizer)(nil)
