// Copyright 2025 PolyCrypt GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package asset

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/multi"
	"perun.network/go-perun/wallet"
	"perun.network/go-perun/wire/perunio"
	// "perun.network/perun-stellar-backend/wallet/types"
)

var _ channel.Asset = new(EthAsset)

// ChainID identifies a specific Ethereum backend.
type ChainID struct {
	*big.Int
}
type EthAddress common.Address

// BackendID returns the official identifier for the eth-backend.
func (a *EthAddress) BackendID() wallet.BackendID {
	return 1
}

// bytes returns the address as a byte slice.
func (a *EthAddress) bytes() []byte {
	return (*common.Address)(a).Bytes()
}

// MarshalBinary marshals the address into its binary representation.
// Error will always be nil, it is for implementing BinaryMarshaler.
func (a *EthAddress) MarshalBinary() ([]byte, error) {
	return (*common.Address)(a).Bytes(), nil
}

// UnmarshalBinary unmarshals the address from its binary representation.
func (a *EthAddress) UnmarshalBinary(data []byte) error {
	if len(data) != 20 { //nolint:gomnd
		return fmt.Errorf("unexpected address length %d, want %d", len(data), 20) //nolint:gomnd
	}

	(*common.Address)(a).SetBytes(data)
	return nil
}

// String converts this address to a string.
func (a *EthAddress) String() string {
	return (*common.Address)(a).String()
}

// Equal checks the equality of two addresses. The implementation must be
// equivalent to checking `Address.Cmp(Address) == 0`.
func (a *EthAddress) Equal(addr wallet.Address) bool {
	addrTyped, ok := addr.(*EthAddress)
	if !ok {
		return false
	}
	return bytes.Equal(a.bytes(), addrTyped.bytes())
}

// Cmp checks ordering of two addresses.
//
//	0 if a==b,
//
// -1 if a < b,
// +1 if a > b.
// https://godoc.org/bytes#Compare
//
// Panics if the input is not of the same type as the receiver.
func (a *EthAddress) Cmp(addr wallet.Address) int {
	addrTyped, ok := addr.(*EthAddress)
	if !ok {
		panic(fmt.Sprintf("wrong type: expected %T, got %T", a, addr))
	}
	return bytes.Compare(a.bytes(), addrTyped.bytes())
}

// MakeChainID makes a ChainID for the given id.
func MakeChainID(id *big.Int) ChainID {
	if id.Sign() < 0 {
		panic("must not be smaller than zero")
	}
	return ChainID{id}
}

// UnmarshalBinary unmarshals the chainID from its binary representation.
func (id *ChainID) UnmarshalBinary(data []byte) error {
	id.Int = new(big.Int).SetBytes(data)
	return nil
}

// MarshalBinary marshals the chainID into its binary representation.
func (id ChainID) MarshalBinary() (data []byte, err error) {
	if id.Sign() == -1 {
		return nil, errors.New("cannot marshal negative ChainID")
	}
	return id.Bytes(), nil
}

// MapKey returns the asset's map key representation.
func (id ChainID) MapKey() multi.LedgerIDMapKey {
	return multi.LedgerIDMapKey(id.Int.String())
}

type (
	// EthAsset is an Ethereum asset.
	EthAsset struct {
		assetID     LedgerBackendID
		AssetHolder wallet.Address
	}
	// LedgerBackendID holds the ChainID and BackendID of an Asset.
	LedgerBackendID struct {
		backendID uint32
		ledgerID  ChainID
	}

	// AssetMapKey is the map key representation of an asset.
	AssetMapKey string
)

// MakeEthAsset creates a new Ethereum asset with the given id and holder.
func MakeEthAsset(id *big.Int, holder wallet.Address) EthAsset {
	return EthAsset{assetID: LedgerBackendID{backendID: EthBackendID, ledgerID: MakeChainID(id)}, AssetHolder: holder}
}

// BackendID returns the identifier for the eth-backend.
func (id LedgerBackendID) BackendID() uint32 {
	return id.backendID
}

// LedgerID returns the ledger ID the asset lives on.
func (id LedgerBackendID) LedgerID() multi.LedgerID {
	return &id.ledgerID
}

// MakeLedgerBackendID makes a LedgerBackendID for the given id.
func MakeLedgerBackendID(id *big.Int) multi.LedgerBackendID {
	if id.Sign() < 0 {
		panic("must not be smaller than zero")
	}
	return LedgerBackendID{backendID: EthBackendID, ledgerID: MakeChainID(id)}
}

// MapKey returns the asset's map key representation.
func (a EthAsset) MapKey() AssetMapKey {
	d, err := a.MarshalBinary()
	if err != nil {
		panic(err)
	}

	return AssetMapKey(d)
}

// MarshalBinary marshals the asset into its binary representation.
func (a EthAsset) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	err := perunio.Encode(&buf, a.assetID.LedgerID, a.assetID.backendID, &a.AssetHolder)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary unmarshals the asset from its binary representation.
func (a *EthAsset) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	return perunio.Decode(buf, &a.assetID.ledgerID, &a.assetID.backendID, &a.AssetHolder)
}

// LedgerID returns the ledger ID the asset lives on.
func (a EthAsset) LedgerID() multi.LedgerID {
	return a.LedgerBackendID().LedgerID()
}

// LedgerBackendID returns the ledger ID the asset lives on.
func (a EthAsset) LedgerBackendID() multi.LedgerBackendID {
	return a.assetID
}

// Equal returns true iff the asset equals the given asset.
func (a EthAsset) Equal(b channel.Asset) bool {
	ethAsset, ok := b.(*EthAsset)
	if !ok {
		return false
	}
	return a.assetID.ledgerID.MapKey() == ethAsset.assetID.ledgerID.MapKey() && a.AssetHolder.Equal(ethAsset.AssetHolder)
}

// Address returns the address of the asset.
func (a EthAsset) Address() []byte {
	data, _ := a.AssetHolder.MarshalBinary()
	return data
}
