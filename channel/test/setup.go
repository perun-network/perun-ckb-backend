// Copyright 2020 - See NOTICE file for copyright holders.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/stretchr/testify/require"

	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"

	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/channel/adjudicator"
	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/channel/funder"
	"perun.network/perun-ckb-backend/client"
	"perun.network/perun-ckb-backend/wallet"
	"perun.network/perun-ckb-backend/wallet/address"
	ckbwallettest "perun.network/perun-ckb-backend/wallet/test"
)

const (
	RpcNodeURL      = "http://localhost:8114"
	network         = types.NetworkTest
	devNetDir       = "../devnet"
	sudtMaxCapacity = 200_00_000_000
)

type Setup struct {
	t            *testing.T
	Rng          *rand.Rand
	Deployment   backend.Deployment
	SUDTInfo     SUDTInfo
	Asset        *asset.Asset
	SudtAsset    *asset.Asset
	Participants []*address.Participant

	WalletAccs       []*wallet.Account
	AccKeys          []secp256k1.PrivateKey
	EphemeralWallets []*ckbwallettest.TestEphemeralWallet

	Funders []channel.Funder
	Adjs    []channel.Adjudicator
}

func NewSetup(t *testing.T, rng *rand.Rand) *Setup {
	setup := &Setup{}
	setup.t = t
	setup.Rng = rng

	sudtOwnerLockArg, err := parseSUDTOwnerLockArg(devNetDir + "/accounts/sudt-owner-lock-hash.txt")
	require.NoError(t, err, "error getting SUDT owner lock arg")

	d, sudtInfo, err := GetDeployment(devNetDir+"/contracts/migrations/dev/", devNetDir+"/system_scripts", sudtOwnerLockArg)
	require.NoError(t, err, "error getting deployment")
	setup.Deployment = d
	setup.SUDTInfo = sudtInfo

	setup.Asset = asset.NewCKBytesAsset()
	setup.SudtAsset = &asset.Asset{
		IsCKBytes: false,
		SUDT:      asset.NewSUDT(*sudtInfo.Script, uint64(sudtMaxCapacity)),
	}
	wallets := make([]*ckbwallettest.TestEphemeralWallet, 2)
	setup.EphemeralWallets = wallets

	parts := make([]*address.Participant, 2)
	keyAlice, err := GetKey(devNetDir + "/accounts/alice.pk")
	require.NoError(t, err, "error getting alice's private key")

	keyBob, err := GetKey(devNetDir + "/accounts/bob.pk")
	require.NoError(t, err, "error getting bob's private key")

	parts[0], _ = address.NewDefaultParticipant(keyAlice.PubKey())
	parts[1], _ = address.NewDefaultParticipant(keyBob.PubKey())
	setup.Participants = parts
	aliceAccount := wallet.NewAccountFromPrivateKey(keyAlice)
	bobAccount := wallet.NewAccountFromPrivateKey(keyBob)

	wallets[0] = ckbwallettest.NewTestEphemeralWallet(aliceAccount)
	err = wallets[0].AddAccount(aliceAccount)
	require.NoError(t, err, "error adding alice's account")

	wallets[1] = ckbwallettest.NewTestEphemeralWallet(bobAccount)
	err = wallets[1].AddAccount(bobAccount)
	require.NoError(t, err, "error adding bob's account")

	setup.WalletAccs = []*wallet.Account{aliceAccount, bobAccount}
	setup.AccKeys = []secp256k1.PrivateKey{*keyAlice, *keyBob}

	funders, adjs := createFundersAndAdjudicators(t, setup.WalletAccs, setup.AccKeys, d, RpcNodeURL)
	setup.Funders = funders
	setup.Adjs = adjs

	return setup
}

func NewVirtualChannelSetup(t *testing.T, rng *rand.Rand) *Setup {
	setup := &Setup{}
	setup.t = t
	setup.Rng = rng

	sudtOwnerLockArg, err := parseSUDTOwnerLockArg(devNetDir + "/accounts/sudt-owner-lock-hash.txt")
	require.NoError(t, err, "error getting SUDT owner lock arg")

	d, sudtInfo, err := GetDeployment(devNetDir+"/contracts/migrations/dev/", devNetDir+"/system_scripts", sudtOwnerLockArg)
	require.NoError(t, err, "error getting deployment")
	setup.Deployment = d
	setup.SUDTInfo = sudtInfo

	setup.Asset = asset.NewCKBytesAsset()

	wallets := make([]*ckbwallettest.TestEphemeralWallet, 3)
	setup.EphemeralWallets = wallets

	keyAlice, err := GetKey(devNetDir + "/accounts/alice.pk")
	require.NoError(t, err, "error getting alice's private key")

	keyBob, err := GetKey(devNetDir + "/accounts/bob.pk")
	require.NoError(t, err, "error getting bob's private key")

	keyIngrid, err := GetKey(devNetDir + "/accounts/ingrid.pk")
	require.NoError(t, err, "error getting ingrid's private key")

	aliceAccount := wallet.NewAccountFromPrivateKey(keyAlice)
	bobAccount := wallet.NewAccountFromPrivateKey(keyBob)
	ingridAccount := wallet.NewAccountFromPrivateKey(keyIngrid)

	wallets[0] = ckbwallettest.NewTestEphemeralWallet(aliceAccount)
	err = wallets[0].AddAccount(aliceAccount)
	require.NoError(t, err, "error adding alice's account")

	wallets[1] = ckbwallettest.NewTestEphemeralWallet(bobAccount)
	err = wallets[1].AddAccount(bobAccount)
	require.NoError(t, err, "error adding bob's account")

	wallets[2] = ckbwallettest.NewTestEphemeralWallet(ingridAccount)
	err = wallets[2].AddAccount(ingridAccount)
	require.NoError(t, err, "error adding ingrid's account")

	setup.WalletAccs = []*wallet.Account{aliceAccount, bobAccount, ingridAccount}
	setup.AccKeys = []secp256k1.PrivateKey{*keyAlice, *keyBob, *keyIngrid}

	funders, adjs := createFundersAndAdjudicators(t, setup.WalletAccs, setup.AccKeys, d, RpcNodeURL)
	setup.Funders = funders
	setup.Adjs = adjs

	return setup
}

func parseSUDTOwnerLockArg(pathToSUDTOwnerLockArg string) (string, error) {
	b, err := os.ReadFile(pathToSUDTOwnerLockArg)
	if err != nil {
		return "", fmt.Errorf("reading sudt owner lock arg from file: %w", err)
	}
	sudtOwnerLockArg := string(b)
	if sudtOwnerLockArg == "" {
		return "", errors.New("sudt owner lock arg not found in file")
	}
	return sudtOwnerLockArg, nil
}

func createFundersAndAdjudicators(t *testing.T, accs []*wallet.Account, keys []secp256k1.PrivateKey, deployment backend.Deployment, rpcURL string) ([]channel.Funder, []channel.Adjudicator) {
	t.Helper()
	funders := make([]channel.Funder, len(accs))
	adjs := make([]channel.Adjudicator, len(accs))

	for i, acc := range accs {
		rpcClient, err := rpc.Dial(rpcURL)
		require.NoError(t, err, "error connecting to ckb node")
		log.Println("Participant: ", address.AsParticipant(acc.Address()).ToCKBAddress(network).Script.Hash())
		signer := backend.NewSignerInstance(address.AsParticipant(acc.Address()).ToCKBAddress(network), keys[i], network)

		ckbClient, err := client.NewClient(rpcClient, *signer, deployment)
		require.NoError(t, err, "error creating ckb client")
		funders[i] = funder.NewDefaultFunder(ckbClient, deployment)
		adjs[i] = adjudicator.NewAdjudicator(ckbClient)
	}
	return funders, adjs
}
