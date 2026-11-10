package encoding

import (
	"encoding/asn1"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestRawSignatureStartingWithDERTag(t *testing.T) {
	key, err := crypto.ToECDSA(append(make([]byte, 31), 1))
	require.NoError(t, err)
	for i := int64(0); i < 10000; i++ {
		hash := crypto.Keccak256(big.NewInt(i).Bytes())
		sig, err := crypto.Sign(hash, key)
		require.NoError(t, err)
		if sig[0] != 0x30 {
			continue
		}
		require.True(t, crypto.VerifySignature(crypto.CompressPubkey(&key.PublicKey), hash, sig[:64]))
		for _, raw := range [][]byte{sig, sig[:64]} {
			packed, err := NewMoleculeSignature(raw)
			require.NoError(t, err)
			var got ecdsaSignature
			// Molecule Bytes prefixes its payload with a 4-byte element count.
			rest, err := asn1.Unmarshal(packed.AsSlice()[4:], &got)
			require.NoError(t, err)
			require.Empty(t, rest)
			require.Equal(t, 0, got.R.Cmp(new(big.Int).SetBytes(sig[:32])))
			require.Equal(t, 0, got.S.Cmp(new(big.Int).SetBytes(sig[32:64])))
		}
		return
	}
	t.Fatal("could not generate the deterministic regression signature")
}

func TestSignatureRejectsInvalidDERAndScalars(t *testing.T) {
	for _, raw := range [][]byte{nil, {0x30}, {0x30, 0}, make([]byte, 64), make([]byte, 65)} {
		_, err := NewMoleculeSignature(raw)
		require.Error(t, err)
	}
	der, err := asn1.Marshal(ecdsaSignature{R: big.NewInt(1), S: big.NewInt(2)})
	require.NoError(t, err)
	_, err = NewMoleculeSignature(der)
	require.NoError(t, err)
	_, err = NewMoleculeSignature(append(der, 0))
	require.Error(t, err)
	negative, err := asn1.Marshal(ecdsaSignature{R: big.NewInt(-1), S: big.NewInt(2)})
	require.NoError(t, err)
	_, err = NewMoleculeSignature(negative)
	require.Error(t, err)
}

func TestSignatureScalarRangeAndLowS(t *testing.T) {
	n := crypto.S256().Params().N
	for _, sig := range []ecdsaSignature{
		{R: new(big.Int).Set(n), S: big.NewInt(1)},
		{R: big.NewInt(1), S: new(big.Int).Set(n)},
		{R: big.NewInt(0), S: big.NewInt(1)},
	} {
		der, err := asn1.Marshal(sig)
		require.NoError(t, err)
		_, err = NewMoleculeSignature(der)
		require.Error(t, err)
	}
	highS := new(big.Int).Sub(n, big.NewInt(1))
	der, err := asn1.Marshal(ecdsaSignature{R: big.NewInt(1), S: highS})
	require.NoError(t, err)
	packed, err := NewMoleculeSignature(der)
	require.NoError(t, err)
	var got ecdsaSignature
	_, err = asn1.Unmarshal(packed.AsSlice()[4:], &got)
	require.NoError(t, err)
	require.Equal(t, 0, got.S.Cmp(big.NewInt(1)))
}
