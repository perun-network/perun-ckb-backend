//go:build xudt_integration

package client_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	ckbsigner "github.com/nervosnetwork/ckb-sdk-go/v2/transaction/signer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
	gptest "perun.network/go-perun/channel/test"
	gpwallet "perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/backend"
	ckbchannel "perun.network/perun-ckb-backend/channel"
	"perun.network/perun-ckb-backend/channel/asset"
	ckbclient "perun.network/perun-ckb-backend/client"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
	"perun.network/perun-ckb-backend/transaction"
	"perun.network/perun-ckb-backend/wallet"
	ckbaddress "perun.network/perun-ckb-backend/wallet/address"
	"perun.network/perun-ckb-backend/xudt"
	ptest "polycry.pt/poly-go/test"
)

// The fixture is produced by devnet/setup-xudt.sh and contains ONLY disposable
// devnet keys. These tests issue real transactions on a local CKB node.
type xudtFixture struct {
	IssuerKey       string
	RegressionKeys  []string
	RegressionToken xudt.Config
	RPC             string
	Deployment      backend.Deployment
	Keys            []string
	Tokens          []xudt.Config
}

func TestXUDTChannelLifecycle(t *testing.T) {
	file := os.Getenv("PERUN_XUDT_FIXTURE")
	require.NotEmpty(t, file, "set PERUN_XUDT_FIXTURE to a disposable devnet fixture")
	raw, err := os.ReadFile(file)
	require.NoError(t, err)
	var f xudtFixture
	require.NoError(t, json.Unmarshal(raw, &f))
	require.Len(t, f.Keys, 2)
	require.Len(t, f.Tokens, 3)
	rpcClient, err := rpc.Dial(f.RPC)
	require.NoError(t, err)
	defer rpcClient.Close()
	for _, omni := range []bool{false, true} {
		name := "secp256k1"
		if omni {
			name = "omnilock"
		}
		t.Run(name, func(t *testing.T) { testXUDTChannelLifecycle(t, f, rpcClient, omni) })
	}
}

func testXUDTChannelLifecycle(t *testing.T, f xudtFixture, rpcClient rpc.Client, omni bool) {
	for tokenIndex, token := range f.Tokens {
		t.Run(big.NewInt(int64(tokenIndex)).String(), func(t *testing.T) {
			mode, err := token.Mode()
			require.NoError(t, err)
			require.Equal(t, uint32(tokenIndex), mode, "fixture must cover modes 0, 1 and 2")
			if mode > 0 {
				token.WitnessData = func(*types.Transaction, []uint32, []uint32) ([][]byte, error) {
					return [][]byte{[]byte("channel-proof")}, nil
				}
				token.OutputData = func(types.CellOutput, *big.Int) ([]byte, error) {
					return xudt.EncodeData(nil, [][]byte{[]byte("channel-data")}), nil
				}
			}
			d := f.Deployment
			d.XUDTs = map[types.Hash]xudt.Config{token.Script.Hash(): token}
			accounts := make([]*wallet.Account, 2)
			clients := make([]*ckbclient.Client, 2)
			signers := make([]backend.Signer, 2)
			parts := make([]map[gpwallet.BackendID]gpwallet.Address, 2)
			for i, keyHex := range f.Keys {
				keyBytes, err := hex.DecodeString(keyHex)
				require.NoError(t, err)
				key := secp256k1.PrivKeyFromBytes(keyBytes)
				accounts[i] = wallet.NewAccountFromPrivateKey(key, d.OmniLockScript.CodeHash, !omni)
				p := accounts[i].Address().(*ckbaddress.Participant)
				parts[i] = map[gpwallet.BackendID]gpwallet.Address{3: p}
				ownerHash := p.UnlockScript.Hash()
				require.False(t, bytes.Equal(token.Script.Args[:32], ownerHash[:]), "participants must not be the token issuer")
				var signer backend.Signer
				if omni {
					_, auth, err := ckbaddress.NewEthereumParticipantFromPublicKey(key.PubKey(), d.OmniLockScript.CodeHash)
					require.NoError(t, err)
					evm := backend.NewEVMSignerInstance(p.ToCKBAddress(types.NetworkTest), *key, types.NetworkTest, auth)
					txSigner := evm.Signer()
					txSigner.RegisterLockSigner(d.OmniLockScript.CodeHash, &ckbsigner.OmnilockSigner{})
					signer = evm
				} else {
					signer = backend.NewSignerInstance(p.ToCKBAddress(types.NetworkTest), *key, types.NetworkTest)
				}
				signers[i] = signer
				clients[i], err = ckbclient.NewClient(rpcClient, signer, d)
				require.NoError(t, err)
			}
			tokenAsset, err := asset.NewXUDTAsset(token.Script, 400*100_000_000)
			require.NoError(t, err)
			nervosToken := asset.NewNervosAsset(*tokenAsset, asset.MakeCCID(asset.MakeContractID("03")))
			amount := new(big.Int).Lsh(big.NewInt(1), 80)
			actions := []string{"close", "abort", "dispute"}
			if mode > 0 {
				actions = append([]string{"reject-proof", "reject-data"}, actions...)
			}
			for _, action := range actions {
				t.Run(action, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
					defer cancel()
					before := make([]*big.Int, 2)
					for i := range before {
						before[i] = xudtBalance(t, ctx, rpcClient, parts[i][3].(*ckbaddress.Participant).PaymentScript, &token.Script)
					}
					params, state := gptest.NewRandomParamsAndState(ptest.Prng(t), gptest.WithParts(parts), gptest.WithLedgerChannel(true), gptest.WithVirtualChannel(false), gptest.WithoutApp(), gptest.WithChallengeDuration(1), gptest.WithBackend(3), gptest.WithBackendIDs([]gpwallet.BackendID{3, 3}), gptest.WithAssets(asset.NewCKBytesNervosAsset(), &nervosToken), gptest.WithNumLocked(0), gptest.WithVersion(0), gptest.WithIsFinal(false), gptest.WithBalances([]*big.Int{big.NewInt(1000 * 100_000_000), big.NewInt(1000 * 100_000_000)}, []*big.Int{new(big.Int).Set(amount), new(big.Int).Set(amount)}))
					if strings.HasPrefix(action, "reject-") {
						bad := token
						if action == "reject-proof" {
							bad.WitnessData = func(*types.Transaction, []uint32, []uint32) ([][]byte, error) {
								return [][]byte{[]byte("wrong-proof")}, nil
							}
						} else {
							bad.OutputData = func(types.CellOutput, *big.Int) ([]byte, error) {
								return xudt.EncodeData(nil, [][]byte{[]byte("wrong-data")}), nil
							}
						}
						badDeployment := d
						badDeployment.XUDTs = map[types.Hash]xudt.Config{token.Script.Hash(): bad}
						badClient, err := ckbclient.NewClient(rpcClient, signers[0], badDeployment)
						require.NoError(t, err)
						_, err = badClient.Start(ctx, params, state)
						require.ErrorContains(t, err, "ValidationFailure")
						for i := range before {
							require.Equal(t, 0, before[i].Cmp(xudtBalance(t, ctx, rpcClient, parts[i][3].(*ckbaddress.Participant).PaymentScript, &token.Script)))
						}
						return
					}
					pcts, err := clients[0].Start(ctx, params, state)
					require.NoError(t, err)
					if action == "abort" {
						require.NoError(t, clients[0].Abort(ctx, pcts, params, state))
					} else {
						require.NoError(t, clients[1].Fund(ctx, pcts, state, params))
						// Transfer a value above uint64 off chain, and verify both signatures.
						delta := new(big.Int).Lsh(big.NewInt(1), 70)
						state.Version++
						state.Balances[1][0].Sub(state.Balances[1][0], delta)
						state.Balances[1][1].Add(state.Balances[1][1], delta)
						state.IsFinal = action == "close"
						sigs := make([]gpwallet.Sig, 2)
						for i := range accounts {
							sigs[i], err = ckbchannel.Backend.Sign(accounts[i], state)
							require.NoError(t, err)
							valid, err := ckbchannel.Backend.Verify(accounts[i].Address(), state, sigs[i])
							require.NoError(t, err)
							require.True(t, valid)
						}
						if action == "close" {
							require.NoError(t, clients[0].Close(ctx, state.ID, state, sigs, params))
						} else {
							require.NoError(t, clients[0].Dispute(ctx, state.ID, state, sigs, params))
							// Wait for a later block timestamp, not just local wall-clock time.
							tip, err := rpcClient.GetTipHeader(ctx)
							require.NoError(t, err)
							require.Eventually(t, func() bool { h, e := rpcClient.GetTipHeader(ctx); return e == nil && h.Timestamp > tip.Timestamp+1000 }, 30*time.Second, 200*time.Millisecond)
							require.NoError(t, clients[1].ForceClose(ctx, state.ID, state, params))
						}
						before[0].Sub(before[0], delta)
						before[1].Add(before[1], delta)
					}
					_, _, _, _, err = clients[0].GetChannelWithID(ctx, state.ID)
					require.ErrorIs(t, err, ckbclient.ErrNoChannelLiveCell)
					for i := range before {
						after := xudtBalance(t, ctx, rpcClient, parts[i][3].(*ckbaddress.Participant).PaymentScript, &token.Script)
						require.Equal(t, 0, before[i].Cmp(after), "participant %d balance", i)
					}
				})
			}
		})
	}
}

func xudtBalance(t *testing.T, ctx context.Context, cl rpc.Client, lock, token *types.Script) *big.Int {
	t.Helper()
	sum := new(big.Int)
	cursor := ""
	for {
		cells, err := cl.GetCells(ctx, &indexer.SearchKey{Script: token, ScriptType: types.ScriptTypeType, ScriptSearchMode: types.ScriptSearchModeExact, WithData: true}, indexer.SearchOrderAsc, 100, cursor)
		require.NoError(t, err)
		for _, cell := range cells.Objects {
			if cell.Output.Lock.Hash() != lock.Hash() {
				continue
			}
			require.GreaterOrEqual(t, len(cell.OutputData), 16)
			be := make([]byte, 16)
			for i := range be {
				be[15-i] = cell.OutputData[i]
			}
			sum.Add(sum, new(big.Int).SetBytes(be))
		}
		if len(cells.Objects) == 0 {
			return sum
		}
		require.NotEqual(t, cursor, cells.LastCursor)
		cursor = cells.LastCursor
	}
}

func TestXUDTSeed(t *testing.T) {
	raw, err := os.ReadFile(os.Getenv("PERUN_XUDT_FIXTURE"))
	require.NoError(t, err)
	var f xudtFixture
	require.NoError(t, json.Unmarshal(raw, &f))
	cl, err := rpc.Dial(f.RPC)
	require.NoError(t, err)
	defer cl.Close()
	keyBytes, err := hex.DecodeString(f.IssuerKey)
	require.NoError(t, err)
	key := secp256k1.PrivKeyFromBytes(keyBytes)
	part, err := ckbaddress.NewDefaultParticipant(key.PubKey())
	require.NoError(t, err)
	addr := part.ToCKBAddress(types.NetworkTest)
	signer := backend.NewSignerInstance(addr, *key, types.NetworkTest)
	addrString, err := addr.Encode()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	for _, cfg := range f.Tokens {
		mode, err := cfg.Mode()
		require.NoError(t, err)
		if mode > 0 {
			cfg.WitnessData = func(*types.Transaction, []uint32, []uint32) ([][]byte, error) { return [][]byte{nil}, nil }
		}
		iter, err := collector.NewLiveCellIteratorFromAddress(cl, addrString)
		require.NoError(t, err)
		d := f.Deployment
		d.XUDTs = nil
		d.SUDTs = nil
		b, err := transaction.NewPerunTransactionBuilderWithDeployment(cl, d, map[types.Hash]collector.CellIterator{types.Hash{}: ckbclient.NewCKBOnlyIterator(iter)}, addr, false)
		require.NoError(t, err)
		for i := range cfg.CellDeps {
			b.AddCellDep(&cfg.CellDeps[i])
		}
		amount, err := molecule2.PackUint128(new(big.Int).Lsh(big.NewInt(1), 100))
		require.NoError(t, err)
		for _, keyHex := range f.Keys {
			kb, err := hex.DecodeString(keyHex)
			require.NoError(t, err)
			p, err := ckbaddress.NewDefaultParticipant(secp256k1.PrivKeyFromBytes(kb).PubKey())
			require.NoError(t, err)
			omni, _, err := ckbaddress.NewEthereumParticipantFromPublicKey(secp256k1.PrivKeyFromBytes(kb).PubKey(), d.OmniLockScript.CodeHash)
			require.NoError(t, err)
			for _, holder := range []*ckbaddress.Participant{p, omni} {
				b.AddOutput(&types.CellOutput{Lock: holder.PaymentScript, Type: &cfg.Script, Capacity: 400 * 100_000_000}, amount.AsSlice())
				b.AddOutput(&types.CellOutput{Lock: holder.PaymentScript, Capacity: 100_000 * 100_000_000}, nil)
			}
		}
		tx, err := b.Build()
		require.NoError(t, err)
		if mode > 0 {
			w, err := cfg.EncodeWitness([][]byte{nil})
			require.NoError(t, err)
			require.NoError(t, b.SetWitness(0, types.WitnessTypeOutputType, w))
			tx = b.BuildTransaction()
			tx.ScriptGroups = b.CopyValidScriptGroups()
		}
		signed, err := signer.SignTransaction(tx)
		require.NoError(t, err)
		hash, err := cl.SendTransaction(ctx, signed)
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			tx, e := cl.GetTransaction(ctx, *hash)
			return e == nil && tx != nil && tx.TxStatus.Status == types.TransactionStatusCommitted
		}, 45*time.Second, 200*time.Millisecond)
	}
}

// TestXUDTSeedRegression funds the existing suite's secp256k1 and OmniLock
// wallets and mints the sample sUDT used by those tests.
func TestXUDTSeedRegression(t *testing.T) {
	raw, err := os.ReadFile(os.Getenv("PERUN_XUDT_FIXTURE"))
	require.NoError(t, err)
	var f xudtFixture
	require.NoError(t, json.Unmarshal(raw, &f))
	require.Len(t, f.RegressionKeys, 3)
	cl, err := rpc.Dial(f.RPC)
	require.NoError(t, err)
	defer cl.Close()
	kb, err := hex.DecodeString(f.Keys[0])
	require.NoError(t, err)
	key := secp256k1.PrivKeyFromBytes(kb)
	p, err := ckbaddress.NewDefaultParticipant(key.PubKey())
	require.NoError(t, err)
	addr := p.ToCKBAddress(types.NetworkTest)
	addrString, err := addr.Encode()
	require.NoError(t, err)
	signer := backend.NewSignerInstance(addr, *key, types.NetworkTest)
	iter, err := collector.NewLiveCellIteratorFromAddress(cl, addrString)
	require.NoError(t, err)
	d := f.Deployment
	d.XUDTs = nil
	d.SUDTs = nil
	b, err := transaction.NewPerunTransactionBuilderWithDeployment(cl, d, map[types.Hash]collector.CellIterator{types.Hash{}: ckbclient.NewCKBOnlyIterator(iter)}, addr, false)
	require.NoError(t, err)
	for i := range f.RegressionToken.CellDeps {
		b.AddCellDep(&f.RegressionToken.CellDeps[i])
	}
	amount, err := molecule2.PackUint128(big.NewInt(1_000_000_000))
	require.NoError(t, err)
	for _, keyHex := range f.RegressionKeys {
		kb, err := hex.DecodeString(keyHex)
		require.NoError(t, err)
		key := secp256k1.PrivKeyFromBytes(kb)
		standard, err := ckbaddress.NewDefaultParticipant(key.PubKey())
		require.NoError(t, err)
		omni, _, err := ckbaddress.NewEthereumParticipantFromPublicKey(key.PubKey(), d.OmniLockScript.CodeHash)
		require.NoError(t, err)
		for _, p := range []*ckbaddress.Participant{standard, omni} {
			b.AddOutput(&types.CellOutput{Lock: p.PaymentScript, Capacity: 100_000 * 100_000_000}, nil)
			b.AddOutput(&types.CellOutput{Lock: p.PaymentScript, Type: &f.RegressionToken.Script, Capacity: 400 * 100_000_000}, amount.AsSlice())
		}
	}
	tx, err := b.Build()
	require.NoError(t, err)
	signed, err := signer.SignTransaction(tx)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	hash, err := cl.SendTransaction(ctx, signed)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		tx, e := cl.GetTransaction(ctx, *hash)
		return e == nil && tx != nil && tx.TxStatus.Status == types.TransactionStatusCommitted
	}, 45*time.Second, 200*time.Millisecond)
}
