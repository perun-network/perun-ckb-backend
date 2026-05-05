package ckblp

import (
	"testing"

	"github.com/Pilatuz/bigz/uint128"
)

// TestDecodeLPCell_RoundTrip verifies that encoding and decoding an LP cell preserves all data.
func TestDecodeLPCell_RoundTrip(t *testing.T) {
	// Create a test LP cell
	original := LPCell{
		PoolID:                  [32]byte{1, 2, 3, 4},
		OwnerLockHash:           [32]byte{5, 6, 7, 8},
		OperatorLockHash:        [32]byte{9, 10, 11, 12},
		AvailableCKB:            1000000,
		ReservedCKB:             500000,
		CumulativeFeesEarnedCKB: 10000,
		Nonce:                   5,
		Active:                  true,
		Policy: LPPolicy{
			MaxTradingVolume: 10000000,
			FeeRateBps:       100,
			PolicyFlags:      policyFlagSafePrice,
			PolicyVersion:    1,
			SafePriceMinX64:  uint128.Uint128{Lo: 100},
			SafePriceMaxX64:  uint128.Uint128{Lo: 200},
		},
	}

	// Encode
	encoded, err := EncodeLPCell(original)
	if err != nil {
		t.Fatalf("EncodeLPCell failed: %v", err)
	}

	// Verify encoded size
	if len(encoded) != 185 {
		t.Errorf("expected encoded size 185, got %d", len(encoded))
	}

	// Verify magic prefix
	if len(encoded) < 4 || encoded[0] != 'L' || encoded[1] != 'P' || encoded[2] != 'L' || encoded[3] != 'C' {
		t.Error("encoded data missing LPLC magic prefix")
	}

	// Decode
	decoded, err := DecodeLPCell(encoded)
	if err != nil {
		t.Fatalf("DecodeLPCell failed: %v", err)
	}

	// Verify roundtrip
	if decoded.OperatorLockHash != original.OperatorLockHash {
		t.Errorf("OperatorLockHash mismatch: expected %x, got %x", original.OperatorLockHash, decoded.OperatorLockHash)
	}
	if decoded.AvailableCKB != original.AvailableCKB {
		t.Errorf("AvailableCKB mismatch: expected %d, got %d", original.AvailableCKB, decoded.AvailableCKB)
	}
	if decoded.ReservedCKB != original.ReservedCKB {
		t.Errorf("ReservedCKB mismatch: expected %d, got %d", original.ReservedCKB, decoded.ReservedCKB)
	}
	if decoded.CumulativeFeesEarnedCKB != original.CumulativeFeesEarnedCKB {
		t.Errorf("CumulativeFeesEarnedCKB mismatch: expected %d, got %d", original.CumulativeFeesEarnedCKB, decoded.CumulativeFeesEarnedCKB)
	}
	if decoded.Nonce != original.Nonce {
		t.Errorf("Nonce mismatch: expected %d, got %d", original.Nonce, decoded.Nonce)
	}
	if decoded.Policy.PolicyFlags != original.Policy.PolicyFlags {
		t.Errorf("PolicyFlags mismatch: expected %x, got %x", original.Policy.PolicyFlags, decoded.Policy.PolicyFlags)
	}
}

// TestDecodeLPCell_InvalidSize verifies that invalid data size returns an error.
func TestDecodeLPCell_InvalidSize(t *testing.T) {
	// Too short
	_, err := DecodeLPCell([]byte("short"))
	if err == nil {
		t.Error("expected error for invalid size, got nil")
	}

	// Wrong size
	_, err = DecodeLPCell(make([]byte, 100))
	if err == nil {
		t.Error("expected error for invalid size, got nil")
	}
}

// TestDecodeLPCell_MissingMagic verifies that missing magic prefix returns an error.
func TestDecodeLPCell_MissingMagic(t *testing.T) {
	data := make([]byte, 185)
	data[0] = 'X' // Wrong magic
	_, err := DecodeLPCell(data)
	if err == nil {
		t.Error("expected error for missing magic prefix, got nil")
	}
}

// TestIsLPCell verifies magic prefix detection.
func TestIsLPCell(t *testing.T) {
	// Valid LP cell
	data := make([]byte, 185)
	data[0], data[1], data[2], data[3] = 'L', 'P', 'L', 'C'
	if !IsLPCell(data) {
		t.Error("IsLPCell should return true for valid LP cell data")
	}

	// Invalid magic
	data[0] = 'X'
	if IsLPCell(data) {
		t.Error("IsLPCell should return false for invalid magic")
	}

	// Too short
	if IsLPCell([]byte("short")) {
		t.Error("IsLPCell should return false for short data")
	}
}
