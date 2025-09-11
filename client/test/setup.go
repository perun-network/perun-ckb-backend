package test

import (
	"log"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"

	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/channel/test"
	"perun.network/perun-ckb-backend/transaction"

	gpwiretest "perun.network/go-perun/backend/sim/wire"
	clienttest "perun.network/go-perun/client/test"
	"perun.network/go-perun/watcher/local"
	"perun.network/go-perun/wire"
)

const (
	// DefaultTimeout is the default timeout for client tests.
	DefaultTimeout = 60 * time.Second
	// BlockInterval is the default block interval for the simulated chain.
	BlockInterval = 500 * time.Millisecond
	// challenge duration in blocks that is used by MakeRoleSetups.
	ChallengeDurationBlocks = 90
)

const (
	// Testnet Config
	TestnetTimeout                 = 5 * time.Minute
	TestnetBlockInterval           = 3 * time.Second
	TestnetChallengeDurationBlocks = 9
)

func MakeRoleSetups(rng *rand.Rand, s *test.Setup, names []string, isTestnet bool) []clienttest.RoleSetup {
	setups := make([]clienttest.RoleSetup, len(names))
	bus := wire.NewLocalBus()

	for i := 0; i < len(setups); i++ {
		watcher, err := local.NewWatcher(s.Adjs[i])
		if err != nil {
			panic("Error initializing watcher: " + err.Error())
		}

		var rpcURL string
		var challengeDuration uint64
		var timeout time.Duration
		if isTestnet {
			rpcURL = test.TestnetRpcNodeURL
			challengeDuration = TestnetChallengeDurationBlocks
			timeout = TestnetTimeout
		} else {
			rpcURL = test.DevnetRpcNodeURL
			challengeDuration = ChallengeDurationBlocks * uint64(time.Second/BlockInterval)
			timeout = DefaultTimeout
		}
		balanceRPC, err := rpc.Dial(rpcURL)
		if err != nil {
			panic("Error dialing RPC: " + err.Error())
		}

		errors := make(chan error)
		// Goroutine to listen for new errors and print them
		go func() {
			for err := range errors {
				if err != nil {
					log.Panicf("Error of %s: %s", names[i], err)
				}
			}
		}()

		setups[i] = clienttest.RoleSetup{
			Name:        names[i],
			Identity:    gpwiretest.NewRandomAccount(rng),
			Bus:         bus,
			Funder:      s.Funders[i],
			Adjudicator: s.Adjs[i],
			Watcher:     watcher,
			Wallet:      s.EphemeralWallets[i],
			Timeout:     timeout,
			// Scaled due to simbackend automining progressing faster than real time.
			ChallengeDuration: challengeDuration,
			Errors:            errors,
			BalanceReader:     test.NewBalanceReader(balanceRPC, s.WalletAccs[i].Address()),
		}

	}

	return setups
}

func MakePaymentChannelSetup(t *testing.T, rng *rand.Rand, isTestnet bool) clienttest.PaymentChannelSetup {
	t.Helper()
	name := [2]string{"Alice", "Bob"}
	var setup *test.Setup
	if isTestnet {
		setup = test.NewTestnetVirtualChannelSetup(t, rng)
	} else {
		setup = test.NewDevnetVirtualChannelSetup(t, rng)
	}
	roleSetup := MakeRoleSetups(rng, setup, name[:], isTestnet)

	return clienttest.PaymentChannelSetup{
		Clients:           [2]clienttest.RoleSetup(roleSetup),
		ChallengeDuration: roleSetup[0].ChallengeDuration,
		Asset:             setup.Asset,
		Balances: clienttest.PaymentChannelBalances{
			InitBalsAliceBob: []*big.Int{asset.CKByteToShannon(big.NewFloat(100)), asset.CKByteToShannon(big.NewFloat(100))},
			BalsUpdated:      []*big.Int{asset.CKByteToShannon(big.NewFloat(70)), asset.CKByteToShannon(big.NewFloat(130))},
			FinalBals:        []*big.Int{asset.CKByteToShannon(big.NewFloat(70)), asset.CKByteToShannon(big.NewFloat(130))},
		},
		BalanceDelta:       big.NewInt(int64(3 * transaction.DefaultFeeShannon)), // Max Fee: (Open + Dispute + Close) * 1 CKB
		Rng:                rng,
		WaitWatcherTimeout: 1 * time.Second,
		IsUTXO:             true,
	}
}

func MakeVirtualChannelSetup(t *testing.T, rng *rand.Rand, isTestnet bool) clienttest.VirtualChannelSetup {
	t.Helper()
	name := [3]string{"Alice", "Bob", "Ingrid"}
	var setup *test.Setup
	if isTestnet {
		setup = test.NewTestnetVirtualChannelSetup(t, rng)
	} else {
		setup = test.NewDevnetVirtualChannelSetup(t, rng)
	}

	roleSetup := MakeRoleSetups(rng, setup, name[:], isTestnet)

	return clienttest.VirtualChannelSetup{
		Clients:           [3]clienttest.RoleSetup(roleSetup),
		ChallengeDuration: roleSetup[0].ChallengeDuration,
		Asset:             setup.Asset,
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
}
