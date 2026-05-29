package encoding

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"perun.network/go-perun/channel/multi"

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
	allocBuilder := molecule.NewAllocationBuilder()
	for _, pAsset := range state.Assets {
		a, ckb := asset.IsCompatibleAsset(pAsset)
		if !ckb {
			d, err := PackEthBalances(
				[2]*big.Int{
					state.Balance(0, pAsset),
					state.Balance(1, pAsset),
				}, pAsset)
			if err != nil {
				return molecule.Balances{}, err
			}
			d_union := molecule.AnyBalancesUnionFromETHBalances(d)
			ab := molecule.NewAnyBalancesBuilder()
			ab.Set(d_union)
			allocBuilder.Push(ab.Build())
		} else {
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
				d_union := molecule.AnyBalancesUnionFromCKByteDistribution(d)
				ab := molecule.NewAnyBalancesBuilder()
				ab.Set(d_union)
				allocBuilder.Push(ab.Build())
			} else {
				d, err := PackSUDTBalances(a,
					[2]*big.Int{
						state.Balance(0, a),
						state.Balance(1, a),
					})
				if err != nil {
					return molecule.Balances{}, err
				}
				d_union := molecule.AnyBalancesUnionFromSUDTBalances(d)
				ab := molecule.NewAnyBalancesBuilder()
				ab.Set(d_union)
				allocBuilder.Push(ab.Build())
			}
		}
	}

	balancesBuilder.Assets(allocBuilder.Build())

	// Locked_Balances — an empty vector is valid; the contract's
	// verify_no_locked_funds accepts both empty and one dummy entry.
	lockedBalancesBuilder := molecule.NewLockedBalancesBuilder()
	for _, subAlloc := range state.Locked {
		sa, err := PackSubAlloc(&subAlloc, state)
		if err != nil {
			return molecule.Balances{}, err
		}
		lockedBalancesBuilder.Push(sa)
	}

	balancesBuilder.Locked(lockedBalancesBuilder.Build())

	return balancesBuilder.Build(), nil
}

// PackSubAlloc converts a perun suballocation to a molecule SubAlloc.
//
// SubBalances is a flat vector<Uint128> aligned with state.Assets order. Every
// asset type (CKByte, SUDT, ETH) is encoded as a single Uint128 holding the
// total amount locked for that asset; per-participant attribution is recovered
// downstream via idx_map.
func PackSubAlloc(subAlloc *pchannel.SubAlloc, state *pchannel.State) (molecule.SubAlloc, error) {
	if len(subAlloc.Bals) != len(state.Assets) {
		return molecule.SubAlloc{}, fmt.Errorf("suballoc has %d balance rows but state has %d assets", len(subAlloc.Bals), len(state.Assets))
	}

	subBalancesBuilder := molecule.NewSubBalancesBuilder()
	for i := range state.Assets {
		u128, err := packUint128LE(subAlloc.Bals[i])
		if err != nil {
			return molecule.SubAlloc{}, fmt.Errorf("packing suballoc balance %d: %w", i, err)
		}
		subBalancesBuilder.Push(u128)
	}

	// idx_map defaults to identity [0, 1] when perun's SubAlloc.IndexMap is empty.
	var idx0, idx1 byte = 0, 1
	if len(subAlloc.IndexMap) >= 2 {
		idx0 = byte(subAlloc.IndexMap[0])
		idx1 = byte(subAlloc.IndexMap[1])
	}
	idxMap := molecule.NewIndexMapBuilder().
		Nth0(*types.PackByte(idx0)).
		Nth1(*types.PackByte(idx1)).
		Build()

	return molecule.NewSubAllocBuilder().
		Id(*molecule2.PackByte32(subAlloc.ID)).
		Balances(subBalancesBuilder.Build()).
		IdxMap(idxMap).
		Build(), nil
}

// ShannonsPerCapacityByte is the cell-capacity cost of one byte of on-chain data: one byte
// of occupied capacity requires one CKByte (1e8 shannons).
const ShannonsPerCapacityByte = 100_000_000

// LockedSubAllocReserve returns the extra channel-cell capacity (in shannons) needed to hold
// one locked sub-allocation, sized for the channel's own asset count (a sub-alloc's flat
// SubBalances vector always has one slot per channel asset).
//
// The funded channel cell is pre-sized by this amount so that materialising a virtual
// channel's locked sub-alloc on-chain at dispute/register does not grow the cell. Without the
// reserve the registrant funds that growth while party 0 reclaims it at force close, leaking
// capacity between parties. The reserve is kept out of the *signed* channel state (it lives
// only in the cell's capacity field), so it does not affect the EVM/Keccak state signature.
func LockedSubAllocReserve(state *pchannel.State) (uint64, error) {
	numAssets := len(state.Assets)

	// Measure the on-chain size of adding one locked sub-allocation (with numAssets balance
	// slots) to an otherwise empty LockedBalances vector. Because molecule tables carry the
	// nested-field delta verbatim up to the enclosing ChannelStatus, this dynvec delta equals
	// the channel cell's data growth when the sub-alloc is materialised.
	zero, err := packUint128LE(new(big.Int))
	if err != nil {
		return 0, fmt.Errorf("packing zero sub-balance: %w", err)
	}
	subBalances := molecule.NewSubBalancesBuilder()
	for i := 0; i < numAssets; i++ {
		subBalances.Push(zero)
	}
	idxMap := molecule.NewIndexMapBuilder().
		Nth0(*types.PackByte(0)).
		Nth1(*types.PackByte(1)).
		Build()
	sub := molecule.NewSubAllocBuilder().
		Id(molecule.Byte32Default()).
		Balances(subBalances.Build()).
		IdxMap(idxMap).
		Build()

	empty := molecule.NewLockedBalancesBuilder().Build()
	withOne := molecule.NewLockedBalancesBuilder().Push(sub).Build()
	delta := len(withOne.AsSlice()) - len(empty.AsSlice())
	if delta < 0 {
		delta = 0
	}
	return uint64(delta) * ShannonsPerCapacityByte, nil
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

// PackEthDistribution converts a perun channel state to a molecule EthDistribution.
func PackEthDistribution(d [2]*big.Int) (molecule.ETHDistribution, error) {
	balA, err := molecule2.PackUint128(d[0])
	if err != nil {
		return molecule.ETHDistribution{}, err
	}
	balB, err := molecule2.PackUint128(d[1])
	if err != nil {
		return molecule.ETHDistribution{}, err
	}
	return molecule.NewETHDistributionBuilder().Nth0(*balA).Nth1(*balB).Build(), nil
}

// toU128LE returns a 16-byte little-endian encoding of x (u128).
func toU128LE(x *big.Int) ([16]byte, error) {
	var out [16]byte
	if x.Sign() < 0 {
		return out, fmt.Errorf("negative not allowed")
	}
	b := x.Bytes()
	if len(b) > 16 {
		return out, fmt.Errorf("overflow: %d bits", 8*len(b))
	}
	for i := 0; i < len(b); i++ {
		out[i] = b[len(b)-1-i]
	}
	return out, nil
}

func packUint128LE(x *big.Int) (molecule.Uint128, error) {
	le, err := toU128LE(x)
	if err != nil {
		return molecule.Uint128{}, err
	}
	u := molecule.Uint128FromSliceUnchecked(le[:])
	return *u, nil
}

func PackEthAsset(a pchannel.Asset) (molecule.ETHAsset, error) {
	multiAsset, ok := a.(multi.Asset)
	if !ok {
		errorMessage := "failed to parse asset to multi.Asset" + hex.EncodeToString(a.Address())
		return molecule.ETHAsset{}, errors.New(errorMessage)
	}
	id := new(big.Int)
	if _, ok := id.SetString(string(multiAsset.
		LedgerBackendID().
		LedgerID().
		MapKey()), 10); !ok {
		return molecule.ETHAsset{}, fmt.Errorf("parse chain id")
	}
	chainID, err := packUint128LE(id)
	if err != nil {
		return molecule.ETHAsset{}, err
	}
	addrBytes := a.Address()
	if len(addrBytes) != 20 {
		return molecule.ETHAsset{}, fmt.Errorf("asset address must be 20 bytes, got %d", len(addrBytes))
	}
	var arr20 [20]molecule.Byte
	for i, b := range addrBytes {
		arr20[i] = molecule.NewByte(b)
	}
	assetAddress := molecule.NewEthAddressBuilder().Set(arr20).Build()
	return molecule.NewETHAssetBuilder().
		ChainId(chainID).
		AssetAddress(assetAddress).
		Build(), nil
}

// PackEthBalances converts a perun channel state to a molecule EthDistribution.
func PackEthBalances(d [2]*big.Int, asset pchannel.Asset) (molecule.ETHBalances, error) {
	ethDist, err := PackEthDistribution(d)
	if err != nil {
		return molecule.ETHBalances{}, err
	}
	ethAsset, err := PackEthAsset(asset)
	if err != nil {
		return molecule.ETHBalances{}, err
	}
	return molecule.NewETHBalancesBuilder().Asset(ethAsset).Distribution(ethDist).Build(), nil
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
		a, ckb := asset.IsCompatibleAsset(pAsset)
		if !ckb {
			continue
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
