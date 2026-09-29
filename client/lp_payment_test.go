//go:build !testnet && devnet
// +build !testnet,devnet

package client_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/ethereum/go-ethereum/crypto"
	ckbaddress "github.com/nervosnetwork/ckb-sdk-go/v2/address"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	ckbsigner "github.com/nervosnetwork/ckb-sdk-go/v2/transaction/signer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/client"
	"perun.network/go-perun/log"
	"perun.network/go-perun/wallet"
	"perun.network/go-perun/wire"
	pkgtest "polycry.pt/poly-go/test"

	clienttest "perun.network/go-perun/client/test"

	ckblp "perun.network/perun-ckb-backend/adapter/ckb_lp"
	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/channel/asset"
	btest "perun.network/perun-ckb-backend/channel/test"
	ctest "perun.network/perun-ckb-backend/client/test"
)

// TestLPFundedPaymentChannelDevnet is an end-to-end devnet showcase: Ingrid
// provides liquidity, Alice operates that liquidity to back a Perun payment
// channel between Alice and Bob, and after the channel closes Alice settles
// principal + fee back to Ingrid's LP cell and reclaims her operator-locked
// proxy cell.
//
// The LP's channel_id label is synthetic (matches the contract harness pattern
// at devnet/contract/tests/src/lp_tests.rs:154-159). In production the
// operator would label LP ops with the real channel.ID, but tying the two
// together would require driving the Perun channel manually instead of using
// clienttest.Execute; that's out of scope for this first-cut showcase. The
// real Perun channel itself is fully exercised: Open, Fund, Pay, Close.
func TestLPFundedPaymentChannelDevnet(t *testing.T) {

	log.Info("Starting LP-funded payment channel test")
	rng := pkgtest.Prng(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// --- LP deployment fixture --------------------------------------------
	lpDeployment, err := ckblp.LoadLPDeploymentFromDevnet()
	if errors.Is(err, ckblp.ErrFixtureUnavailable) {
		t.Skipf("LP deployment fixture unavailable: %v", err)
	}
	require.NoError(t, err)

	// --- 3-party Perun setup (Alice, Bob, Ingrid) -------------------------
	const (
		A = 0
		B = 1
		I = 2
	)
	names := []string{"Alice", "Bob", "Ingrid"}
	// omni=true mirrors TestPaymentHappy and the production deployment: every
	// participant uses omni-lock + eth-auth, so signing requires an EVMSigner
	// with the OmnilockSigner registered on its TxSigner. NewSignerInstance
	// (default sighash) would skip the input, yielding an Inputs[0].Lock
	// ValidationFailure on chain.
	s := btest.NewVirtualChannelSetup(t, rng, true)
	roleSetup := ctest.MakeRoleSetupsCross(rng, s, names)

	// --- RPC client + eventual readiness of LP deployment on chain --------
	rpcClient, err := rpc.Dial(btest.DevnetRpcNodeURL)
	require.NoError(t, err)

	// Devnet bring-up is slow and the .devnet-ready sentinel can be stale.
	// Don't fail on the first RPC miss; give the LP scripts up to 30s to
	// appear at the expected outpoints.
	require.Eventually(t, func() bool {
		return ckblp.EnsureLPDeploymentOnChain(ctx, rpcClient, lpDeployment) == nil
	}, 30*time.Second, 2*time.Second, "LP deployment never became live on devnet")

	// --- LP adapters: Ingrid signs the deposit; Alice signs extract/settle/reclaim
	network := s.Deployment.Network
	ingridAddr := s.Participants[I].ToCKBAddress(network)
	aliceAddr := s.Participants[A].ToCKBAddress(network)

	ingridSigner := newOmniSigner(t, ingridAddr, s.AccKeys[I], network, s.Deployment.OmniLockScript.CodeHash)
	aliceSigner := newOmniSigner(t, aliceAddr, s.AccKeys[A], network, s.Deployment.OmniLockScript.CodeHash)

	ingridTransactor := backend.NewRPCTransactor(rpcClient, ingridSigner)
	aliceTransactor := backend.NewRPCTransactor(rpcClient, aliceSigner)

	ingridAdapter := ckblp.NewAdapter(rpcClient, ingridSigner, ingridTransactor, s.Deployment, lpDeployment)
	aliceAdapter := ckblp.NewAdapter(rpcClient, aliceSigner, aliceTransactor, s.Deployment, lpDeployment)

	aliceLockHash := aliceSigner.Address().Script.Hash()
	ingridLockHash := ingridSigner.Address().Script.Hash()

	// ===================================================================
	// Step 1 — Ingrid deposits an LP cell with Alice as operator
	// ===================================================================
	lpSpec, err := ckblp.LoadLPCellSpecFromDevnet()
	if errors.Is(err, ckblp.ErrFixtureUnavailable) {
		t.Skipf("LP cell spec fixture unavailable: %v", err)
	}
	require.NoError(t, err)

	// Defaults explicit to insulate against fixture drift. The 800 CKB seed
	// is above the LP cell's ~323 CKB occupied-capacity minimum and leaves
	// headroom to extract 250 CKB.
	lpSpec.AvailableCKB = 80_000_000_000
	lpSpec.Policy.PolicyFlags = 0
	lpSpec.Policy.SafePriceMinX64 = uint128.Uint128{}
	lpSpec.Policy.SafePriceMaxX64 = uint128.Max()

	depositTxID, err := ingridAdapter.BuildLPDepositTxWithOperator(ctx, lpSpec, aliceLockHash)
	require.NoError(t, err, "Ingrid LP deposit failed")
	depositTxHash := outpointTxHash(t, depositTxID)

	lpCellID, err := resolveLPCellOutpoint(ctx, rpcClient, depositTxHash, lpDeployment.TypeScriptCodeHash)
	require.NoError(t, err)

	postDeposit, err := aliceAdapter.GetLPCell(ctx, lpCellID)
	require.NoError(t, err)
	require.Equal(t, aliceLockHash, types.Hash(postDeposit.Cell.OperatorLockHash), "operator hash must be Alice")
	require.Equal(t, ingridLockHash, types.Hash(postDeposit.Cell.OwnerLockHash), "owner hash must be Ingrid")

	// priceX64 = 1 is only valid when neither SafePrice nor RequirePrice is
	// set in policy_flags. Asserting here surfaces a misconfigured fixture
	// next to the deposit, not deep inside the settle step.
	require.Zero(t, postDeposit.Cell.Policy.PolicyFlags&ckblp.PolicyFlagSafePrice,
		"LP policy must not require safe-price for this test (priceX64=1)")
	require.Zero(t, postDeposit.Cell.Policy.PolicyFlags&ckblp.PolicyFlagRequirePrice,
		"LP policy must not require price for this test (priceX64=1)")

	log.Infof("Step 1 done: Ingrid deposited LP cell at %s (operator=Alice)", lpCellID)

	// ===================================================================
	// Step 2 — Alice (operator) extracts liquidity from Ingrid's LP cell
	// ===================================================================
	var channelHash types.Hash
	_, err = rand.Read(channelHash[:])
	require.NoError(t, err)
	channelID := channelHash.String()

	const extractCKB = uint64(25_000_000_000) // 250 CKB

	fundHash, err := aliceAdapter.BuildFundChannelTx(ctx, channelID, lpCellID, extractCKB, "")
	require.NoError(t, err, "Alice fund-extract failed")

	lpCellIDAfterFund, err := resolveLPCellOutpoint(ctx, rpcClient, fundHash, lpDeployment.TypeScriptCodeHash)
	require.NoError(t, err)

	postFund, err := aliceAdapter.GetLPCell(ctx, lpCellIDAfterFund)
	require.NoError(t, err)
	require.Equal(t, postDeposit.Cell.AvailableCKB-extractCKB, postFund.Cell.AvailableCKB,
		"AvailableCKB should decrease by extract amount")
	require.Equal(t, postDeposit.Cell.ReservedCKB+extractCKB, postFund.Cell.ReservedCKB,
		"ReservedCKB should increase by extract amount")
	require.Equal(t, postDeposit.Cell.Nonce+1, postFund.Cell.Nonce, "Nonce should increment")
	require.Equal(t, postDeposit.Capacity-extractCKB, postFund.Capacity,
		"LP cell capacity should decrease by extract amount")

	proxyOutpointID, proxyCapacity, err := resolveProxyOutpoint(ctx, rpcClient, fundHash, channelHash)
	require.NoError(t, err)
	require.Equal(t, extractCKB, proxyCapacity, "proxy capacity should equal extract amount")

	log.Infof("Step 2 done: Alice extracted %d shannons; proxy at %s", extractCKB, proxyOutpointID)

	// ===================================================================
	// Step 3 — Alice (operator) reclaims the proxy cell
	// ===================================================================
	// The proxy is a transient on-chain marker: the LP typescript only checks
	// that channel_id appears in the fund-extract tx's outputs (the proxy is
	// that output). After the extract commits, the proxy is just operator-
	// locked CKB with non-empty data. Critically it is locked under Alice's
	// lock script, so the Perun client's indexer queries WILL pick it up as
	// one of Alice's input cells when funding the channel — and consume it.
	// Reclaim here, before Perun runs, to:
	//   (a) recover the LP-extracted CKB into a plain operator cell that
	//       Alice can freely use to fund the channel, and
	//   (b) avoid the indexer/proxy contention.
	// Narrative: extract + reclaim together model "Alice borrows CKB from
	// Ingrid's LP cell"; the proxy is just the on-chain receipt.
	reclaimHash, err := aliceAdapter.ReclaimProxyCell(ctx, proxyOutpointID)
	require.NoError(t, err, "Alice reclaim failed")

	// Reclaim is a single-input-single-output sweep by construction.
	reclaimTx, err := rpcClient.GetTransaction(ctx, reclaimHash)
	require.NoError(t, err)
	require.NotNil(t, reclaimTx)
	require.NotNil(t, reclaimTx.Transaction)
	require.Len(t, reclaimTx.Transaction.Outputs, 1, "reclaim tx must have exactly one output")

	reclaimed, err := rpcClient.GetLiveCell(ctx, &types.OutPoint{TxHash: reclaimHash, Index: 0}, false)
	require.NoError(t, err)
	require.NotNil(t, reclaimed)
	require.NotNil(t, reclaimed.Cell)
	require.Nil(t, reclaimed.Cell.Output.Type, "reclaimed cell must have no type script")
	require.Equal(t, aliceLockHash, reclaimed.Cell.Output.Lock.Hash(),
		"reclaimed cell must be operator-locked (Alice)")
	require.Equal(t, proxyCapacity-uint64(10_000), reclaimed.Cell.Output.Capacity,
		"reclaimed capacity should equal proxy capacity minus fee_shannon (0.0001 CKB)")

	proxyAfter, err := rpcClient.GetLiveCell(ctx, outpointFromID(t, proxyOutpointID), false)
	require.NoError(t, err)
	if proxyAfter != nil && proxyAfter.Cell != nil {
		require.FailNow(t, "proxy cell still live after reclaim")
	}

	log.Info("Step 3 done: proxy reclaimed; Alice has the LP-extracted CKB free for channel funding")

	// ===================================================================
	// Step 4 — Alice + Bob run a real Perun payment channel (Open → Pay → Close)
	// Mirrors TestPaymentHappy but with panic-safe goroutine error propagation.
	// ===================================================================
	var roles [2]clienttest.Executer
	roles[A] = clienttest.NewAlice(t, roleSetup[A])
	roles[B] = clienttest.NewBob(t, roleSetup[B])
	stages := roles[A].EnableStages()
	roles[B].SetStages(stages)

	execConfig := &clienttest.AliceBobExecConfig{
		BaseExecConfig: clienttest.MakeBaseExecConfig(
			[2]map[wallet.BackendID]wire.Address{
				{3: roleSetup[A].Identity[3].Address()},
				{3: roleSetup[B].Identity[3].Address()},
			},
			[]channel.Asset{s.CkbAsset},
			[]wallet.BackendID{3},
			[][2]*big.Int{{
				asset.CKByteToShannon(big.NewFloat(100)),
				asset.CKByteToShannon(big.NewFloat(100)),
			}},
			client.WithoutApp(),
		),
		NumPayments: [2]int{2, 2},
		TxAmounts: [2]*big.Int{
			asset.CKByteToShannon(big.NewFloat(5)),
			asset.CKByteToShannon(big.NewFloat(5)),
		},
	}

	// payment_test.go's bare wg.Wait() pattern swallows panics in spawned
	// goroutines — a failing Execute would hang or pass silently. Recover
	// and forward panics so t.Fatal actually fires on the main goroutine.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errs <- fmt.Errorf("%s panicked: %v\n%s", names[i], r, debug.Stack())
				}
			}()
			log.Infof("Starting %s.Execute", names[i])
			roles[i].Execute(execConfig)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	log.Info("Step 4 done: Alice + Bob completed the Perun channel flow")

	// ===================================================================
	// Step 5 — Alice (operator) settles principal + fee back to Ingrid's LP
	// ===================================================================
	const (
		principal = extractCKB
		feeCKB    = uint64(100_000_000) // 1 CKB
	)
	priceX64 := uint128.FromBig(big.NewInt(1))

	settleHash, err := aliceAdapter.BuildSettleChannelInsertTx(ctx, channelID, "", lpCellIDAfterFund, principal, feeCKB, priceX64)
	require.NoError(t, err, "Alice settle-insert failed")

	lpCellIDAfterSettle, err := resolveLPCellOutpoint(ctx, rpcClient, settleHash, lpDeployment.TypeScriptCodeHash)
	require.NoError(t, err)

	postSettle, err := aliceAdapter.GetLPCell(ctx, lpCellIDAfterSettle)
	require.NoError(t, err)
	require.Equal(t, postFund.Cell.AvailableCKB+principal+feeCKB, postSettle.Cell.AvailableCKB,
		"AvailableCKB should increase by principal+fee")
	require.Equal(t, postFund.Cell.ReservedCKB-principal, postSettle.Cell.ReservedCKB,
		"ReservedCKB should decrease by principal")
	require.Equal(t, postFund.Cell.CumulativeFeesEarnedCKB+feeCKB, postSettle.Cell.CumulativeFeesEarnedCKB,
		"CumulativeFeesEarnedCKB should increase by fee")
	require.Equal(t, postFund.Cell.Nonce+1, postSettle.Cell.Nonce, "Nonce should increment")
	require.Equal(t, postFund.Capacity+principal+feeCKB, postSettle.Capacity,
		"LP cell capacity should increase by principal+fee")

	log.Info("Step 5 done: principal+fee returned to LP cell; LP-funded payment channel showcase complete")
}

// resolveLPCellOutpoint scans the tx's outputs for the first one carrying the
// LP typescript code hash and returns its outpoint as "txhash:index". This
// avoids hard-coding output indices that would break under reordering.
func resolveLPCellOutpoint(ctx context.Context, rpcClient rpc.Client, txHash types.Hash, lpTypeCodeHash types.Hash) (string, error) {
	tx, err := rpcClient.GetTransaction(ctx, txHash)
	if err != nil {
		return "", fmt.Errorf("get tx %s: %w", txHash.String(), err)
	}
	if tx == nil || tx.Transaction == nil {
		return "", fmt.Errorf("tx %s not found", txHash.String())
	}
	for i, out := range tx.Transaction.Outputs {
		if out == nil || out.Type == nil {
			continue
		}
		if out.Type.CodeHash == lpTypeCodeHash {
			return fmt.Sprintf("%s:%d", txHash.String(), i), nil
		}
	}
	return "", fmt.Errorf("no LP cell output found in tx %s (matching code hash %s)", txHash.String(), lpTypeCodeHash.String())
}

// resolveProxyOutpoint scans the tx's outputs for the first one whose data
// contains the channel hash bytes AND has no type script (proxy cells have
// a lock + ChannelStatus data but no type). The channel_id sits inside
// molecule framing in the ChannelStatus payload, not at byte offset 0, so
// substring-search is the robust check. Returns its outpoint as
// "txhash:index" along with its capacity.
func resolveProxyOutpoint(ctx context.Context, rpcClient rpc.Client, txHash types.Hash, channelHash types.Hash) (string, uint64, error) {
	tx, err := rpcClient.GetTransaction(ctx, txHash)
	if err != nil {
		return "", 0, fmt.Errorf("get tx %s: %w", txHash.String(), err)
	}
	if tx == nil || tx.Transaction == nil {
		return "", 0, fmt.Errorf("tx %s not found", txHash.String())
	}
	for i, data := range tx.Transaction.OutputsData {
		if !bytes.Contains(data, channelHash[:]) {
			continue
		}
		out := tx.Transaction.Outputs[i]
		if out == nil {
			continue
		}
		if out.Type != nil {
			// Skip the LP cell, whose data may incidentally contain bytes
			// matching the synthetic channel hash but always carries a type
			// script.
			continue
		}
		return fmt.Sprintf("%s:%d", txHash.String(), i), out.Capacity, nil
	}
	return "", 0, fmt.Errorf("no proxy cell with channel_id %s found in tx %s", channelHash.String(), txHash.String())
}

// outpointTxHash parses "txhash:index" and returns the tx hash.
func outpointTxHash(t *testing.T, id string) types.Hash {
	t.Helper()
	op := outpointFromID(t, id)
	return op.TxHash
}

// newOmniSigner constructs an EVMSigner with the OmnilockSigner registered on
// its TxSigner. Mirrors the pattern in channel/test/setup.go::createFundersAndAdjudicators
// (lines 455-459): without the RegisterLockSigner call the omni-lock script
// group is recognized but skipped, leaving the input unsigned.
//
// The 20-byte authContent (eth-style address) is derived from the pubkey via
// keccak256, matching wallet/address/address.go::NewEthereumParticipantFromPublicKey:64-68.
func newOmniSigner(t *testing.T, addr ckbaddress.Address, key secp256k1.PrivateKey, network types.Network, omniCodeHash types.Hash) *backend.EVMSigner {
	t.Helper()
	pubBytes := crypto.FromECDSAPub(key.PubKey().ToECDSA())[1:]
	var authContent [20]byte
	copy(authContent[:], crypto.Keccak256(pubBytes)[12:])

	signer := backend.NewEVMSignerInstance(addr, key, network, authContent)
	txSigner := signer.Signer()
	txSigner.RegisterLockSigner(omniCodeHash, &ckbsigner.OmnilockSigner{})
	return signer
}

func outpointFromID(t *testing.T, id string) *types.OutPoint {
	t.Helper()
	var hashStr string
	var idx uint32
	parts := bytes.SplitN([]byte(id), []byte(":"), 2)
	require.Len(t, parts, 2, "outpoint id must be txhash:index, got %q", id)
	hashStr = string(parts[0])
	_, err := fmt.Sscanf(string(parts[1]), "%d", &idx)
	require.NoError(t, err, "parsing index from %q", id)
	return &types.OutPoint{TxHash: types.HexToHash(hashStr), Index: idx}
}
