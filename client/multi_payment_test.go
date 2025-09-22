package client_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/sirupsen/logrus"
	"math/big"
	"math/rand"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/wallet/address"
	"testing"
	"time"

	"perun.network/go-perun/client"
	"perun.network/go-perun/log"
	"perun.network/go-perun/wire"
	"perun.network/perun-ckb-backend/channel/asset"
	btest "perun.network/perun-ckb-backend/channel/test"
	ctest "perun.network/perun-ckb-backend/client/test"
	"perun.network/perun-ckb-backend/transaction"
	"polycry.pt/poly-go/sync"
	pkgtest "polycry.pt/poly-go/test"

	clienttest "perun.network/go-perun/client/test"
)

// TestPaymentHappy tests the happy path of the payment channel.
// It creates a payment channel between Alice and Bob, and then performs a series of payments.
// The test checks if the final balances are as expected and if the channel state is updated correctly.
// The test also checks if the payment channel is closed correctly.
func TestMultiPaymentHappy(t *testing.T) {
	log.Info("Starting happy test")
	rng := pkgtest.Prng(t)

	const A, B = 0, 1 // Indices of Alice and Bob
	var (
		name = [2]string{"Alice", "Bob"}
		role [2]clienttest.Executer
	)

	s := btest.NewSetup(t, rng, false)
	setup := ctest.MakeRoleSetups(rng, s, name[:])

	role[A] = clienttest.NewAlice(t, setup[A])
	role[B] = clienttest.NewBob(t, setup[B])
	balAlice := setup[A].BalanceReader.Balance(s.Asset)
	balBob := setup[B].BalanceReader.Balance(s.Asset)

	logrus.Printf("Initial Balances - Alice: %s, Bob: %s", balAlice.String(), balBob.String())
	balAlice = setup[A].BalanceReader.Balance(s.SudtAsset)
	balBob = setup[B].BalanceReader.Balance(s.SudtAsset)
	logrus.Printf("Initial SUDT Balances - Alice: %s, Bob: %s", balAlice.String(), balBob.String())
	// enable stages synchronization
	stages := role[A].EnableStages()
	role[B].SetStages(stages)

	execConfig := &clienttest.AliceBobExecConfig{
		BaseExecConfig: clienttest.MakeBaseExecConfig(
			[2]map[wallet.BackendID]wire.Address{{3: setup[A].Identity[3].Address()}, {3: setup[B].Identity[3].Address()}},
			[]channel.Asset{s.Asset, s.SudtAsset},
			[]wallet.BackendID{3, 3},
			[][2]*big.Int{
				{asset.CKByteToShannon(big.NewFloat(80)), asset.CKByteToShannon(big.NewFloat(100))},
				{new(big.Int).SetUint64(uint64(2)), new(big.Int).SetUint64(uint64(1))},
			},
			client.WithoutApp(),
		),
		NumPayments: [2]int{2, 2},
		TxAmounts:   [2]*big.Int{asset.CKByteToShannon(big.NewFloat(2)), asset.CKByteToShannon(big.NewFloat(2))},
	}

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			defer wg.Done()
			log.Infof("Starting %s.Execute", name[i])
			role[i].Execute(execConfig)
		}(i)
	}

	wg.Wait()
	balAlice = setup[A].BalanceReader.Balance(s.Asset)
	balBob = setup[B].BalanceReader.Balance(s.Asset)
	logrus.Printf("Initial Balances - Alice: %s, Bob: %s", balAlice.String(), balBob.String())
	balAlice = setup[A].BalanceReader.Balance(s.SudtAsset)
	balBob = setup[B].BalanceReader.Balance(s.SudtAsset)
	logrus.Printf("Initial SUDT Balances - Alice: %s, Bob: %s", balAlice.String(), balBob.String())

	logrus.Info("Happy test done")
}

// TestPaymentDispute tests the payment dispute scenario.
// It creates a payment channel between Alice and Bob, and then disputes the
// channel state. The test checks if the dispute is resolved correctly and the final balances
// are as expected.
func TestMultiPaymentDispute(t *testing.T) {
	log.Info("Starting payment dispute test")
	rng := pkgtest.Prng(t)

	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	setup := makeMultiPaymentChannelSetup(t, rng)
	clienttest.TestPaymentChannelDispute(ctx, t, setup)
	log.Info("Payment dispute test done")
}

func makeMultiPaymentChannelSetup(t *testing.T, rng *rand.Rand) clienttest.PaymentChannelSetup {
	t.Helper()
	name := [2]string{"Alice", "Bob"}
	setup := btest.NewVirtualChannelSetup(t, rng)

	roleSetup := ctest.MakeRoleSetups(rng, setup, name[:])

	return clienttest.PaymentChannelSetup{
		Clients:           [2]clienttest.RoleSetup(roleSetup),
		ChallengeDuration: roleSetup[0].ChallengeDuration,
		Asset:             setup.Asset,
		Balances: clienttest.PaymentChannelBalances{
			InitBalsAliceBob: []*big.Int{asset.CKByteToShannon(big.NewFloat(100)), asset.CKByteToShannon(big.NewFloat(100))},
			BalsUpdated:      []*big.Int{asset.CKByteToShannon(big.NewFloat(70)), asset.CKByteToShannon(big.NewFloat(130))},
			FinalBals:        []*big.Int{asset.CKByteToShannon(big.NewFloat(70)), asset.CKByteToShannon(big.NewFloat(130))},
		},
		BalanceDelta:       big.NewInt(int64(3 * transaction.DefaultFeeShannon)), // Max Fee: (Open + Dispute + Close) * 1 CKB
		Rng:                rng,
		WaitWatcherTimeout: 1 * time.Second,
		IsUTXO:             true,
	}
}

func getBalance(asset asset.Asset, participant address.Participant) (*big.Int, error) {
	ctx := context.Background()
	fmt.Println("Participant: ", participant.UnlockScript.CodeHash, "hashtype: ", participant.UnlockScript.HashType, "Args: ", participant.UnlockScript.Args)
	indexerClient, _ := indexer.Dial("http://127.0.0.1:8114")
	lockScript := &types.Script{
		CodeHash: participant.UnlockScript.CodeHash, // secp256k1_blake160_sighash_all
		HashType: participant.UnlockScript.HashType,
		Args:     participant.UnlockScript.Args, // from public key
	}

	// Query CKB balance
	capacityResp, err := indexerClient.GetCellsCapacity(ctx, &indexer.SearchKey{
		Script:     lockScript,
		ScriptType: types.ScriptTypeLock,
	})
	if err != nil {
		log.Errorf("failed to get CKB balance: %v", err)
		return big.NewInt(0), err
	}
	ckbAmount := capacityResp.Capacity
	log.Println("CKB balance", ckbAmount)

	// If no SUDT requested
	if asset.IsCKBytes {
		log.Println("no SUDT requested, CKB: ", ckbAmount)
		return big.NewInt(int64(ckbAmount)), nil
	}

	// Query SUDT balance by filtering cells with type script
	var balance = big.NewInt(0)
	cursor := ""
	outputrange := [2]uint64{1, 129}
	fmt.Println("getting sudt balance", asset.SUDT.TypeScript.CodeHash.String(), asset.SUDT.TypeScript.HashType, hex.EncodeToString(asset.SUDT.TypeScript.Args), hex.EncodeToString(lockScript.Args))
	for {
		resp, err := indexerClient.GetCells(ctx, &indexer.SearchKey{
			Script: &types.Script{
				CodeHash: asset.SUDT.TypeScript.CodeHash,
				HashType: asset.SUDT.TypeScript.HashType,
				Args:     asset.SUDT.TypeScript.Args,
			},
			WithData:   true,
			ScriptType: types.ScriptTypeType,
			Filter: &indexer.Filter{
				Script:             lockScript,
				OutputDataLenRange: &outputrange,
			},
		}, indexer.SearchOrderAsc, 500, cursor)
		if err != nil {
			log.Println("indexer query failed:", err)
			return balance, err
		}
		log.Println("Found: ", len(resp.Objects), "cells with cursor:", resp.LastCursor)
		for i, cell := range resp.Objects {
			if len(cell.OutputData) < 16 {
				log.Printf("Skipping cell %d due to short or missing data: %v", i, cell.OutputData)
				continue
			}
			amount := new(big.Int).SetBytes(reverseBytes(cell.OutputData[:16]))
			log.Printf("Cell %d amount: %s", i, amount.String())
			balance.Add(balance, amount)
		}

		if resp.LastCursor == cursor || len(resp.Objects) == 0 {
			break
		}
		cursor = resp.LastCursor
	}
	log.Println("Final SUDT balance:", balance)
	return balance, nil
}

func reverseBytes(b []byte) []byte {
	out := make([]byte, len(b))
	for i := range b {
		out[i] = b[len(b)-1-i]
	}
	return out
}
