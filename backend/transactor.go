package backend

import (
	"context"

	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

// Transactor interacts with the blockchain and is able to submit unsigned
// transactions to the blockchain.
type Transactor interface {
	// SubmitTransaction submits the given transaction to the blockchain.
	SubmitTransaction(ctx context.Context, tx *transaction.TransactionWithScriptGroups) (types.Hash, error)
}
