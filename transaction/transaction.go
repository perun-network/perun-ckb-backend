package transaction

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector/builder"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

type PerunTransactionBuilder struct {
	*builder.CkbTransactionBuilder
}

func NewPerunTransactionBuilder(network types.Network, iterator collector.CellIterator, psh *PerunScriptHandler) *PerunTransactionBuilder {
	b := &PerunTransactionBuilder{
		CkbTransactionBuilder: builder.NewCkbTransactionBuilder(network, iterator),
	}
	b.Register(psh)
	return b
}
