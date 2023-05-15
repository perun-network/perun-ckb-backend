package backend

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
)

func MinCapacityForPFLS() uint64 {
	pctsArgs := molecule.NewChannelConstantsBuilder().Build()
	pcts := types.Script{
		CodeHash: [32]byte{},
		HashType: types.HashTypeData,
		Args:     pctsArgs.AsSlice(),
	}
	return pcts.OccupiedCapacity()
}
