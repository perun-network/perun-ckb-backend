package test

import (
	"testing"

	"github.com/stretchr/testify/require"
	btest "perun.network/perun-ckb-backend/backend/test"
	"perun.network/perun-ckb-backend/channel/asset"
	ptest "polycry.pt/poly-go/test"
)

func TestMarshalling(t *testing.T) {
	assetIn := asset.CKBAsset
	bytes, err := assetIn.MarshalBinary()
	require.NoError(t, err)
	assetOut := *asset.CKBAsset
	err = assetOut.UnmarshalBinary(bytes)
	require.NoError(t, err)

	require.Equal(t, assetIn, &assetOut)
}

func TestMarshalling_SUDTAsset(t *testing.T) {
	rng := ptest.Prng(t)
	sudtTypeScript := btest.NewRandomScript(rng)
	assetIn := asset.SUDTAsset{
		TypeScript:  *sudtTypeScript,
		MaxCapacity: 100,
	}
	if assetIn.TypeScript.Args == nil {
		assetIn.TypeScript.Args = []byte{}
	}

	bytes, err := assetIn.MarshalBinary()
	require.NoError(t, err)
	var assetOut asset.SUDTAsset
	err = assetOut.UnmarshalBinary(bytes)
	require.NoError(t, err)

	require.Equal(t, assetIn, assetOut)
}
