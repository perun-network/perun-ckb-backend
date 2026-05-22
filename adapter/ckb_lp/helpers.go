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
