package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
)

type DisputeInfo struct {
	ChannelCell types.OutPoint
	Status      molecule.ChannelStatus
	Params      *channel.Params
	Header      types.Hash
	Token       molecule.ChannelToken
	SigA        molecule.Bytes
	SigB        molecule.Bytes
}
