package client

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

// CKBOnlyIterator is cell iterator which ONLY matches live cells NOT guarded
// by any type script.
type CKBOnlyIterator struct {
	collector.CellIterator
	nextInput *types.TransactionInput
}

// HasNext implements collector.CellIterator.
func (i *CKBOnlyIterator) HasNext() bool {
	if i.nextInput != nil {
		return true
	}
	// HasNext loads the next SDK page, so check it before every advance,
	// including when skipping token cells at the end of the current page.
	for i.CellIterator.HasNext() {
		input := i.CellIterator.Next()
		if input != nil && input.Output != nil && input.Output.Type == nil {
			i.nextInput = input
			return true
		}
	}
	return false
}

// Next implements collector.CellIterator.
func (i *CKBOnlyIterator) Next() *types.TransactionInput {
	if !i.HasNext() {
		return nil
	}
	input := i.nextInput
	i.nextInput = nil
	return input
}

func NewCKBOnlyIterator(iter collector.CellIterator) *CKBOnlyIterator {
	return &CKBOnlyIterator{CellIterator: iter}
}

var _ collector.CellIterator = (*CKBOnlyIterator)(nil)
