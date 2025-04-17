package test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/perun-ckb-backend/backend"
)

const PFLSMinCapacity = 4100000032

type SUDTInfo struct {
	Script  *types.Script
	CellDep *types.CellDep
}

type Migration struct {
	CellRecipes []struct {
		Name             string      `json:"name"`
		TxHash           string      `json:"tx_hash"`
		Index            uint32      `json:"index"`
		OccupiedCapacity int64       `json:"occupied_capacity"`
		DataHash         string      `json:"data_hash"`
		TypeId           interface{} `json:"type_id"`
	} `json:"cell_recipes"`
	DepGroupRecipes []interface{} `json:"dep_group_recipes"`
}

func (m Migration) MakeDeployment(systemScripts SystemScripts, sudtOwnerLockArg string) (backend.Deployment, SUDTInfo, error) {
	sudtInfo, err := m.GetSUDT()
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}
	pcts := m.CellRecipes[1]
	if pcts.Name != "pcts" {
		return backend.Deployment{}, SUDTInfo{}, fmt.Errorf("second cell recipe must be pcts")
	}
	pcls := m.CellRecipes[2]
	if pcls.Name != "pcls" {
		return backend.Deployment{}, SUDTInfo{}, fmt.Errorf("third cell recipe must be pcls")
	}
	pfls := m.CellRecipes[3]
	if pfls.Name != "pfls" {
		return backend.Deployment{}, SUDTInfo{}, fmt.Errorf("fourth cell recipe must be pfls")
	}

	// Virtual channel scripts.
	vcts := m.CellRecipes[4]
	if vcts.Name != "vcts" {
		return backend.Deployment{}, SUDTInfo{}, fmt.Errorf("fifth cell recipe must be vcts")
	}
	vcls := m.CellRecipes[5]
	if vcls.Name != "vcls" {
		return backend.Deployment{}, SUDTInfo{}, fmt.Errorf("sixth cell recipe must be vcls")
	}
	// NOTE: The SUDT lock-arg always contains a newline character at the end.
	hexString := strings.ReplaceAll(sudtOwnerLockArg[2:], "\n", "")
	hexString = strings.ReplaceAll(hexString, "\r", "")
	hexString = strings.ReplaceAll(hexString, " ", "")
	sudtInfo.Script.Args, err = hex.DecodeString(hexString)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, fmt.Errorf("invalid sudt owner lock arg: %v", err)
	}

	return backend.Deployment{
		Network: types.NetworkTest,
		PCTSDep: types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash(pcts.TxHash),
				Index:  m.CellRecipes[0].Index,
			},
			DepType: types.DepTypeCode,
		},
		PCLSDep: types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash(pcls.TxHash),
				Index:  m.CellRecipes[0].Index,
			},
			DepType: types.DepTypeCode,
		},
		VCTSDep: types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash(vcts.TxHash),
				Index:  m.CellRecipes[0].Index,
			},
			DepType: types.DepTypeCode,
		},
		VCLSDep: types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash(vcls.TxHash),
				Index:  m.CellRecipes[0].Index,
			},
			DepType: types.DepTypeCode,
		},
		PFLSDep: types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash(pfls.TxHash),
				Index:  m.CellRecipes[0].Index,
			},
			DepType: types.DepTypeCode,
		},
		PCTSCodeHash:    types.HexToHash(pcts.DataHash),
		PCTSHashType:    types.HashTypeData1,
		PCLSCodeHash:    types.HexToHash(pcls.DataHash),
		PCLSHashType:    types.HashTypeData1,
		VCTSCodeHash:    types.HexToHash(vcts.DataHash),
		VCTSHashType:    types.HashTypeData1,
		VCLSCodeHash:    types.HexToHash(vcls.DataHash),
		VCLSHashType:    types.HashTypeData1,
		PFLSCodeHash:    types.HexToHash(pfls.DataHash),
		PFLSHashType:    types.HashTypeData1,
		PFLSMinCapacity: PFLSMinCapacity,
		DefaultLockScript: types.Script{
			CodeHash: systemScripts.Secp256k1Blake160SighashAll.ScriptID.CodeHash,
			HashType: systemScripts.Secp256k1Blake160SighashAll.ScriptID.HashType,
			Args:     make([]byte, 32),
		},
		DefaultLockScriptDep: systemScripts.Secp256k1Blake160SighashAll.CellDep,
		SUDTDeps: map[types.Hash]types.CellDep{
			sudtInfo.Script.Hash(): *sudtInfo.CellDep,
		},
		SUDTs: map[types.Hash]types.Script{
			sudtInfo.Script.Hash(): *sudtInfo.Script,
		},
	}, *sudtInfo, nil
}

func (m Migration) GetSUDT() (*SUDTInfo, error) {
	sudt := m.CellRecipes[0]
	if sudt.Name != "sudt" {
		return nil, fmt.Errorf("fourth cell recipe must be sudt")
	}

	sudtScript := types.Script{
		CodeHash: types.HexToHash(sudt.DataHash),
		HashType: types.HashTypeData1,
		Args:     []byte{},
	}
	sudtCellDep := types.CellDep{
		OutPoint: &types.OutPoint{
			TxHash: types.HexToHash(sudt.TxHash),
			Index:  sudt.Index,
		},
		DepType: types.DepTypeCode,
	}
	return &SUDTInfo{
		Script:  &sudtScript,
		CellDep: &sudtCellDep,
	}, nil
}

func GetDeployment(migrationDir, systemScriptsDir, sudtOwnerLockArg string) (backend.Deployment, SUDTInfo, error) {
	dir, err := os.ReadDir(migrationDir)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}
	if len(dir) != 1 {
		return backend.Deployment{}, SUDTInfo{}, fmt.Errorf("migration dir must contain exactly one file")
	}
	migrationName := dir[0].Name()
	migrationFile, err := os.Open(path.Join(migrationDir, migrationName))
	defer migrationFile.Close()
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}
	migrationData, err := io.ReadAll(migrationFile)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}
	var migration Migration
	err = json.Unmarshal(migrationData, &migration)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}

	ss, err := GetSystemScripts(systemScriptsDir)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}
	return migration.MakeDeployment(ss, sudtOwnerLockArg)
}
