package wallet_test

import (
	"encoding/asn1"
	"fmt"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	decredEcdsa "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"perun.network/perun-ckb-backend/wallet"
	"testing"
)

func TestDecredAndGethSignatures(t *testing.T) {
	decredKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("failed to generate decred key: %v", err)
	}

	msg := []byte("test message for signing")
	phash := gethcrypto.Keccak256(
		[]byte("\x19Ethereum Signed Message:\n32"),
		gethcrypto.Keccak256(msg),
	)

	decredSig := decredEcdsa.Sign(decredKey, phash)
	decredSigBytes := decredSig.Serialize()
	// Parse DER signature to R, S
	var sigStruct wallet.ECDSASignature
	if _, err := asn1.Unmarshal(decredSigBytes, &sigStruct); err != nil {
		t.Fatalf("failed to parse decred DER signature: %v", err)
	}

	// Convert Decred key to Go std ecdsa.PrivateKey for go-ethereum Sign
	ecdsaKey := wallet.ConvertDecredKeyToECDSA(decredKey)

	// Sign with go-ethereum crypto.Sign (produces R||S||V)
	gethSig, err := gethcrypto.Sign(phash, ecdsaKey)
	if err != nil {
		t.Fatalf("go-ethereum signing failed: %v", err)
	}
	fmt.Printf("Geth signature bytes (R || S || V): %x\n", gethSig)
	// Extract R, S from geth signature (first 64 bytes)
	gethR := new(big.Int).SetBytes(gethSig[:32])
	gethS := new(big.Int).SetBytes(gethSig[32:64])

	// Compare R values
	if sigStruct.R.Cmp(gethR) != 0 {
		t.Errorf("R values differ\nDecred: %x\nGeth:   %x", sigStruct.R, gethR)
	}
	// Compare S values
	if sigStruct.S.Cmp(gethS) != 0 {
		t.Errorf("S values differ\nDecred: %x\nGeth:   %x", sigStruct.S, gethS)
	}

	// Verify Decred signature
	if !decredSig.Verify(phash, decredKey.PubKey()) {
		t.Error("failed to verify decred signature")
	}

	// Verify Geth signature (exclude recovery byte)
	if !gethcrypto.VerifySignature(gethcrypto.FromECDSAPub(&ecdsaKey.PublicKey), phash, gethSig[:64]) {
		t.Error("failed to verify go-ethereum signature")
	}
}
