package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/backend"
)

type DisputeInfo struct {
	ChannelCell types.OutPoint
	Status      molecule.ChannelStatus
	Params      *channel.Params
	Header      types.Hash
	Token       backend.Token
	SigA        molecule.Bytes
	SigB        molecule.Bytes
}
