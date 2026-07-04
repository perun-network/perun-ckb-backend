package ckblp

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

func parseHash32(value string) (types.Hash, error) {
	trimmed := strings.TrimPrefix(strings.ToLower(value), "0x")
	if len(trimmed) != 64 {
		return types.Hash{}, ErrInvalidLPCellArg
	}
	decoded, err := hex.DecodeString(trimmed)
	if err != nil || len(decoded) != 32 {
		return types.Hash{}, ErrInvalidLPCellArg
	}
	var hash types.Hash
	copy(hash[:], decoded)
	return hash, nil
}

func parseOutPoint(value string) (*types.OutPoint, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return nil, ErrInvalidLPCellArg
	}
	hash, err := parseHash32(parts[0])
	if err != nil {
		return nil, err
	}
	idx, err := strconv.ParseUint(parts[1], 0, 32)
	if err != nil {
		return nil, ErrInvalidLPCellArg
	}
	return &types.OutPoint{TxHash: hash, Index: uint32(idx)}, nil
}

func outPointKey(outPoint *types.OutPoint) string {
	return fmt.Sprintf("%s:%d", outPoint.TxHash.String(), outPoint.Index)
}

func isZeroHash(hash types.Hash) bool {
	return hash == (types.Hash{})
}

// updatedLPCellCapacity returns the rebuilt LP cell's capacity after taking
// take shannons out of a live cell with capacity current. It enforces CKB's
// occupied-capacity rule for the LP cell shape: the remainder must stay at or
// above MinLPCellOccupiedShannons, or the verifier rejects the transaction
// with InsufficientCellCapacity. Both violations are deterministic — retrying
// the same transaction can never succeed.
func updatedLPCellCapacity(current, take uint64) (uint64, error) {
	if current < take {
		return 0, Deterministic(ErrInvalidLPCellArg)
	}
	remaining := current - take
	if remaining < MinLPCellOccupiedShannons {
		return 0, Deterministic(ErrInsufficientLPCellCapacity)
	}
	return remaining, nil
}
