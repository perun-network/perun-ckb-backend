package test

import (
	"context"
	"encoding/binary"
	"log"
	"math"
	"math/big"
	"time"

	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"

	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/wallet/address"

	perunchannel "perun.network/go-perun/channel"

	ckbasset "perun.network/perun-ckb-backend/channel/asset"
)

// BalanceReader is a balance reader used for testing. It is associated with a
// given account.
type BalanceReader struct {
	rpcClient rpc.Client
	acc       wallet.Address
}

// NewBalanceReader creates a new balance reader associated with the given account.
func NewBalanceReader(rpcClient rpc.Client, acc wallet.Address) *BalanceReader {
	return &BalanceReader{rpcClient: rpcClient, acc: acc}
}

// Balance returns the asset balance of the associated account.
func (br *BalanceReader) Balance(asset perunchannel.Asset) perunchannel.Bal {
	pollingInterval := time.Duration(1) * time.Second
	searchKey := &indexer.SearchKey{
		Script:           address.AsParticipant(br.acc).PaymentScript,
		ScriptType:       types.ScriptTypeLock,
		ScriptSearchMode: types.ScriptSearchModeExact,
		Filter:           nil,
		WithData:         true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), pollingInterval)
	defer cancel()

	cells, err := br.rpcClient.GetCells(ctx, searchKey, indexer.SearchOrderDesc, math.MaxUint32, "")
	if err != nil {
		log.Fatalf("Error getting cells: %v", err)
	}
	ckbBalance := big.NewInt(0)
	sudtBalance := big.NewInt(0)
	for _, cell := range cells.Objects {
		ckbBalance = new(big.Int).Add(ckbBalance, ckbBalanceExtractor(cell))
		sudtBalance = new(big.Int).Add(sudtBalance, sudtBalanceExtractor(cell))
	}

	if asset.Equal(ckbasset.NewCKBytesAsset()) {
		return ckbBalance
	} else if _, err := ckbasset.IsSUDTAsset(asset); err != nil {
		return sudtBalance
	}

	panic("unknown asset")
}

func ckbBalanceExtractor(cell *indexer.LiveCell) *big.Int {
	return new(big.Int).SetUint64(cell.Output.Capacity)
}

func sudtBalanceExtractor(cell *indexer.LiveCell) *big.Int {
	if len(cell.OutputData) != 16 {
		return big.NewInt(0)
	}
	return new(big.Int).SetUint64(binary.LittleEndian.Uint64(cell.OutputData))
}
