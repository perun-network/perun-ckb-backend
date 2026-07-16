//go:build fundsudt

package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/nervosnetwork/ckb-sdk-go/v2/address"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/crypto/blake2b"
	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/channel/test"
	"perun.network/perun-ckb-backend/client"
	"perun.network/perun-ckb-backend/transaction"
	"perun.network/perun-ckb-backend/wallet"
	ckbaddress "perun.network/perun-ckb-backend/wallet/address"

	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

func ckbRPCURL() string {
	if u := os.Getenv("CKB_RPC_URL"); u != "" {
		return u
	}
	return "http://127.0.0.1:8114"
}

func dialRPC(url string) (rpc.Client, error) {
	var lastErr error
	deadline := time.Now().Add(30 * time.Second)
	for {
		cl, err := rpc.Dial(url)
		if err == nil {
			if _, perr := cl.GetTipBlockNumber(context.Background()); perr == nil {
				return cl, nil
			} else {
				lastErr = perr
			}
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("rpc not reachable at %s after 30s: %w", url, lastErr)
		}
		time.Sleep(2 * time.Second)
	}
}

func dialIndexer(url string) (indexer.Client, error) {
	var lastErr error
	deadline := time.Now().Add(30 * time.Second)
	for {
		cl, err := indexer.Dial(url)
		if err == nil {
			return cl, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("indexer dial failed at %s after 30s: %w", url, lastErr)
		}
		time.Sleep(2 * time.Second)
	}
}

func waitForTxCommitted(cl rpc.Client, h types.Hash, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		st, err := cl.GetTransaction(context.Background(), h)
		if err == nil && st != nil && st.TxStatus.Status == types.TransactionStatusCommitted {
			return nil
		}
		if time.Now().After(deadline) {
			status := ""
			if st != nil {
				status = string(st.TxStatus.Status)
			}
			return fmt.Errorf("tx %s did not commit within %s (last status=%q, err=%v)", h.String(), timeout, status, err)
		}
		time.Sleep(2 * time.Second)
	}
}

type WrappedTx struct {
	Transaction     *types.Transaction `json:"transaction"`
	MultisigConfigs map[string]any     `json:"multisig_configs"`
	Signatures      map[string]any     `json:"signatures"`
}

const devNetDir = "./devnet"

func iteratorsForDeployment(cl rpc.Client, deployment backend.Deployment, sender address.Address) (map[types.Hash]collector.CellIterator, error) {
	zeroHash := types.Hash{}
	iters := make(map[types.Hash]collector.CellIterator)
	// Iterator for the lockscript:
	senderString, err := sender.Encode()
	if err != nil {
		return nil, fmt.Errorf("encoding sender address: %w", err)
	}
	iter, err := collector.NewLiveCellIteratorFromAddress(cl, senderString)
	if err != nil {
		return nil, fmt.Errorf("creating cell iterator for default lockscript: %w", err)
	}
	// NOTE: This is to gather CKBytes.
	iters[zeroHash] = client.NewCKBOnlyIterator(iter)

	// Iterator for udts:
	for _, udt := range deployment.SUDTs {
		searchKey := &indexer.SearchKey{
			Script:           &udt,
			ScriptType:       types.ScriptTypeType,
			ScriptSearchMode: types.ScriptSearchModePrefix,
			Filter:           nil,
			WithData:         true,
		}
		sudtIter := collector.NewLiveCellIterator(cl, searchKey)
		iters[udt.Hash()] = sudtIter
	}
	return iters, nil
}

func padLE(b []byte, length int) []byte {
	padded := make([]byte, length)
	for i := range b {
		padded[length-1-i] = b[len(b)-1-i]
	}
	return padded
}

func parseLockArg(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "lock_arg:") {
			return strings.TrimSpace(strings.Split(line, ":")[1]), nil
		}
	}
	return "", fmt.Errorf("lock_arg not found in %s", path)
}

func main() {
	rpcURL := ckbRPCURL()
	rpcClient, err := dialRPC(rpcURL)
	if err != nil {
		log.Fatal(err)
	}
	sudtOwnerLockArg, err := test.ParseSUDTOwnerLockArg("accounts/sudt-owner-lock-hash.txt")
	if err != nil {
		log.Fatal("getting sudt owner args: ", err)
	}
	d, sudtInfo, err := test.GetDeployment("contract/migrations_0/dev/", "contract/migrations_1/dev/", "contract/migrations_vc/dev/", "system_scripts", sudtOwnerLockArg)
	if err != nil {
		log.Fatal("getting deployment: ", err)
	}
	privKey, err := test.GetKey("accounts/genesis-2.pk")
	if err != nil {
		log.Fatal("getting genesis key: ", err)
	}
	genAccount := wallet.NewAccountFromPrivateKey(privKey, types.Hash{}, true)
	genParticipant, _ := ckbaddress.NewDefaultParticipant(privKey.PubKey())
	addr, err := genParticipant.ToCKBAddress(types.NetworkTest).EncodeFullBech32m()

	fmt.Println("Genesis Address:", addr)
	signer := backend.NewSignerInstance(
		ckbaddress.AsParticipant(genAccount.Address()).ToCKBAddress(types.NetworkTest),
		*privKey, types.NetworkTest,
	)
	aliceArgHex, err := parseLockArg("accounts/alice.txt")
	if err != nil {
		log.Fatal("parsing alice: ", err)
	}
	bobArgHex, err := parseLockArg("accounts/bob.txt")
	if err != nil {
		log.Fatal("parsing bob: ", err)
	}
	ingridArgHex, err := parseLockArg("accounts/ingrid.txt")
	if err != nil {
		log.Fatal("parsing ingrid: ", err)
	}

	aliceArgs, _ := hex.DecodeString(strings.TrimPrefix(aliceArgHex, "0x"))
	bobArgs, _ := hex.DecodeString(strings.TrimPrefix(bobArgHex, "0x"))
	ingridArgs, _ := hex.DecodeString(strings.TrimPrefix(ingridArgHex, "0x"))

	iters, err := iteratorsForDeployment(rpcClient, d, signer.Address())
	if err != nil {
		log.Fatal(err)
	}

	builder, err := transaction.NewPerunTransactionBuilderWithDeployment(rpcClient, d, iters, signer.Address(), true)
	if err != nil {
		log.Fatal(err)
	}
	lockScript := &types.Script{
		CodeHash: genParticipant.UnlockScript.CodeHash, // secp256k1_blake160_sighash_all
		HashType: genParticipant.UnlockScript.HashType,
		Args:     genParticipant.UnlockScript.Args, // from public key
	}
	log.Printf("Expected args: %x", blake2b.Blake160(privKey.PubKey().SerializeCompressed()))
	log.Printf("Actual input script args: %x", lockScript.Args)
	indexerClient, err := dialIndexer(rpcURL)
	if err != nil {
		log.Fatal(err)
	}
	// Query CKB balance
	capacityResp, err := indexerClient.GetCellsCapacity(context.Background(), &indexer.SearchKey{
		Script:     lockScript,
		ScriptType: types.ScriptTypeLock,
	})
	if err != nil {
		log.Fatalf("failed to get CKB balance: %v", err)
	}
	ckbAmount := capacityResp.Capacity
	log.Println("CKB balance", ckbAmount)
	builder.AddCellDep(&d.DefaultLockScriptDep)
	builder.AddCellDep(sudtInfo.CellDep)
	outputrange := [2]uint64{1, 129}
	searchKey := &indexer.SearchKey{
		Script: &types.Script{
			CodeHash: sudtInfo.Script.CodeHash,
			HashType: types.HashTypeData1,
			Args:     sudtInfo.Script.Args,
		},
		WithData:   true,
		ScriptType: types.ScriptTypeType,
		Filter: &indexer.Filter{
			Script:             lockScript,
			OutputDataLenRange: &outputrange,
		},
	}
	log.Println("SUDT: ", sudtInfo.CellDep, "CodeHash:", sudtInfo.Script.CodeHash, "Hash:", sudtInfo.Script.Hash(), "TypeArgs:", hex.EncodeToString(sudtInfo.Script.Args))
	genAddr, err := ckbaddress.AsParticipant(genAccount.Address()).ToCKBAddress(types.NetworkTest).EncodeFullBech32m()
	log.Println("Account: ", genAddr, "Searching for cells with SUDT type script:", sudtInfo.Script.CodeHash, "and lock script:", signer.Addr.Script.CodeHash, "and hash:", signer.Addr.Script.Hash(), "and type args:", hex.EncodeToString(sudtInfo.Script.Args))
	var inputAmount uint64 = 0
	var required uint64 = 200_000_000
	cursor := ""

	for {
		resp, err := indexerClient.GetCells(context.Background(), searchKey, indexer.SearchOrderAsc, 500, cursor)
		if err != nil {
			log.Fatal("GetCells failed:", err)
		}
		log.Println("Found cells:", len(resp.Objects), "with cursor:", resp.LastCursor)
		for _, cell := range resp.Objects {
			input := &types.CellInput{
				PreviousOutput: cell.OutPoint,
				Since:          0,
			}
			builder.AddInput(input)
			inputAmount += binary.LittleEndian.Uint64(cell.OutputData[:8])
			if inputAmount >= required {
				break
			}
		}
		if inputAmount >= required || resp.LastCursor == cursor || len(resp.Objects) == 0 {
			break
		}
		cursor = resp.LastCursor
	}

	if inputAmount < required {
		log.Fatalf("insufficient SUDT balance: have %d, need %d", inputAmount, required)
	}

	// SUDT outputs for Alice and Bob
	for _, args := range [][]byte{aliceArgs, bobArgs, ingridArgs} {
		lock := &types.Script{
			CodeHash: d.OmniLockScript.CodeHash,
			HashType: types.HashTypeType,
			Args:     args,
		}

		output := &types.CellOutput{
			Capacity: 204 * 100_000_000,
			Lock:     lock,
			Type:     sudtInfo.Script,
		}

		totalAmount := inputAmount
		eachAmount := totalAmount / 3
		buf := make([]byte, 16)
		binary.LittleEndian.PutUint64(buf, eachAmount)
		builder.AddOutput(output, buf)
	}

	tx, err := builder.Build()
	if err != nil {
		log.Fatal("build error:", err)
	}

	log.Println("Transaction built successfully, tx:", tx.TxView.Witnesses, tx.TxView.Inputs)
	index := tx.ScriptGroups[0].InputIndices[0]
	lock := [65]byte{}
	err = builder.SetWitness(uint(index), types.WitnessTypeLock, lock[:])
	if err != nil {
		log.Fatal("could not att witness:", err)
	}
	_, err = signer.SignTransaction(tx)
	if err != nil {
		log.Fatal("signing error:", err)
	}
	log.Println("Transaction signed successfully, tx:", tx.TxView.Witnesses, tx.TxView.Inputs)

	txHash, err := rpcClient.SendTransaction(context.Background(), tx.TxView)
	if err != nil {
		log.Fatal("send tx error:", err)
	}

	fmt.Println("📦 Transaction successfully sent!")
	fmt.Println("🔑 Genesis Signer Address:", addr)
	fmt.Println("🔢 Transaction Hash:", txHash.String())
	fmt.Println("🔍 You can query it with:")
	fmt.Printf("ckb-cli rpc get_transaction --hash %s\n", txHash.String())

	if err := waitForTxCommitted(rpcClient, *txHash, 180*time.Second); err != nil {
		log.Fatal("waiting for commit: ", err)
	}
	fmt.Println("✅ SUDT distribution tx committed")
}
