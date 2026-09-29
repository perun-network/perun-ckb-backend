package ckblp

import (
	"encoding/binary"
)

// LP-deposit / LP-withdraw witness encoders live here. The
// fund-channel-extract and settle-channel-insert witnesses are now built by
// the transaction package (transaction/lphelpers.go) since those operations
// are dispatched through PerunTransactionBuilder.

const (
	opLPDeposit  = 0x41
	opLPWithdraw = 0x42

	witnessLenLPDeposit  = 1
	witnessLenLPWithdraw = 9
)

// EncodeLPDepositWitness encodes an LPDeposit witness payload.
func EncodeLPDepositWitness() []byte {
	buf := make([]byte, 0, witnessLenLPDeposit)
	buf = append(buf, byte(opLPDeposit))
	return buf
}

// EncodeLPWithdrawWitness encodes an LPWithdraw witness payload.
func EncodeLPWithdrawWitness(ckbOut uint64) []byte {
	buf := make([]byte, 0, witnessLenLPWithdraw)
	buf = append(buf, byte(opLPWithdraw))
	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], ckbOut)
	buf = append(buf, tmp[:]...)
	return buf
}
