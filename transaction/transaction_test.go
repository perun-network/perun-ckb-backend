package transaction_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/numeric"
	"github.com/stretchr/testify/require"
	"perun.network/go-perun/channel/test"
	btest "perun.network/perun-ckb-backend/backend/test"
	"perun.network/perun-ckb-backend/transaction"
	txtest "perun.network/perun-ckb-backend/transaction/test"
	ptest "polycry.pt/poly-go/test"
)

func TestScriptHandler(t *testing.T) {
	rng := ptest.Prng(t)
	defaultLock := btest.NewRandomScript(rng)
	pctsDep := btest.NewRandomCellDep(rng)
	pclsDep := btest.NewRandomCellDep(rng)
	pflsDep := btest.NewRandomCellDep(rng)
	psh := transaction.NewPerunScriptHandler(
		*pctsDep, *pclsDep, *pflsDep,
		types.Hash{}, types.HashTypeData,
		types.Hash{}, types.HashTypeData,
		types.Hash{}, types.HashTypeData,
		*defaultLock,
	)

	mockHandler := txtest.NewMockHandler(defaultLock)
	funding := uint64(numeric.NewCapacityFromCKBytes(420_690))
	changeAmount := uint64(numeric.NewCapacityFromCKBytes(100_000))
	mockIterator := txtest.NewMockIterator(
		txtest.WithLockScript(defaultLock),
		txtest.WithTypeScript(nil),
		txtest.WithCapacityAtLeast(funding+changeAmount),
	)
	mockIterator.GenerateInput(ptest.Prng(t))
	mockIterator.GenerateInput(ptest.Prng(t))
	mockIterator.GenerateInput(ptest.Prng(t))
	b := transaction.NewPerunTransactionBuilder(types.NetworkTest, mockIterator, psh)
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
	oi := transaction.OpenInfo{
		ChannelID:    [32]byte{},
		ChannelToken: btest.NewRandomToken(rng),
		Funding:      funding,
		Params:       params,
		State:        state,
	}

	twsg, err := b.Build(oi, txtest.MockContext{})
	// TODO: The mock inputs have to have registered scripthandlers for them to
	// be added to the final transaction:
	//	* Set a fixed lockscript for the mock inputs.
	//	* Create a scripthandler which can act on an injectable lockscript.
	//	* Register the testing scripthandler using the predefined lockscript.
	require.NoError(t, err)
	fmt.Println(twsg)
}
