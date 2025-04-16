package encoding

import (
	"errors"
	"math/big"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	pchannel "perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/channel/asset"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
)

// PackChannelState converts a perun channel state to a molecule ChannelState.
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

// PackBalances extracts the balances from a perun channel state to a molecule Balances.
func PackBalances(state *pchannel.State) (molecule.Balances, error) {
	balancesBuilder := molecule.NewBalancesBuilder()
	sudtAllocBuilder := molecule.NewSUDTAllocationBuilder()
	for _, pAsset := range state.Assets {
		a, err := asset.IsCompatibleAsset(pAsset)
		if err != nil {
			return molecule.Balances{}, err
		}
		if a.IsInvalid() {
			return molecule.Balances{}, errors.New("invalid asset")
		}
		if a.IsCKBytes {
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
	balancesBuilder.Sudts(sudtAllocBuilder.Build())

	// Locked_Balances
	lockedBalancesBuilder := molecule.NewLockedBalancesBuilder()
	for _, subAlloc := range state.Locked {
		sa, err := PackSubAlloc(&subAlloc, state)
		if err != nil {
			return molecule.Balances{}, err
		}
		lockedBalancesBuilder.Push(sa)
	}

	if len(state.Locked) == 0 {
		sa, err := PackDefaultSubAlloc()
		if err != nil {
			return molecule.Balances{}, err
		}
		lockedBalancesBuilder.Push(sa)
	}

	balancesBuilder.Locked(lockedBalancesBuilder.Build())

	return balancesBuilder.Build(), nil
}

// PackSubAlloc converts a perun suballocation to a molecule SubAlloc.
func PackSubAlloc(subAlloc *pchannel.SubAlloc, state *pchannel.State) (molecule.SubAlloc, error) {
	subAllocBuilder := molecule.NewSubAllocBuilder()
	subAllocBuilder.Id(*molecule2.PackByte32(subAlloc.ID))
	sudtAllocBuilder := molecule.NewSUDTAllocationBuilder()
	subBalancesBuilder := molecule.NewSubBalancesBuilder()
	for _, pAsset := range state.Assets {
		a, err := asset.IsCompatibleAsset(pAsset)
		if err != nil {
			return molecule.SubAlloc{}, err
		}
		if a.IsInvalid() {
			return molecule.SubAlloc{}, errors.New("invalid asset")
		}
		if a.IsCKBytes {
			d, err := PackCKByteDistribution(
				[2]*big.Int{
					state.Balance(0, a),
					state.Balance(1, a),
				})
			if err != nil {
				return molecule.SubAlloc{}, err
			}
			subBalancesBuilder.Ckbytes(d)
		} else {
			b, err := PackSUDTBalances(a,
				[2]*big.Int{
					state.Balance(0, a),
					state.Balance(1, a),
				})
			if err != nil {
				return molecule.SubAlloc{}, err
			}
			sudtAllocBuilder.Push(b)
		}
	}
	subBalancesBuilder.Sudts(sudtAllocBuilder.Build())

	subAllocBuilder.Balances(subBalancesBuilder.Build())
	return subAllocBuilder.Build(), nil
}

// PackDefaultSubAlloc creates a default suballocation with a default ID and empty balances.
func PackDefaultSubAlloc() (molecule.SubAlloc, error) {
	subAllocBuilder := molecule.NewSubAllocBuilder()
	subAllocBuilder.Id(molecule.Byte32Default())
	sudtAllocBuilder := molecule.NewSUDTAllocationBuilder()
	subBalancesBuilder := molecule.NewSubBalancesBuilder()
	subBalancesBuilder.Sudts(sudtAllocBuilder.Build())

	subAllocBuilder.Balances(subBalancesBuilder.Build())
	return subAllocBuilder.Build(), nil
}

// PackCKByteDistribution converts a perun channel state to a molecule CKByteDistribution.
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

// PackSUDTBalances converts a perun SUDT asset and its distribution to a molecule SUDTBalances.
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

// PackSUDTDistribution converts a perun SUDT distribution to a molecule SUDTDistribution.
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

// GetSUDTBalancesSlice extracts SUDT balances from a perun channel state and returns them as a slice of SUDTBalances.
func GetSUDTBalancesSlice(state *pchannel.State) ([]asset.SUDTBalances, error) {
	sudtBalancesSlice := make([]asset.SUDTBalances, 0)
	for _, pAsset := range state.Assets {
		a, err := asset.IsCompatibleAsset(pAsset)
		if err != nil {
			return nil, err
		}
		if a.IsInvalid() {
			return nil, errors.New("invalid asset")
		}
		if a.IsCKBytes {
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
