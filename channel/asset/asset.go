package asset

import (
	"errors"
	"fmt"
	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	pchannel "perun.network/go-perun/channel"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
)

var CKByteMagic byte = 0x00
var SUDTMagic byte = 0x01

type Asset struct {
	IsCKBytes bool
	SUDT      *SUDT
}

func (a Asset) MarshalBinary() (data []byte, err error) {
	if a.IsCKBytes {
		return []byte{CKByteMagic}, nil
	}
	e, err := a.SUDT.Encode()
	if err != nil {
		return nil, err
	}
	return append([]byte{SUDTMagic}, e...), nil
}

func (a Asset) UnmarshalBinary(data []byte) error {
	if len(data) < 1 {
		return errors.New("asset invalid: empty")
	}
	switch data[0] {
	case CKByteMagic:
		if len(data) != 1 {
			return errors.New("asset invalid: invalid CKByte asset")
		}
		a.IsCKBytes = true
		a.SUDT = nil
		return nil
	case SUDTMagic:
		s := &SUDT{}
		err := s.Decode(data[1:])
		if err != nil {
			return fmt.Errorf("unable to decode SUDT asset: %w", err)
		}
		a.IsCKBytes = false
		a.SUDT = s
		return nil
	default:
		return errors.New("asset invalid: unknown asset type")
	}
}

func (a Asset) Equal(other pchannel.Asset) bool {
	o, ok := other.(*Asset)
	if !ok {
		return false
	}
	if a.IsCKBytes && o.IsCKBytes {
		return true
	}
	if a.IsCKBytes || o.IsCKBytes {
		return false
	}
	// This should not trigger for valid assets, but we add it for nil-safety.
	// This implies, if an invalid asset is compared to anything, it will return false.
	if a.SUDT == nil || o.SUDT == nil {
		return false
	}
	return a.SUDT.Equal(*o.SUDT)
}

// IsInvalid returns true if the asset is invalid.
func (a Asset) IsInvalid() bool {
	return (!a.IsCKBytes) && (a.SUDT == nil)
}

func NewInvalidAsset() *Asset {
	return &Asset{IsCKBytes: false, SUDT: nil}
}

func NewCKBytesAsset() *Asset {
	return &Asset{IsCKBytes: true}
}

func NewSUDTAsset(sudt *SUDT) *Asset {
	return &Asset{IsCKBytes: false, SUDT: sudt}
}

// IsCompatibleAsset returns the Asset if the asset is compatible with the CKB backend.
func IsCompatibleAsset(asset pchannel.Asset) (*Asset, error) {
	a, ok := asset.(*Asset)
	if !ok {
		return nil, errors.New("asset is not of type Asset")
	}
	return a, nil
}

// SUDT is the asset type for SUDT tokens.
type SUDT struct {
	TypeScript  types.Script
	MaxCapacity uint64
}

func NewSUDT(typeScript types.Script, maxCapacity uint64) *SUDT {
	return &SUDT{TypeScript: typeScript, MaxCapacity: maxCapacity}
}

// Encode encodes the SUDT to bytes in molecule SUDTAsset representation.
func (s SUDT) Encode() (data []byte, err error) {
	asset := s.Pack()
	return asset.AsSlice(), nil
}

// Decode decodes the SUDT from a molecule SUDTAsset byte representation.
func (s *SUDT) Decode(data []byte) error {
	sudtAsset, err := molecule.SUDTAssetFromSlice(data, false)
	if err != nil {
		return err
	}
	s.Unpack(sudtAsset)
	return nil
}

// Pack encodes the SUDT into a molecule SUDTAsset.
func (s SUDT) Pack() molecule.SUDTAsset {
	return molecule.NewSUDTAssetBuilder().
		TypeScript(*s.TypeScript.Pack()).
		MaxCapacity(*types.PackUint64(s.MaxCapacity)).Build()
}

// Unpack unpacks the SUDT from a molecule SUDTAsset.
func (s *SUDT) Unpack(a *molecule.SUDTAsset) {
	s.TypeScript = *types.UnpackScript(a.TypeScript())
	s.MaxCapacity = molecule2.UnpackUint64(a.MaxCapacity())
}

// Equal returns true if the SUDTs are the same.
func (s SUDT) Equal(other SUDT) bool {
	return s.TypeScript.Equals(&other.TypeScript) && s.MaxCapacity == other.MaxCapacity
}

// IsSUDTAsset returns true if the asset is a SUDT asset.
func IsSUDTAsset(asset pchannel.Asset) (*SUDT, error) {
	a, ok := asset.(*Asset)
	if !ok {
		return nil, errors.New("asset is not of type SUDT")
	}
	if a.IsCKBytes {
		return nil, errors.New("asset is not of type SUDT")
	}
	if a.SUDT == nil {
		return nil, errors.New("asset invalid: SUDT is nil but not a CKByte")
	}
	return a.SUDT, nil
}

type SUDTBalances struct {
	Asset        SUDT
	Distribution [2]uint128.Uint128
}
