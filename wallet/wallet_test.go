package wallet_test

import (
	"github.com/stretchr/testify/require"
	"math/rand"
	gptest "perun.network/go-perun/wallet/test"
	"perun.network/perun-ckb-backend/wallet"
	pkgtest "polycry.pt/poly-go/test"
	"testing"
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

func setup(rng *rand.Rand) *gptest.Setup {
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
		ZeroAddress:       wallet.GetZeroAddress(),
		DataToSign:        []byte("pls sign me"),
		AddressMarshalled: binAddr2,
	}
}

func TestAddress(t *testing.T) {
	gptest.TestAddress(t, setup(pkgtest.Prng(t)))
}

func TestGenericSignatureSizeTest(t *testing.T) {
	gptest.GenericSignatureSizeTest(t, setup(pkgtest.Prng(t)))
}

func TestAccountWithWalletAndBackend(t *testing.T) {
	gptest.TestAccountWithWalletAndBackend(t, setup(pkgtest.Prng(t)))
}
