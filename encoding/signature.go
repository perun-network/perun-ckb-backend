package encoding

import (
	"encoding/asn1"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
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
//   - 65 bytes: Ethereum R||S||V (recovery byte is not needed by the contract)
//   - 64 bytes: R||S (no V)
//   - DER:      a complete ASN.1 sequence, canonicalized to low-S
//
// Fixed-width 64/65-byte encodings take precedence over DER.
func NewMoleculeSignature(sig []byte) (*molecule.Bytes, error) {
	var r, s *big.Int
	switch len(sig) {
	case 64, 65:
		// R is an arbitrary scalar and can start with 0x30. Fixed-width
		// signatures must not be mistaken for DER based on that byte alone.
		r = new(big.Int).SetBytes(sig[:32])
		s = new(big.Int).SetBytes(sig[32:64])
	default:
		var decoded ecdsaSignature
		rest, err := asn1.Unmarshal(sig, &decoded)
		if err != nil || len(rest) != 0 {
			return nil, fmt.Errorf("invalid ECDSA signature encoding")
		}
		r, s = decoded.R, decoded.S
	}
	n := secp256k1.S256().Params().N
	if r == nil || s == nil || r.Sign() <= 0 || s.Sign() <= 0 || r.Cmp(n) >= 0 || s.Cmp(n) >= 0 {
		return nil, fmt.Errorf("ECDSA signature scalar outside secp256k1 range")
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
