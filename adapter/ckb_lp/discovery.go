package ckblp

import (
	"context"

	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/perun-ckb-backend/client"
)

func (a *Adapter) DiscoverLPCells(ctx context.Context, operatorLockHash [32]byte) ([]LPCellInfo, error) {
	searchKey := &indexer.SearchKey{
		Script: &types.Script{
			CodeHash: a.lpDeployment.TypeScriptCodeHash,
			HashType: a.lpDeployment.TypeScriptHashType,
			Args:     []byte{},
		},
		ScriptType:       types.ScriptTypeType,
		ScriptSearchMode: types.ScriptSearchModePrefix,
		WithData:         true,
	}

	cells, err := a.rpcClient.GetCells(ctx, searchKey, indexer.SearchOrderDesc, client.SearchIndexerLimit, "")
	if err != nil {
		return nil, Retriable(err)
	}

	infos := make([]LPCellInfo, 0, len(cells.Objects))
	for _, cell := range cells.Objects {
		if cell.Output == nil {
			continue
		}
		if cell.Output.Type == nil || len(cell.Output.Type.Args) != 32 {
			continue
		}
		if !IsLPCell(cell.OutputData) {
			continue
		}
		decoded, err := DecodeLPCell(cell.OutputData)
		if err != nil {
			continue
		}
		if string(cell.Output.Type.Args) != string(decoded.PoolID[:]) {
			continue
		}
		if decoded.OperatorLockHash != operatorLockHash {
			continue
		}
		info := LPCellInfo{
			Cell:        decoded,
			Capacity:    cell.Output.Capacity,
			OutPointHex: outPointKey(cell.OutPoint),
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (a *Adapter) GetLPCell(ctx context.Context, lpCellID string) (LPCellInfo, error) {
	outPoint, err := parseOutPoint(lpCellID)
	if err != nil {
		return LPCellInfo{}, Deterministic(err)
	}
	cell, err := a.rpcClient.GetLiveCell(ctx, outPoint, true)
	if err != nil {
		return LPCellInfo{}, Retriable(err)
	}
	if cell == nil || cell.Cell == nil || cell.Cell.Output == nil || cell.Cell.Data == nil {
		return LPCellInfo{}, Deterministic(ErrInvalidLPCell)
	}
	decoded, err := DecodeLPCell(cell.Cell.Data.Content)
	if err != nil {
		return LPCellInfo{}, Deterministic(ErrInvalidLPCell)
	}
	return LPCellInfo{
		Cell:        decoded,
		Capacity:    cell.Cell.Output.Capacity,
		OutPointHex: outPointKey(outPoint),
	}, nil
}
