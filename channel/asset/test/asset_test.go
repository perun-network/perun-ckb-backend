package test

import (
	"testing"

	"github.com/stretchr/testify/require"
	btest "perun.network/perun-ckb-backend/backend/test"
	"perun.network/perun-ckb-backend/channel/asset"
	pkgtest "polycry.pt/poly-go/test"
)

func TestMarsahllingCKByte(t *testing.T) {
	//rng := pkgtest.Prng(t)
	//randomizer := &assetTest.Randomizer{}
	assetIn := asset.NewCKBytesAsset()
	bytes, err := assetIn.MarshalBinary()
	require.NoError(t, err)
	var assetOut asset.Asset
	err = assetOut.UnmarshalBinary(bytes)
	require.NoError(t, err)
	require.Equal(t, assetIn, &assetOut)
}

func TestMarshallingSUDT(t *testing.T) {
	rng := pkgtest.Prng(t)
	randomScript := btest.NewRandomScript(rng)
	sudt := asset.NewSUDT(*randomScript, 1000)
	assetIn := asset.NewSUDTAsset(sudt)
	assetIn.SUDT.TypeScript.Args = []byte{}

	bytes, err := assetIn.MarshalBinary()
	require.NoError(t, err)

	var assetOut asset.Asset
	err = assetOut.UnmarshalBinary(bytes)
	require.NoError(t, err)
	require.Equal(t, assetIn, &assetOut)

}
