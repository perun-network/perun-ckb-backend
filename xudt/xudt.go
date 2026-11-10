// Package xudt implements the transfer encodings from CKB RFC 0052.
package xudt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"

	"github.com/nervosnetwork/ckb-sdk-go/v2/crypto/blake2b"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

// Config describes one exact xUDT type script. Extensions are application code:
// callers must verify their rules permit channel funding and unilateral exits.
// Callbacks must be deterministic and safe for concurrent use. They must not
// mutate their arguments. OutputData returns the bytes AFTER the token amount.
// WitnessData is invoked after balancing, before signing, once per type group.
// It returns one entry per extension in ExtensionScripts, in the same order.
type Config struct {
	Script           types.Script                                                            `json:"script"`
	CellDeps         []types.CellDep                                                         `json:"cell_deps"`
	ExtensionScripts []types.Script                                                          `json:"extension_scripts,omitempty"`
	OutputData       func(output types.CellOutput, amount *big.Int) ([]byte, error)          `json:"-"`
	WitnessData      func(tx *types.Transaction, inputs, outputs []uint32) ([][]byte, error) `json:"-"`
}

func (c Config) Validate() error {
	if _, err := types.SerializeHashTypeByte(c.Script.HashType); err != nil {
		return err
	}
	if len(c.CellDeps) == 0 {
		return fmt.Errorf("xUDT requires its script cell dependency")
	}
	for _, dep := range c.CellDeps {
		if dep.OutPoint == nil {
			return fmt.Errorf("xUDT cell dependency has no outpoint")
		}
		if dep.DepType != types.DepTypeCode && dep.DepType != types.DepTypeDepGroup {
			return fmt.Errorf("invalid cell dependency type")
		}
	}
	mode, err := c.Mode()
	if err != nil {
		return err
	}
	for i, script := range c.ExtensionScripts {
		if _, err := types.SerializeHashTypeByte(script.HashType); err != nil {
			return fmt.Errorf("extension %d: %w", i, err)
		}
	}
	scripts := EncodeScriptVec(c.ExtensionScripts)
	switch mode {
	case 0:
		if len(c.ExtensionScripts) != 0 {
			return fmt.Errorf("extensions supplied for plain xUDT")
		}
	case 1:
		if !bytes.Equal(c.Script.Args[36:], scripts) {
			return fmt.Errorf("inline extension scripts do not match type args")
		}
	case 2:
		if !bytes.Equal(c.Script.Args[36:], blake2b.Blake160(scripts)) {
			return fmt.Errorf("extension script hash does not match type args")
		}
	}
	if len(c.ExtensionScripts) > 0 && c.WitnessData == nil {
		return fmt.Errorf("xUDT extensions require an explicit witness provider")
	}
	return nil
}

// Mode validates the args envelope and returns the extension encoding mode.
// Owner flags in the top three bits are retained verbatim in the type script.
func (c Config) Mode() (uint32, error) {
	args := c.Script.Args
	if len(args) == 32 {
		return 0, nil
	}
	if len(args) < 36 {
		return 0, fmt.Errorf("xUDT args must contain 32 owner bytes and optionally 4 flag bytes")
	}
	mode := binary.LittleEndian.Uint32(args[32:36]) & 0x1fffffff
	switch mode {
	case 0:
		if len(args) != 36 {
			return 0, fmt.Errorf("plain xUDT has unexpected extension args")
		}
	case 1:
		if len(args) < 40 {
			return 0, fmt.Errorf("missing inline ScriptVec")
		}
	case 2:
		if len(args) != 56 {
			return 0, fmt.Errorf("hashed xUDT extensions require a 20-byte hash")
		}
	default:
		return 0, fmt.Errorf("unsupported xUDT extension flags %#x", mode)
	}
	return mode, nil
}

// EncodeScriptVec encodes a Molecule dynvec of CKB scripts.
func EncodeScriptVec(scripts []types.Script) []byte {
	fields := make([][]byte, len(scripts))
	for i := range scripts {
		fields[i] = scripts[i].Serialize()
	}
	return encodeSequence(fields)
}

// EncodeData encodes optional cell metadata (excluding the 16-byte amount).
func EncodeData(lock []byte, data [][]byte) []byte {
	return encodeSequence([][]byte{encodeBytes(lock), encodeBytesVec(data)})
}

// EncodeWitness encodes a transfer witness. Owner authorization is deliberately
// absent: payment channel transactions conserve tokens and never mint them.
func (c Config) EncodeWitness(data [][]byte) ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if len(data) != len(c.ExtensionScripts) {
		return nil, fmt.Errorf("expected %d extension witness entries, got %d", len(c.ExtensionScripts), len(data))
	}
	mode, _ := c.Mode()
	var scripts []byte
	if mode == 2 {
		scripts = EncodeScriptVec(c.ExtensionScripts)
	}
	return encodeSequence([][]byte{nil, nil, scripts, encodeBytesVec(data)}), nil
}

// ValidateData checks optional cell metadata against the extension count.
func (c Config) ValidateData(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	fields, err := decodeSequence(data)
	if err != nil || len(fields) != 2 {
		return fmt.Errorf("invalid XudtData table")
	}
	if err := validateBytes(fields[0]); err != nil {
		return err
	}
	entries, err := decodeSequence(fields[1])
	if err != nil {
		return err
	}
	if len(entries) != len(c.ExtensionScripts) {
		return fmt.Errorf("xUDT cell data extension count mismatch")
	}
	for _, entry := range entries {
		if err := validateBytes(entry); err != nil {
			return err
		}
	}
	return nil
}

func encodeBytes(b []byte) []byte {
	out := make([]byte, 4+len(b))
	binary.LittleEndian.PutUint32(out, uint32(len(b)))
	copy(out[4:], b)
	return out
}
func encodeBytesVec(data [][]byte) []byte {
	fields := make([][]byte, len(data))
	for i := range data {
		fields[i] = encodeBytes(data[i])
	}
	return encodeSequence(fields)
}
func encodeSequence(fields [][]byte) []byte {
	if len(fields) == 0 {
		return []byte{4, 0, 0, 0}
	}
	size := 4 * (len(fields) + 1)
	for _, field := range fields {
		size += len(field)
	}
	if uint64(size) > math.MaxUint32 {
		panic("Molecule encoding exceeds uint32")
	}
	out := make([]byte, size)
	binary.LittleEndian.PutUint32(out, uint32(size))
	offset := 4 * (len(fields) + 1)
	for i, field := range fields {
		binary.LittleEndian.PutUint32(out[4*(i+1):], uint32(offset))
		copy(out[offset:], field)
		offset += len(field)
	}
	return out
}
func decodeSequence(data []byte) ([][]byte, error) {
	if len(data) < 4 || uint64(binary.LittleEndian.Uint32(data)) != uint64(len(data)) {
		return nil, fmt.Errorf("invalid Molecule size")
	}
	if len(data) == 4 {
		return nil, nil
	}
	if len(data) < 8 {
		return nil, fmt.Errorf("invalid Molecule header")
	}
	first := uint64(binary.LittleEndian.Uint32(data[4:]))
	if first < 8 || first%4 != 0 || first > uint64(len(data)) {
		return nil, fmt.Errorf("invalid Molecule offset")
	}
	count := int(first/4) - 1
	fields := make([][]byte, count)
	for i := range fields {
		start := uint64(binary.LittleEndian.Uint32(data[4*(i+1):]))
		end := uint64(len(data))
		if i+1 < count {
			end = uint64(binary.LittleEndian.Uint32(data[4*(i+2):]))
		}
		if start < first || end < start || end > uint64(len(data)) {
			return nil, fmt.Errorf("invalid Molecule field offset")
		}
		fields[i] = data[start:end]
	}
	return fields, nil
}
func validateBytes(data []byte) error {
	if len(data) < 4 || uint64(binary.LittleEndian.Uint32(data))+4 != uint64(len(data)) {
		return fmt.Errorf("invalid Molecule bytes")
	}
	return nil
}
