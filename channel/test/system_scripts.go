package test

import (
	"encoding/json"
	"io"
	"os"
	"path"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

type OutPoint struct {
	TxHash types.Hash `json:"txHash"`
	Index  uint32     `json:"index"`
}

type CellDep struct {
	OutPoint *OutPoint     `json:"outPoint"`
	DepType  types.DepType `json:"depType"`
}
type ScriptDep struct {
	CellDep CellDep `json:"cellDep"`
}

type ScriptEntry struct {
	CodeHash types.Hash           `json:"codeHash"`
	HashType types.ScriptHashType `json:"hashType"`
	CellDeps []ScriptDep          `json:"cellDeps"`
}

type SystemScripts struct {
	Secp256k1Blake160 ScriptEntry `json:"Secp256k1Blake160"`
	Secp256k1Multisig ScriptEntry `json:"Secp256k1Multisig"`
	AnyoneCanPay      ScriptEntry `json:"AnyoneCanPay"`
	OmniLock          ScriptEntry `json:"OmniLock"`
	XUdt              ScriptEntry `json:"XUdt"`
	TypeID            ScriptEntry `json:"TypeId"`
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
