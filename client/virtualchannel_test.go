package client_test

/*import (
	"context"
	"math/big"
	"math/rand"
	"testing"
	"time"

	clienttest "perun.network/go-perun/client/test"
	"perun.network/go-perun/log"
	"perun.network/perun-ckb-backend/channel/asset"
	btest "perun.network/perun-ckb-backend/channel/test"
	ctest "perun.network/perun-ckb-backend/client/test"
	"perun.network/perun-ckb-backend/transaction"
	"polycry.pt/poly-go/test"
)

const (
	testDuration = 120 * time.Second
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

	setup := makeVirtualChannelSetup(t, rng)
	clienttest.TestVirtualChannelOptimistic(ctx, t, setup)
	log.Info("Virtual channel happy test done")
}

// TestVirtualChannelDispute tests the virtual channel dispute scenario.
// It creates a virtual channel between Alice and Bob via Ingrid, and then disputes the
// channel state. The test checks if the dispute is resolved correctly and the final balances
// are as expected.
func TestVirtualChannelDispute(t *testing.T) {
	log.Info("Starting virtual channel dispute test")
	rng := test.Prng(t)
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	setup := makeVirtualChannelSetup(t, rng)
	clienttest.TestVirtualChannelDispute(ctx, t, setup)
	log.Info("Virtual channel dispute test done")
}

func makeVirtualChannelSetup(t *testing.T, rng *rand.Rand) clienttest.VirtualChannelSetup {
	t.Helper()
	name := [3]string{"Alice", "Bob", "Ingrid"}
	setup := btest.NewVirtualChannelSetup(t, rng, false)

	roleSetup := ctest.MakeRoleSetupsCross(rng, setup, name[:])

	return clienttest.VirtualChannelSetup{
		Clients:           [3]clienttest.RoleSetup(roleSetup),
		ChallengeDuration: roleSetup[0].ChallengeDuration,
		Asset:             setup.CkbAsset,
		Balances: clienttest.VirtualChannelBalances{
			InitBalsAliceIngrid: []*big.Int{asset.CKByteToShannon(big.NewFloat(100)), asset.CKByteToShannon(big.NewFloat(100))},
			InitBalsBobIngrid:   []*big.Int{asset.CKByteToShannon(big.NewFloat(100)), asset.CKByteToShannon(big.NewFloat(100))},
			InitBalsAliceBob:    []*big.Int{asset.CKByteToShannon(big.NewFloat(50)), asset.CKByteToShannon(big.NewFloat(50))},
			VirtualBalsUpdated:  []*big.Int{asset.CKByteToShannon(big.NewFloat(20)), asset.CKByteToShannon(big.NewFloat(80))},
			FinalBalsAlice:      []*big.Int{asset.CKByteToShannon(big.NewFloat(70)), asset.CKByteToShannon(big.NewFloat(130))},
			FinalBalsBob:        []*big.Int{asset.CKByteToShannon(big.NewFloat(130)), asset.CKByteToShannon(big.NewFloat(70))},
		},
		BalanceDelta:       big.NewInt(int64(6 * transaction.DefaultFeeShannon)), // Max Fee (Ingrid): (Open + Fund + 2 * Dispute + 2 * Close) * 1 CKB
		Rng:                rng,
		WaitWatcherTimeout: 1 * time.Second,
		IsUTXO:             true,
	}
}*/
