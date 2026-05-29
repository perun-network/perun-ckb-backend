package transaction

import (
	"fmt"

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

func (oi *OpenInfo) MkInitialChannelCell(channelLockScript, channelTypeScript types.Script) (types.CellOutput, []byte, error) {
	oi.pcts = &channelTypeScript
	channelStatus, err := mkInitialChannelStatus(oi.State)
	if err != nil {
		return types.CellOutput{}, nil, err
	}
	channelOutput := types.CellOutput{
		Capacity: 0,
		Lock:     &channelLockScript,
		Type:     &channelTypeScript,
	}
	// Pre-size the channel cell so it can later hold one locked sub-allocation (a virtual
	// channel) without growing on-chain at dispute/register time. The reserve is funded by
	// party 0 here and reclaimed by party 0 at close, keeping channel-cell capacity a
	// conserved quantity (see encoding.LockedSubAllocReserve). All rebuilds preserve this
	// capacity rather than recomputing it from the (possibly smaller) data.
	capacity := channelOutput.OccupiedCapacity(channelStatus.AsSlice())
	reserve, err := encoding.LockedSubAllocReserve(oi.State)
	if err != nil {
		return types.CellOutput{}, nil, fmt.Errorf("computing locked sub-alloc reserve: %w", err)
	}
	channelOutput.Capacity = capacity + reserve
	return channelOutput, channelStatus.AsSlice(), nil
}

func mkInitialChannelStatus(state *channel.State) (molecule.ChannelStatus, error) {
	packedState, err := encoding.PackChannelState(state)
	if err != nil {
		return molecule.ChannelStatus{}, fmt.Errorf("packing channel state: %w", err)
	}
	return molecule.NewChannelStatusBuilder().
		State(packedState).
		Funded(initialFundedStatus(state)).
		Disputed(encoding.False).
		VcDisputed(encoding.False).
		VctsHash(molecule.Byte32Default()).
		Build(), nil
}

func (oi OpenInfo) GetPCTS() (*types.Script, error) {
	if oi.pcts == nil {
		return nil, fmt.Errorf("PCTS not set on OpenInfo")
	}
	return oi.pcts, nil
}

func initialFundedStatus(state *channel.State) molecule.Bool {
	// TODO: Verify that sum of max_capacity of the assets is 0 instead of that there are no assets.
	// We shortcut here, because assets with 0 max_capacity make no sense.
	CKBytes := asset.NewCKBytesAsset()
	if len(state.Assets) == 1 &&
		state.Assets[0].Equal(CKBytes) &&
		state.Balance(1, CKBytes).Sign() == 0 {
		return encoding.True
	}
	return encoding.False
}
