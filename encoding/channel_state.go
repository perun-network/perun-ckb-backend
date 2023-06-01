package encoding

import (
	"errors"
	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"math/big"
	pchannel "perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/channel/asset"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
)

func PackChannelState(state *pchannel.State) (molecule.ChannelState, error) {
	balances, err := PackBalances(state.Clone())
	if err != nil {
		return molecule.ChannelState{}, err
	}
	return molecule.NewChannelStateBuilder().
		ChannelId(*molecule2.PackByte32(state.ID)).
		Version(*types.PackUint64(state.Version)).
		IsFinal(FromBool(state.IsFinal)).
		Balances(balances).
		Build(), nil
}

func PackBalances(state *pchannel.State) (molecule.Balances, error) {
	balancesBuilder := molecule.NewBalancesBuilder()
	sudtAllocBuilder := molecule.NewSUDTAllocationBuilder()
	for _, a := range state.Assets {
		if a.Equal(asset.CKBAsset) {
			d, err := PackCKByteDistribution(
				[2]*big.Int{
					state.Balance(0, a),
					state.Balance(1, a),
				})
			if err != nil {
				return molecule.Balances{}, err
			}
			balancesBuilder.Ckbytes(d)
		} else {
			b, err := PackSUDTBalances(a,
				[2]*big.Int{
					state.Balance(0, a),
					state.Balance(1, a),
				})
			if err != nil {
				return molecule.Balances{}, err
			}
			sudtAllocBuilder.Push(b)
		}
	}
	return balancesBuilder.Sudts(sudtAllocBuilder.Build()).Build(), nil
}

func PackCKByteDistribution(d [2]*big.Int) (molecule.CKByteDistribution, error) {
	if !d[0].IsUint64() {
		return molecule.CKByteDistribution{}, errors.New("ckbyte balance of participant 0 is not a uint64")
	}
	balA := d[0].Uint64()
	if !d[1].IsUint64() {
		return molecule.CKByteDistribution{}, errors.New("ckbyte balance of participant 1 is not a uint64")
	}
	balB := d[1].Uint64()
	return molecule.NewCKByteDistributionBuilder().
		Set([2]molecule.Uint64{*types.PackUint64(balA), *types.PackUint64(balB)}).
		Build(), nil
}

func PackSUDTBalances(a pchannel.Asset, d [2]*big.Int) (molecule.SUDTBalances, error) {
	sudtAsset, err := asset.IsSUDTAsset(a)
	if err != nil {
		return molecule.SUDTBalances{}, err
	}
	sudtDistribution, err := PackSUDTDistribution(d)
	if err != nil {
		return molecule.SUDTBalances{}, err
	}

	return molecule.NewSUDTBalancesBuilder().
		Asset(sudtAsset.Pack()).
		Distribution(sudtDistribution).
		Build(), nil
}

func PackSUDTDistribution(d [2]*big.Int) (molecule.SUDTDistribution, error) {
	balA, err := molecule2.PackUint128(d[0])
	if err != nil {
		return molecule.SUDTDistribution{}, err
	}
	balB, err := molecule2.PackUint128(d[1])
	if err != nil {
		return molecule.SUDTDistribution{}, err
	}
	return molecule.NewSUDTDistributionBuilder().Nth0(*balA).Nth1(*balB).Build(), nil
}

func GetSUDTBalancesSlice(state *pchannel.State) ([]asset.SUDTBalances, error) {
	sudtBalancesSlice := make([]asset.SUDTBalances, 0)
	for _, a := range state.Assets {
		if a.Equal(asset.CKBAsset) {
			continue
		} else {
			sudtAsset, err := asset.IsSUDTAsset(a)
			if err != nil {
				return nil, err
			}
			balA, err := molecule2.ToUint128(state.Balance(0, a))
			if err != nil {
				return nil, err
			}
			balB, err := molecule2.ToUint128(state.Balance(1, a))
			if err != nil {
				return nil, err
			}
			sudtBalances := asset.SUDTBalances{
				Asset: *sudtAsset,
				Distribution: [2]uint128.Uint128{
					balA,
					balB,
				},
			}
			sudtBalancesSlice = append(sudtBalancesSlice, sudtBalances)
		}
	}
	return sudtBalancesSlice, nil
}
