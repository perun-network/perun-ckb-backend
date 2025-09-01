package test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

func parseDepType(depTypeRaw string) types.DepType {
	switch strings.ToLower(depTypeRaw) {
	case "code":
		return types.DepTypeCode
	case "depgroup", "dep_group":
		return types.DepTypeDepGroup
	default:
		log.Fatalf("Unknown dep type: %s", depTypeRaw)
		return "" // unreachable
	}
}

func (m Migration) MakeDeployment(systemScripts SystemScripts, sudtOwnerLockArg string, vcm Migration) (backend.Deployment, SUDTInfo, error) {
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
	vcts := vcm.CellRecipes[0]
	if vcts.Name != "vcts" {
		return backend.Deployment{}, SUDTInfo{}, fmt.Errorf("fifth cell recipe must be vcts")
	}
	vcls := vcm.CellRecipes[1]
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
				Index:  pcts.Index,
			},
			DepType: types.DepTypeCode,
		},
		PCLSDep: types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash(pcls.TxHash),
				Index:  pcls.Index,
			},
			DepType: types.DepTypeCode,
		},
		VCTSDep: types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash(vcts.TxHash),
				Index:  vcts.Index,
			},
			DepType: types.DepTypeCode,
		},
		VCLSDep: types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash(vcls.TxHash),
				Index:  vcls.Index,
			},
			DepType: types.DepTypeCode,
		},
		PFLSDep: types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash(pfls.TxHash),
				Index:  pfls.Index,
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
			CodeHash: systemScripts.Secp256k1Blake160.CodeHash,
			HashType: systemScripts.Secp256k1Blake160.HashType,
			Args:     make([]byte, 32),
		},
		DefaultLockScriptDep: types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: systemScripts.Secp256k1Blake160.CellDeps[0].CellDep.OutPoint.TxHash,
				Index:  systemScripts.Secp256k1Blake160.CellDeps[0].CellDep.OutPoint.Index,
			},
			DepType: parseDepType(string(systemScripts.Secp256k1Blake160.CellDeps[0].CellDep.DepType)),
		},
		OmniLockScript: types.Script{
			CodeHash: systemScripts.OmniLock.CodeHash,
			HashType: systemScripts.OmniLock.HashType,
			Args:     make([]byte, 32),
		},
		OmniLockScriptDep: []types.CellDep{
			{
				OutPoint: &types.OutPoint{
					TxHash: systemScripts.OmniLock.CellDeps[0].CellDep.OutPoint.TxHash,
					Index:  systemScripts.OmniLock.CellDeps[0].CellDep.OutPoint.Index,
				},
				DepType: parseDepType(string(systemScripts.OmniLock.CellDeps[0].CellDep.DepType)),
			},
			{
				OutPoint: &types.OutPoint{
					TxHash: systemScripts.OmniLock.CellDeps[1].CellDep.OutPoint.TxHash,
					Index:  systemScripts.OmniLock.CellDeps[1].CellDep.OutPoint.Index,
				},
				DepType: parseDepType(string(systemScripts.OmniLock.CellDeps[1].CellDep.DepType)),
			},
		},
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
		return nil, fmt.Errorf("first cell recipe must be sudt")
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

func GetDeployment(migrationDir, migrationDirVC, systemScriptsDir, sudtOwnerLockArg string) (backend.Deployment, SUDTInfo, error) {
	dir, err := os.ReadDir(migrationDir)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}
	if len(dir) != 1 {
		return backend.Deployment{}, SUDTInfo{}, fmt.Errorf("migration dir must contain exactly one file")
	}

	vc_dir, err := os.ReadDir(migrationDirVC)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}
	if len(vc_dir) != 1 {
		return backend.Deployment{}, SUDTInfo{}, fmt.Errorf("vc migration dir must contain exactly one file")
	}

	migrationName := dir[0].Name()
	migrationFile, err := os.Open(path.Join(migrationDir, migrationName))
	defer func() {
		if err := migrationFile.Close(); err != nil {
			log.Fatalf("failed to close migration file: %v\n", err)
		}
	}()
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}

	vcMigrationName := vc_dir[0].Name()
	vcMigrationFile, err := os.Open(path.Join(migrationDirVC, vcMigrationName))
	defer func() {
		if err := vcMigrationFile.Close(); err != nil {
			log.Fatalf("failed to close vc migration file: %v\n", err)
		}
	}()
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}

	// Read and unmarshall migration file
	migrationData, err := io.ReadAll(migrationFile)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}
	var migration Migration
	err = json.Unmarshal(migrationData, &migration)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}

	// Read and unmarshall vc migration file
	vcMigrationData, err := io.ReadAll(vcMigrationFile)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}
	var vcMigration Migration
	err = json.Unmarshal(vcMigrationData, &vcMigration)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}

	// Read system scripts
	ss, err := GetSystemScripts(systemScriptsDir)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}
	fmt.Printf("Migration: %v\n", migration)
	fmt.Printf("VC Migration: %v\n", vcMigration)
	return migration.MakeDeployment(ss, sudtOwnerLockArg, vcMigration)
}
