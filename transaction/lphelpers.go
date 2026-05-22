package transaction

import (
	"encoding/binary"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	ckbencoding "perun.network/perun-ckb-backend/encoding"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
)

const (
	opFundChannelExtract  byte = 0x43
	opSettleChannelInsert byte = 0x44

	witnessLenFundChannelExtract  = 73
	witnessLenSettleChannelInsert = 97
)

// BuildProxyChannelData encodes a minimal ChannelStatus molecule whose only
// populated field is the channel_id. The LP typescript scans for cells whose
// data decodes to a ChannelStatus with a matching channel_id; a proxy cell
// carrying this payload satisfies that lookup without invoking the real
// perun-channel-typescript.
func BuildProxyChannelData(channelHash types.Hash) []byte {
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

// EncodeFundChannelExtractWitness lays out the PoolWitness::FundChannelExtract
// payload exactly as expected by liquidity-pool-typescript (`perun-common/src/pool.rs`):
//
//	[0]        opcode 0x43
//	[1..33]    channel_id (32 B)
//	[33..65]   contribution_id (32 B)
//	[65..73]   extract_ckb (u64 LE)
func EncodeFundChannelExtractWitness(channelID, contributionID types.Hash, extractCKB uint64) []byte {
	buf := make([]byte, 0, witnessLenFundChannelExtract)
	buf = append(buf, opFundChannelExtract)
	buf = append(buf, channelID[:]...)
	buf = append(buf, contributionID[:]...)

	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], extractCKB)
	buf = append(buf, tmp[:]...)
	return buf
}

// EncodeSettleChannelInsertWitness lays out the PoolWitness::SettleChannelInsert
// payload:
//
//	[0]        opcode 0x44
//	[1..33]    channel_id (32 B)
//	[33..65]   contribution_id (32 B)
//	[65..73]   principal_returned (u64 LE)
//	[73..81]   fee_ckb (u64 LE)
//	[81..97]   price_x64 (u128 LE)
func EncodeSettleChannelInsertWitness(channelID, contributionID types.Hash, principal, feeCKB uint64, priceX64 uint128.Uint128) []byte {
	buf := make([]byte, 0, witnessLenSettleChannelInsert)
	buf = append(buf, opSettleChannelInsert)
	buf = append(buf, channelID[:]...)
	buf = append(buf, contributionID[:]...)

	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], principal)
	buf = append(buf, tmp[:]...)
	binary.LittleEndian.PutUint64(tmp[:], feeCKB)
	buf = append(buf, tmp[:]...)

	var tmp128 [16]byte
	uint128.StoreLittleEndian(tmp128[:], priceX64)
	buf = append(buf, tmp128[:]...)
	return buf
}
