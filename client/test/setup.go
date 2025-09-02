package test

import (
	"log"
	"math/rand"
	"time"

	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"

	"perun.network/perun-ckb-backend/channel/test"

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
	TestnetChallengeDurationBlocks = 120
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
