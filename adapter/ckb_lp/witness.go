package ckblp

import (
	"encoding/binary"

	"github.com/Pilatuz/bigz/uint128"
)

const (
	opLPDeposit           = 0x41
	opLPWithdraw          = 0x42
	opFundChannelExtract  = 0x43
	opSettleChannelInsert = 0x44
	opCancelReservation   = 0x45
	opRotateOperator      = 0x46
)

const (
	witnessLenFundChannelExtract  = 73
	witnessLenSettleChannelInsert = 97
)

// EncodeFundChannelExtractWitness encodes a FundChannelExtract witness payload.
func EncodeFundChannelExtractWitness(w FundChannelExtractWitness) []byte {
	buf := make([]byte, 0, witnessLenFundChannelExtract)
	buf = append(buf, byte(opFundChannelExtract))
	buf = append(buf, w.ChannelID[:]...)
	buf = append(buf, w.ContributionID[:]...)

	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], w.ExtractCKB)
	buf = append(buf, tmp[:]...)
	return buf
}

// EncodeSettleChannelInsertWitness encodes a SettleChannelInsert witness payload.
func EncodeSettleChannelInsertWitness(w SettleChannelInsertWitness) []byte {
	buf := make([]byte, 0, witnessLenSettleChannelInsert)
	buf = append(buf, byte(opSettleChannelInsert))
	buf = append(buf, w.ChannelID[:]...)
	buf = append(buf, w.ContributionID[:]...)

	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], w.PrincipalReturned)
	buf = append(buf, tmp[:]...)
	binary.LittleEndian.PutUint64(tmp[:], w.FeeCKB)
	buf = append(buf, tmp[:]...)

	var tmp128 [16]byte
	uint128.StoreLittleEndian(tmp128[:], w.PriceX64)
	buf = append(buf, tmp128[:]...)
	return buf
}
