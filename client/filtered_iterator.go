package client

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

type FilteredCellIterator struct {
	base      collector.CellIterator
	filter    func(*types.CellOutput) bool
	nextInput *types.TransactionInput
	loaded    bool
}

func NewFilteredCellIterator(base collector.CellIterator, filter func(*types.CellOutput) bool) *FilteredCellIterator {
	return &FilteredCellIterator{
		base:   base,
		filter: filter,
	}
}

func (f *FilteredCellIterator) HasNext() bool {
	if f.loaded {
		return true
	}
	for f.base.HasNext() {
		candidate := f.base.Next()
		if f.filter(candidate.Output) {
			f.nextInput = candidate
			f.loaded = true
			return true
		}
	}

	return false
}

func (f *FilteredCellIterator) Next() *types.TransactionInput {
	if !f.loaded && !f.HasNext() {
		return nil
	}
	f.loaded = false
	return f.nextInput
}
