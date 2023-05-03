package channel

import "github.com/nervosnetwork/ckb-sdk-go/v2/types"

// TODO: Set defaults
var (
	DefaultPCLSCodeHash    types.Hash           = types.Hash{}
	DefaultPCLSHashType    types.ScriptHashType = types.HashTypeType
	DefaultPFLSCodeHash    types.Hash           = types.Hash{}
	DefaultPFLSHashType    types.ScriptHashType = types.HashTypeType
	DefaultPFLSMinCapacity uint64               = 0
)
