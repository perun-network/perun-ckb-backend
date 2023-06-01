package asset

import (
	"errors"
	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	pchannel "perun.network/go-perun/channel"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
)

// ckbAsset is the native asset of the chain (CKBytes).
type ckbAsset struct{}

// CKBAsset is the unique asset that is supported by the chain (CKBytes).
var CKBAsset = &ckbAsset{}

// MarshalBinary does nothing and returns nil since the backend has a singleton native asset.
func (ckbAsset) MarshalBinary() (data []byte, err error) {
	return
}

// UnmarshalBinary does nothing and returns nil since the backend has a singleton native asset.
func (*ckbAsset) UnmarshalBinary(data []byte) error {
	return nil
}

// Equal returns true if the assets are the same.
func (ckbAsset) Equal(b pchannel.Asset) bool {
	_, ok := b.(*ckbAsset)
	return ok
}

// SUDTAsset is the asset type for SUDT tokens.
type SUDTAsset struct {
	TypeScript  types.Script
	MaxCapacity uint64
}

// MarshalBinary encodes the SUDTAsset into a molecule SUDTAsset.
func (s SUDTAsset) MarshalBinary() (data []byte, err error) {
	asset := s.Pack()
	return asset.AsSlice(), nil
}

// UnmarshalBinary decodes the SUDTAsset from a molecule SUDTAsset.
func (s *SUDTAsset) UnmarshalBinary(data []byte) error {
	sudtAsset, err := molecule.SUDTAssetFromSlice(data, false)
	if err != nil {
		return err
	}
	s.Unpack(sudtAsset)
	return nil
}

// Pack encodes the SUDTAsset into a molecule SUDTAsset.
func (s SUDTAsset) Pack() molecule.SUDTAsset {
	return molecule.NewSUDTAssetBuilder().
		TypeScript(*s.TypeScript.Pack()).
		MaxCapacity(*types.PackUint64(s.MaxCapacity)).Build()
}

// Unpack unpacks the SUDTAsset from a molecule SUDTAsset.
func (s *SUDTAsset) Unpack(a *molecule.SUDTAsset) {
	s.TypeScript = *types.UnpackScript(a.TypeScript())
	s.MaxCapacity = molecule2.UnpackUint64(a.MaxCapacity())
}

// Equal returns true if the assets are the same.
func (s SUDTAsset) Equal(other pchannel.Asset) bool {
	otherSUDT, ok := other.(*SUDTAsset)
	if !ok {
		return false
	}
	return s.TypeScript.Equals(&otherSUDT.TypeScript) && s.MaxCapacity == otherSUDT.MaxCapacity
}

func IsSUDTAsset(asset pchannel.Asset) (*SUDTAsset, error) {
	a, ok := asset.(*SUDTAsset)
	if !ok {
		return nil, errors.New("asset is not of type SUDTAsset")
	}
	return a, nil
}

type SUDTBalances struct {
	Asset        SUDTAsset
	Distribution [2]uint128.Uint128
}
