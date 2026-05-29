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

//go:build !testnet
// +build !testnet

package client_test

import (
	"context"
	"testing"

	clienttest "perun.network/go-perun/client/test"
	"perun.network/go-perun/log"
	ctest "perun.network/perun-ckb-backend/client/test"
	"polycry.pt/poly-go/test"
)

// TestCrossVirtualChannelOptimistic exercises the virtual channel optimistic
// scenario with cross-chain participants (omni-lock + EVM signers), the VC
// counterpart of TestCrossPaymentHappy.
func TestCrossVirtualChannelOptimistic(t *testing.T) {
	log.Info("Starting cross-chain virtual channel happy test")
	rng := test.Prng(t)
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	setup := ctest.MakeVirtualChannelSetupCross(t, rng)
	clienttest.TestVirtualChannelOptimistic(ctx, t, setup)
	log.Info("Cross-chain virtual channel happy test done")
}

// TestCrossVirtualChannelDispute exercises the virtual channel dispute
// scenario with cross-chain participants. This is the path enabled by the
// cross-chain-vc-fix contract branch (idx_map + flat SubBalances vector).
func TestCrossVirtualChannelDispute(t *testing.T) {
	log.Info("Starting cross-chain virtual channel dispute test")
	rng := test.Prng(t)
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	setup := ctest.MakeVirtualChannelSetupCross(t, rng)
	clienttest.TestVirtualChannelDispute(ctx, t, setup)
	log.Info("Cross-chain virtual channel dispute test done")
}
