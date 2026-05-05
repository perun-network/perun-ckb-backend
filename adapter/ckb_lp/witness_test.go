package ckblp

import (
	"testing"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

// TestEncodeFundChannelExtractWitness verifies FundChannelExtract witness encoding.
func TestEncodeFundChannelExtractWitness(t *testing.T) {
	channelID := types.Hash{1, 2, 3, 4, 5, 6, 7, 8}
	contribID := types.Hash{9, 10, 11, 12}

	witness := FundChannelExtractWitness{
		ChannelID:      channelID,
		ContributionID: contribID,
		ExtractCKB:     100000,
	}

	encoded := EncodeFundChannelExtractWitness(witness)

	// Verify non-empty
	if len(encoded) == 0 {
		t.Error("encoded witness should not be empty")
	}

	// Verify opcode is present (0x43 for FundChannelExtract)
	if encoded[0] != 0x43 {
		t.Errorf("expected opcode 0x43 for FundChannelExtract, got 0x%02x", encoded[0])
	}
}

// TestEncodeSettleChannelInsertWitness verifies SettleChannelInsert witness encoding.
func TestEncodeSettleChannelInsertWitness(t *testing.T) {
	channelID := types.Hash{1, 2, 3, 4}
	contribID := types.Hash{5, 6, 7, 8}
	price := uint128.Uint128{Lo: 150}

	witness := SettleChannelInsertWitness{
		ChannelID:         channelID,
		ContributionID:    contribID,
		PrincipalReturned: 100000,
		FeeCKB:            5000,
		PriceX64:          price,
	}

	encoded := EncodeSettleChannelInsertWitness(witness)

	// Verify non-empty
	if len(encoded) == 0 {
		t.Error("encoded witness should not be empty")
	}

	// Verify opcode is present (0x44 for SettleChannelInsert)
	if encoded[0] != 0x44 {
		t.Errorf("expected opcode 0x44 for SettleChannelInsert, got 0x%02x", encoded[0])
	}
}

// TestEncodeSettleChannelInsertWitness_ZeroPrice verifies that zero price is allowed in witness.
// (The zero-price guard should be in the adapter method, not the witness encoder.)
func TestEncodeSettleChannelInsertWitness_ZeroPrice(t *testing.T) {
	witness := SettleChannelInsertWitness{
		ChannelID:         types.Hash{1},
		ContributionID:    types.Hash{2},
		PrincipalReturned: 100000,
		FeeCKB:            5000,
		PriceX64:          uint128.Uint128{}, // Zero price
	}

	// Should not panic or error (encoding should work; validation is upstream)
	encoded := EncodeSettleChannelInsertWitness(witness)
	if len(encoded) == 0 {
		t.Error("witness encoding should work even for zero price")
	}
}
