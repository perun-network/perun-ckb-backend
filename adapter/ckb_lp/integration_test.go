//go:build devnet
// +build devnet

package ckblp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
	"perun.network/perun-ckb-backend/backend"
	ckbtest "perun.network/perun-ckb-backend/channel/test"
)

type lpMigration struct {
	CellRecipes []struct {
		Name     string `json:"name"`
		TxHash   string `json:"tx_hash"`
		Index    uint32 `json:"index"`
		DataHash string `json:"data_hash"`
	} `json:"cell_recipes"`
}

func TestDiscoverLPCellsDevnet(t *testing.T) {
	operatorLockHash := requireEnv(t, "PERUN_LP_OPERATOR_LOCK_HASH")
	lpDeployment, ok := loadLPDeploymentFromDevnet(t)
	if !ok {
		return
	}

	rpcClient, err := rpc.Dial(ckbtest.DevnetRpcNodeURL)
	require.NoError(t, err)

	operatorHash := mustParseHash32(t, operatorLockHash)
	adapter := NewAdapter(rpcClient, nil, nil, backend.Deployment{}, lpDeployment)
	cells, err := adapter.DiscoverLPCells(context.Background(), operatorHash)
	require.NoError(t, err)
	require.NotEmpty(t, cells)
}

func TestGetLPCellDevnet(t *testing.T) {
	lpCellID := requireEnv(t, "PERUN_LP_CELL_ID")
	poolID := os.Getenv("PERUN_LP_POOL_ID")
	lpDeployment, ok := loadLPDeploymentFromDevnet(t)
	if !ok {
		return
	}

	rpcClient, err := rpc.Dial(ckbtest.DevnetRpcNodeURL)
	require.NoError(t, err)

	adapter := NewAdapter(rpcClient, nil, nil, backend.Deployment{}, lpDeployment)
	info, err := adapter.GetLPCell(context.Background(), lpCellID)
	require.NoError(t, err)

	if poolID != "" {
		expected := mustParseHash32(t, poolID)
		require.Equal(t, expected, info.Cell.PoolID)
	}
}

func loadLPDeploymentFromDevnet(t *testing.T) (LPDeployment, bool) {
	dir := filepath.Join("..", "..", "devnet", "contract", "migrations_lp", "dev")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("missing LP migration dir: %v", err)
		return LPDeployment{}, false
	}

	var migrationFile string
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == ".gitkeep" {
			continue
		}
		migrationFile = filepath.Join(dir, entry.Name())
		break
	}
	if migrationFile == "" {
		t.Skip("LP migration file not found; deploy LP scripts before running devnet tests")
		return LPDeployment{}, false
	}

	data, err := os.ReadFile(migrationFile)
	require.NoError(t, err)

	var migration lpMigration
	require.NoError(t, json.Unmarshal(data, &migration))

	lpts, lpls := findLPRecipes(migration)
	if lpts == nil || lpls == nil {
		t.Skip("LP migration missing lpts/lpls entries")
		return LPDeployment{}, false
	}

	return LPDeployment{
		TypeScriptDep: types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash(lpts.TxHash),
				Index:  lpts.Index,
			},
			DepType: types.DepTypeCode,
		},
		LockScriptDep: types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash(lpls.TxHash),
				Index:  lpls.Index,
			},
			DepType: types.DepTypeCode,
		},
		TypeScriptCodeHash: types.HexToHash(lpts.DataHash),
		TypeScriptHashType: types.HashTypeData1,
	}, true
}

type lpRecipe struct {
	Name     string
	TxHash   string
	Index    uint32
	DataHash string
}

func findLPRecipes(migration lpMigration) (*lpRecipe, *lpRecipe) {
	var lpts *lpRecipe
	var lpls *lpRecipe
	for _, recipe := range migration.CellRecipes {
		entry := lpRecipe{
			Name:     recipe.Name,
			TxHash:   recipe.TxHash,
			Index:    recipe.Index,
			DataHash: recipe.DataHash,
		}
		switch recipe.Name {
		case "lpts":
			lpts = &entry
		case "lpls":
			lpls = &entry
		}
	}
	return lpts, lpls
}

func requireEnv(t *testing.T, key string) string {
	value := os.Getenv(key)
	if value == "" {
		t.Skipf("missing env %s", key)
	}
	return value
}

func mustParseHash32(t *testing.T, value string) [32]byte {
	hash, err := parseHash32(value)
	require.NoError(t, err)
	var out [32]byte
	copy(out[:], hash[:])
	return out
}
