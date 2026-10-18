package backend_test

import (
	"testing"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/xudt"
)

func TestTokenScripts(t *testing.T) {
	script := types.Script{HashType: types.HashTypeData1, Args: make([]byte, 32)}
	hash := script.Hash()
	config := xudt.Config{Script: script, CellDeps: []types.CellDep{{OutPoint: &types.OutPoint{}, DepType: types.DepTypeCode}}}
	d := backend.Deployment{XUDTs: map[types.Hash]xudt.Config{hash: config}}
	scripts, err := d.TokenScripts()
	require.NoError(t, err)
	require.Equal(t, script, scripts[hash])
	delete(scripts, hash)
	require.Len(t, d.XUDTs, 1)
	d.SUDTs = map[types.Hash]types.Script{hash: script}
	_, err = d.TokenScripts()
	require.ErrorContains(t, err, "both")
	d.SUDTs = nil
	delete(d.XUDTs, hash)
	d.XUDTs[types.Hash{}] = config
	_, err = d.TokenScripts()
	require.ErrorContains(t, err, "map key")
}

func TestInvalidTokenHashTypeReturnsError(t *testing.T) {
	for _, d := range []backend.Deployment{
		{XUDTs: map[types.Hash]xudt.Config{{}: {Script: types.Script{HashType: "invalid"}}}},
		{SUDTs: map[types.Hash]types.Script{{}: {HashType: "invalid"}}},
	} {
		_, err := d.TokenScripts()
		require.ErrorContains(t, err, "hash type")
	}
}
