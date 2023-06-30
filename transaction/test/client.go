package test

import (
	"context"
	"fmt"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/perun-ckb-backend/transaction"
)

type MockClient struct {
	CellMap map[string]*types.CellWithStatus
}

type MockClientOpt func(*MockClient)

func NewMockClient(opts ...MockClientOpt) *MockClient {
	mc := &MockClient{
		CellMap: make(map[string]*types.CellWithStatus),
	}

	for _, opt := range opts {
		opt(mc)
	}

	return mc
}

// GetLiveCell implements transaction.LiveCellFetcher
func (mc *MockClient) GetLiveCell(_ context.Context, outPoint *types.OutPoint, withData bool) (*types.CellWithStatus, error) {
	key := MakeKeyFromOutpoint(outPoint)
	return mc.CellMap[key], nil
}

func MakeKeyFromOutpoint(outPoint *types.OutPoint) string {
	return fmt.Sprintf("%s:%v", outPoint.TxHash, outPoint.Index)
}

func WithMockLiveCell(outPoint *types.OutPoint, cell *types.CellWithStatus) MockClientOpt {
	return func(mc *MockClient) {
		mc.CellMap[MakeKeyFromOutpoint(outPoint)] = cell
	}
}

func WithMockLiveCells(m map[string]*types.CellWithStatus) MockClientOpt {
	return func(mc *MockClient) {
		mc.CellMap = m
	}
}

var _ transaction.LiveCellFetcher = (*MockClient)(nil)
