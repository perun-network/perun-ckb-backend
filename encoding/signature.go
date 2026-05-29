package encoding

import (
	"encoding/asn1"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"math/big"
	gpwallet "perun.network/go-perun/wallet"
)

// ASN.1 container for ECDSA (r,s)
type ecdsaSignature struct {
	R, S *big.Int
}

func toLowS(s *big.Int) *big.Int {
	n := secp256k1.S256().Params().N
	halfN := new(big.Int).Rsh(new(big.Int).Set(n), 1)
	if s.Cmp(halfN) > 0 {
		return new(big.Int).Sub(n, s)
	}
	return s
}

// NewMoleculeSignature converts an incoming signature to DER (R,S) and packs it.
// Accepts:
//   - 65 bytes: Ethereum R||S||V (with V=27/28 or 0/1)  <-- your SignData output
//   - 64 bytes: R||S (no V)
//   - DER:      starts with 0x30 (passes through)
func NewMoleculeSignature(sig []byte) (*molecule.Bytes, error) {
	// Switch on length FIRST. A raw 65/64-byte signature must always be
	// converted: its leading byte is the first byte of R, which is 0x30 (the DER
	// SEQUENCE tag) ~1/256 of the time. Sniffing for 0x30 before checking the
	// length would misclassify those raw signatures as DER and pass them through
	// unconverted, so the on-chain Signature::from_der fails ~1/256 of the time
	// (a flaky SignatureVerificationError).
	var r, s *big.Int
	switch len(sig) {
	case 65: // R||S||V
		r = new(big.Int).SetBytes(sig[:32])
		s = new(big.Int).SetBytes(sig[32:64])
		// ignore V (sig[64])
	case 64: // R||S
		r = new(big.Int).SetBytes(sig[:32])
		s = new(big.Int).SetBytes(sig[32:64])
	default:
		// Not a raw signature: assume it is already DER (SEQUENCE tag 0x30).
		if len(sig) > 0 && sig[0] == 0x30 {
			return types.PackBytes(sig), nil
		}
		return nil, fmt.Errorf("unexpected signature length %d (want 65/64 or DER)", len(sig))
	}

	s = toLowS(s) // k256 expects canonical (low-S)
	der, err := asn1.Marshal(ecdsaSignature{R: r, S: s})
	if err != nil {
		return nil, fmt.Errorf("asn1 marshal: %w", err)
	}
	return types.PackBytes(der), nil
}

// PackSignature converts a perun signature to a molecule signature.
func PackSignature(sig gpwallet.Sig) *molecule.Bytes {
	return types.PackBytes(sig)
}

// PackVCDispute encodes the signatures needed for a VCDispute into the contract' witnesses.
func PackVCDispute(sigA, sigB, parentSigA, parentSigB *molecule.Bytes) molecule.VCDispute {
	vcdispute := molecule.NewVCDisputeBuilder().
		SigA(*sigA).
		SigB(*sigB).
		ParentStateSigs(molecule.NewDisputeBuilder().
			SigA(*parentSigA).
			SigB(*parentSigB).
			Build()).
		Build()
	return vcdispute
}
