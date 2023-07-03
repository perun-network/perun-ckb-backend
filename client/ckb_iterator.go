package client

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

// CKBOnlyIterator is cell iterator which ONLY matches live cells NOT guarded
// by any type script.
type CKBOnlyIterator struct {
	collector.CellIterator
}

// HasNext implements collector.CellIterator.
func (i *CKBOnlyIterator) HasNext() bool {
	return i.CellIterator.HasNext()
}

// Next implements collector.CellIterator.
func (i *CKBOnlyIterator) Next() *types.TransactionInput {
	ti := i.CellIterator.Next()

	for ti != nil && ti.Output.Type != nil {
		// Skip all cells guarded by type script.
		ti = i.CellIterator.Next()
	}

	return ti
}

func NewCKBOnlyIterator(iter collector.CellIterator) *CKBOnlyIterator {
	return &CKBOnlyIterator{iter}
}

var _ collector.CellIterator = (*CKBOnlyIterator)(nil)
