//go:build devnet
// +build devnet

package ckblp

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
	"perun.network/perun-ckb-backend/backend"
	ckbtest "perun.network/perun-ckb-backend/channel/test"
	ckbaddress "perun.network/perun-ckb-backend/wallet/address"
)

func loadOrSkipLPDeployment(t *testing.T) (LPDeployment, bool) {
	lpDeployment, err := LoadLPDeploymentFromDevnet()
	if errors.Is(err, ErrFixtureUnavailable) {
		t.Skipf("LP deployment fixture unavailable: %v", err)
		return LPDeployment{}, false
	}
	require.NoError(t, err)
	return lpDeployment, true
}

func loadOrSkipLPCellSpec(t *testing.T) (LPCell, bool) {
	lpCell, err := LoadLPCellSpecFromDevnet()
	if errors.Is(err, ErrFixtureUnavailable) {
		t.Skipf("LP cell spec fixture unavailable: %v", err)
		return LPCell{}, false
	}
	require.NoError(t, err)
	return lpCell, true
}

func loadDevnetDeploymentRequire(t *testing.T) backend.Deployment {
	d, err := LoadDevnetDeployment()
	require.NoError(t, err)
	return d
}

func ensureLPDeploymentOnChainOrSkip(t *testing.T, rpcClient rpc.Client, lpDeployment LPDeployment) {
	if err := EnsureLPDeploymentOnChain(context.Background(), rpcClient, lpDeployment); err != nil {
		if errors.Is(err, ErrFixtureUnavailable) {
			t.Skipf("LP deployment not live on chain: %v", err)
			return
		}
		require.NoError(t, err)
	}
}

func mustParseHash32(t *testing.T, value string) [32]byte {
	out, err := parseHash32Fixed(value)
	require.NoError(t, err)
	return out
}

func TestDiscoverLPCellsDevnet(t *testing.T) {
	if os.Getenv("RUN_DEVNET_TESTS") == "" {
		t.Skip("devnet E2E test: requires proper transaction signing setup")
	}
	lpDeployment, ok := loadOrSkipLPDeployment(t)
	if !ok {
		return
	}

	rpcClient, err := rpc.Dial(ckbtest.DevnetRpcNodeURL)
	require.NoError(t, err)
	ensureLPDeploymentOnChainOrSkip(t, rpcClient, lpDeployment)

	deployment := loadDevnetDeploymentRequire(t)
	signer := newBobSigner(t, deployment.Network)
	transactor := backend.NewRPCTransactor(rpcClient, signer)
	adapter := NewAdapter(rpcClient, signer, transactor, deployment, lpDeployment)

	operatorHash := signer.Address().Script.Hash()
	cells, err := adapter.DiscoverLPCells(context.Background(), operatorHash)
	require.NoError(t, err)
	if len(cells) == 0 {
		lpCell, ok := loadOrSkipLPCellSpec(t)
		if !ok {
			return
		}
		// The on-disk spec default (200 CKB) is below the LP cell's
		// occupied-capacity minimum (~323 CKB). Bump to 800 CKB.
		lpCell.AvailableCKB = 80_000_000_000
		_, err := adapter.BuildLPDepositTx(context.Background(), lpCell)
		require.NoError(t, err)

		cells, err = adapter.DiscoverLPCells(context.Background(), operatorHash)
		require.NoError(t, err)
	}
	require.NotEmpty(t, cells)
}

func TestGetLPCellDevnet(t *testing.T) {
	if os.Getenv("RUN_DEVNET_TESTS") == "" {
		t.Skip("devnet E2E test: requires proper transaction signing setup")
	}
	lpCellID := os.Getenv("PERUN_LP_CELL_ID")
	poolID := os.Getenv("PERUN_LP_POOL_ID")
	lpDeployment, ok := loadOrSkipLPDeployment(t)
	if !ok {
		return
	}

	rpcClient, err := rpc.Dial(ckbtest.DevnetRpcNodeURL)
	require.NoError(t, err)
	ensureLPDeploymentOnChainOrSkip(t, rpcClient, lpDeployment)

	deployment := loadDevnetDeploymentRequire(t)
	signer := newBobSigner(t, deployment.Network)
	transactor := backend.NewRPCTransactor(rpcClient, signer)
	adapter := NewAdapter(rpcClient, signer, transactor, deployment, lpDeployment)
	if lpCellID == "" {
		operatorHash := signer.Address().Script.Hash()
		cells, err := adapter.DiscoverLPCells(context.Background(), operatorHash)
		require.NoError(t, err)
		if len(cells) == 0 {
			lpCell, ok := loadOrSkipLPCellSpec(t)
			if !ok {
				return
			}
			// The on-disk spec default (200 CKB) is below the LP cell's
			// occupied-capacity minimum (~323 CKB). Bump to 800 CKB.
			lpCell.AvailableCKB = 80_000_000_000
			lpCellID, err = adapter.BuildLPDepositTx(context.Background(), lpCell)
			require.NoError(t, err)
		} else {
			lpCellID = cells[0].OutPointHex
		}
	}
	info, err := adapter.GetLPCell(context.Background(), lpCellID)
	require.NoError(t, err)

	if poolID != "" {
		expected := mustParseHash32(t, poolID)
		require.Equal(t, expected, info.Cell.PoolID)
	}
}

func TestBobCreatesLPCellAndWithdrawDevnet(t *testing.T) {
	if os.Getenv("RUN_DEVNET_TESTS") == "" {
		t.Skip("devnet E2E test: requires proper transaction signing setup")
	}
	lpDeployment, ok := loadOrSkipLPDeployment(t)
	if !ok {
		return
	}

	lpCell, ok := loadOrSkipLPCellSpec(t)
	if !ok {
		return
	}

	rpcClient, err := rpc.Dial(ckbtest.DevnetRpcNodeURL)
	require.NoError(t, err)
	ensureLPDeploymentOnChainOrSkip(t, rpcClient, lpDeployment)

	deployment := loadDevnetDeploymentRequire(t)
	signer := newBobSigner(t, deployment.Network)
	transactor := backend.NewRPCTransactor(rpcClient, signer)

	ctx := context.Background()

	adapter := NewAdapter(rpcClient, signer, transactor, deployment, lpDeployment)
	// The on-disk spec default (200 CKB) is below the LP cell's
	// occupied-capacity minimum (~323 CKB). Bump to 800 CKB.
	lpCell.AvailableCKB = 80_000_000_000
	lpCellID, err := adapter.BuildLPDepositTx(ctx, lpCell)
	require.NoError(t, err)
	info, err := adapter.GetLPCell(ctx, lpCellID)
	require.NoError(t, err)

	withdrawAmount := uint64(100_000_000)
	if withdrawAmount > info.Cell.AvailableCKB {
		withdrawAmount = info.Cell.AvailableCKB
	}
	require.NotZero(t, withdrawAmount)

	_, err = adapter.BuildLPWithdrawTx(ctx, info.OutPointHex, withdrawAmount)
	require.NoError(t, err)
}

// TestLPFundAndSettleChannelDevnet is the end-to-end LP integration test.
// It runs both halves of the two-step LP usage flow against a live devnet:
//   - LP fund-extract creates a proxy channel cell labeled with a fresh,
//     synthetic channel_id (matches the contract harness pattern in
//     devnet/contract/tests/src/lp_tests.rs:154-159, which also uses synthetic
//     channel_ids — LP semantics are decoupled from real Perun channels by
//     design).
//   - LP settle-insert returns principal + fee to the LP cell. Because the
//     channel_id was never live on-chain, the settle-insert precondition
//     "channel must not appear in inputs/outputs" is trivially satisfied.
//
// The test is self-contained: it does not require PERUN_CHANNEL_ID and does
// not require an out-of-band channel to be open. The real Perun channel flow
// (Open/Fund/Close) is exercised by TestPaymentHappy in the client package.
func TestLPFundAndSettleChannelDevnet(t *testing.T) {
	if os.Getenv("RUN_DEVNET_TESTS") == "" {
		t.Skip("devnet E2E test: requires proper transaction signing setup")
	}
	lpDeployment, ok := loadOrSkipLPDeployment(t)
	if !ok {
		return
	}
	deployment := loadDevnetDeploymentRequire(t)

	rpcClient, err := rpc.Dial(ckbtest.DevnetRpcNodeURL)
	require.NoError(t, err)
	ensureLPDeploymentOnChainOrSkip(t, rpcClient, lpDeployment)

	signer := newBobSigner(t, deployment.Network)
	transactor := backend.NewRPCTransactor(rpcClient, signer)
	adapter := NewAdapter(rpcClient, signer, transactor, deployment, lpDeployment)
	ctx := context.Background()

	// Always deposit a fresh LP cell so the test starts from a known state.
	// The LP cell's CKB capacity must cover both its own occupied-capacity
	// minimum (~323 CKB: 8 capacity + 185 data + 65 lock + 65 type) AND the
	// proxy cell's minimum (~217 CKB: 8 capacity + 65 lock + ChannelStatus
	// data) plus headroom. Seed at 800 CKB.
	lpSpec, ok := loadOrSkipLPCellSpec(t)
	if !ok {
		return
	}
	lpSpec.AvailableCKB = 80_000_000_000
	lpCellID, err := adapter.BuildLPDepositTx(ctx, lpSpec)
	require.NoError(t, err)

	// Capture pre-LP state.
	preLP, err := adapter.GetLPCell(ctx, lpCellID)
	require.NoError(t, err)

	// Mint a fresh synthetic channel_id for this test run.
	var channelHash types.Hash
	_, err = rand.Read(channelHash[:])
	require.NoError(t, err)
	channelID := channelHash.String()

	// The proxy cell created by fund-extract has lock + ChannelStatus data,
	// requiring ~217 CKB of occupied capacity. Use 250 CKB so the proxy
	// comfortably satisfies the rule.
	amount := uint64(25_000_000_000)
	require.LessOrEqual(t, amount, preLP.Cell.AvailableCKB,
		"LP cell does not have enough available CKB for the test extract amount")

	// Step 1: LP fund-extract.
	fundHash, err := adapter.BuildFundChannelTx(ctx, channelID, lpCellID, amount, "")
	require.NoError(t, err)

	lpCellIDAfterFund := fmt.Sprintf("%s:0", fundHash.String())
	postFundLP, err := adapter.GetLPCell(ctx, lpCellIDAfterFund)
	require.NoError(t, err)
	require.Equal(t, preLP.Cell.AvailableCKB-amount, postFundLP.Cell.AvailableCKB, "AvailableCKB should decrease by extract amount")
	require.Equal(t, preLP.Cell.ReservedCKB+amount, postFundLP.Cell.ReservedCKB, "ReservedCKB should increase by extract amount")
	require.Equal(t, preLP.Cell.Nonce+1, postFundLP.Cell.Nonce, "Nonce should increment by 1")
	require.Equal(t, preLP.Capacity-amount, postFundLP.Capacity, "LP cell capacity should decrease by extract amount")

	// Step 2: LP settle-insert. The proxy cell from step 1 is operator-locked
	// and no longer carries the channel_id (it's a regular operator cell now,
	// having been consumed/spent), so the channel_id label is once again free
	// of any matching cell on-chain — satisfying the settle-insert precondition.
	principal := amount
	feeCKB := uint64(100_000_000)
	priceX64 := uint128.FromBig(big.NewInt(1))

	settleHash, err := adapter.BuildSettleChannelInsertTx(ctx, channelID, "", lpCellIDAfterFund, principal, feeCKB, priceX64)
	require.NoError(t, err)

	lpCellIDAfterSettle := fmt.Sprintf("%s:0", settleHash.String())
	postSettleLP, err := adapter.GetLPCell(ctx, lpCellIDAfterSettle)
	require.NoError(t, err)
	require.Equal(t, postFundLP.Cell.AvailableCKB+principal+feeCKB, postSettleLP.Cell.AvailableCKB, "AvailableCKB should increase by principal+fee")
	require.Equal(t, postFundLP.Cell.ReservedCKB-principal, postSettleLP.Cell.ReservedCKB, "ReservedCKB should decrease by principal")
	require.Equal(t, postFundLP.Cell.CumulativeFeesEarnedCKB+feeCKB, postSettleLP.Cell.CumulativeFeesEarnedCKB, "CumulativeFeesEarnedCKB should increase by fee")
	require.Equal(t, postFundLP.Cell.Nonce+1, postSettleLP.Cell.Nonce, "Nonce should increment by 1")
	require.Equal(t, postFundLP.Capacity+principal+feeCKB, postSettleLP.Capacity, "LP cell capacity should increase by principal+fee")

	// Step 3: Reclaim the proxy cell created by step 1. The proxy is at output
	// index 1 of the fund-extract tx (output 0 is the LP cell, output 2 is
	// operator change). It is operator-locked with no type script and carries
	// the channel_id in ChannelStatus data. Settle-insert cannot consume it
	// in-band because the LP typescript forbids channel_id from appearing in
	// inputs (see liquidity-pool-typescript/src/main.rs:578-582), so the
	// operator reclaims its capacity here in a separate, LP-script-free tx.
	proxyOutpointID := fmt.Sprintf("%s:1", fundHash.String())
	proxyLive, err := rpcClient.GetLiveCell(ctx, &types.OutPoint{TxHash: fundHash, Index: 1}, false)
	require.NoError(t, err)
	require.NotNil(t, proxyLive)
	require.NotNil(t, proxyLive.Cell)
	proxyCapacity := proxyLive.Cell.Output.Capacity
	require.Equal(t, amount, proxyCapacity, "proxy cell capacity should equal extract amount")

	reclaimHash, err := adapter.ReclaimProxyCell(ctx, proxyOutpointID)
	require.NoError(t, err)

	reclaimedLive, err := rpcClient.GetLiveCell(ctx, &types.OutPoint{TxHash: reclaimHash, Index: 0}, false)
	require.NoError(t, err)
	require.NotNil(t, reclaimedLive)
	require.NotNil(t, reclaimedLive.Cell)
	require.Nil(t, reclaimedLive.Cell.Output.Type, "reclaimed cell must have no type script")
	require.Equal(t, signer.Address().Script.Hash(), reclaimedLive.Cell.Output.Lock.Hash(), "reclaimed cell must be operator-locked")
	require.Equal(t, proxyCapacity-uint64(10_000), reclaimedLive.Cell.Output.Capacity, "reclaimed capacity should equal proxy capacity minus fee_shannon (0.0001 CKB)")

	// Verify the proxy is no longer live.
	proxyAfterReclaim, err := rpcClient.GetLiveCell(ctx, &types.OutPoint{TxHash: fundHash, Index: 1}, false)
	require.NoError(t, err)
	if proxyAfterReclaim != nil && proxyAfterReclaim.Cell != nil {
		require.FailNow(t, "proxy cell still live after reclaim")
	}
}

func newBobSigner(t *testing.T, network types.Network) backend.Signer {
	keyBob, err := ckbtest.GetKey(filepath.Join("..", "..", "devnet", "accounts", "bob.pk"))
	require.NoError(t, err)

	participant, err := ckbaddress.NewDefaultParticipant(keyBob.PubKey())
	require.NoError(t, err)

	addr := participant.ToCKBAddress(network)
	return backend.NewSignerInstance(addr, *keyBob, network)
}
