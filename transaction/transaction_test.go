package transaction_test

import (
	"math/big"
	"reflect"
	"testing"

	ckbtransaction "github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/numeric"
	"github.com/stretchr/testify/require"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/test"
	btest "perun.network/perun-ckb-backend/backend/test"
	"perun.network/perun-ckb-backend/transaction"
	txtest "perun.network/perun-ckb-backend/transaction/test"
	"perun.network/perun-ckb-backend/wallet/address"
	wtest "perun.network/perun-ckb-backend/wallet/test"
	ptest "polycry.pt/poly-go/test"
)

func TestScriptHandler(t *testing.T) {
	rng := ptest.Prng(t)
	sender := wtest.NewRandomParticipant(rng)
	senderCkbAddr, err := sender.ToCKBAddress(types.NetworkTest)
	require.NoError(t, err, "converting perun.backend.Participant to ckb-sdk-go address")
	defaultLock := btest.NewRandomScript(rng)
	defaultLockDep := btest.NewRandomCellDep(rng)
	pctsDep := btest.NewRandomCellDep(rng)
	pclsDep := btest.NewRandomCellDep(rng)
	pflsDep := btest.NewRandomCellDep(rng)
	psh := transaction.NewPerunScriptHandler(
		*pctsDep, *pclsDep, *pflsDep,
		types.Hash{}, types.HashTypeData,
		types.Hash{}, types.HashTypeData,
		types.Hash{}, types.HashTypeData,
		*defaultLock,
		*defaultLockDep,
	)

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

	t.Run("Open", func(t *testing.T) {
		mockIterator := mkMockIterator()
		b, err := transaction.NewPerunTransactionBuilder(types.NetworkTest, mockIterator, psh, senderCkbAddr)
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
		oi := transaction.OpenInfo{
			ChannelID:    [32]byte{},
			ChannelToken: btest.NewRandomToken(rng),
			Funding:      funding,
			Params:       params,
			State:        state,
		}

		_, err = b.Build(oi, txtest.MockContext{})
		require.NoError(t, err)
	})

	t.Run("Fund", func(t *testing.T) {
		mockIterator := mkMockIterator()
		b, err := transaction.NewPerunTransactionBuilder(types.NetworkTest, mockIterator, psh, senderCkbAddr)
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

		fi := transaction.FundInfo{
			Amount:      420_690,
			ChannelCell: *btest.NewRandomOutpoint(rng),
			Params:      params,
			Token:       btest.NewRandomToken(rng),
			Status:      *btest.NewRandomChannelStatus(rng, btest.WithState(state), btest.WithFunding(molecule.BalancesDefault())),
		}

		_, err = b.Build(fi, txtest.MockContext{})
		require.NoError(t, err)
	})

}

// Example output:
// *github.com/nervosnetwork/ckb-sdk-go/v2/types.Transaction {
//		Version: 0, Hash: github.com/nervosnetwork/ckb-sdk-go/v2/types.Hash [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],
//    CellDeps: []*github.com/nervosnetwork/ckb-sdk-go/v2/types.CellDep len: 1, cap: 1, [
//              *(*"github.com/nervosnetwork/ckb-sdk-go/v2/types.CellDep")(0xc00013e360),
//    ],
//    HeaderDeps: []github.com/nervosnetwork/ckb-sdk-go/v2/types.Hash len: 0, cap: 0, nil,
//    Inputs: []*github.com/nervosnetwork/ckb-sdk-go/v2/types.CellInput len: 2, cap: 2, [
//              *(*"github.com/nervosnetwork/ckb-sdk-go/v2/types.CellInput")(0xc00018a3b0),
//							*(*"github.com/nervosnetwork/ckb-sdk-go/v2/types.CellInput")(0xc00018a3c0),
//    ],
//    Outputs: []*github.com/nervosnetwork/ckb-sdk-go/v2/types.CellOutput len: 3, cap: 4, [
//              *(*"github.com/nervosnetwork/ckb-sdk-go/v2/types.CellOutput")(0xc000012768),
//							*(*"github.com/nervosnetwork/ckb-sdk-go/v2/types.CellOutput")(0xc000013470),
//    					*(*"github.com/nervosnetwork/ckb-sdk-go/v2/types.CellOutput")(0xc000013680),
//    ],
//    OutputsData: [][]uint8 len: 3, cap: 4, [
//              [],
//              [0,0,0,0],
//              [127,0,0,0,127,0,0,0,20,0,0,0,101,0,0,0,106,0,0,0,122,0,0,0,81,0,0,0,20,0,0,0,52,0,0,0,68,0,0,0,76,0,0,0,90,86,116,59,198,53,20,55,241,48,110,209,59,70,224,83,24,195,74,108,...+67 more],
//    ],
//    Witnesses: [][]uint8 len: 2, cap: 2, [
//              [],
//              [],
//    ],
// }

type OpenArgs struct {
	ChannelID channel.ID
	Data      []byte
	PCTS      *types.Script
	PCLS      *types.Script
	Initiator address.Participant
}

// verifyOpenTransaction verifies the given tx to contain the minimal set of
// cells and data, s.t. it can be used to open a channel.
func verifyOpenTransaction(t *testing.T, tx *ckbtransaction.TransactionWithScriptGroups, args OpenArgs) {
	// Open transaction has to contain two celldeps:
	// 1. The funding cell's lockscript cell dependency.
	// 2. The Channel's typescript cell dependency.
	require.Len(t, tx.TxView.CellDeps, 2)

	// We expect 3 outputs:
	// 1. The channel cell.
	// 2. The funding cell.
	// 3. The change cell.
	require.Len(t, tx.TxView.Outputs, 3)
	verifyRequiredOutputs(t, tx.TxView.Outputs, tx.TxView.OutputsData, args)
}

func verifyRequiredOutputs(t *testing.T, outputs []*types.CellOutput, outputsData [][]byte, args OpenArgs) {
	validate := func(pred func(*types.CellOutput, []byte) bool) bool {
		var r bool
		for oIdx, o := range outputs {
			if ok := pred(o, outputsData[oIdx]); ok {
				r = ok
				break
			}
		}
		return r
	}

	containsValidChannelCell := func(o *types.CellOutput, d []byte) bool {
		if ok := isCorrectChannelTypeScript(o, args); !ok {
			return ok
		}

		if ok := isCorrectChannelLockScript(o, args); !ok {
			return ok
		}

		if ok := isCorrectChannelData(d, args); !ok {
			return ok
		}
		return true
	}

	require.True(t, validate(containsValidChannelCell), "valid channel must be present")
}

func isCorrectChannelTypeScript(o *types.CellOutput, args OpenArgs) bool {
	return o.Type == args.PCTS
}

func isCorrectChannelLockScript(o *types.CellOutput, args OpenArgs) bool {
	return o.Lock == args.PCLS
}

func isCorrectChannelData(d []byte, args OpenArgs) bool {
	return reflect.DeepEqual(d, args.Data)
}
