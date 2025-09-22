package test

import (
	"encoding/json"
	"io"
	"os"
	"path"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

type SystemScripts struct {
	DAO struct {
		CellDep  types.CellDep `json:"cell_dep"`
		ScriptID ScriptID      `json:"script_id"`
	} `json:"dao"`
	Secp256k1Blake160MultisigAll struct {
		CellDep  types.CellDep `json:"cell_dep"`
		ScriptID ScriptID      `json:"script_id"`
	} `json:"secp256k1_blake160_multisig_all"`
	Secp256k1Blake160SighashAll struct {
		CellDep  types.CellDep `json:"cell_dep"`
		ScriptID ScriptID      `json:"script_id"`
	} `json:"secp256k1_blake160_sighash_all"`
	Secp256k1Data types.OutPoint `json:"secp256k1_data"`
	TypeID        struct {
		ScriptID ScriptID `json:"script_id"`
	} `json:"type_id"`
}

type ScriptID struct {
	CodeHash types.Hash           `json:"code_hash"`
	HashType types.ScriptHashType `json:"hash_type"`
}

const systemScriptName = "default_scripts.json"

func GetSystemScripts(systemScriptDir string) (SystemScripts, error) {
	var ss SystemScripts
	err := readJSON(systemScriptDir, &ss)
	if err != nil {
		return SystemScripts{}, err
	}
	return ss, nil
}

func readJSON(systemScriptDir string, systemScripts *SystemScripts) error {
	systemScriptFile, err := os.Open(path.Join(systemScriptDir, systemScriptName))
	defer func() { _ = systemScriptFile.Close() }()
	if err != nil {
		return err
	}
	systemScriptContent, err := io.ReadAll(systemScriptFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(systemScriptContent, systemScripts)
}
