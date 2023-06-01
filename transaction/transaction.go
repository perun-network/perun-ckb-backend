package transaction

import (
	"fmt"

	"github.com/nervosnetwork/ckb-sdk-go/v2/address"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector/builder"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

type PerunTransactionBuilder struct {
	*builder.CkbTransactionBuilder
}

func NewPerunTransactionBuilder(network types.Network, iterator collector.CellIterator, psh *PerunScriptHandler, sender address.Address) (*PerunTransactionBuilder, error) {
	b := &PerunTransactionBuilder{
		CkbTransactionBuilder: builder.NewCkbTransactionBuilder(network, iterator),
	}
	b.Register(psh)

	encodedAddr, err := sender.Encode()
	if err != nil {
		return nil, fmt.Errorf("encoding address: %w", err)
	}
	err = b.AddChangeOutputByAddress(encodedAddr)
	if err != nil {
		return nil, fmt.Errorf("adding change output: %w", err)
	}
	return b, nil
}
