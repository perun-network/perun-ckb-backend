package ckblp

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nervosnetwork/ckb-sdk-go/v2/crypto/blake2b"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	ckbencoding "perun.network/perun-ckb-backend/encoding"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
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

func isLPOrchestrationEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("PERUN_LP_ORCHESTRATION")))
	return value == "1" || value == "true" || value == "yes"
}

func deriveContributionID(tag string, channelHash types.Hash, amount uint64, fee uint64) types.Hash {
	buf := make([]byte, 0, len(tag)+32+16)
	buf = append(buf, []byte(tag)...)
	buf = append(buf, channelHash[:]...)

	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], amount)
	buf = append(buf, tmp[:]...)
	binary.LittleEndian.PutUint64(tmp[:], fee)
	buf = append(buf, tmp[:]...)

	hashBytes := blake2b.Blake256(buf)
	var out types.Hash
	copy(out[:], hashBytes)
	return out
}

func buildProxyChannelData(channelHash types.Hash) []byte {
	state := molecule.NewChannelStateBuilder().
		ChannelId(*molecule2.PackByte32(channelHash)).
		Version(*types.PackUint64(0)).
		IsFinal(ckbencoding.False).
		Build()
	status := molecule.NewChannelStatusBuilder().
		State(state).
		Funded(ckbencoding.False).
		Disputed(ckbencoding.False).
		VcDisputed(ckbencoding.False).
		VctsHash(molecule.Byte32Default()).
		Build()
	return status.AsSlice()
}
