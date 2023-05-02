package wallet

import (
	"errors"
	"fmt"
)

const PaddedSignatureLength = 73
const MarkerByte byte = 0xff
const ZeroByte byte = 0x00

// A DER encoded secp256k1 signature does not have a fixed length. Its length varies depending on the values of r and s.
// The maximum length of a DER encoded signature is 72 bytes.
// As go-perun requires a fixed length signature, we pad the signature. We Pad the signature to a fixed length of
// PaddedSignatureLength bytes by appending one MarkerByte and then appending ZeroBytes until the signature is of length
// PaddedSignatureLength (possibly no ZeroBytes are needed).
//
// Examples:
// Input: <DER encoded signature of length 70 bytes>
// Output: <DER encoded signature of length 70 byte> | MarkerByte | ZeroByte | ZeroByte
//
// Input: <DER encoded signature of length 72 bytes>
// Output: <DER encoded signature of length 72 byte> | MarkerByte

// PadDEREncodedSignature pads the DER encoded signature to a length of PaddedSignatureLength bytes.
func PadDEREncodedSignature(sig []byte) ([]byte, error) {
	if len(sig) >= PaddedSignatureLength {
		return nil, fmt.Errorf("signature is too long. Expected at most %d bytes, got %d bytes", PaddedSignatureLength-1, len(sig))
	}
	paddedSignature := make([]byte, PaddedSignatureLength)
	copy(paddedSignature[:len(sig)], sig)
	paddedSignature[len(sig)] = MarkerByte
	for i := len(sig) + 1; i < PaddedSignatureLength; i++ {
		paddedSignature[i] = ZeroByte
	}
	return paddedSignature, nil
}

// RemovePadding returns the DER encoded signature without the padding without allocating a new slice.
func RemovePadding(sig []byte) ([]byte, error) {
	if len(sig) != PaddedSignatureLength {
		return nil, fmt.Errorf("signature is of wrong length. Expected %d bytes, got %d bytes", PaddedSignatureLength, len(sig))
	}
	for i := len(sig) - 1; i >= 0; i-- {
		if sig[i] == MarkerByte {
			return sig[:i], nil
		}
		if sig[i] != ZeroByte {
			return nil, fmt.Errorf("invalid padding")
		}
	}
	return nil, errors.New("invalid padding: missing marker")
}
