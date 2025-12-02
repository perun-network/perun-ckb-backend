// Copyright 2025 PolyCrypt GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package address

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"perun.network/go-perun/wallet"
)

// AddressBinaryLen is the length of the binary representation of Address, in
// bytes.
const (
	AddressBinaryLen = common.AddressLength
	EthBackendID     = 1
)

// EthAddress represents an ethereum address as a perun address.
type EthAddress common.Address

// compile time check that we implement the perun Address interface.
var _ wallet.Address = (*EthAddress)(nil)

// bytes returns the address as a byte slice.
func (a *EthAddress) bytes() []byte {
	return (*common.Address)(a).Bytes()
}

// BackendID returns the ethereum backend ID.
func (a *EthAddress) BackendID() wallet.BackendID {
	return EthBackendID
}

// MarshalBinary marshals the address into its binary representation.
// Error will always be nil, it is for implementing BinaryMarshaler.
func (a *EthAddress) MarshalBinary() ([]byte, error) {
	return (*common.Address)(a).Bytes(), nil
}

// UnmarshalBinary unmarshals the address from its binary representation.
func (a *EthAddress) UnmarshalBinary(data []byte) error {
	if len(data) != AddressBinaryLen {
		return fmt.Errorf("unexpected address length %d, want %d", len(data), AddressBinaryLen) //nolint: goerr113
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

// AsEthAddr is a helper function to convert an address interface back into an
// ethereum address.
func AsEthAddr(a wallet.Address) common.Address {
	addrTyped, ok := a.(*EthAddress)
	if !ok {
		panic(fmt.Sprintf("wrong type: expected %T, got %T", &EthAddress{}, a))
	}
	return common.Address(*addrTyped)
}

// AsWalletAddr is a helper function to convert an ethereum address to an
// address interface.
func AsWalletAddr(addr common.Address) *EthAddress {
	return (*EthAddress)(&addr)
}
