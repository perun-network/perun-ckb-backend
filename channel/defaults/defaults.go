package defaults

import "github.com/nervosnetwork/ckb-sdk-go/v2/types"

// TODO: Set defaults
var (
	DefaultPCTSCodeHash    types.Hash           = types.Hash{}
	DefaultPCTSHashType    types.ScriptHashType = types.HashTypeType
	DefaultPCLSCodeHash    types.Hash           = types.Hash{}
	DefaultPCLSHashType    types.ScriptHashType = types.HashTypeType
	DefaultPFLSCodeHash    types.Hash           = types.Hash{}
	DefaultPFLSHashType    types.ScriptHashType = types.HashTypeType
	DefaultPFLSMinCapacity uint64               = 0
)
