package test

import (
	"math/rand"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/test"
	btest "perun.network/perun-ckb-backend/backend/test"
	"perun.network/perun-ckb-backend/channel/asset"
)

type Randomizer struct{}

func (*Randomizer) NewRandomAsset(rng *rand.Rand) channel.Asset {
	// TODO: Also allow to generate random SUDT asset.
	randomScript := btest.NewRandomScript(rng)
	assetSUDT := asset.NewSUDTAsset(&asset.SUDT{
		TypeScript:  *randomScript,
		MaxCapacity: 1000,
	})
	assetCKbyte := asset.NewCKBytesAsset()
	var chosenAsset asset.Asset
	if rng.Intn(2) == 0 {
		chosenAsset = *assetSUDT
	} else {
		chosenAsset = *assetCKbyte
	}
	return &chosenAsset
}

var _ test.Randomizer = (*Randomizer)(nil)
