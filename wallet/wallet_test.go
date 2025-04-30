package wallet_test

import (
	"encoding/hex"
	"log"
	"math/big"
	"perun.network/go-perun/channel"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gptest "perun.network/go-perun/wallet/test"
	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/encoding"
	"perun.network/perun-ckb-backend/wallet"
	"perun.network/perun-ckb-backend/wallet/address"
)

func TestEphemeralWallet(t *testing.T) {
	w := wallet.NewEphemeralWallet()

	acc, err := w.AddNewAccount()
	require.NoError(t, err)

	unlockedAccount, err := w.Unlock(acc.Address())
	require.NoError(t, err)
	require.Equal(t, acc.Address(), unlockedAccount.Address())

	msg := []byte("hello world")
	sig, err := unlockedAccount.SignData(msg)
	require.NoError(t, err)

	valid, err := wallet.Backend.VerifySignature(msg, sig, acc.Address())
	require.NoError(t, err)
	require.True(t, valid)
}

func TestStateSignature(t *testing.T) {
	w := wallet.NewEphemeralWallet()
	acc, err := w.AddNewAccount()
	require.NoError(t, err)

	participant := address.AsParticipant(acc.Address())
	publicKey := participant.PubKey
	require.NotNil(t, publicKey)
	log.Println("public key:", hex.EncodeToString(publicKey.SerializeCompressed()))

	alloc := &channel.Allocation{
		Assets: []channel.Asset{asset.NewCKBytesAsset()},
		Balances: [][]channel.Bal{
			{big.NewInt(10), big.NewInt(11)},
		},
		Locked: []channel.SubAlloc{},
	}

	state := &channel.State{
		ID:         channel.Zero,
		Version:    10,
		App:        channel.NoApp(),
		Allocation: *alloc,
		Data:       channel.NoData(),
		IsFinal:    true,
	}

	packedState, err := encoding.PackChannelState(state)
	require.NoError(t, err)

	signature, err := acc.SignData(packedState.AsSlice())
	log.Println("signature:", hex.EncodeToString(signature))
	log.Println("packed state:", "0x"+hex.EncodeToString(packedState.AsSlice()))
	require.NoError(t, err)

	b, err := wallet.Backend.VerifySignature(packedState.AsSlice(), signature, acc.Address())
	require.NoError(t, err)
	assert.Truef(t, b, "signature verification failed for address %s", acc.Address().String())
}

func TestPackAdddress(t *testing.T) {
	w := wallet.NewEphemeralWallet()

	acc, err := w.AddNewAccount()
	require.NoError(t, err)

	unlockedAccount, err := w.Unlock(acc.Address())
	require.NoError(t, err)
	require.Equal(t, acc.Address(), unlockedAccount.Address())

	participant := address.AsParticipant(acc.Address())
	encodedParticipant, err := participant.PackOnChainParticipant()
	require.NoError(t, err)

	var restoredParticipant address.Participant
	err = restoredParticipant.UnpackOnChainParticipant(&encodedParticipant)
	require.NoError(t, err)
	require.Equal(t, participant.PubKey, restoredParticipant.PubKey)
	require.Equal(t, participant.PaymentScript, restoredParticipant.PaymentScript)
	require.Equal(t, participant.UnlockScript, restoredParticipant.UnlockScript)
}

func setup() *gptest.Setup {
	w := wallet.NewEphemeralWallet()
	acc, err := w.AddNewAccount()
	if err != nil {
		panic(err)
	}
	acc2, err := wallet.NewAccount()
	if err != nil {
		panic(err)
	}
	binAddr2, err := acc2.Address().MarshalBinary()
	if err != nil {
		panic(err)
	}
	return &gptest.Setup{
		Backend:           wallet.Backend,
		Wallet:            w,
		AddressInWallet:   acc.Address(),
		ZeroAddress:       address.GetZeroAddress(),
		DataToSign:        []byte("pls sign me"),
		AddressMarshalled: binAddr2,
	}
}

func TestAddress(t *testing.T) {
	gptest.TestAddress(t, setup())
}

func TestGenericSignatureSizeTest(t *testing.T) {
	gptest.GenericSignatureSizeTest(t, setup())
}

func TestAccountWithWalletAndBackend(t *testing.T) {
	gptest.TestAccountWithWalletAndBackend(t, setup())
}
