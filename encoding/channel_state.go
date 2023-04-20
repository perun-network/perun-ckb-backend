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
	balA, balB, err := RestrictBalances(state.Allocation)
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

func RestrictBalances(alloc channel.Allocation) (uint64, uint64, error) {
	if len(alloc.Balances) < 1 {
		return 0, 0, errors.New("state has invalid balance")
	}
	if len(alloc.Assets) != len(alloc.Balances) {
		return 0, 0, errors.New("number of assets does not match number of balances")
	}
	// Necessary because this backend currently only supports a single (native) asset and no sub-channels.
	if len(alloc.Assets) != 1 || len(alloc.Balances) != 1 || len(alloc.Locked) != 0 {
		return 0, 0, errors.New("allocation incompatible with this backend")
	}
	if !alloc.Assets[0].Equal(asset.Asset) {
		return 0, 0, errors.New("allocation has asset other than native asset")
	}

	if len(alloc.Balances[0]) != 2 {
		return 0, 0, errors.New("allocation does not have exactly two participants")
	}
	balA, err := RestrictBalance(alloc.Balances[0][0])
	if err != nil {
		return 0, 0, err
	}
	balB, err := RestrictBalance(alloc.Balances[0][1])
	if err != nil {
		return 0, 0, err
	}
	return balA, balB, nil

}

func RestrictBalance(b *big.Int) (uint64, error) {
	if !b.IsUint64() {
		return 0, errors.New("invalid balance")
	}
	return b.Uint64(), nil
}
