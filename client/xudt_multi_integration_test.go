//go:build xudt_integration

package client_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	ckbsigner "github.com/nervosnetwork/ckb-sdk-go/v2/transaction/signer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
	"perun.network/perun-ckb-backend/backend"
	ckbclient "perun.network/perun-ckb-backend/client"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
	"perun.network/perun-ckb-backend/transaction"
	"perun.network/perun-ckb-backend/wallet"
	ckbaddress "perun.network/perun-ckb-backend/wallet/address"
	"perun.network/perun-ckb-backend/xudt"
)

// Exercise multiple independent xUDT script groups sharing a wallet lock group.
// Each extension witness must coexist with the wallet's signature and the other
// token witnesses, and all three token balances must be conserved exactly.
func TestXUDTMultiAssetTransfer(t *testing.T) {
	raw, err := os.ReadFile(os.Getenv("PERUN_XUDT_FIXTURE"))
	require.NoError(t, err)
	var f xudtFixture
	require.NoError(t, json.Unmarshal(raw, &f))
	require.Len(t, f.Keys, 2)
	require.Len(t, f.Tokens, 3)
	cl, err := rpc.Dial(f.RPC)
	require.NoError(t, err)
	defer cl.Close()
	for _, omni := range []bool{false, true} {
		name := "secp256k1"
		if omni {
			name = "omnilock"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			parts := make([]*ckbaddress.Participant, 2)
			var signer backend.Signer
			for i, keyHex := range f.Keys {
				kb, err := hex.DecodeString(keyHex)
				require.NoError(t, err)
				key := secp256k1.PrivKeyFromBytes(kb)
				parts[i] = wallet.NewAccountFromPrivateKey(key, f.Deployment.OmniLockScript.CodeHash, !omni).Address().(*ckbaddress.Participant)
				if i != 0 {
					continue
				}
				addr := parts[i].ToCKBAddress(types.NetworkTest)
				if omni {
					_, auth, err := ckbaddress.NewEthereumParticipantFromPublicKey(key.PubKey(), f.Deployment.OmniLockScript.CodeHash)
					require.NoError(t, err)
					evm := backend.NewEVMSignerInstance(addr, *key, types.NetworkTest, auth)
					ts := evm.Signer()
					ts.RegisterLockSigner(f.Deployment.OmniLockScript.CodeHash, &ckbsigner.OmnilockSigner{})
					signer = evm
				} else {
					signer = backend.NewSignerInstance(addr, *key, types.NetworkTest)
				}
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
				mode, err := cfg.Mode()
				require.NoError(t, err)
				if mode > 0 {
					cfg.WitnessData = func(*types.Transaction, []uint32, []uint32) ([][]byte, error) {
						return [][]byte{[]byte("channel-proof")}, nil
					}
					cfg.OutputData = func(types.CellOutput, *big.Int) ([]byte, error) {
						return xudt.EncodeData(nil, [][]byte{[]byte("channel-data")}), nil
					}
				}
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
			require.Eventually(t, func() bool {
				tx, e := cl.GetTransaction(ctx, *hash)
				return e == nil && tx != nil && tx.TxStatus.Status == types.TransactionStatusCommitted
			}, 45*time.Second, 200*time.Millisecond)
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
