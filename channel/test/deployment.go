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

	pctsCodeHash, pctsHashType, err := getCodeHashAndType(pcts.DataHash, pcts.TypeId)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}
	pclsCodeHash, pclsHashType, err := getCodeHashAndType(pcls.DataHash, pcls.TypeId)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}
	pflsCodeHash, pflsHashType, err := getCodeHashAndType(pfls.DataHash, pfls.TypeId)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}
	vctsCodeHash, vctsHashType, err := getCodeHashAndType(vcts.DataHash, vcts.TypeId)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}
	vclsCodeHash, vclsHashType, err := getCodeHashAndType(vcls.DataHash, vcls.TypeId)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
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
		PCTSCodeHash:    pctsCodeHash,
		PCTSHashType:    pctsHashType,
		PCLSCodeHash:    pclsCodeHash,
		PCLSHashType:    pclsHashType,
		VCTSCodeHash:    vctsCodeHash,
		VCTSHashType:    vctsHashType,
		VCLSCodeHash:    vclsCodeHash,
		VCLSHashType:    vclsHashType,
		PFLSCodeHash:    pflsCodeHash,
		PFLSHashType:    pflsHashType,
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
		return nil, fmt.Errorf("first cell recipe must be sudt")
	}

	var codeHash types.Hash
	var hashType types.ScriptHashType

	if sudt.TypeId != nil {
		// If TypeId is given, prefer that
		typeIDStr, ok := sudt.TypeId.(string)
		if !ok {
			return nil, fmt.Errorf("invalid type_id format")
		}
		codeHash = types.HexToHash(typeIDStr)
		hashType = types.HashTypeType
	} else {
		// Default: use data hash
		codeHash = types.HexToHash(sudt.DataHash)
		hashType = types.HashTypeData1
	}

	sudtScript := types.Script{
		CodeHash: codeHash,
		HashType: hashType,
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

func GetDeploymentDevnet(migrationDir, migrationDirVC, systemScriptsDir, sudtOwnerLockArg string) (backend.Deployment, SUDTInfo, error) {
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
		return backend.Deployment{}, SUDTInfo{}, fmt.Errorf("migration dir must contain exactly one file")
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
	return migration.MakeDeployment(ss, sudtOwnerLockArg, vcMigration)
}

func GetDeploymentTestnet(systemScriptsDir string, sudtOwnerLockArg string) (backend.Deployment, SUDTInfo, error) {
	// Hardcoded deployed contract info on testnet
	migration := Migration{
		CellRecipes: []struct {
			Name             string      `json:"name"`
			TxHash           string      `json:"tx_hash"`
			Index            uint32      `json:"index"`
			OccupiedCapacity int64       `json:"occupied_capacity"`
			DataHash         string      `json:"data_hash"`
			TypeId           interface{} `json:"type_id"`
		}{
			// SUDT
			{
				Name:     "sudt",
				TxHash:   "0xc247df0052ab5d67b6da04bf6f0743696a83db0cf94e2fef192cd29ef4cfe799",
				Index:    0,
				DataHash: "0xb875ff254fcaee9c5e164f3f2bf02f8e10a0d00526db46571ff22abae0766f11",
				TypeId:   "0xd7cb2e882ae04f0ba2d00d46d49ae2a7375f0e0d0a5d0d4aa48cef428d5bc5e5",
			},
			// PCTS
			{
				Name:     "pcts",
				TxHash:   "0xc247df0052ab5d67b6da04bf6f0743696a83db0cf94e2fef192cd29ef4cfe799",
				Index:    1,
				DataHash: "0x9c8eb7243aef83b0135d450407292da728753870b804a1d64a27d901f7b9640f",
				TypeId:   "0x96b5e79709e3c4931a35e5af67356e4ab752e5a990fce241fa17c4f6c3d510e2",
			},
			// PCLS
			{
				Name:     "pcls",
				TxHash:   "0xc247df0052ab5d67b6da04bf6f0743696a83db0cf94e2fef192cd29ef4cfe799",
				Index:    2,
				DataHash: "0x358519445ce23f8befc6580c5359c3477b8b5283f397a29806f0e95522a6adb8",
				TypeId:   "0x4fa6fd8c0ae0e4b870ed748f86cc42afcb47380f51a6864852820c127acb8f83",
			},
			// PFLS
			{
				Name:     "pfls",
				TxHash:   "0xc247df0052ab5d67b6da04bf6f0743696a83db0cf94e2fef192cd29ef4cfe799",
				Index:    3,
				DataHash: "0xd0507f41f9ddc3ef784e3a1c561d7d6e2dd08f0b24f9c9d61b9b93747d4a8295",
				TypeId:   "0xa8690a18bde4123fa04e7e5823f0554f196ec0bd04f3bbf8ed4360902fed05a9",
			},
		},
		DepGroupRecipes: nil,
	}

	vcMigration := Migration{
		CellRecipes: []struct {
			Name             string      `json:"name"`
			TxHash           string      `json:"tx_hash"`
			Index            uint32      `json:"index"`
			OccupiedCapacity int64       `json:"occupied_capacity"`
			DataHash         string      `json:"data_hash"`
			TypeId           interface{} `json:"type_id"`
		}{
			// VCTS
			{
				Name:     "vcts",
				TxHash:   "0x...testnet_txhash_vcts",
				Index:    0,
				DataHash: "0x...datahash_vcts",
			},
			// VCLS
			{
				Name:     "vcls",
				TxHash:   "0x...testnet_txhash_vcls",
				Index:    0,
				DataHash: "0x...datahash_vcls",
			},
		},
	}

	ss, err := GetSystemScripts(systemScriptsDir)
	if err != nil {
		return backend.Deployment{}, SUDTInfo{}, err
	}

	return migration.MakeDeployment(ss, sudtOwnerLockArg, vcMigration)
}

func getCodeHashAndType(dataHash string, typeId interface{}) (types.Hash, types.ScriptHashType, error) {
	if typeId != nil {
		typeIDStr, ok := typeId.(string)
		if !ok {
			return types.Hash{}, types.HashTypeData, fmt.Errorf("invalid type_id format")
		}
		return types.HexToHash(typeIDStr), types.HashTypeType, nil
	}
	return types.HexToHash(dataHash), types.HashTypeData1, nil
}
