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
	Funding      uint64
	Params       *channel.Params
	State        *channel.State
}

func (oi OpenInfo) MkInitialChannelCell(channelLockScript, channelTypeScript types.Script) backend.CKBOutput {
	channelData := mkInitialChannelStatus(oi.Params, oi.State)
	channelOutput := molecule.NewCellOutputBuilder().
		Lock(*channelLockScript.Pack()).
		Type(molecule.NewScriptOptBuilder().Set(*channelTypeScript.Pack()).Build()).
		Build()
	return backend.CKBOutput{
		Output: channelOutput,
		Data:   channelData,
	}
}

func mkInitialChannelStatus(params *channel.Params, state *channel.State) molecule.Bytes {
	packedState, err := encoding.PackChannelState(state)
	if err != nil {
		panic(err)
	}
	partyA := channel.Index(0)
	balances := mkBalancesForParty(state, partyA)
	status := molecule.NewChannelStatusBuilder().
		State(packedState).
		Funded(encoding.False).
		Disputed(encoding.False).
		Funding(balances).
		Build()
	return *types.PackBytes(status.AsSlice())
}

func mkBalancesForParty(state *channel.State, index channel.Index) molecule.Balances {
	balances := molecule.NewBalancesBuilder()
	if index == channel.Index(0) {
		balances = balances.Nth0(*types.PackUint64(state.Allocation.Balance(index, asset.Asset).Uint64()))
	} else { // channel.Index(1), we only have two party channels.
		balances = balances.Nth1(*types.PackUint64(state.Allocation.Balance(index, asset.Asset).Uint64()))
	}
	return balances.Build()
}

// MinFunding returns the max between the requested funding amount and the
// minimum capacity required to accomodate the PFLS script in a funds cell.
func (oi OpenInfo) MinFunding() uint64 {
	minFunding := backend.MinCapacityForPFLS()
	if oi.Funding < minFunding {
		return minFunding
	}
	return oi.Funding
}
