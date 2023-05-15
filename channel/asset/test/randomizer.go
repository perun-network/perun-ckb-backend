package test

import (
	"math/rand"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/test"
	"perun.network/perun-ckb-backend/channel/asset"
)

type Randomizer struct{}

func (*Randomizer) NewRandomAsset(*rand.Rand) channel.Asset {
	return asset.Asset
}

var _ test.Randomizer = (*Randomizer)(nil)
