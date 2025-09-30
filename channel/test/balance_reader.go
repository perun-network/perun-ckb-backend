// Copyright 2025 PolyCrypt GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package test

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"log"
	"math"
	"math/big"
	"time"

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

func (br *BalanceReader) Balance(asset perunchannel.Asset) perunchannel.Bal {
	pollingInterval := time.Second
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

	switch a := asset.(type) {
	case *ckbasset.EthAsset:

		// EthAsset: Not available on the NERVOS CKB blockchain, so it cannot be extracted from a Cell

		return big.NewInt(0)

	case *ckbasset.NervosAsset:

		// Check if CKBytes or SUDT inside NervosAsset
		if a.Asset.IsCKBytes {
			return ckbBalance
		}
		if a.Asset.SUDT != nil {
			return sudtBalance
		}
		panic("NervosAsset invalid: neither CKBytes nor SUDT")

	case *ckbasset.Asset:
		fmt.Println("ckbasset.Asset balance requested")
		// Raw Asset passed - check CKBytes or SUDT similarly
		if a.IsCKBytes {
			return ckbBalance
		}
		if a.SUDT != nil {
			return sudtBalance
		}
		panic("Asset invalid: neither CKBytes nor SUDT")

	default:
		panic(fmt.Sprintf("Unknown asset type: %T", a))
	}
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
