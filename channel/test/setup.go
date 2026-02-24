// Copyright 2025 PolyCrypt GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"log"
	"math/big"
	"math/rand"
	"os"
	"testing"

	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	ckbsigner "github.com/nervosnetwork/ckb-sdk-go/v2/transaction/signer"
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
	RpcNodeURL = "http://localhost:8114"
	network    = types.NetworkTest
	devNetDir  = "../devnet"
)

type SetupCross struct {
	t            *testing.T
	Rng          *rand.Rand
	Deployment   backend.Deployment
	Asset        *asset.NervosAsset
	SUDTInfo     SUDTInfo
	CkbAsset     *asset.NervosAsset
	SudtAsset    *asset.NervosAsset
	EthAsset     *asset.EthAsset
	Participants []*address.Participant

	WalletAccs       []*wallet.Account
	AccKeys          []secp256k1.PrivateKey
	EphemeralWallets []*ckbwallettest.TestEphemeralWallet

	Funders []channel.Funder
	Adjs    []channel.Adjudicator
}
type Setup struct {
	t            *testing.T
	Rng          *rand.Rand
	Deployment   backend.Deployment
	Asset        *asset.Asset
	SUDTInfo     SUDTInfo
	CkbAsset     *asset.Asset
	SudtAsset    *asset.Asset
	EthAsset     *asset.EthAsset
	Participants []*address.Participant

	WalletAccs       []*wallet.Account
	AccKeys          []secp256k1.PrivateKey
	EphemeralWallets []*ckbwallettest.TestEphemeralWallet

	Funders []channel.Funder
	Adjs    []channel.Adjudicator
}

func NewCrossSetup(t *testing.T, rng *rand.Rand, omni bool) *SetupCross {
	setup := &SetupCross{}
	setup.t = t
	setup.Rng = rng

	sudtOwnerLockArg, err := ParseSUDTOwnerLockArg(devNetDir + "/accounts/sudt-owner-lock-hash.txt")
	require.NoError(t, err, "error getting SUDT owner lock arg")

	d, sudtInfo, err := GetDeployment(devNetDir+"/contract/migrations_0/dev/", devNetDir+"/contract/migrations_1/dev/", devNetDir+"/contract/migrations_vc/dev/", devNetDir+"/system_scripts", sudtOwnerLockArg)
	require.NoError(t, err, "error getting deployment")
	setup.Deployment = d
	setup.SUDTInfo = *sudtInfo

	sudt := asset.NewSUDT(*sudtInfo.Script, sudtMaxCapacity)
	sudtAsset := asset.NewSUDTAsset(sudt)
	ledgerID := asset.MakeContractID("03") // Set this appropriately
	ccid := asset.MakeCCID(ledgerID)
	nervosSUDTAsset := asset.NewNervosAsset(*sudtAsset, ccid)
	setup.SudtAsset = &nervosSUDTAsset

	ckbAsset := asset.NewCKBytesNervosAsset()
	setup.CkbAsset = ckbAsset
	setup.Asset = ckbAsset

	var ethAddrArray [20]byte
	newEthAddrBytes := common.BytesToAddress(ethAddrArray[:])
	newEthAddr := address.EthAddress(newEthAddrBytes)

	chainID := new(big.Int)
	chainID.SetUint64(uint64(1))
	ethAsset := asset.MakeEthAsset(chainID, &newEthAddr)
	setup.EthAsset = &ethAsset

	wallets := make([]*ckbwallettest.TestEphemeralWallet, 2)
	setup.EphemeralWallets = wallets

	parts := make([]*address.Participant, 2)
	keyAlice, err := GetKey(devNetDir + "/accounts/alice.pk")
	require.NoError(t, err, "error getting alice's private key")

	keyBob, err := GetKey(devNetDir + "/accounts/bob.pk")
	require.NoError(t, err, "error getting bob's private key")
	var evmAddresses [][20]byte
	var aliceAccount, bobAccount *wallet.Account
	if omni {
		evmAddresses = make([][20]byte, 2)
		parts[0], evmAddresses[0], _ = address.NewEthereumParticipantFromPublicKey(keyAlice.PubKey(), d.OmniLockScript.CodeHash)
		parts[1], evmAddresses[1], _ = address.NewEthereumParticipantFromPublicKey(keyBob.PubKey(), d.OmniLockScript.CodeHash)
		add0, _ := parts[0].ToCKBAddress(network).EncodeFullBech32m()
		log.Println("Alice's CKB address: ", add0)
		add1, _ := parts[1].ToCKBAddress(network).EncodeFullBech32m()
		log.Println("Bob's CKB address: ", add1)
		setup.Participants = parts
		aliceAccount = wallet.NewAccountFromPrivateKey(keyAlice, d.OmniLockScript.CodeHash, false)
		bobAccount = wallet.NewAccountFromPrivateKey(keyBob, d.OmniLockScript.CodeHash, false)
	} else {
		parts[0], _ = address.NewDefaultParticipant(keyAlice.PubKey())
		parts[1], _ = address.NewDefaultParticipant(keyBob.PubKey())
		add0, _ := parts[0].ToCKBAddress(network).EncodeFullBech32m()
		log.Println("Alice's CKB address: ", add0)
		add1, _ := parts[1].ToCKBAddress(network).EncodeFullBech32m()
		log.Println("Bob's CKB address: ", add1)
		setup.Participants = parts
		aliceAccount = wallet.NewAccountFromPrivateKey(keyAlice, types.Hash{}, true)
		bobAccount = wallet.NewAccountFromPrivateKey(keyBob, types.Hash{}, true)
	}

	wallets[0] = ckbwallettest.NewTestEphemeralWallet(aliceAccount)
	err = wallets[0].AddAccount(aliceAccount)
	require.NoError(t, err, "error adding alice's account")

	wallets[1] = ckbwallettest.NewTestEphemeralWallet(bobAccount)
	err = wallets[1].AddAccount(bobAccount)
	require.NoError(t, err, "error adding bob's account")

	setup.WalletAccs = []*wallet.Account{aliceAccount, bobAccount}
	setup.AccKeys = []secp256k1.PrivateKey{*keyAlice, *keyBob}

	funders, adjs := createFundersAndAdjudicators(t, setup.WalletAccs, setup.AccKeys, d, RpcNodeURL, omni, evmAddresses)
	setup.Funders = funders
	setup.Adjs = adjs

	return setup
}

func NewSetup(t *testing.T, rng *rand.Rand, omni bool) *Setup {
	setup := &Setup{}
	setup.t = t
	setup.Rng = rng

	sudtOwnerLockArg, err := ParseSUDTOwnerLockArg(devNetDir + "/accounts/sudt-owner-lock-hash.txt")
	require.NoError(t, err, "error getting SUDT owner lock arg")

	d, sudtInfo, err := GetDeployment(devNetDir+"/contract/migrations_0/dev/", devNetDir+"/contract/migrations_1/dev/", devNetDir+"/contract/migrations_vc/dev/", devNetDir+"/system_scripts", sudtOwnerLockArg)
	require.NoError(t, err, "error getting deployment")
	setup.Deployment = d
	setup.SUDTInfo = *sudtInfo

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
	var evmAddresses [][20]byte
	var aliceAccount, bobAccount *wallet.Account
	if omni {
		evmAddresses = make([][20]byte, 2)
		parts[0], evmAddresses[0], _ = address.NewEthereumParticipantFromPublicKey(keyAlice.PubKey(), d.OmniLockScript.CodeHash)
		parts[1], evmAddresses[1], _ = address.NewEthereumParticipantFromPublicKey(keyBob.PubKey(), d.OmniLockScript.CodeHash)
		add0, _ := parts[0].ToCKBAddress(network).EncodeFullBech32m()
		log.Println("Alice's CKB address: ", add0)
		add1, _ := parts[1].ToCKBAddress(network).EncodeFullBech32m()
		log.Println("Bob's CKB address: ", add1)
		setup.Participants = parts
		aliceAccount = wallet.NewAccountFromPrivateKey(keyAlice, d.OmniLockScript.CodeHash, false)
		bobAccount = wallet.NewAccountFromPrivateKey(keyBob, d.OmniLockScript.CodeHash, false)
	} else {
		parts[0], _ = address.NewDefaultParticipant(keyAlice.PubKey())
		parts[1], _ = address.NewDefaultParticipant(keyBob.PubKey())
		add0, _ := parts[0].ToCKBAddress(network).EncodeFullBech32m()
		log.Println("Alice's CKB address: ", add0)
		add1, _ := parts[1].ToCKBAddress(network).EncodeFullBech32m()
		log.Println("Bob's CKB address: ", add1)
		setup.Participants = parts
		aliceAccount = wallet.NewAccountFromPrivateKey(keyAlice, types.Hash{}, true)
		bobAccount = wallet.NewAccountFromPrivateKey(keyBob, types.Hash{}, true)
	}

	wallets[0] = ckbwallettest.NewTestEphemeralWallet(aliceAccount)
	err = wallets[0].AddAccount(aliceAccount)
	require.NoError(t, err, "error adding alice's account")

	wallets[1] = ckbwallettest.NewTestEphemeralWallet(bobAccount)
	err = wallets[1].AddAccount(bobAccount)
	require.NoError(t, err, "error adding bob's account")

	setup.WalletAccs = []*wallet.Account{aliceAccount, bobAccount}
	setup.AccKeys = []secp256k1.PrivateKey{*keyAlice, *keyBob}

	funders, adjs := createFundersAndAdjudicators(t, setup.WalletAccs, setup.AccKeys, d, RpcNodeURL, omni, evmAddresses)
	setup.Funders = funders
	setup.Adjs = adjs

	return setup
}

func NewVirtualChannelSetup(t *testing.T, rng *rand.Rand, omni bool) *SetupCross {
	setup := &SetupCross{}
	setup.t = t
	setup.Rng = rng

	sudtOwnerLockArg, err := ParseSUDTOwnerLockArg(devNetDir + "/accounts/sudt-owner-lock-hash.txt")
	require.NoError(t, err, "error getting SUDT owner lock arg")

	d, sudtInfo, err := GetDeployment(devNetDir+"/contract/migrations_0/dev/", devNetDir+"/contract/migrations_1/dev/", devNetDir+"/contract/migrations_vc/dev/", devNetDir+"/system_scripts", sudtOwnerLockArg)
	require.NoError(t, err, "error getting deployment")
	setup.Deployment = d
	setup.SUDTInfo = *sudtInfo

	ckbAsset := asset.NewCKBytesNervosAsset()
	setup.CkbAsset = ckbAsset
	setup.Asset = ckbAsset

	wallets := make([]*ckbwallettest.TestEphemeralWallet, 3)
	setup.EphemeralWallets = wallets
	parts := make([]*address.Participant, 3)
	keyAlice, err := GetKey(devNetDir + "/accounts/alice.pk")
	require.NoError(t, err, "error getting alice's private key")

	keyBob, err := GetKey(devNetDir + "/accounts/bob.pk")
	require.NoError(t, err, "error getting bob's private key")

	keyIngrid, err := GetKey(devNetDir + "/accounts/ingrid.pk")
	require.NoError(t, err, "error getting ingrid's private key")
	var evmAddresses [][20]byte
	var aliceAccount, bobAccount, ingridAccount *wallet.Account
	if omni {
		evmAddresses = make([][20]byte, 3)
		parts[0], evmAddresses[0], _ = address.NewEthereumParticipantFromPublicKey(keyAlice.PubKey(), d.OmniLockScript.CodeHash)
		parts[1], evmAddresses[1], _ = address.NewEthereumParticipantFromPublicKey(keyBob.PubKey(), d.OmniLockScript.CodeHash)
		parts[2], evmAddresses[2], _ = address.NewEthereumParticipantFromPublicKey(keyIngrid.PubKey(), d.OmniLockScript.CodeHash)
		add0, _ := parts[0].ToCKBAddress(network).EncodeFullBech32m()
		log.Println("Alice's CKB address: ", add0)
		add1, _ := parts[1].ToCKBAddress(network).EncodeFullBech32m()
		log.Println("Bob's CKB address: ", add1)
		add2, _ := parts[2].ToCKBAddress(network).EncodeFullBech32m()
		log.Println("Ingrid's CKB address: ", add2)
		setup.Participants = parts
		aliceAccount = wallet.NewAccountFromPrivateKey(keyAlice, d.OmniLockScript.CodeHash, false)
		bobAccount = wallet.NewAccountFromPrivateKey(keyBob, d.OmniLockScript.CodeHash, false)
		ingridAccount = wallet.NewAccountFromPrivateKey(keyIngrid, d.OmniLockScript.CodeHash, false)
	} else {
		parts[0], _ = address.NewDefaultParticipant(keyAlice.PubKey())
		parts[1], _ = address.NewDefaultParticipant(keyBob.PubKey())
		parts[2], _ = address.NewDefaultParticipant(keyIngrid.PubKey())
		add0, _ := parts[0].ToCKBAddress(network).EncodeFullBech32m()
		log.Println("Alice's CKB address: ", add0)
		add1, _ := parts[1].ToCKBAddress(network).EncodeFullBech32m()
		log.Println("Bob's CKB address: ", add1)
		add2, _ := parts[2].ToCKBAddress(network).EncodeFullBech32m()
		log.Println("Ingrid's CKB address: ", add2)
		setup.Participants = parts
		aliceAccount = wallet.NewAccountFromPrivateKey(keyAlice, types.Hash{}, true)
		bobAccount = wallet.NewAccountFromPrivateKey(keyBob, types.Hash{}, true)
		ingridAccount = wallet.NewAccountFromPrivateKey(keyIngrid, types.Hash{}, true)
	}
	setup.Participants = parts
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

	funders, adjs := createFundersAndAdjudicators(t, setup.WalletAccs, setup.AccKeys, d, RpcNodeURL, omni, evmAddresses)
	setup.Funders = funders
	setup.Adjs = adjs

	return setup
}

func ParseSUDTOwnerLockArg(pathToSUDTOwnerLockArg string) (string, error) {
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

func createFundersAndAdjudicators(t *testing.T, accs []*wallet.Account, keys []secp256k1.PrivateKey, deployment backend.Deployment, rpcURL string, omni bool, evmAddresses [][20]byte) ([]channel.Funder, []channel.Adjudicator) {
	t.Helper()
	funders := make([]channel.Funder, len(accs))
	adjs := make([]channel.Adjudicator, len(accs))

	for i, acc := range accs {
		rpcClient, err := rpc.Dial(rpcURL)
		require.NoError(t, err, "error connecting to ckb node")
		log.Println("Participant: ", address.AsParticipant(acc.Address()).ToCKBAddress(network).Script.Hash())
		var signer backend.Signer
		if omni {
			evmsigner := backend.NewEVMSignerInstance(address.AsParticipant(acc.Address()).ToCKBAddress(network), keys[i], network, evmAddresses[i])
			txSigner := evmsigner.Signer()
			txSigner.RegisterLockSigner(deployment.OmniLockScript.CodeHash, &ckbsigner.OmnilockSigner{})
			signer = evmsigner

		} else {
			signer = backend.NewSignerInstance(address.AsParticipant(acc.Address()).ToCKBAddress(network), keys[i], network)
		}

		ckbClient, err := client.NewClient(rpcClient, signer, deployment)
		require.NoError(t, err, "error creating ckb client")
		funders[i] = funder.NewDefaultFunder(ckbClient, deployment)
		adjs[i] = adjudicator.NewAdjudicator(ckbClient)
	}
	return funders, adjs
}
