package test

import (
	"testing"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
	"perun.network/perun-ckb-backend/channel/asset"
)

func TestXUDTAssetRoundTrip(t *testing.T) {
	script := types.Script{CodeHash: types.HexToHash("0x1234"), HashType: types.HashTypeData1, Args: make([]byte, 36)}
	a, err := asset.NewXUDTAsset(script, 400*100_000_000)
	require.NoError(t, err)
	encoded, err := a.MarshalBinary()
	require.NoError(t, err)
	decoded := asset.NewInvalidAsset()
	require.NoError(t, decoded.UnmarshalBinary(encoded))
	require.True(t, a.Equal(decoded))
	require.Equal(t, script, decoded.SUDT.TypeScript)
}
