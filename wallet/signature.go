package wallet

import (
	"errors"
	"fmt"
)

const PaddedSignatureLength = 73
const MarkerPad byte = 0xff
const ZeroPad byte = 0x00

// PadDEREncodedSignature pads the DER encoded signature to a length of PaddedSignatureLength bytes.
func PadDEREncodedSignature(sig []byte) ([]byte, error) {
	if len(sig) >= PaddedSignatureLength {
		return nil, fmt.Errorf("signature is too long. Expected at most %d bytes, got %d bytes", PaddedSignatureLength-1, len(sig))
	}
	paddedSignature := make([]byte, PaddedSignatureLength)
	copy(paddedSignature[:len(sig)], sig)
	paddedSignature[len(sig)] = MarkerPad
	for i := len(sig) + 1; i < PaddedSignatureLength; i++ {
		paddedSignature[i] = ZeroPad
	}
	return paddedSignature, nil
}

// RemovePadding returns the DER encoded signature without the padding without allocating a new slice.
func RemovePadding(sig []byte) ([]byte, error) {
	if len(sig) != PaddedSignatureLength {
		return nil, fmt.Errorf("signature is of wrong length. Expected %d bytes, got %d bytes", PaddedSignatureLength, len(sig))
	}
	for i := len(sig) - 1; i >= 0; i-- {
		if sig[i] == MarkerPad {
			return sig[:i], nil
		}
		if sig[i] != ZeroPad {
			return nil, fmt.Errorf("invalid padding")
		}
	}
	return nil, errors.New("invalid padding: missing marker")
}
