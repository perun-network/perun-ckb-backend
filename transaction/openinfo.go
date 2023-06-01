package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/encoding"
)

type OpenInfo struct {
	ChannelID    channel.ID
	ChannelToken backend.Token
	Params       *channel.Params
	State        *channel.State

	// Cached PCTS after this OpenInfo was used in building a transaction.
	pcts *types.Script
}

func NewOpenInfo(channelID channel.ID, channelToken backend.Token, params *channel.Params, state *channel.State) *OpenInfo {
	return &OpenInfo{
		ChannelID:    channelID,
		ChannelToken: channelToken,
		Params:       params,
		State:        state,
	}
}

func (oi *OpenInfo) MkInitialChannelCell(channelLockScript, channelTypeScript types.Script) (types.CellOutput, []byte) {
	oi.pcts = &channelTypeScript
	channelStatus := mkInitialChannelStatus(oi.State)
	channelOutput := types.CellOutput{
		Capacity: 0,
		Lock:     &channelLockScript,
		Type:     &channelTypeScript,
	}
	capacity := channelOutput.OccupiedCapacity(channelStatus.AsSlice())
	channelOutput.Capacity = capacity
	return channelOutput, channelStatus.AsSlice()
}

func mkInitialChannelStatus(state *channel.State) molecule.ChannelStatus {
	packedState, err := encoding.PackChannelState(state)
	if err != nil {
		panic(err)
	}
	return molecule.NewChannelStatusBuilder().
		State(packedState).
		Funded(initialFundedStatus(state)).
		Disputed(encoding.False).
		Build()
}

func (oi OpenInfo) GetPCTS() *types.Script {
	if oi.pcts == nil {
		panic("PCTS not set on OpenInfo")
	}
	return oi.pcts
}

func initialFundedStatus(state *channel.State) molecule.Bool {
	// TODO: Verify that sum of max_capacity of the assets is 0 instead of that there are no assets.
	// We shortcut here, because assets with 0 max_capacity make no sense.
	if len(state.Assets) == 0 &&
		state.Assets[0].Equal(asset.CKBAsset) &&
		state.Balance(1, asset.CKBAsset).Sign() == 0 {
		return encoding.True
	}
	return encoding.False
}
