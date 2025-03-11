package test

import (
	"log"
	"math/rand"
	"time"

	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"

	"perun.network/perun-ckb-backend/channel/test"
	ckbchanneltest "perun.network/perun-ckb-backend/channel/test"

	gpwiretest "perun.network/go-perun/backend/sim/wire"
	clienttest "perun.network/go-perun/client/test"
	"perun.network/go-perun/watcher/local"
	"perun.network/go-perun/wire"
)

const (
	// DefaultTimeout is the default timeout for client tests.
	DefaultTimeout = 60 * time.Second
	// BlockInterval is the default block interval for the simulated chain.
	BlockInterval = 200 * time.Millisecond
	// challenge duration in blocks that is used by MakeRoleSetups.
	challengeDurationBlocks = 90
)

func MakeRoleSetups(rng *rand.Rand, s *ckbchanneltest.Setup, names []string) []clienttest.RoleSetup {
	setups := make([]clienttest.RoleSetup, len(names))
	bus := wire.NewLocalBus()

	for i := 0; i < len(setups); i++ {
		watcher, err := local.NewWatcher(s.Adjs[i])
		if err != nil {
			panic("Error initializing watcher: " + err.Error())
		}

		balanceRPC, err := rpc.Dial(test.RpcNodeURL)
		if err != nil {
			panic("Error dialing RPC: " + err.Error())
		}

		errors := make(chan error)
		// Goroutine to listen for new errors and print them
		go func() {
			for err := range errors {
				if err != nil {
					log.Printf("Error of %s: %s", names[i], err)
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
			Timeout:     DefaultTimeout,
			// Scaled due to simbackend automining progressing faster than real time.
			ChallengeDuration: uint64(10),
			Errors:            errors,
			BalanceReader:     ckbchanneltest.NewBalanceReader(balanceRPC, s.WalletAccs[i].Address()),
		}

	}

	return setups
}
