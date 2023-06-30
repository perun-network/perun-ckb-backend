package transaction_test

import (
	"math/big"
	"testing"

	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/numeric"
	"github.com/stretchr/testify/require"
	"perun.network/go-perun/channel/test"
	btest "perun.network/perun-ckb-backend/backend/test"
	"perun.network/perun-ckb-backend/transaction"
	txtest "perun.network/perun-ckb-backend/transaction/test"
	wtest "perun.network/perun-ckb-backend/wallet/test"
	ptest "polycry.pt/poly-go/test"
)

var zeroHash types.Hash = types.Hash{}

func TestScriptHandler(t *testing.T) {
	rng := ptest.Prng(t)
	sender := wtest.NewRandomParticipant(rng)
	senderCkbAddr := sender.ToCKBAddress(types.NetworkTest)
	defaultLock := btest.NewRandomScript(rng)
	defaultLockDep := btest.NewRandomCellDep(rng)
	pctsDep := btest.NewRandomCellDep(rng)
	pclsDep := btest.NewRandomCellDep(rng)
	pflsDep := btest.NewRandomCellDep(rng)
	deployment := btest.NewRandomDeployment(rng,
		btest.WithPCTS(types.Hash{}, *pctsDep, types.HashTypeData),
		btest.WithPCLS(types.Hash{}, *pclsDep, types.HashTypeData),
		btest.WithPFLS(types.Hash{}, *pflsDep, types.HashTypeData),
		btest.WithDefaultLockScript(*defaultLock, *defaultLockDep),
	)
	psh := transaction.NewPerunScriptHandlerWithDeployment(*deployment)

	mockHandler := txtest.NewMockHandler(defaultLock)
	funding := uint64(numeric.NewCapacityFromCKBytes(420_690))
	changeAmount := uint64(numeric.NewCapacityFromCKBytes(100_000))
	mkMockIterator := func() *txtest.MockIterator {
		mockIterator := txtest.NewMockIterator(
			txtest.WithLockScript(defaultLock),
			// required because the iterator is only concerned with cells that can be
			// used to satisfy CKB invariants about CKBytes in cells.
			txtest.WithTypeScript(nil),
			txtest.WithCapacityAtLeast(funding+changeAmount),
		)
		mockIterator.GenerateInput(rng)
		mockIterator.GenerateInput(rng)
		mockIterator.GenerateInput(rng)
		return mockIterator
	}

	t.Run("Balancing", func(t *testing.T) {
		mockIterator := mkMockIterator()
		iters := map[types.Hash]collector.CellIterator{
			zeroHash: mockIterator,
		}
		client := txtest.NewMockClient()
		b, err := transaction.NewPerunTransactionBuilder(client, iters, make(map[types.Hash]types.Script), psh, senderCkbAddr)
		require.NoError(t, err, "creating perun transaction builder")
		b.Register(mockHandler)
		// Open
		state := test.NewRandomState(rng,
			test.WithNumParts(2),
			test.WithNumAssets(1),
			test.WithNumLocked(0),
			test.WithBalancesInRange(big.NewInt(100), big.NewInt(10_000)),
		)
		params := test.NewRandomParams(rng,
			test.WithNumParts(2),
			test.WithLedgerChannel(true),
			test.WithVirtualChannel(false),
			test.WithoutApp())
		oi := transaction.NewOpenInfo([32]byte{}, btest.NewRandomToken(rng), params, state)
		_, err = b.Build(oi, txtest.MockContext{})
		require.NoError(t, err)
	})

	t.Run("Fund", func(t *testing.T) {
		mockIterator := mkMockIterator()
		iters := map[types.Hash]collector.CellIterator{
			zeroHash: mockIterator,
		}
		client := txtest.NewMockClient()
		b, err := transaction.NewPerunTransactionBuilder(client, iters, make(map[types.Hash]types.Script), psh, senderCkbAddr)
		require.NoError(t, err, "creating perun transaction builder")
		b.Register(mockHandler)
		// Open
		state := test.NewRandomState(rng,
			test.WithNumParts(2),
			test.WithNumAssets(1),
			test.WithNumLocked(0),
			test.WithBalancesInRange(big.NewInt(100), big.NewInt(10_000)),
		)
		params := test.NewRandomParams(rng,
			test.WithNumParts(2),
			test.WithLedgerChannel(true),
			test.WithVirtualChannel(false),
			test.WithoutApp())

		fi := transaction.NewFundInfo(*btest.NewRandomOutpoint(rng),
			params,
			state,
			btest.NewRandomScript(rng),
			*btest.NewRandomChannelStatus(rng, btest.WithState(state)),
			btest.NewRandomHash(rng))

		_, err = b.Build(fi, txtest.MockContext{})
		require.NoError(t, err)
	})

}
