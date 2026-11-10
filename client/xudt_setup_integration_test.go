//go:build xudt_integration

package client_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nervosnetwork/ckb-sdk-go/v2/crypto/blake2b"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	ckbsigner "github.com/nervosnetwork/ckb-sdk-go/v2/transaction/signer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
	"perun.network/perun-ckb-backend/backend"
	btest "perun.network/perun-ckb-backend/channel/test"
	"perun.network/perun-ckb-backend/wallet"
	ckbaddress "perun.network/perun-ckb-backend/wallet/address"
	"perun.network/perun-ckb-backend/xudt"
)

type xudtFixture struct {
	Deployment backend.Deployment
	IssuerKey  *secp256k1.PrivateKey
	Keys       []*secp256k1.PrivateKey
	Tokens     []xudt.Config
}

// Use the same deployment, system scripts and disposable accounts as the
// existing client tests. Only the test extension needs an additional deployment.
func newXUDTFixture(t *testing.T) (xudtFixture, rpc.Client) {
	t.Helper()
	const devnet = "../devnet"
	owner, err := btest.ParseSUDTOwnerLockArg(devnet + "/accounts/sudt-owner-lock-hash.txt")
	require.NoError(t, err)
	deployment, _, err := btest.GetDeployment(
		devnet+"/contract/migrations_0/dev", devnet+"/contract/migrations_1/dev",
		devnet+"/contract/migrations_vc/dev", devnet+"/system_scripts", owner)
	require.NoError(t, err)
	system, err := btest.GetSystemScripts(devnet + "/system_scripts")
	require.NoError(t, err)
	issuer, err := btest.GetKey(devnet + "/accounts/genesis-2.pk")
	require.NoError(t, err)
	participant, err := ckbaddress.NewDefaultParticipant(issuer.PubKey())
	require.NoError(t, err)
	ownerHash := participant.PaymentScript.Hash()
	f := xudtFixture{Deployment: deployment, IssuerKey: issuer}
	for _, name := range []string{"alice", "bob"} {
		key, err := btest.GetKey(devnet + "/accounts/" + name + ".pk")
		require.NoError(t, err)
		f.Keys = append(f.Keys, key)
	}
	// ckb-cli deploy writes the same migration format used by GetDeployment.
	files, err := filepath.Glob(devnet + "/contract/migrations_xudt/dev/*.json")
	require.NoError(t, err)
	require.Len(t, files, 1, "deploy the Rust extension with PERUN_XUDT_TESTS=1 ./deploy_contracts.sh")
	raw, err := os.ReadFile(files[0])
	require.NoError(t, err)
	var migration btest.Migration
	require.NoError(t, json.Unmarshal(raw, &migration))
	require.Len(t, migration.CellRecipes, 1)
	extension := migration.CellRecipes[0]
	require.Equal(t, "xudt-extension", extension.Name)
	extensionScript := types.Script{CodeHash: types.HexToHash(extension.DataHash), HashType: types.HashTypeData1, Args: []byte{42}}
	extensionDep := types.CellDep{OutPoint: &types.OutPoint{TxHash: types.HexToHash(extension.TxHash), Index: extension.Index}, DepType: types.DepTypeCode}
	vector := xudt.EncodeScriptVec([]types.Script{extensionScript})
	for mode := uint32(0); mode < 3; mode++ {
		args := make([]byte, 36)
		copy(args, ownerHash[:])
		binary.LittleEndian.PutUint32(args[32:], mode)
		cfg := xudt.Config{Script: types.Script{CodeHash: system.XUdt.CodeHash, HashType: system.XUdt.HashType}}
		for _, dep := range system.XUdt.CellDeps {
			cfg.CellDeps = append(cfg.CellDeps, types.CellDep{OutPoint: &types.OutPoint{TxHash: dep.CellDep.OutPoint.TxHash, Index: dep.CellDep.OutPoint.Index}, DepType: dep.CellDep.DepType})
		}
		if mode > 0 {
			cfg.ExtensionScripts = []types.Script{extensionScript}
			cfg.WitnessData = func(*types.Transaction, []uint32, []uint32) ([][]byte, error) {
				return [][]byte{[]byte("channel-proof")}, nil
			}
			cfg.OutputData = func(types.CellOutput, *big.Int) ([]byte, error) {
				return xudt.EncodeData(nil, [][]byte{[]byte("channel-data")}), nil
			}
			cfg.CellDeps = append(cfg.CellDeps, extensionDep)
			if mode == 1 {
				args = append(args, vector...)
			} else {
				args = append(args, blake2b.Blake160(vector)...)
			}
		}
		cfg.Script.Args = args
		f.Tokens = append(f.Tokens, cfg)
	}
	cl, err := rpc.Dial(btest.DevnetRpcNodeURL)
	require.NoError(t, err)
	t.Cleanup(cl.Close)
	seedXUDT(t, f, cl)
	return f, cl
}

// Construct both supported wallet locks in one place for lifecycle and transfer
// coverage. Each invocation returns independent accounts and signers.
func (f xudtFixture) wallets(t *testing.T, omni bool) ([]*wallet.Account, []backend.Signer) {
	t.Helper()
	var accounts []*wallet.Account
	var signers []backend.Signer
	for _, key := range f.Keys {
		account := wallet.NewAccountFromPrivateKey(key, f.Deployment.OmniLockScript.CodeHash, !omni)
		accounts = append(accounts, account)
		participant := account.Address().(*ckbaddress.Participant)
		addr := participant.ToCKBAddress(types.NetworkTest)
		if omni {
			_, auth, err := ckbaddress.NewEthereumParticipantFromPublicKey(key.PubKey(), f.Deployment.OmniLockScript.CodeHash)
			require.NoError(t, err)
			signer := backend.NewEVMSignerInstance(addr, *key, types.NetworkTest, auth)
			txSigner := signer.Signer()
			txSigner.RegisterLockSigner(f.Deployment.OmniLockScript.CodeHash, &ckbsigner.OmnilockSigner{})
			signers = append(signers, signer)
		} else {
			signers = append(signers, backend.NewSignerInstance(addr, *key, types.NetworkTest))
		}
	}
	return accounts, signers
}

// A committed transaction can briefly precede indexer visibility. Wait for the
// indexed block before selecting the next input or checking token balances.
func waitForXUDTTransaction(t *testing.T, ctx context.Context, cl rpc.Client, hash types.Hash) {
	t.Helper()
	require.Eventually(t, func() bool {
		tx, err := cl.GetTransaction(ctx, hash)
		if err != nil || tx == nil || tx.TxStatus == nil || tx.TxStatus.Status != types.TransactionStatusCommitted || tx.TxStatus.BlockHash == nil {
			return false
		}
		header, err := cl.GetHeader(ctx, *tx.TxStatus.BlockHash)
		if err != nil {
			return false
		}
		tip, err := cl.GetIndexerTip(ctx)
		return err == nil && tip != nil && tip.BlockNumber >= header.Number
	}, 45*time.Second, 200*time.Millisecond)
}
