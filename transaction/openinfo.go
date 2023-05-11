package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/backend"
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
	channelData := mkInitialChannelStatus(oi.State, oi.Funding)
	channelOutput := types.CellOutput{
		Capacity: 0,
		Lock:     &channelLockScript,
		Type:     &channelTypeScript,
	}
	capacity := channelOutput.OccupiedCapacity(channelData.AsSlice())
	channelOutput.Capacity = capacity
	return backend.CKBOutput{
		Output: *channelOutput.Pack(),
		Data:   channelData,
	}
}

func mkInitialChannelStatus(state *channel.State, fundingPartyA uint64) molecule.Bytes {
	packedState, err := encoding.PackChannelState(state)
	if err != nil {
		panic(err)
	}
	partyA := channel.Index(0)
	balances := mkBalancesForParty(partyA, fundingPartyA)
	status := molecule.NewChannelStatusBuilder().
		State(packedState).
		Funded(encoding.False).
		Disputed(encoding.False).
		Funding(balances).
		Build()
	return *types.PackBytes(status.AsSlice())
}

func mkBalancesForParty(index channel.Index, funding uint64) molecule.Balances {
	balances := molecule.NewBalancesBuilder()
	if index == channel.Index(0) {
		balances = balances.Nth0(*types.PackUint64(funding))
	} else { // channel.Index(1), we only have two party channels.
		balances = balances.Nth1(*types.PackUint64(funding))
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
