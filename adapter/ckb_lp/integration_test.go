//go:build devnet
// +build devnet

package ckblp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Pilatuz/bigz/uint128"
	ckbbuilder "github.com/nervosnetwork/ckb-sdk-go/v2/collector/builder"
	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
	"perun.network/perun-ckb-backend/backend"
	ckbtest "perun.network/perun-ckb-backend/channel/test"
	"perun.network/perun-ckb-backend/transaction"
	ckbaddress "perun.network/perun-ckb-backend/wallet/address"
)

type lpMigration struct {
	CellRecipes []struct {
		Name     string `json:"name"`
		TxHash   string `json:"tx_hash"`
		Index    uint32 `json:"index"`
		DataHash string `json:"data_hash"`
	} `json:"cell_recipes"`
}

type lpCellSpec struct {
	PoolID        string `json:"pool_id"`
	OwnerLockHash string `json:"owner_lock_hash"`
	OperatorLock  string `json:"operator_lock_hash"`
	AvailableCKB  uint64 `json:"available_ckb"`
	ReservedCKB   uint64 `json:"reserved_ckb"`
	FeesEarnedCKB uint64 `json:"cumulative_fees_earned_ckb"`
	Policy        struct {
		MaxTradingVolume uint64 `json:"max_trading_volume"`
		FeeRateBps       uint32 `json:"fee_rate_bps"`
		PolicyFlags      uint32 `json:"policy_flags"`
		PolicyVersion    uint32 `json:"policy_version"`
	} `json:"policy"`
	Nonce  uint64 `json:"nonce"`
	Active bool   `json:"active"`
}

func TestDiscoverLPCellsDevnet(t *testing.T) {
	t.Skip("devnet E2E test: requires proper transaction signing setup")
	operatorLockHash := getOperatorLockHash(t)
	lpDeployment, ok := loadLPDeploymentFromDevnet(t)
	if !ok {
		return
	}

	rpcClient, err := rpc.Dial(ckbtest.DevnetRpcNodeURL)
	require.NoError(t, err)
	ensureLPDeploymentOnChain(t, rpcClient, lpDeployment)

	operatorHash := mustParseHash32(t, operatorLockHash)
	adapter := NewAdapter(rpcClient, nil, nil, backend.Deployment{}, lpDeployment)
	cells, err := adapter.DiscoverLPCells(context.Background(), operatorHash)
	require.NoError(t, err)
	if len(cells) == 0 {
		lpCell, ok := loadLPCellSpecFromDevnet(t)
		if !ok {
			return
		}
		deployment := loadDevnetDeployment(t)
		signer := newBobSigner(t, deployment.Network)
		transactor := backend.NewRPCTransactor(rpcClient, signer)
		_, err := buildAndSubmitLPDeposit(context.Background(), rpcClient, signer, transactor, deployment, lpDeployment, lpCell)
		require.NoError(t, err)

		cells, err = adapter.DiscoverLPCells(context.Background(), operatorHash)
		require.NoError(t, err)
	}
	require.NotEmpty(t, cells)
}

func TestGetLPCellDevnet(t *testing.T) {
	t.Skip("devnet E2E test: requires proper transaction signing setup")
	lpCellID := os.Getenv("PERUN_LP_CELL_ID")
	poolID := os.Getenv("PERUN_LP_POOL_ID")
	lpDeployment, ok := loadLPDeploymentFromDevnet(t)
	if !ok {
		return
	}

	rpcClient, err := rpc.Dial(ckbtest.DevnetRpcNodeURL)
	require.NoError(t, err)
	ensureLPDeploymentOnChain(t, rpcClient, lpDeployment)

	adapter := NewAdapter(rpcClient, nil, nil, backend.Deployment{}, lpDeployment)
	if lpCellID == "" {
		operatorLockHash := getOperatorLockHash(t)
		operatorHash := mustParseHash32(t, operatorLockHash)
		cells, err := adapter.DiscoverLPCells(context.Background(), operatorHash)
		require.NoError(t, err)
		if len(cells) == 0 {
			lpCell, ok := loadLPCellSpecFromDevnet(t)
			if !ok {
				return
			}
			deployment := loadDevnetDeployment(t)
			signer := newBobSigner(t, deployment.Network)
			transactor := backend.NewRPCTransactor(rpcClient, signer)
			lpCellID, err = buildAndSubmitLPDeposit(context.Background(), rpcClient, signer, transactor, deployment, lpDeployment, lpCell)
			require.NoError(t, err)
		} else {
			lpCellID = cells[0].OutPointHex
		}
	}
	info, err := adapter.GetLPCell(context.Background(), lpCellID)
	require.NoError(t, err)

	if poolID != "" {
		expected := mustParseHash32(t, poolID)
		require.Equal(t, expected, info.Cell.PoolID)
	}
}

func TestBobCreatesLPCellAndWithdrawDevnet(t *testing.T) {
	t.Skip("devnet E2E test: requires proper transaction signing setup")
	lpDeployment, ok := loadLPDeploymentFromDevnet(t)
	if !ok {
		return
	}

	lpCell, ok := loadLPCellSpecFromDevnet(t)
	if !ok {
		return
	}

	deployment := loadDevnetDeployment(t)

	rpcClient, err := rpc.Dial(ckbtest.DevnetRpcNodeURL)
	require.NoError(t, err)
	ensureLPDeploymentOnChain(t, rpcClient, lpDeployment)

	signer := newBobSigner(t, deployment.Network)
	transactor := backend.NewRPCTransactor(rpcClient, signer)

	ctx := context.Background()

	lpCellID, err := buildAndSubmitLPDeposit(ctx, rpcClient, signer, transactor, deployment, lpDeployment, lpCell)
	require.NoError(t, err)

	adapter := NewAdapter(rpcClient, signer, transactor, deployment, lpDeployment)
	info, err := adapter.GetLPCell(ctx, lpCellID)
	require.NoError(t, err)

	withdrawAmount := uint64(100_000_000)
	if withdrawAmount > info.Cell.AvailableCKB {
		withdrawAmount = info.Cell.AvailableCKB
	}
	require.NotZero(t, withdrawAmount)

	_, err = buildAndSubmitLPWithdraw(ctx, rpcClient, signer, transactor, deployment, lpDeployment, info.OutPointHex, withdrawAmount)
	require.NoError(t, err)
}

func ensureLPDeploymentOnChain(t *testing.T, rpcClient rpc.Client, lpDeployment LPDeployment) {
	ctx := context.Background()
	if lpDeployment.TypeScriptDep.OutPoint == nil || lpDeployment.LockScriptDep.OutPoint == nil {
		t.Skip("LP deployment missing outpoints; deploy LP scripts before running devnet tests")
	}
	cell, err := rpcClient.GetLiveCell(ctx, lpDeployment.TypeScriptDep.OutPoint, false)
	if err != nil || cell == nil || cell.Cell == nil {
		t.Skip("LP typescript not found on chain; deploy LP scripts before running devnet tests")
	}
	cell, err = rpcClient.GetLiveCell(ctx, lpDeployment.LockScriptDep.OutPoint, false)
	if err != nil || cell == nil || cell.Cell == nil {
		t.Skip("LP lockscript not found on chain; deploy LP scripts before running devnet tests")
	}
}

func loadLPDeploymentFromDevnet(t *testing.T) (LPDeployment, bool) {
	dir := filepath.Join("..", "..", "devnet", "contract", "migrations_lp", "dev")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("missing LP migration dir: %v", err)
		return LPDeployment{}, false
	}

	var migrationFile string
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == ".gitkeep" {
			continue
		}
		migrationFile = filepath.Join(dir, entry.Name())
		break
	}
	if migrationFile == "" {
		t.Skip("LP migration file not found; deploy LP scripts before running devnet tests")
		return LPDeployment{}, false
	}

	data, err := os.ReadFile(migrationFile)
	require.NoError(t, err)

	var migration lpMigration
	require.NoError(t, json.Unmarshal(data, &migration))

	lpts, lpls := findLPRecipes(migration)
	if lpts == nil || lpls == nil {
		t.Skip("LP migration missing lpts/lpls entries")
		return LPDeployment{}, false
	}

	return LPDeployment{
		TypeScriptDep: types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash(lpts.TxHash),
				Index:  lpts.Index,
			},
			DepType: types.DepTypeCode,
		},
		LockScriptDep: types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash(lpls.TxHash),
				Index:  lpls.Index,
			},
			DepType: types.DepTypeCode,
		},
		TypeScriptCodeHash: types.HexToHash(lpts.DataHash),
		TypeScriptHashType: types.HashTypeData1,
		LockScriptCodeHash: types.HexToHash(lpls.DataHash),
		LockScriptHashType: types.HashTypeData1,
	}, true
}

type lpRecipe struct {
	Name     string
	TxHash   string
	Index    uint32
	DataHash string
}

func findLPRecipes(migration lpMigration) (*lpRecipe, *lpRecipe) {
	var lpts *lpRecipe
	var lpls *lpRecipe
	for _, recipe := range migration.CellRecipes {
		entry := lpRecipe{
			Name:     recipe.Name,
			TxHash:   recipe.TxHash,
			Index:    recipe.Index,
			DataHash: recipe.DataHash,
		}
		switch recipe.Name {
		case "lpts":
			lpts = &entry
		case "lpls":
			lpls = &entry
		}
	}
	return lpts, lpls
}

func requireEnv(t *testing.T, key string) string {
	value := os.Getenv(key)
	if value == "" {
		t.Skipf("missing env %s", key)
	}
	return value
}

func getOperatorLockHash(t *testing.T) string {
	if value := os.Getenv("PERUN_LP_OPERATOR_LOCK_HASH"); value != "" {
		return value
	}

	cell, ok := loadLPCellSpecFromDevnet(t)
	if !ok {
		return ""
	}

	return fmt.Sprintf("0x%x", cell.OperatorLockHash)
}

func mustParseHash32(t *testing.T, value string) [32]byte {
	hash, err := parseHash32(value)
	require.NoError(t, err)
	var out [32]byte
	copy(out[:], hash[:])
	return out
}

func loadLPCellSpecFromDevnet(t *testing.T) (LPCell, bool) {
	path := filepath.Join("..", "..", "devnet", "contract", "migrations_lp", "lp_cell_spec.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return loadDefaultLPCellSpecFromDevnet(t)
	}

	var spec lpCellSpec
	require.NoError(t, json.Unmarshal(data, &spec))

	poolID := mustParseHash32(t, ensureHexPrefix(spec.PoolID))
	owner := mustParseHash32(t, ensureHexPrefix(spec.OwnerLockHash))
	operator := mustParseHash32(t, ensureHexPrefix(spec.OperatorLock))

	return LPCell{
		PoolID:                  poolID,
		OwnerLockHash:           owner,
		OperatorLockHash:        operator,
		AvailableCKB:            spec.AvailableCKB,
		ReservedCKB:             spec.ReservedCKB,
		CumulativeFeesEarnedCKB: spec.FeesEarnedCKB,
		Policy: LPPolicy{
			MaxTradingVolume: spec.Policy.MaxTradingVolume,
			FeeRateBps:       spec.Policy.FeeRateBps,
			PolicyFlags:      spec.Policy.PolicyFlags,
			PolicyVersion:    spec.Policy.PolicyVersion,
			SafePriceMinX64:  uint128.Uint128{},
			SafePriceMaxX64:  uint128.Max(),
		},
		Nonce:  spec.Nonce,
		Active: spec.Active,
	}, true
}

func loadDefaultLPCellSpecFromDevnet(t *testing.T) (LPCell, bool) {
	keyAlice, err := ckbtest.GetKey(filepath.Join("..", "..", "devnet", "accounts", "alice.pk"))
	require.NoError(t, err)
	keyBob, err := ckbtest.GetKey(filepath.Join("..", "..", "devnet", "accounts", "bob.pk"))
	require.NoError(t, err)

	alice, err := ckbaddress.NewDefaultParticipant(keyAlice.PubKey())
	require.NoError(t, err)
	bob, err := ckbaddress.NewDefaultParticipant(keyBob.PubKey())
	require.NoError(t, err)

	ownerHash := bob.ToCKBAddress(types.NetworkTest).Script.Hash()
	operatorHash := alice.ToCKBAddress(types.NetworkTest).Script.Hash()

	poolID := [32]byte{0x11}

	return LPCell{
		PoolID:                  poolID,
		OwnerLockHash:           ownerHash,
		OperatorLockHash:        operatorHash,
		AvailableCKB:            50_000_000_000,
		ReservedCKB:             0,
		CumulativeFeesEarnedCKB: 0,
		Policy: LPPolicy{
			MaxTradingVolume: 0,
			FeeRateBps:       30,
			PolicyFlags:      0,
			PolicyVersion:    1,
			SafePriceMinX64:  uint128.Uint128{},
			SafePriceMaxX64:  uint128.Max(),
		},
		Nonce:  0,
		Active: true,
	}, true
}

func ensureHexPrefix(value string) string {
	if value == "" {
		return value
	}
	if len(value) > 1 && value[0:2] == "0x" {
		return value
	}
	return "0x" + value
}

func loadDevnetDeployment(t *testing.T) backend.Deployment {
	sudtOwnerLockArg, err := ckbtest.ParseSUDTOwnerLockArg(filepath.Join("..", "..", "devnet", "accounts", "sudt-owner-lock-hash.txt"))
	require.NoError(t, err)

	deployment, _, err := ckbtest.GetDeployment(
		filepath.Join("..", "..", "devnet", "contract", "migrations_0", "dev"),
		filepath.Join("..", "..", "devnet", "contract", "migrations_1", "dev"),
		filepath.Join("..", "..", "devnet", "contract", "migrations_vc", "dev"),
		filepath.Join("..", "..", "devnet", "system_scripts"),
		sudtOwnerLockArg,
	)
	require.NoError(t, err)
	return deployment
}

func newBobSigner(t *testing.T, network types.Network) backend.Signer {
	keyBob, err := ckbtest.GetKey(filepath.Join("..", "..", "devnet", "accounts", "bob.pk"))
	require.NoError(t, err)

	participant, err := ckbaddress.NewDefaultParticipant(keyBob.PubKey())
	require.NoError(t, err)

	addr := participant.ToCKBAddress(network)
	return backend.NewSignerInstance(addr, *keyBob, network)
}

func buildAndSubmitLPDeposit(
	ctx context.Context,
	rpcClient rpc.Client,
	signer backend.Signer,
	transactor backend.Transactor,
	deployment backend.Deployment,
	lpDeployment LPDeployment,
	lpCell LPCell,
) (string, error) {
	inputCell, err := selectLargestCKBCell(ctx, rpcClient, signer.Address().Script, nil)
	if err != nil {
		return "", err
	}

	if inputCell.Output == nil {
		return "", Deterministic(ErrInvalidLPCell)
	}

	data, err := EncodeLPCell(lpCell)
	if err != nil {
		return "", err
	}

	fee := transaction.DefaultFeeShannon
	if inputCell.Output.Capacity <= lpCell.AvailableCKB+fee {
		return "", Deterministic(ErrInsufficientOperatorFunds)
	}
	change := inputCell.Output.Capacity - lpCell.AvailableCKB - fee

	lockScript, typeScript, err := buildLPScripts(lpDeployment, lpCell.PoolID)
	if err != nil {
		return "", err
	}

	builder := newSimpleBuilderWithDeps(signer, deployment)
	builder.AddCellDep(&lpDeployment.TypeScriptDep)
	builder.AddCellDep(&lpDeployment.LockScriptDep)

	builder.AddInput(&types.CellInput{PreviousOutput: inputCell.OutPoint})

	builder.AddOutput(&types.CellOutput{
		Capacity: lpCell.AvailableCKB,
		Lock:     &lockScript,
		Type:     &typeScript,
	}, data)

	builder.AddOutput(&types.CellOutput{
		Capacity: change,
		Lock:     inputCell.Output.Lock,
		Type:     nil,
	}, []byte{})

	tx, err := builder.Build()
	if err != nil {
		return "", err
	}

	txHash, err := transactor.SubmitTransaction(ctx, tx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%d", txHash.String(), 0), nil
}

func buildAndSubmitLPWithdraw(
	ctx context.Context,
	rpcClient rpc.Client,
	signer backend.Signer,
	transactor backend.Transactor,
	deployment backend.Deployment,
	lpDeployment LPDeployment,
	lpCellID string,
	ckbOut uint64,
) (types.Hash, error) {
	if ckbOut == 0 {
		return types.Hash{}, Deterministic(ErrInvalidLPCellArg)
	}
	outPoint, err := parseOutPoint(lpCellID)
	if err != nil {
		return types.Hash{}, err
	}
	lpCellWithStatus, err := rpcClient.GetLiveCell(ctx, outPoint, true)
	if err != nil {
		return types.Hash{}, err
	}
	if lpCellWithStatus == nil || lpCellWithStatus.Cell == nil || lpCellWithStatus.Cell.Output == nil || lpCellWithStatus.Cell.Data == nil {
		return types.Hash{}, Deterministic(ErrInvalidLPCell)
	}
	inputLPData := lpCellWithStatus.Cell.Data.Content
	inputLP, err := DecodeLPCell(inputLPData)
	if err != nil {
		return types.Hash{}, err
	}
	if ckbOut > inputLP.AvailableCKB {
		return types.Hash{}, Deterministic(ErrInvalidLPCellArg)
	}

	updatedLP := inputLP
	updatedLP.AvailableCKB -= ckbOut
	updatedLP.Nonce += 1
	updatedLPData, err := EncodeLPCell(updatedLP)
	if err != nil {
		return types.Hash{}, err
	}

	ownerCell, err := selectLargestCKBCell(ctx, rpcClient, signer.Address().Script, outPoint)
	if err != nil {
		return types.Hash{}, err
	}
	if ownerCell.Output == nil {
		return types.Hash{}, Deterministic(ErrInvalidLPCell)
	}

	fee := transaction.DefaultFeeShannon
	if ownerCell.Output.Capacity <= fee {
		return types.Hash{}, Deterministic(ErrInsufficientOperatorFunds)
	}
	change := ownerCell.Output.Capacity + ckbOut - fee

	builder := newSimpleBuilderWithDeps(signer, deployment)
	builder.AddCellDep(&lpDeployment.TypeScriptDep)
	builder.AddCellDep(&lpDeployment.LockScriptDep)

	builder.AddInput(&types.CellInput{PreviousOutput: outPoint})

	builder.AddInput(&types.CellInput{PreviousOutput: ownerCell.OutPoint})

	builder.AddOutput(&types.CellOutput{
		Capacity: lpCellWithStatus.Cell.Output.Capacity - ckbOut,
		Lock:     lpCellWithStatus.Cell.Output.Lock,
		Type:     lpCellWithStatus.Cell.Output.Type,
	}, updatedLPData)

	builder.AddOutput(&types.CellOutput{
		Capacity: change,
		Lock:     ownerCell.Output.Lock,
		Type:     nil,
	}, []byte{})

	tx, err := builder.Build()
	if err != nil {
		return types.Hash{}, err
	}

	txHash, err := transactor.SubmitTransaction(ctx, tx)
	if err != nil {
		return types.Hash{}, err
	}
	return txHash, nil
}

func newSimpleBuilderWithDeps(signer backend.Signer, deployment backend.Deployment) *ckbbuilder.SimpleTransactionBuilder {
	builder := transaction.NewSimpleTransactionBuilder(signer.Address().Script.CodeHash, deployment.DefaultLockScriptDep, false)
	if signer.Address().Script.CodeHash != deployment.DefaultLockScript.CodeHash && len(deployment.OmniLockScriptDep) >= 2 {
		builder = transaction.NewSimpleTransactionBuilder(deployment.OmniLockScript.CodeHash, deployment.OmniLockScriptDep[1], true)
		builder.AddCellDep(&deployment.OmniLockScriptDep[0])
		builder.AddCellDep(&deployment.OmniLockScriptDep[1])
	} else {
		builder.AddCellDep(&deployment.DefaultLockScriptDep)
	}
	return builder
}

func buildLPScripts(lpDeployment LPDeployment, poolID [32]byte) (types.Script, types.Script, error) {
	typeScript := types.Script{
		CodeHash: lpDeployment.TypeScriptCodeHash,
		HashType: lpDeployment.TypeScriptHashType,
		Args:     poolID[:],
	}
	tsHash := typeScript.Hash()
	lockArgs := tsHash[:]
	lockScript := types.Script{
		CodeHash: lpDeployment.LockScriptCodeHash,
		HashType: lpDeployment.LockScriptHashType,
		Args:     lockArgs,
	}
	return lockScript, typeScript, nil
}

func selectLargestCKBCell(ctx context.Context, rpcClient rpc.Client, lockScript *types.Script, exclude *types.OutPoint) (*indexer.LiveCell, error) {
	searchKey := &indexer.SearchKey{
		Script:           lockScript,
		ScriptType:       types.ScriptTypeLock,
		ScriptSearchMode: types.ScriptSearchModeExact,
		WithData:         true,
	}
	resp, err := rpcClient.GetCells(ctx, searchKey, indexer.SearchOrderDesc, 100, "")
	if err != nil {
		return nil, err
	}
	var best *indexer.LiveCell
	for _, cell := range resp.Objects {
		if cell.Output == nil || cell.Output.Type != nil {
			continue
		}
		if exclude != nil && cell.OutPoint != nil && cell.OutPoint.TxHash == exclude.TxHash && cell.OutPoint.Index == exclude.Index {
			continue
		}
		if IsLPCell(cell.OutputData) {
			continue
		}
		if best == nil || cell.Output.Capacity > best.Output.Capacity {
			best = cell
		}
	}
	if best == nil {
		return nil, Deterministic(ErrInsufficientOperatorFunds)
	}
	return best, nil
}
