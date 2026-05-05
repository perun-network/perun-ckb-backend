package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	ckbtransaction "github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

// RPCTransactor signs and submits transactions, then waits for commitment.
type RPCTransactor struct {
	rpcClient rpc.Client
	signer    Signer
}

// NewRPCTransactor creates a transactor that submits via the given RPC client.
func NewRPCTransactor(rpcClient rpc.Client, signer Signer) *RPCTransactor {
	return &RPCTransactor{
		rpcClient: rpcClient,
		signer:    signer,
	}
}

// SubmitTransaction signs, sends, and waits for commitment.
func (t *RPCTransactor) SubmitTransaction(ctx context.Context, tx *ckbtransaction.TransactionWithScriptGroups) (types.Hash, error) {
	signedTx, err := t.signer.SignTransaction(tx)
	if err != nil {
		return types.Hash{}, err
	}
	return sendAndAwait(ctx, t.rpcClient, signedTx)
}

const defaultPollingInterval = 2 * time.Second

func sendAndAwait(ctx context.Context, rpcClient rpc.Client, tx *types.Transaction) (types.Hash, error) {
	var txHash *types.Hash
	for i := 0; i < 3; i++ {
		var err error
		txHash, err = rpcClient.SendTransaction(ctx, tx)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return types.Hash{}, fmt.Errorf("sending transaction: %w", ctx.Err())
		}
		time.Sleep(10 * time.Second)
	}
	if txHash == nil {
		return types.Hash{}, fmt.Errorf("sending transaction: retries exhausted")
	}

	var txWithStatus *types.TransactionWithStatus
	for i := 0; i < 3; i++ {
		var err error
		txWithStatus, err = rpcClient.GetTransaction(ctx, *txHash)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return types.Hash{}, fmt.Errorf("polling transaction: %w", ctx.Err())
		}
		time.Sleep(10 * time.Second)
	}
	if txWithStatus == nil {
		return types.Hash{}, fmt.Errorf("polling transaction: retries exhausted")
	}

	ticker := time.NewTicker(defaultPollingInterval)
	defer ticker.Stop()
	for txWithStatus.TxStatus.Status != types.TransactionStatusCommitted {
		if txWithStatus.TxStatus.Status == types.TransactionStatusRejected {
			return types.Hash{}, fmt.Errorf("transaction rejected with: %v", *txWithStatus.TxStatus.Reason)
		}
		select {
		case <-ctx.Done():
			return types.Hash{}, fmt.Errorf("context done: %w", ctx.Err())
		case <-ticker.C:
			_, err := rpcClient.GetTransaction(ctx, *txHash)
			if err != nil {
				return types.Hash{}, fmt.Errorf("polling transaction: %w", err)
			}
		}
	}

	return *txHash, nil
}
