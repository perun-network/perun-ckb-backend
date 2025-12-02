// Copyright 2025 PolyCrypt GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package test

import (
	"testing"

	"github.com/stretchr/testify/require"
	btest "perun.network/perun-ckb-backend/backend/test"
	"perun.network/perun-ckb-backend/channel/asset"
	pkgtest "polycry.pt/poly-go/test"
)

func TestMarsahllingCKByte(t *testing.T) {
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

func TestMarshallingNervosAsset(t *testing.T) {
	t.Run("CKBytes", func(t *testing.T) {
		// CKByte asset with a fixed contract LID for reproducibility
		contractLid := asset.MakeContractID("03") // example hex string
		ccid := asset.MakeCCID(contractLid)
		nervosCKB := asset.NewNervosAsset(*asset.NewCKBytesAsset(), ccid)

		bytes, err := nervosCKB.MarshalBinary()
		require.NoError(t, err)

		var nervosOut asset.NervosAsset
		err = nervosOut.UnmarshalBinary(bytes)
		require.NoError(t, err)

		require.Equal(t, nervosCKB, nervosOut)
	})

	t.Run("CKBytesDirect", func(t *testing.T) {
		nervosCKB := asset.NewCKBytesNervosAsset()

		bytes, err := nervosCKB.MarshalBinary()
		require.NoError(t, err)

		var nervosOut asset.NervosAsset
		err = nervosOut.UnmarshalBinary(bytes)
		require.NoError(t, err)

		require.Equal(t, *nervosCKB, nervosOut)
	})

	t.Run("SUDT", func(t *testing.T) {
		rng := pkgtest.Prng(t)
		randomScript := btest.NewRandomScript(rng)
		sudt := asset.NewSUDT(*randomScript, 42)
		assetIn := asset.NewSUDTAsset(sudt)
		contractLid := asset.MakeContractID("03")
		ccid := asset.MakeCCID(contractLid)
		nervosSUDT := asset.NewNervosAsset(*assetIn, ccid)

		bytes, err := nervosSUDT.MarshalBinary()
		require.NoError(t, err)

		var nervosOut asset.NervosAsset
		err = nervosOut.UnmarshalBinary(bytes)
		require.NoError(t, err)

		require.Equal(t, nervosSUDT, nervosOut)
	})
}
