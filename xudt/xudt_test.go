package xudt

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/nervosnetwork/ckb-sdk-go/v2/crypto/blake2b"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
)

func testConfig(mode uint32) Config {
	extension := types.Script{CodeHash: types.HexToHash("0x01"), HashType: types.HashTypeData1, Args: []byte{7}}
	c := Config{Script: types.Script{HashType: types.HashTypeData1, Args: make([]byte, 36)}, CellDeps: []types.CellDep{{OutPoint: &types.OutPoint{}, DepType: types.DepTypeCode}}}
	binary.LittleEndian.PutUint32(c.Script.Args[32:], mode)
	if mode > 0 {
		c.ExtensionScripts = []types.Script{extension}
		c.WitnessData = func(*types.Transaction, []uint32, []uint32) ([][]byte, error) { return [][]byte{nil}, nil }
		raw := EncodeScriptVec(c.ExtensionScripts)
		if mode == 1 {
			c.Script.Args = append(c.Script.Args, raw...)
		} else {
			c.Script.Args = append(c.Script.Args, blake2b.Blake160(raw)...)
		}
	}
	return c
}

func TestRFC0052Encodings(t *testing.T) {
	// Independent literal fixture: four-field table, three absent options and
	// an empty dynvec (total 24, all offsets 20).
	plain := testConfig(0)
	witness, err := plain.EncodeWitness(nil)
	require.NoError(t, err)
	require.Equal(t, "180000001400000014000000140000001400000004000000", hex.EncodeToString(witness))
	require.Equal(t, "140000000c000000100000000000000004000000", hex.EncodeToString(EncodeData(nil, nil)))
	for _, mode := range []uint32{1, 2} {
		c := testConfig(mode)
		require.NoError(t, c.Validate())
		w, err := c.EncodeWitness([][]byte{[]byte("proof")})
		require.NoError(t, err)
		fields, err := decodeSequence(w)
		require.NoError(t, err)
		require.Len(t, fields, 4)
		require.Empty(t, fields[0])
		require.Empty(t, fields[1])
		if mode == 1 {
			require.Empty(t, fields[2])
		} else {
			require.Equal(t, EncodeScriptVec(c.ExtensionScripts), fields[2])
		}
		entries, err := decodeSequence(fields[3])
		require.NoError(t, err)
		require.Equal(t, []byte("proof"), entries[0][4:])
		data := EncodeData([]byte("lock metadata"), [][]byte{[]byte("cell metadata")})
		require.NoError(t, c.ValidateData(data))
		require.Error(t, c.ValidateData(EncodeData(nil, nil)))
		_, err = c.EncodeWitness(nil)
		require.Error(t, err)
		c.Script.Args[len(c.Script.Args)-1] ^= 1
		require.Error(t, c.Validate())
	}
}

func TestArgsValidation(t *testing.T) {
	for n := 0; n < 36; n++ {
		c := testConfig(0)
		c.Script.Args = make([]byte, n)
		if n == 32 {
			require.NoError(t, c.Validate())
		} else {
			require.Error(t, c.Validate())
		}
	}
	for _, flags := range []uint32{0x20000000, 0x40000000, 0x80000000, 0xe0000000} {
		c := testConfig(0)
		binary.LittleEndian.PutUint32(c.Script.Args[32:], flags)
		require.NoError(t, c.Validate())
	}
	c := testConfig(0)
	binary.LittleEndian.PutUint32(c.Script.Args[32:], 3)
	require.Error(t, c.Validate())
	c = testConfig(1)
	c.WitnessData = nil
	require.Error(t, c.Validate())
	c = testConfig(0)
	c.CellDeps = nil
	require.Error(t, c.Validate())
	c = testConfig(0)
	c.CellDeps[0].OutPoint = nil
	require.Error(t, c.Validate())
}

func FuzzValidateData(f *testing.F) {
	f.Add([]byte{})
	f.Add(EncodeData(nil, [][]byte{nil}))
	f.Add(bytes.Repeat([]byte{255}, 16))
	c := testConfig(1)
	f.Fuzz(func(t *testing.T, data []byte) { _ = c.ValidateData(data) })
}
