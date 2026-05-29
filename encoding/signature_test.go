package encoding

import (
	"crypto/ecdsa"
	"encoding/asn1"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/ethereum/go-ethereum/crypto"
	gethsecp "github.com/ethereum/go-ethereum/crypto/secp256k1"
)

// prehash reproduces the digest that wallet.Account.SignData signs over.
func prehash(msg []byte) []byte {
	hash := crypto.Keccak256(msg)
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	return crypto.Keccak256(prefix, hash)
}

// NewMoleculeSignature must convert every raw 65/64-byte signature to DER, even
// when R's first byte is 0x30 (the DER SEQUENCE tag). Previously a 0x30-sniff
// ran before the length check, so ~1/256 of raw signatures (those with R[0]==0x30)
// were passed through unconverted and failed the on-chain Signature::from_der,
// producing a flaky SignatureVerificationError.
func TestNewMoleculeSignatureConvertsRawSigStartingWith0x30(t *testing.T) {
	phash := prehash([]byte("disputed channel state"))
	halfN := new(big.Int).Rsh(new(big.Int).Set(gethsecp.S256().Params().N), 1)

	// Find a real signature whose R starts with 0x30 (the bug's trigger).
	var sig []byte
	var pub *ecdsa.PublicKey
	for i := 0; i < 100000; i++ {
		key, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		priv, err := crypto.HexToECDSA(hex.EncodeToString(key.Serialize()))
		if err != nil {
			t.Fatal(err)
		}
		s, err := crypto.Sign(phash, priv)
		if err != nil {
			t.Fatal(err)
		}
		s[64] += 27
		if s[0] == 0x30 {
			sig = s
			pub = &priv.PublicKey
			break
		}
	}
	if sig == nil {
		t.Skip("could not find a signature with R[0]==0x30")
	}

	mol, err := NewMoleculeSignature(sig)
	if err != nil {
		t.Fatalf("NewMoleculeSignature: %v", err)
	}
	der := mol.RawData()

	// The raw 65-byte sig must NOT be passed through as-is.
	if len(der) == 65 {
		t.Fatalf("raw 65-byte signature was passed through unconverted (len=65): %x", der)
	}
	// Output must be valid, fully-consumed DER.
	var parsed ecdsaSignature
	rest, err := asn1.Unmarshal(der, &parsed)
	if err != nil || len(rest) != 0 {
		t.Fatalf("output is not valid DER: err=%v rest=%d der=%x", err, len(rest), der)
	}
	// R preserved, S normalized to low-S, and the (R,S) still verify.
	if parsed.R.Cmp(new(big.Int).SetBytes(sig[:32])) != 0 {
		t.Errorf("R mismatch")
	}
	if parsed.S.Cmp(halfN) > 0 {
		t.Errorf("S is not low-S")
	}
	if !ecdsa.Verify(pub, phash, parsed.R, parsed.S) {
		t.Errorf("converted signature does not verify against the signer pubkey")
	}
}

// Over many random keys, every signature must convert to DER that parses and
// verifies — i.e. zero flaky failures.
func TestNewMoleculeSignatureNoFlakyFailures(t *testing.T) {
	phash := prehash([]byte("disputed channel state"))
	const N = 3000
	var parseFail, verifyFail int
	for i := 0; i < N; i++ {
		key, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		priv, err := crypto.HexToECDSA(hex.EncodeToString(key.Serialize()))
		if err != nil {
			t.Fatal(err)
		}
		sig, err := crypto.Sign(phash, priv)
		if err != nil {
			t.Fatal(err)
		}
		sig[64] += 27

		mol, err := NewMoleculeSignature(sig)
		if err != nil {
			t.Fatalf("NewMoleculeSignature: %v", err)
		}
		var parsed ecdsaSignature
		if rest, err := asn1.Unmarshal(mol.RawData(), &parsed); err != nil || len(rest) != 0 {
			parseFail++
			continue
		}
		if !ecdsa.Verify(&priv.PublicKey, phash, parsed.R, parsed.S) {
			verifyFail++
		}
	}
	if parseFail != 0 || verifyFail != 0 {
		t.Errorf("N=%d parseFail=%d verifyFail=%d (want 0/0)", N, parseFail, verifyFail)
	}
}
