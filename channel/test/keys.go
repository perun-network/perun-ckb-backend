package test

import (
	"encoding/hex"
	"io"
	"os"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

func GetKey(path string) (*secp256k1.PrivateKey, error) {

	keyFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer keyFile.Close()

	rawBytes, err := io.ReadAll(keyFile)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(rawBytes), "\n")
	if len(lines) == 2 {
		keyStr := strings.Trim(lines[0], " \n")
		if strings.HasPrefix(keyStr, "0x") || strings.HasPrefix(keyStr, "0X") {
			keyStr = keyStr[2:]
		}
		xBytes, err := hex.DecodeString(keyStr)
		if err != nil {
			return nil, err
		}
		return secp256k1.PrivKeyFromBytes(xBytes), nil
	} else {
		keyStr := lines[0]
		if strings.HasPrefix(keyStr, "0x") || strings.HasPrefix(keyStr, "0X") {
			keyStr = keyStr[2:]
		}
		xBytes, err := hex.DecodeString(keyStr)
		if err != nil {
			return nil, err
		}
		return secp256k1.PrivKeyFromBytes(xBytes), nil
	}
}
