package asset

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"perun.network/go-perun/wire/perunio"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	pchannel "perun.network/go-perun/channel"
	"perun.network/go-perun/channel/multi"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
)

var (
	CKByteMagic  byte = 0x00
	SUDTMagic    byte = 0x01
	CKBBackendID      = 3
)

type (
	Asset struct {
		IsCKBytes bool
		SUDT      *SUDT
	}

	NervosAsset struct {
		Asset Asset
		id    CCID
	}

	// CCID is a unique identifier for a channel asset.
	CCID struct {
		backendID uint32
		ledgerID  ContractLID
	}

	// ContractLID is a unique identifier for a contract.
	ContractLID struct{ string }
)

// MarshalBinary marshals the NervosAsset into its binary representation.
func (C NervosAsset) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	err := perunio.Encode(&buf, C.id.ledgerID, C.id.backendID, C.Asset)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary unmarshals the NervosAsset from its binary representation.
func (C *NervosAsset) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	return perunio.Decode(buf, &C.id.ledgerID, &C.id.backendID, &C.Asset)
}

// Equal returns true if the CKBAssets are the same.
func (C NervosAsset) Equal(asset pchannel.Asset) bool {
	if nervAsset, ok := asset.(*NervosAsset); ok {
		log.Println("Nervos Equal", C.Asset, nervAsset.Asset)
		return C.Asset.Equal(&nervAsset.Asset)
	}
	log.Println("Equal", C.Asset, asset)
	return C.Asset.Equal(asset)
}

// Address returns the address of the asset.
func (C NervosAsset) Address() []byte {
	return C.Asset.Address()
}

// LedgerBackendID returns the ledger backend ID of the asset.
func (C NervosAsset) LedgerBackendID() multi.LedgerBackendID {
	return C.id
}

// NewNervosAsset creates a new NervosAsset.
func NewNervosAsset(asset Asset, id CCID) NervosAsset {
	return NervosAsset{Asset: asset, id: id}
}

// MakeCCID makes a CCID for the given id.
func MakeCCID(ledgerID ContractLID) CCID {
	return CCID{uint32(CKBBackendID), ledgerID}
}

// UnmarshalBinary unmarshals the contractID from its binary representation.
func (id *ContractLID) UnmarshalBinary(data []byte) error {
	str := hex.EncodeToString(data) // Convert binary data to hex string
	id.string = str
	return nil
}

// MarshalBinary marshals the contractID into its binary representation.
func (id ContractLID) MarshalBinary() ([]byte, error) {
	if id.string == "" {
		return nil, errors.New("nil ContractID")
	}
	return hex.DecodeString(id.string)
}

// MapKey returns the asset's map key representation.
func (id ContractLID) MapKey() multi.LedgerIDMapKey {
	if id.string == "" {
		return ""
	}
	return multi.LedgerIDMapKey(id.string)
}

// BackendID returns the backend ID of the asset.
func (c CCID) BackendID() uint32 {
	return c.backendID
}

// LedgerID returns the ledger ID of the asset.
func (c CCID) LedgerID() multi.LedgerID {
	return c.ledgerID
}

// MakeContractID makes a ChainID for the given id.
func MakeContractID(id string) ContractLID {
	return ContractLID{id}
}

func (a Asset) Address() []byte {
	if a.IsCKBytes {
		return nil
	}
	enc, _ := a.SUDT.Encode()
	return enc
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

func (a *Asset) UnmarshalBinary(data []byte) error {
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
	var otherAsset Asset
	o, ok := other.(*Asset)
	if !ok {
		o, ok := other.(*NervosAsset)
		if !ok {
			return false
		}
		otherAsset = o.Asset
	} else {
		otherAsset = *o
	}
	if a.IsCKBytes && otherAsset.IsCKBytes {
		return true
	}
	if a.IsCKBytes || otherAsset.IsCKBytes {
		return false
	}
	// This should not trigger for valid assets, but we add it for nil-safety.
	// This implies, if an invalid asset is compared to anything, it will return false.
	if a.SUDT == nil || otherAsset.SUDT == nil {
		return false
	}
	return a.SUDT.Equal(*otherAsset.SUDT)
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

func NewCKBytesNervosAsset() *NervosAsset {
	a := Asset{IsCKBytes: true}
	return &NervosAsset{Asset: a, id: CCID{uint32(CKBBackendID), ContractLID{"3"}}}
}

func NewSUDTAsset(sudt *SUDT) *Asset {
	return &Asset{IsCKBytes: false, SUDT: sudt}
}

// IsCompatibleAsset returns the Asset if the asset is compatible with the CKB backend.
func IsCompatibleAsset(asset pchannel.Asset) (*Asset, error) {
	a, ok := asset.(*NervosAsset)
	if !ok {
		b, ok := asset.(*Asset)
		if !ok {
			return nil, errors.New("asset is not of type Asset")
		} else {
			return b, nil
		}
	}
	return &a.Asset, nil
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
	var a *Asset
	na, ok := asset.(*NervosAsset)
	if !ok {
		a, ok = asset.(*Asset)
		if !ok {
			return nil, errors.New("asset is not of type SUDT")
		}
	} else {
		a = &na.Asset
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

// CKByteToShannon converts a given amount in CKByte to Shannon.
func CKByteToShannon(ckbyteAmount *big.Float) (shannonAmount *big.Int) {
	shannonPerCKByte := new(big.Int).Exp(big.NewInt(10), big.NewInt(8), nil)
	shannonPerCKByteFloat := new(big.Float).SetInt(shannonPerCKByte)
	shannonAmountFloat := new(big.Float).Mul(ckbyteAmount, shannonPerCKByteFloat)
	shannonAmount, _ = shannonAmountFloat.Int(nil)
	return shannonAmount
}

// ShannonToCKByte converts a given amount in Shannon to CKByte.
func ShannonToCKByte(shannonAmount *big.Int) *big.Float {
	shannonPerCKByte := new(big.Int).Exp(big.NewInt(10), big.NewInt(8), nil)
	shannonPerCKByteFloat := new(big.Float).SetInt(shannonPerCKByte)
	shannonAmountFloat := new(big.Float).SetInt(shannonAmount)
	return new(big.Float).Quo(shannonAmountFloat, shannonPerCKByteFloat)
}
