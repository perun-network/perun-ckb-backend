package test

import (
	"encoding/hex"
	"fmt"
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
	if len(lines) != 2 {
		return nil, fmt.Errorf("key file must contain exactly two lines")
	}
	x := strings.Trim(lines[0], " \n")
	xBytes, err := hex.DecodeString(x)
	if err != nil {
		return nil, err
	}
	return secp256k1.PrivKeyFromBytes(xBytes), nil
}
