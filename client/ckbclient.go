package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/nervosnetwork/ckb-sdk-go/v2/address"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	ckbtransaction "github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/backend"
	ckbchannel "perun.network/perun-ckb-backend/channel"
	"perun.network/perun-ckb-backend/encoding"
	"perun.network/perun-ckb-backend/transaction"
)

var ErrNoChannelLiveCell = errors.New("no channel live cell found")

type BlockNumber = uint64

type CKBClient interface {
	// Start starts a new channel on-chain with the given parameters and initial state.
	// It returns the resulting channel token or an error.
	// Start should block until the starting transaction is committed on-chain.
	// The implementation can assume that Start will only ever be performed by Party A.
	Start(ctx context.Context, params *channel.Params, state *channel.State) (*types.Script, error)

	// Abort aborts the channel with the given channel token.
	Abort(ctx context.Context, pcts *types.Script, params *channel.Params, state *channel.State) error

	// Fund funds the channel with the given channel token. The implementation can assume that Fund will only ever
	// be performed by Party B.
	Fund(ctx context.Context, pcts *types.Script, state *channel.State, params *channel.Params) error

	// Dispute registers a dispute for the channel with the given channel ID on chain.
	// It should register the given state with the given signatures as witness.
	// Note: The given signatures are padded (see encoding.NewDEREncodedSignatureFromPadded).
	Dispute(ctx context.Context, id channel.ID, state *channel.State, sigs []wallet.Sig, params *channel.Params) error

	// Close closes the channel with the given channel ID on chain.
	// The implementation can assume that the given state is final.
	// Note: The given signatures are padded (see encoding.NewDEREncodedSignatureFromPadded).
	Close(ctx context.Context, id channel.ID, state *channel.State, sigs []wallet.Sig, params *channel.Params) error

	// ForceClose closes the channel with the given channel ID on chain.
	// The implementation can assume that the channel has already been disputed and that the challenge duration
	// is expired in real-time, though it may be necessary to wait until a block is produced with a timestamp strictly
	// later than the expiration of the challenge duration.
	ForceClose(ctx context.Context, id channel.ID, state *channel.State, params *channel.Params) error

	// GetChannelWithID returns an on-chain channel with the given channel ID.
	// Note: Only the channel ID field in the state must be checked, as the pcts verifies the integrity of said
	// field upon channel start (i.e. that it is equal to the hash of the channel parameters).
	// If there are multiple channels with the same ID, the implementation can return any of them, but the returned
	// constants and status must belong to the same channel.
	// Iff all RPC calls succeed but no live cell for the given channel ID is found, the returned error is
	// ErrNoChannelLiveCell.
	GetChannelWithID(ctx context.Context, id channel.ID) (BlockNumber, *types.Script, *molecule.ChannelConstants, *molecule.ChannelStatus, error)

	// GetChannelWithExactPCTS returns the on-chain channel status for the given type script.
	// Iff all RPC calls succeed but no live cell for the given channel ID is found, the returned error is
	// ErrNoChannelLiveCell.
	GetChannelWithExactPCTS(ctx context.Context, pcts *types.Script) (BlockNumber, *molecule.ChannelStatus, error)

	// GetBlockTime returns the timestamp of the block with the given block number.
	GetBlockTime(ctx context.Context, blockNumber BlockNumber) (time.Time, error)
}

// TODO: Require sudt handlers!

type Client struct {
	client rpc.Client

	signer     backend.Signer
	deployment backend.Deployment

	psh   *transaction.PerunScriptHandler
	cache StableScriptCache
}

func NewClient(rpcClient rpc.Client, signer backend.Signer, deployment backend.Deployment) (*Client, error) {
	psh := transaction.NewPerunScriptHandlerWithDeployment(deployment)
	return &Client{
		client:     rpcClient,
		signer:     signer,
		deployment: deployment,
		psh:        psh,
		cache:      NewStableScriptCache(),
	}, nil
}

var _ CKBClient = (*Client)(nil)

func (c Client) Start(ctx context.Context, params *channel.Params, state *channel.State) (*types.Script, error) {
	// TODO: Override defaulthash logic.
	iter, _, err := c.mkMyCKBCellIterator()
	if err != nil {
		return nil, fmt.Errorf("creating cell iterator: %w", err)
	}
	channelToken, err := c.createOrGetChannelToken(ctx, iter)
	if err != nil {
		return nil, fmt.Errorf("creating channel token: %w", err)
	}
	cid := ckbchannel.Backend.CalcID(params)
	oi := transaction.NewOpenInfo(cid, channelToken, params, state)

	zeroHash := types.Hash{}
	builder, err := c.newPerunTransactionBuilder(map[types.Hash]collector.CellIterator{zeroHash: iter})
	if err != nil {
		return nil, fmt.Errorf("creating Perun transaction builder: %w", err)
	}
	if err := builder.Open(oi); err != nil {
		return nil, fmt.Errorf("creating open transaction: %w", err)
	}
	tx, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("building open transaction: %w", err)
	}
	if err := c.submitTx(ctx, tx); err != nil {
		return nil, fmt.Errorf("submitting transaction: %w", err)
	}

	return oi.GetPCTS(), nil
}

// newPerunScriptHandler creates a new PerunScriptHandler. The iterator used to
// fetch the live cells for the account associated with this client can be
// injected if it was necessary in the outer scope.
func (c Client) newPerunTransactionBuilder(withIterators map[types.Hash]collector.CellIterator) (*transaction.PerunTransactionBuilder, error) {
	iters, err := iteratorsForDeployment(c.client, c.deployment, c.signer.Address())
	if err != nil {
		return nil, fmt.Errorf("creating cell iterators: %w", err)
	}

	// Override custom specified iterators.
	for hash, iter := range withIterators {
		iters[hash] = iter
	}

	return transaction.NewPerunTransactionBuilderWithDeployment(c.client, c.deployment, iters, c.signer.Address())
}

func iteratorsForDeployment(cl rpc.Client, deployment backend.Deployment, sender address.Address) (map[types.Hash]collector.CellIterator, error) {
	zeroHash := types.Hash{}
	iters := make(map[types.Hash]collector.CellIterator)
	// Iterator for the lockscript:
	senderString, err := sender.Encode()
	if err != nil {
		return nil, fmt.Errorf("encoding sender address: %w", err)
	}
	iter, err := collector.NewLiveCellIteratorFromAddress(cl, senderString)
	if err != nil {
		return nil, fmt.Errorf("creating cell iterator for default lockscript: %w", err)
	}
	// NOTE: This is to gather CKBytes.
	iters[zeroHash] = NewCKBOnlyIterator(iter)

	// Iterator for udts:
	for _, udt := range deployment.SUDTs {
		searchKey := &indexer.SearchKey{
			Script:           &udt,
			ScriptType:       types.ScriptTypeType,
			ScriptSearchMode: types.ScriptSearchModePrefix,
			Filter:           nil,
			WithData:         true,
		}
		sudtIter := collector.NewLiveCellIterator(cl, searchKey)
		iters[udt.Hash()] = sudtIter
	}
	return iters, nil
}

func (c Client) submitTx(ctx context.Context, tx *ckbtransaction.TransactionWithScriptGroups) error {
	sTx, err := c.signer.SignTransaction(tx)
	*tx = *sTx
	if err != nil {
		return fmt.Errorf("signing transaction: %w", err)
	}
	return c.sendAndAwait(ctx, tx.TxView)
}

// submitTxWithArgument submits a transaction whose type is determined by the
// txTypeArgument.
func (c Client) submitTxWithArgument(ctx context.Context, txTypeArgument ...interface{}) error {
	b, err := c.newPerunTransactionBuilder(nil)
	if err != nil {
		return fmt.Errorf("creating Perun transaction builder: %w", err)
	}

	tx, err := b.Build(txTypeArgument)
	if err != nil {
		return fmt.Errorf("building transaction: %w", err)
	}
	return c.submitTx(ctx, tx)
}

// mkMyCKBCellIterator returns a celliterator together with the associated script
// hash it is meant to be used with.
func (c Client) mkMyCKBCellIterator() (collector.CellIterator, types.Hash, error) {
	defaultLockScript := c.signer.Address().Script
	key := &indexer.SearchKey{
		Script:     defaultLockScript,
		ScriptType: types.ScriptTypeLock,
		// Use `ScriptSearchModeExact` to make sure we only get cells that have no
		// type script set and could be SUDT cells.
		ScriptSearchMode: types.ScriptSearchModeExact,
		Filter:           nil,
		WithData:         true,
	}
	iter := collector.NewLiveCellIterator(c.client, key)
	return NewCKBOnlyIterator(iter), defaultLockScript.Hash(), nil
}

func (c Client) createOrGetChannelToken(ctx context.Context, iter collector.CellIterator) (backend.Token, error) {
	// TODO: This just takes the first available cell. This should be improved in
	// the next version, where we make sure to reuse the funding cells instead
	// of a dedicated cell for the channel token.
	if !iter.HasNext() {
		return backend.Token{}, errors.New("sending account has no funds available")
	}
	transactionInput := iter.Next()
	channelToken := molecule.
		NewChannelTokenBuilder().
		OutPoint(*transactionInput.OutPoint.Pack()).
		Build()
	return backend.Token{
		Idx:      transactionInput.OutPoint.Index,
		Outpoint: *transactionInput.OutPoint.Pack(),
		Token:    channelToken,
	}, nil
}

func (c Client) Fund(ctx context.Context, pcts *types.Script, state *channel.State, params *channel.Params) error {
	channelCell, err := c.getExactChannelLiveCell(ctx, pcts)
	if err != nil {
		return fmt.Errorf("getting channel live cell: %w", err)
	}
	header, err := c.client.GetTipHeader(ctx)
	if err != nil {
		return fmt.Errorf("getting tip header: %w", err)
	}
	channelStatus, err := molecule.ChannelStatusFromSlice(channelCell.OutputData, false)
	if err != nil {
		return err
	}

	fi := transaction.NewFundInfo(*channelCell.OutPoint, params, state, pcts, *channelStatus, header.Hash)
	builder, err := c.newPerunTransactionBuilder(nil)
	if err != nil {
		return fmt.Errorf("creating Perun transaction builder: %w", err)
	}
	if err := builder.Fund(fi); err != nil {
		return fmt.Errorf("creating fund transaction: %w", err)
	}
	tx, err := builder.Build()
	if err != nil {
		return fmt.Errorf("building fund transaction: %w", err)
	}
	return c.submitTx(ctx, tx)
}

func (c Client) Dispute(ctx context.Context, id channel.ID, state *channel.State, sigs []wallet.Sig, params *channel.Params) error {
	channelCell, status, err := c.getChannelLiveCellWithCache(ctx, id)
	if err != nil {
		return fmt.Errorf("getting channel live cell: %w", err)
	}
	header, err := c.client.GetTipHeader(ctx)
	if err != nil {
		return fmt.Errorf("getting tip header: %w", err)
	}

	if len(sigs) != 2 {
		return fmt.Errorf("expected 2 signatures, got %d", len(sigs))
	}
	sigA := encoding.PackSignature(sigs[0])
	sigB := encoding.PackSignature(sigs[1])
	di := transaction.NewDisputeInfo(*channelCell.OutPoint, *status, params, header.Hash, channelCell.Output.Type, *sigA, *sigB)

	builder, err := c.newPerunTransactionBuilder(nil)
	if err != nil {
		return fmt.Errorf("creating Perun transaction builder: %w", err)
	}
	if err := builder.Dispute(di); err != nil {
		return fmt.Errorf("creating dispute transaction: %w", err)
	}
	tx, err := builder.Build()
	if err != nil {
		return fmt.Errorf("building dispute transaction: %w", err)
	}
	return c.submitTx(ctx, tx)
}

func (c Client) Close(ctx context.Context, id channel.ID, state *channel.State, sigs []wallet.Sig, params *channel.Params) error {
	channelCell, _, err := c.getChannelLiveCellWithCache(ctx, id)
	if err != nil {
		return fmt.Errorf("getting channel live cell: %w", err)
	}
	pcts := channelCell.Output.Type
	assets, err := c.getAssets(ctx, pcts)
	if err != nil {
		return fmt.Errorf("retrieving assets locked in channel: %w", err)
	}
	header, err := c.client.GetTipHeader(ctx)
	if err != nil {
		return fmt.Errorf("getting tip header: %w", err)
	}
	occupiedChannelCapacity := channelCell.Output.OccupiedCapacity(channelCell.OutputData)

	ci := transaction.NewCloseInfo(
		occupiedChannelCapacity,
		types.CellInput{PreviousOutput: channelCell.OutPoint},
		mkCellInputs(assets),
		[]types.Hash{header.Hash},
		params,
		state,
		sigs,
	)

	builder, err := c.newPerunTransactionBuilder(nil)
	if err != nil {
		return fmt.Errorf("creating Perun transaction builder: %w", err)
	}
	if err := builder.Close(ci); err != nil {
		return fmt.Errorf("creating close transaction: %w", err)
	}
	tx, err := builder.Build()
	if err != nil {
		return fmt.Errorf("building close transaction: %w", err)
	}
	return c.submitTx(ctx, tx)
}

// Turns a list of live cells into a list of input cells.
func mkCellInputs(lcs *indexer.LiveCells) []types.CellInput {
	res := make([]types.CellInput, len(lcs.Objects))
	for idx, lc := range lcs.Objects {
		res[idx] = types.CellInput{
			Since:          0,
			PreviousOutput: lc.OutPoint,
		}
	}
	return res
}

// getAssets retrieves a list of all assets that are locked in the channel
// identified by the given PCTS.
func (c Client) getAssets(ctx context.Context, pcts *types.Script) (*indexer.LiveCells, error) {
	pctsScriptHash := pcts.Hash()
	pflsPrefix := &types.Script{
		CodeHash: c.deployment.PFLSCodeHash,
		HashType: c.deployment.PFLSHashType,
		Args:     pctsScriptHash[:],
	}
	searchKey := &indexer.SearchKey{
		Script:           pflsPrefix,
		ScriptType:       types.ScriptTypeLock,
		ScriptSearchMode: types.ScriptSearchModePrefix,
		Filter:           nil,
		WithData:         true,
	}
	return c.client.GetCells(ctx, searchKey, indexer.SearchOrderDesc, math.MaxUint32, "")
}

func (c Client) ForceClose(ctx context.Context, id channel.ID, state *channel.State, params *channel.Params) error {
	channelCell, _, err := c.getChannelLiveCellWithCache(ctx, id)
	if err != nil {
		return fmt.Errorf("getting channel live cell: %w", err)
	}
	pcts := channelCell.Output.Type
	assets, err := c.getAssets(ctx, pcts)
	if err != nil {
		return fmt.Errorf("retrieving assets locked in channel: %w", err)
	}
	header, err := c.client.GetTipHeader(ctx)
	if err != nil {
		return fmt.Errorf("getting tip header: %w", err)
	}
	occupiedChannelCapacity := channelCell.Output.OccupiedCapacity(channelCell.OutputData)
	fci := transaction.NewForceCloseInfo(
		types.CellInput{PreviousOutput: channelCell.OutPoint},
		mkCellInputs(assets),
		[]types.Hash{header.Hash},
		state,
		params,
		occupiedChannelCapacity,
	)

	builder, err := c.newPerunTransactionBuilder(nil)
	if err != nil {
		return fmt.Errorf("creating Perun transaction builder: %w", err)
	}
	if err := builder.ForceClose(fci); err != nil {
		return fmt.Errorf("creating force close transaction: %w", err)
	}
	tx, err := builder.Build()
	if err != nil {
		return fmt.Errorf("building force close transaction: %w", err)
	}
	return c.submitTx(ctx, tx)
}

func (c Client) Abort(ctx context.Context, script *types.Script, params *channel.Params, state *channel.State) error {
	channelCell, err := c.getExactChannelLiveCell(ctx, script)
	if err != nil {
		return fmt.Errorf("getting channel live cell: %w", err)
	}
	pcts := channelCell.Output.Type
	assets, err := c.getAssets(ctx, pcts)
	if err != nil {
		return fmt.Errorf("retrieving assets locked in channel: %w", err)
	}
	header, err := c.client.GetTipHeader(ctx)
	if err != nil {
		return fmt.Errorf("getting tip header: %w", err)
	}
	occupiedChannelCapacity := channelCell.Output.OccupiedCapacity(channelCell.OutputData)
	ai := transaction.NewAbortInfo(
		types.CellInput{PreviousOutput: channelCell.OutPoint},
		mkCellInputs(assets),
		state,
		params,
		[]types.Hash{header.Hash},
		occupiedChannelCapacity,
	)

	builder, err := c.newPerunTransactionBuilder(nil)
	if err != nil {
		return fmt.Errorf("creating Perun transaction builder: %w", err)
	}
	if err := builder.Abort(ai); err != nil {
		return fmt.Errorf("creating abort transaction: %w", err)
	}
	tx, err := builder.Build()
	if err != nil {
		return fmt.Errorf("building abort transaction: %w", err)
	}
	return c.submitTx(ctx, tx)
}

func (c Client) GetChannelWithExactPCTS(ctx context.Context, pcts *types.Script) (BlockNumber, *molecule.ChannelStatus, error) {
	cell, err := c.getExactChannelLiveCell(ctx, pcts)
	if err != nil {
		return 0, nil, fmt.Errorf("getting exact channel live cell: %w", err)
	}
	channelStatus, err := molecule.ChannelStatusFromSlice(cell.OutputData, false)
	if err != nil {
		return 0, nil, err
	}
	return cell.BlockNumber, channelStatus, nil
}

const defaultPollingInterval = 2 * time.Second

// sendAndAwait sends the given transaction and waits for it to be committed
// on-chain.
func (c Client) sendAndAwait(ctx context.Context, tx *types.Transaction) error {
	txHash, err := c.client.SendTransaction(ctx, tx)
	if err != nil {
		return fmt.Errorf("sending transaction: %w", err)
	}

	// Wait for the transaction to be committed on-chain.
	txWithStatus, err := c.client.GetTransaction(ctx, *txHash)
	if err != nil {
		return fmt.Errorf("initially polling transaction: %w", err)
	}

	ticker := time.NewTicker(defaultPollingInterval)
	for txWithStatus.TxStatus.Status != types.TransactionStatusCommitted {
		if txWithStatus.TxStatus.Status == types.TransactionStatusRejected {
			return fmt.Errorf("transaction rejected with: %v", *txWithStatus.TxStatus.Reason)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("context done: %w", ctx.Err())
		case <-ticker.C:
			txWithStatus, err = c.client.GetTransaction(ctx, *txHash)
			if err != nil {
				return fmt.Errorf("polling transaction: %w", err)
			}
		}
	}

	return nil
}

func (c Client) GetChannelWithID(ctx context.Context, id channel.ID) (BlockNumber, *types.Script, *molecule.ChannelConstants, *molecule.ChannelStatus, error) {
	cell, status, err := c.getChannelLiveCellWithCache(ctx, id)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	log.Println("GetChannelWithID: got channel live cell")
	channelConstants, err := molecule.ChannelConstantsFromSlice(cell.Output.Type.Args, false)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	return cell.BlockNumber, cell.Output.Type, channelConstants, status, nil
}

func (c Client) getFirstChannelLiveCellWithID(channels *indexer.LiveCells, id channel.ID) (*indexer.LiveCell, *molecule.ChannelStatus, error) {
	for _, cell := range channels.Objects {
		if !c.isValidChannelLiveCell(cell) {
			continue
		}
		channelStatus, err := molecule.ChannelStatusFromSlice(cell.OutputData, false)
		if err != nil {
			continue
		}
		if types.UnpackHash(channelStatus.State().ChannelId()) != id {
			continue
		}
		return cell, channelStatus, nil
	}
	return nil, nil, ErrNoChannelLiveCell
}

func (c Client) getAllChannelLiveCells(ctx context.Context) (*indexer.LiveCells, error) {
	pctsPrefix := &types.Script{
		CodeHash: c.deployment.PCTSCodeHash,
		HashType: c.deployment.PCTSHashType,
		Args:     []byte{},
	}
	searchKey := &indexer.SearchKey{
		Script:           pctsPrefix,
		ScriptType:       types.ScriptTypeType,
		ScriptSearchMode: types.ScriptSearchModePrefix,
		Filter:           nil,
		WithData:         true,
	}
	return c.client.GetCells(ctx, searchKey, indexer.SearchOrderDesc, math.MaxUint32, "")
}

func (c Client) getExactChannelLiveCell(ctx context.Context, pcts *types.Script) (*indexer.LiveCell, error) {
	searchKey := &indexer.SearchKey{
		Script:           pcts,
		ScriptType:       types.ScriptTypeType,
		ScriptSearchMode: types.ScriptSearchModeExact,
		Filter:           nil,
		WithData:         true,
	}
	cells, err := c.client.GetCells(ctx, searchKey, indexer.SearchOrderDesc, math.MaxUint32, "")
	log.Println("getExactChannelLiveCell: GetCells")
	if err != nil {
		log.Println("getExactChannelLiveCell: GetCells error: ", err)
		return nil, err
	}
	if len(cells.Objects) > 1 {
		return nil, errors.New("more than one live cell found for channel")
	}
	if len(cells.Objects) == 0 {
		return nil, ErrNoChannelLiveCell
	}
	return cells.Objects[0], nil
}

func (c Client) isValidChannelLiveCell(cell *indexer.LiveCell) bool {
	if cell.Output == nil ||
		cell.Output.Type == nil ||
		cell.Output.Type.CodeHash != c.deployment.PCTSCodeHash ||
		cell.Output.Type.HashType != c.deployment.PCTSHashType {
		return false
	}
	return true
}

func (c Client) GetBlockTime(ctx context.Context, blockNumber BlockNumber) (time.Time, error) {
	block, err := c.client.GetBlockByNumber(ctx, blockNumber)
	if err != nil {
		return time.Time{}, err
	}
	if block.Header.Timestamp > math.MaxInt64 {
		return time.Time{}, errors.New("block timestamp is too large")
	}
	return time.UnixMilli(int64(block.Header.Timestamp)), nil
}

func (c Client) getChannelLiveCellWithCache(ctx context.Context, id channel.ID) (*indexer.LiveCell, *molecule.ChannelStatus, error) {
	script, cached := c.cache.Get(id)
	log.Println("getChannelLiveCellWithCache: cached?", cached)
	if cached {
		cell, err := c.getExactChannelLiveCell(ctx, script)
		if err != nil {
			return nil, nil, err
		}
		status, err := molecule.ChannelStatusFromSlice(cell.OutputData, false)
		if err != nil {
			return nil, nil, fmt.Errorf("converting cell outputdata to ChannelStatus: %w", err)
		}
		return cell, status, nil
	}
	liveChannelCells, err := c.getAllChannelLiveCells(ctx)
	log.Println("getChannelLiveCellWithCache: getAllChannelLiveCells returned error: ", err)
	if err != nil {
		return nil, nil, err
	}
	cell, status, err := c.getFirstChannelLiveCellWithID(liveChannelCells, id)
	if err != nil {
		return nil, nil, err
	}
	errCache := c.cache.Set(id, cell.Output.Type)
	if errCache != nil {
		return c.getChannelLiveCellWithCache(ctx, id)
	}
	return cell, status, err
}
