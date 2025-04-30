package test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystemScripts(t *testing.T) {
	var ss SystemScripts
	require.NoError(t, json.Unmarshal([]byte(systemScriptCase), &ss))
	msg, err := json.Marshal(ss)
	require.NoError(t, err)
	recovered := new(SystemScripts)
	require.NoError(t, json.Unmarshal(msg, recovered))
	require.Equal(t, ss, *recovered)
}

var systemScriptCase string = "{\"dao\":{\"cell_dep\":{\"dep_type\":\"code\",\"out_point\":{\"index\":\"0x2\",\"tx_hash\":\"0x297d19805fee99a53a6274a976df562d678beeff286776e1cd5ac9d8e1870780\"}},\"script_id\":{\"code_hash\":\"0x82d76d1b75fe2fd9a27dfbaa65a039221a380d76c926f378d3f81cf3e7e13f2e\",\"hash_type\":\"type\"}},\"secp256k1_blake160_multisig_all\":{\"cell_dep\":{\"dep_type\":\"dep_group\",\"out_point\":{\"index\":\"0x1\",\"tx_hash\":\"0xad69fbce31c6d8a8516789dec3cd4ddecbeb63619b4fa6cd3a7d00cdc788bf33\"}},\"script_id\":{\"code_hash\":\"0x5c5069eb0857efc65e1bca0c07df34c31663b3622fd3876c876320fc9634e2a8\",\"hash_type\":\"type\"}},\"secp256k1_blake160_sighash_all\":{\"cell_dep\":{\"dep_type\":\"dep_group\",\"out_point\":{\"index\":\"0x0\",\"tx_hash\":\"0xad69fbce31c6d8a8516789dec3cd4ddecbeb63619b4fa6cd3a7d00cdc788bf33\"}},\"script_id\":{\"code_hash\":\"0x9bd7e06f3ecf4be0f2fcd2188b23f1b9fcc88e5d4b65a8637b17723bbda3cce8\",\"hash_type\":\"type\"}},\"secp256k1_data\":{\"out_point\":{\"index\":\"0x3\",\"tx_hash\":\"0x297d19805fee99a53a6274a976df562d678beeff286776e1cd5ac9d8e1870780\"}},\"type_id\":{\"script_id\":{\"code_hash\":\"0x00000000000000000000000000000000000000000000000000545950455f4944\",\"hash_type\":\"type\"}}}"
