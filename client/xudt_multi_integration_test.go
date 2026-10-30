//go:build xudt_integration

package client_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
	ckbclient "perun.network/perun-ckb-backend/client"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
	"perun.network/perun-ckb-backend/transaction"
	ckbaddress "perun.network/perun-ckb-backend/wallet/address"
	"perun.network/perun-ckb-backend/xudt"
)

// Exercise multiple independent xUDT script groups sharing a wallet lock group.
// Each extension witness must coexist with the wallet's signature and the other
// token witnesses, and all three token balances must be conserved exactly.
func TestXUDTMultiAssetTransfer(t *testing.T) {
	f, cl := newXUDTFixture(t)
	for _, omni := range []bool{false, true} {
		name := "secp256k1"
		if omni {
			name = "omnilock"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			accounts, signers := f.wallets(t, omni)
			signer := signers[0]
			parts := make([]*ckbaddress.Participant, len(accounts))
			for i, account := range accounts {
				parts[i] = account.Address().(*ckbaddress.Participant)
			}
			d := f.Deployment
			d.XUDTs = make(map[types.Hash]xudt.Config)
			iters := make(map[types.Hash]collector.CellIterator)
			addr, err := signer.Address().Encode()
			require.NoError(t, err)
			ckbIter, err := collector.NewLiveCellIteratorFromAddress(cl, addr)
			require.NoError(t, err)
			iters[types.Hash{}] = ckbclient.NewCKBOnlyIterator(ckbIter)
			before := make([][2]*big.Int, len(f.Tokens))
			for i, cfg := range f.Tokens {
				d.XUDTs[cfg.Script.Hash()] = cfg
				iter, err := collector.NewLiveCellIteratorFromAddress(cl, addr)
				require.NoError(t, err)
				hash := cfg.Script.Hash()
				iters[hash] = ckbclient.NewFilteredCellIterator(iter, func(out *types.CellOutput) bool { return out.Type != nil && out.Type.Hash() == hash })
				for j := range parts {
					before[i][j] = xudtBalance(t, ctx, cl, parts[j].PaymentScript, &cfg.Script)
				}
			}
			b, err := transaction.NewPerunTransactionBuilderWithDeployment(cl, d, iters, signer.Address(), omni)
			require.NoError(t, err)
			amount := new(big.Int).Lsh(big.NewInt(1), 70)
			data, err := molecule2.PackUint128(amount)
			require.NoError(t, err)
			for _, cfg := range f.Tokens {
				b.AddOutput(&types.CellOutput{Lock: parts[1].PaymentScript, Type: &cfg.Script, Capacity: 400 * transaction.CKBYTE}, data.AsSlice())
			}
			tx, err := b.Build(signer.Contexts())
			require.NoError(t, err)
			signed, err := signer.SignTransaction(tx)
			require.NoError(t, err)
			hash, err := cl.SendTransaction(ctx, signed)
			require.NoError(t, err)
			waitForXUDTTransaction(t, ctx, cl, *hash)
			for i, cfg := range f.Tokens {
				before[i][0].Sub(before[i][0], amount)
				before[i][1].Add(before[i][1], amount)
				for j := range parts {
					require.Equal(t, 0, before[i][j].Cmp(xudtBalance(t, ctx, cl, parts[j].PaymentScript, &cfg.Script)))
				}
			}
		})
	}
}
