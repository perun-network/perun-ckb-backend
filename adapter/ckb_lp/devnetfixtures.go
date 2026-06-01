package ckblp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Pilatuz/bigz/uint128"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/perun-ckb-backend/backend"
	ckbtest "perun.network/perun-ckb-backend/channel/test"
	ckbaddress "perun.network/perun-ckb-backend/wallet/address"
)

// ErrFixtureUnavailable signals that a devnet fixture (migration dir, spec
// file, on-chain cell) is not present. Callers should map this to t.Skipf.
var ErrFixtureUnavailable = errors.New("devnet fixture unavailable")

// LPMigration mirrors the on-disk LP migration JSON shape.
type LPMigration struct {
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

type lpRecipe struct {
	Name     string
	TxHash   string
	Index    uint32
	DataHash string
}

// FindDevnetDir walks up from the current working directory until it finds a
// directory containing devnet/contract/migrations_lp. Returns the path to the
// devnet directory itself (the parent of contract/migrations_lp).
func FindDevnetDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		candidate := filepath.Join(dir, "devnet", "contract", "migrations_lp")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return filepath.Join(dir, "devnet"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%w: no devnet/contract/migrations_lp found walking up from %s", ErrFixtureUnavailable, cwd)
		}
		dir = parent
	}
}

// LoadLPDeploymentFromDevnet reads the LP migration JSON and returns an
// LPDeployment. Returns ErrFixtureUnavailable when the migration dir or
// required recipes are missing.
func LoadLPDeploymentFromDevnet() (LPDeployment, error) {
	devnetDir, err := FindDevnetDir()
	if err != nil {
		return LPDeployment{}, err
	}
	dir := filepath.Join(devnetDir, "contract", "migrations_lp", "dev")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return LPDeployment{}, fmt.Errorf("%w: missing LP migration dir: %v", ErrFixtureUnavailable, err)
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
		return LPDeployment{}, fmt.Errorf("%w: LP migration file not found in %s", ErrFixtureUnavailable, dir)
	}

	data, err := os.ReadFile(migrationFile)
	if err != nil {
		return LPDeployment{}, fmt.Errorf("reading LP migration: %w", err)
	}

	var migration LPMigration
	if err := json.Unmarshal(data, &migration); err != nil {
		return LPDeployment{}, fmt.Errorf("parsing LP migration: %w", err)
	}

	lpts, lpls := findLPRecipes(migration)
	if lpts == nil || lpls == nil {
		return LPDeployment{}, fmt.Errorf("%w: LP migration missing lpts/lpls entries", ErrFixtureUnavailable)
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
	}, nil
}

// LoadLPCellSpecFromDevnet reads the on-disk LP cell spec; falls back to a
// Bob-owner default if the file is missing.
func LoadLPCellSpecFromDevnet() (LPCell, error) {
	devnetDir, err := FindDevnetDir()
	if err != nil {
		return LPCell{}, err
	}
	path := filepath.Join(devnetDir, "contract", "migrations_lp", "lp_cell_spec.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return LoadDefaultLPCellSpec(devnetDir)
	}

	var spec lpCellSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return LPCell{}, fmt.Errorf("parsing LP cell spec: %w", err)
	}

	poolID, err := parseHash32Fixed(ensureHexPrefix(spec.PoolID))
	if err != nil {
		return LPCell{}, fmt.Errorf("pool_id: %w", err)
	}
	owner, err := parseHash32Fixed(ensureHexPrefix(spec.OwnerLockHash))
	if err != nil {
		return LPCell{}, fmt.Errorf("owner_lock_hash: %w", err)
	}
	operator, err := parseHash32Fixed(ensureHexPrefix(spec.OperatorLock))
	if err != nil {
		return LPCell{}, fmt.Errorf("operator_lock_hash: %w", err)
	}

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
	}, nil
}

// LoadDefaultLPCellSpec returns a hard-coded LP cell spec keyed off Bob.
// Used when the on-disk spec is missing.
func LoadDefaultLPCellSpec(devnetDir string) (LPCell, error) {
	keyBob, err := ckbtest.GetKey(filepath.Join(devnetDir, "accounts", "bob.pk"))
	if err != nil {
		return LPCell{}, fmt.Errorf("%w: cannot load bob.pk: %v", ErrFixtureUnavailable, err)
	}
	bob, err := ckbaddress.NewDefaultParticipant(keyBob.PubKey())
	if err != nil {
		return LPCell{}, fmt.Errorf("building default participant: %w", err)
	}

	bobHash := bob.ToCKBAddress(types.NetworkTest).Script.Hash()
	poolID := [32]byte{0x11}

	return LPCell{
		PoolID:                  poolID,
		OwnerLockHash:           bobHash,
		OperatorLockHash:        bobHash,
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
	}, nil
}

// LoadDevnetDeployment reads the Perun migrations (migrations_0, _1, _vc) and
// system scripts to produce a backend.Deployment.
func LoadDevnetDeployment() (backend.Deployment, error) {
	devnetDir, err := FindDevnetDir()
	if err != nil {
		return backend.Deployment{}, err
	}
	sudtOwnerLockArg, err := ckbtest.ParseSUDTOwnerLockArg(filepath.Join(devnetDir, "accounts", "sudt-owner-lock-hash.txt"))
	if err != nil {
		return backend.Deployment{}, fmt.Errorf("%w: cannot parse SUDT owner lock arg: %v", ErrFixtureUnavailable, err)
	}

	deployment, _, err := ckbtest.GetDeployment(
		filepath.Join(devnetDir, "contract", "migrations_0", "dev"),
		filepath.Join(devnetDir, "contract", "migrations_1", "dev"),
		filepath.Join(devnetDir, "contract", "migrations_vc", "dev"),
		filepath.Join(devnetDir, "system_scripts"),
		sudtOwnerLockArg,
	)
	if err != nil {
		return backend.Deployment{}, fmt.Errorf("loading deployment: %w", err)
	}
	return deployment, nil
}

// EnsureLPDeploymentOnChain verifies the LP type and lock script cells are
// live at the given outpoints. Returns ErrFixtureUnavailable when missing.
func EnsureLPDeploymentOnChain(ctx context.Context, rpcClient rpc.Client, lpDeployment LPDeployment) error {
	if lpDeployment.TypeScriptDep.OutPoint == nil || lpDeployment.LockScriptDep.OutPoint == nil {
		return fmt.Errorf("%w: LP deployment missing outpoints", ErrFixtureUnavailable)
	}
	cell, err := rpcClient.GetLiveCell(ctx, lpDeployment.TypeScriptDep.OutPoint, false)
	if err != nil {
		return fmt.Errorf("%w: querying LP typescript: %v", ErrFixtureUnavailable, err)
	}
	if cell == nil || cell.Cell == nil {
		return fmt.Errorf("%w: LP typescript not found on chain", ErrFixtureUnavailable)
	}
	cell, err = rpcClient.GetLiveCell(ctx, lpDeployment.LockScriptDep.OutPoint, false)
	if err != nil {
		return fmt.Errorf("%w: querying LP lockscript: %v", ErrFixtureUnavailable, err)
	}
	if cell == nil || cell.Cell == nil {
		return fmt.Errorf("%w: LP lockscript not found on chain", ErrFixtureUnavailable)
	}
	return nil
}

func findLPRecipes(migration LPMigration) (*lpRecipe, *lpRecipe) {
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

func ensureHexPrefix(value string) string {
	if value == "" {
		return value
	}
	if len(value) > 1 && value[0:2] == "0x" {
		return value
	}
	return "0x" + value
}

func parseHash32Fixed(value string) ([32]byte, error) {
	h, err := parseHash32(value)
	if err != nil {
		return [32]byte{}, err
	}
	var out [32]byte
	copy(out[:], h[:])
	return out, nil
}
