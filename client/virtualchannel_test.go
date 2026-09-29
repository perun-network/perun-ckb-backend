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

// TestVirtualChannelOptimistic tests the virtual channel optimistic scenario.
// It creates a virtual channel between Alice and Bob via Ingrid, and then
// performs a series of optimistic updates. The test checks if the final balances
// are as expected and if the channel state is updated correctly.
func TestVirtualChannelOptimistic(t *testing.T) {
	log.Info("Starting virtual channel happy test")
	rng := test.Prng(t)
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	setup := ctest.MakeVirtualChannelSetup(t, rng)
	clienttest.TestVirtualChannelOptimistic(ctx, t, setup)
	log.Info("Virtual channel happy test done")
}

// TestVirtualChannelDispute tests the virtual channel dispute scenario.
// It creates a virtual channel between Alice and Bob via Ingrid, and then disputes the
// channel state. The test checks if the dispute is resolved correctly and the final balances
// are as expected.
/* Note: This test is currently expected to fail due to the lack of support for virtual channel disputes in the current implementation. It is kept for future reference when virtual channel dispute support is added.
func TestVirtualChannelDispute(t *testing.T) {
	log.Info("Starting virtual channel dispute test")
	rng := test.Prng(t)
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	setup := ctest.MakeVirtualChannelSetup(t, rng)
	clienttest.TestVirtualChannelDispute(ctx, t, setup)
	log.Info("Virtual channel dispute test done")
}
*/
