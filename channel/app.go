// Copyright 2025 - See NOTICE file for copyright holders.
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

package channel

import (
	"log"
	"math/rand"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"

	address "perun.network/perun-ckb-backend/wallet/address"
	stwallet "perun.network/perun-ckb-backend/wallet/test"
)

var _ channel.AppID = new(AppID)

// AppID is a wrapper around a perun address to implement the AppID interface.
type AppID struct {
	wallet.Address
}

// AppIDKey is the key type for the AppID.
type AppIDKey string

// Equal compares two AppIDs.
func (a AppID) Equal(b channel.AppID) bool {
	bTyped, ok := b.(*AppID)
	if !ok {
		return false
	}
	return a.Address.Equal(bTyped.Address)
}

// Key returns the key of the AppID.
func (a AppID) Key() channel.AppIDKey {
	b, err := a.MarshalBinary()
	if err != nil {
		log.Fatalln(err)
		return "0"
	}
	return channel.AppIDKey(b)
}

// MarshalBinary marshals the AppID to binary.
func (a AppID) MarshalBinary() ([]byte, error) {
	data, err := a.Address.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// UnmarshalBinary unmarshals the AppID from binary.
func (a *AppID) UnmarshalBinary(data []byte) error {
	addr := &address.Participant{}
	err := addr.UnmarshalBinary(data)
	if err != nil {
		return err
	}
	appaddr := &AppID{addr}
	*a = *appaddr
	return nil
}

// NewRandomAppID creates a new random AppID.
func NewRandomAppID(rng *rand.Rand) *AppID {
	r := stwallet.Randomizer{}
	addr := r.NewRandomAddress(rng)
	return &AppID{addr}
}
