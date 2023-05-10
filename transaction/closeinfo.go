package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
)

type CloseInfo struct {
	ChannelCapacity  uint64
	ChannelInput     types.CellInput
	AssetInputs      []types.CellInput
	Params           *channel.Params
	State            *channel.State
	PaddedSignatures []wallet.Sig
}
