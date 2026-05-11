package test

import (
	"encoding/hex"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

func GetKey(path string) (*secp256k1.PrivateKey, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Generate a new private key
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
		privKey, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}
		// Write it to the file in hex format
		hexKey := hex.EncodeToString(privKey.Serialize())
		err = os.WriteFile(path, []byte(hexKey+"\n"), 0600)
		if err != nil {
			return nil, err
		}
		log.Println("Generated new private key and saved to", path)
		return privKey, nil
	}
	keyFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = keyFile.Close() }()

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
