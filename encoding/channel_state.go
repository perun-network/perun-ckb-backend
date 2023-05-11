package encoding

import (
	"errors"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"math/big"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/channel/asset"
)

func PackChannelState(state *channel.State) (molecule.ChannelState, error) {
	balA, balB, err := RestrictedBalances(state)
	if err != nil {
		return molecule.ChannelState{}, err
	}
	return molecule.NewChannelStateBuilder().
		ChannelId(*PackByte32(state.ID)).
		Version(*types.PackUint64(state.Version)).
		IsFinal(FromBool(state.IsFinal)).
		Balances(molecule.NewBalancesBuilder().
			Set([2]molecule.Uint64{*types.PackUint64(balA), *types.PackUint64(balB)}).
			Build()).
		Build(), nil
}

func RestrictedBalances(state *channel.State) (uint64, uint64, error) {
	if len(state.Allocation.Balances) < 1 {
		return 0, 0, errors.New("state has invalid balance")
	}
	if len(state.Allocation.Assets) != len(state.Allocation.Balances) {
		return 0, 0, errors.New("number of assets does not match number of balances")
	}
	// Necessary because this backend currently only supports a single (native) asset and no sub-channels.
	if len(state.Allocation.Assets) != 1 || len(state.Allocation.Balances) != 1 || len(state.Allocation.Locked) != 0 {
		return 0, 0, errors.New("allocation incompatible with this backend")
	}
	if !state.Allocation.Assets[0].Equal(asset.Asset) {
		return 0, 0, errors.New("allocation has asset other than native asset")
	}
	if len(state.Allocation.Balances[0]) != 2 {
		return 0, 0, errors.New("allocation does not have exactly two participants")
	}
	balA, err := RestrictedBalance(state.Balance(0, asset.Asset))
	if err != nil {
		return 0, 0, err
	}
	balB, err := RestrictedBalance(state.Balance(1, asset.Asset))
	if err != nil {
		return 0, 0, err
	}
	return balA, balB, nil

}

func RestrictedBalance(b *big.Int) (uint64, error) {
	if !b.IsUint64() {
		return 0, errors.New("invalid balance")
	}
	return b.Uint64(), nil
}
