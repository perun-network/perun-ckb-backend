package client

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/systemscript"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/backend"
	ckbchannel "perun.network/perun-ckb-backend/channel"
	"perun.network/perun-ckb-backend/channel/asset"
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
	Abort(ctx context.Context, pcts *types.Script) error

	// Fund funds the channel with the given channel token. The implementation can assume that Fund will only ever
	// be performed by Party B.
	Fund(ctx context.Context, pcts *types.Script, state *channel.State) error

	// Dispute registers a dispute for the channel with the given channel ID on chain.
	// It should register the given state with the given signatures as witness.
	// Note: The given signatures are padded (see encoding.NewDEREncodedSignatureFromPadded).
	Dispute(ctx context.Context, id channel.ID, state *channel.State, sigs []wallet.Sig) error

	// Close closes the channel with the given channel ID on chain.
	// The implementation can assume that the given state is final.
	// Note: The given signatures are padded (see encoding.NewDEREncodedSignatureFromPadded).
	Close(ctx context.Context, id channel.ID, state *channel.State, sigs []wallet.Sig) error

	// ForceClose closes the channel with the given channel ID on chain.
	// The implementation can assume that the channel has already been disputed and that the challenge duration
	// is expired in real-time, though it may be necessary to wait until a block is produced with a timestamp strictly
	// later than the expiration of the challenge duration.
	ForceClose(ctx context.Context, id channel.ID, state *channel.State) error

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

type Client struct {
	client rpc.Client

	signer     backend.Signer
	deployment backend.Deployment

	psh   transaction.PerunScriptHandler
	cache StableScriptCache
}

var _ CKBClient = (*Client)(nil)

func (c Client) Start(ctx context.Context, params *channel.Params, state *channel.State) (*types.Script, error) {
	channelToken, err := c.createChannelToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating channel token: %w", err)
	}
	cid := ckbchannel.Backend.CalcID(params)
	funding := state.Balance(channel.Index(0), asset.Asset)
	oi := &transaction.OpenInfo{
		ChannelID:    cid,
		ChannelToken: channelToken,
		Funding:      funding.Uint64(),
		Params:       params,
		State:        state,
	}

	if err := c.submitTxWithArgument(ctx, oi); err != nil {
		return nil, fmt.Errorf("submitting transaction: %w", err)
	}

	return oi.GetPCTS(), nil
}

func (c Client) newPerunTransactionBuilder() (*transaction.PerunTransactionBuilder, error) {
	sender, err := c.signer.Address.Encode()
	if err != nil {
		return nil, fmt.Errorf("encoding sender address: %w", err)
	}
	iter, err := collector.NewLiveCellIteratorFromAddress(c.client, sender)
	if err != nil {
		return nil, fmt.Errorf("creating cell iterator: %w", err)
	}
	return transaction.NewPerunTransactionBuilder(c.deployment.Network, iter, &c.psh, c.signer.Address)

}

// submitTxWithArgument submits a transaction whose type is determined by the
// txTypeArgument.
func (c Client) submitTxWithArgument(ctx context.Context, txTypeArgument ...interface{}) error {
	b, err := c.newPerunTransactionBuilder()
	if err != nil {
		return fmt.Errorf("creating Perun transaction builder: %w", err)
	}

	tx, err := b.Build(txTypeArgument)
	if err != nil {
		return fmt.Errorf("building open transaction: %w", err)
	}

	if err := c.signer.SignTransaction(tx); err != nil {
		return fmt.Errorf("signing open transaction: %w", err)
	}

	if err := c.sendAndAwait(ctx, tx.TxView); err != nil {
		return fmt.Errorf("sending open transaction: %w", err)
	}
	return nil
}

func (c Client) createChannelToken(ctx context.Context) (backend.Token, error) {
	// TODO: We could also just use the first normal input cell instead of a
	// dedicated channel token cell.
	panic("implement me")
}

func (c Client) Fund(ctx context.Context, pcts *types.Script, state *channel.State) error {
	amount := state.Balance(channel.Index(1), asset.Asset)
	channelCell, err := c.getExactChannelLiveCell(ctx, pcts)
	if err != nil {
		return fmt.Errorf("getting channel live cell: %w", err)
	}
	fi := transaction.FundInfo{
		Amount:      amount.Uint64(),
		ChannelCell: *channelCell.OutPoint,
		Params:      &channel.Params{},
		Token:       backend.Token{},
		Status:      molecule.ChannelStatus{},
	}
	return c.submitTxWithArgument(ctx, fi)
}

func (c Client) Dispute(ctx context.Context, id channel.ID, state *channel.State, sigs []wallet.Sig) error {
	channelCell, status, err := c.getChannelLiveCellWithCache(ctx, id)
	if err != nil {
		return fmt.Errorf("getting channel live cell: %w", err)
	}
	header, err := c.client.GetTipHeader(ctx)
	if err != nil {
		return fmt.Errorf("getting tip header: %w", err)
	}
	constants, err := molecule.ChannelConstantsFromSlice(channelCell.Output.Type.Args, false)
	if err != nil {
		return fmt.Errorf("parsing channel constants: %w", err)
	}
	params, err := encoding.UnpackChannelParameters(*constants.Params())
	if err != nil {
		return fmt.Errorf("parsing channel parameters: %w", err)
	}

	if len(sigs) != 2 {
		return fmt.Errorf("expected 2 signatures, got %d", len(sigs))
	}
	sigA := encoding.PackSignature(sigs[0])
	sigB := encoding.PackSignature(sigs[1])
	di := transaction.DisputeInfo{
		ChannelCell: *channelCell.OutPoint,
		Status:      *status,
		Params:      params,
		Header:      header.Hash,
		Token:       *constants.ThreadToken(),
		SigA:        *sigA,
		SigB:        *sigB,
	}
	return c.submitTxWithArgument(ctx, di)
}

func (c Client) Close(ctx context.Context, id channel.ID, state *channel.State, sigs []wallet.Sig) error {
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

	constants, err := molecule.ChannelConstantsFromSlice(channelCell.Output.Type.Args, false)
	if err != nil {
		return fmt.Errorf("parsing channel constants: %w", err)
	}
	params, err := encoding.UnpackChannelParameters(*constants.Params())
	if err != nil {
		return fmt.Errorf("parsing channel parameters: %w", err)
	}

	ci := transaction.CloseInfo{
		ChannelCapacity: occupiedChannelCapacity,
		ChannelInput: types.CellInput{
			PreviousOutput: channelCell.OutPoint,
		},
		AssetInputs:      mkCellInputs(assets),
		Headers:          []types.Hash{header.Hash},
		Params:           params,
		State:            state,
		PaddedSignatures: sigs,
	}

	return c.submitTxWithArgument(ctx, ci)
}

// Turns a list of live cells into a list of input cells.
func mkCellInputs(lcs *indexer.LiveCells) []types.CellInput {
	res := make([]types.CellInput, 0, len(lcs.Objects))
	for _, lc := range lcs.Objects {
		res = append(res, types.CellInput{
			Since:          0,
			PreviousOutput: lc.OutPoint,
		})
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
	return c.client.GetCells(ctx, searchKey, indexer.SearchOrderDesc, math.MaxUint64, "")
}

func (c Client) ForceClose(ctx context.Context, id channel.ID, state *channel.State) error {
	//TODO implement me
	panic("implement me")
}

func (c Client) Abort(ctx context.Context, script *types.Script) error {
	//TODO implement me
	panic("implement me")
}

func (c Client) GetChannelWithExactPCTS(ctx context.Context, pcts *types.Script) (BlockNumber, *molecule.ChannelStatus, error) {
	cell, err := c.getExactChannelLiveCell(ctx, pcts)
	channelStatus, err := molecule.ChannelStatusFromSlice(cell.OutputData, false)
	if err != nil {
		return 0, nil, err
	}
	return cell.BlockNumber, channelStatus, nil
}

func NewDefaultClient(rpcClient rpc.Client) *Client {
	// TODO: Wrap this up.
	return &Client{
		client: rpcClient,
		cache:  NewStableScriptCache(),
	}
}

func NewClient(rpcClient rpc.Client, deployment backend.Deployment) (*Client, error) {
	return &Client{
		client:     rpcClient,
		deployment: deployment,
		cache:      nil,
	}, nil
}

// findLiveCKBCells finds one or more live cells containing at least the given
// capacity belonging to this client.
func (c Client) findLiveCKBCells(ctx context.Context, wanted uint64, pubkey *secp256k1.PublicKey) ([]molecule.CellInput, error) {
	defaultLockscript, err := systemscript.Secp256K1Blake160SignhashAllByPublicKey(pubkey.SerializeCompressed())
	if err != nil {
		return nil, fmt.Errorf("generating default lockscript: %w", err)
	}
	searchKey := &indexer.SearchKey{
		Script:           defaultLockscript,
		ScriptType:       types.ScriptTypeType,
		ScriptSearchMode: types.ScriptSearchModeExact,
		Filter:           nil,
	}

	iter := collector.NewLiveCellIterator(c.client, searchKey)
	return cellsContainingAtLeastValue(wanted, iter)
}

func cellsContainingAtLeastValue(value uint64, iter collector.CellIterator) ([]molecule.CellInput, error) {
	cells := make([]molecule.CellInput, 0, 1)
	accumulatedCapacity := uint64(0)
	for iter.HasNext() {
		cell := iter.Next()
		accumulatedCapacity += cell.Output.Capacity
		cells = append(cells, molecule.NewCellInputBuilder().PreviousOutput(*cell.OutPoint.Pack()).Build())
		if accumulatedCapacity >= value {
			break
		}
	}

	if accumulatedCapacity < value {
		return nil, fmt.Errorf("not enough capacity, wanted %d, got %d", value, accumulatedCapacity)
	}

	return cells, nil
}

const defaultPollingInterval = 4 * time.Second

// sendAndAwait sends the given transaction and waits for it to be committed
// on-chain.
func (c Client) sendAndAwait(ctx context.Context, tx *types.Transaction) error {
	txHash, err := c.client.SendTransaction(ctx, tx)
	if err != nil {
		return fmt.Errorf("sending transaction: %w", err)
	}

	// Wait for the transaction to be committed on-chain.
	txWithStatus := &types.TransactionWithStatus{}
	ticker := time.NewTicker(defaultPollingInterval)
	for !(txWithStatus.TxStatus.Status == types.TransactionStatusCommitted ||
		txWithStatus.TxStatus.Status == types.TransactionStatusRejected) {
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
	return c.client.GetCells(ctx, searchKey, indexer.SearchOrderDesc, math.MaxUint64, "")
}

func (c Client) getExactChannelLiveCell(ctx context.Context, pcts *types.Script) (*indexer.LiveCell, error) {
	searchKey := &indexer.SearchKey{
		Script:           pcts,
		ScriptType:       types.ScriptTypeType,
		ScriptSearchMode: types.ScriptSearchModeExact,
		Filter:           nil,
		WithData:         true,
	}
	cells, err := c.client.GetCells(ctx, searchKey, indexer.SearchOrderDesc, math.MaxUint64, "")
	if err != nil {
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
	if cached {
		cell, err := c.getExactChannelLiveCell(ctx, script)
		if err != nil {
			return nil, nil, err
		}
		status, err := molecule.ChannelStatusFromSlice(cell.OutputData, false)
		return cell, status, nil
	}
	liveChannelCells, err := c.getAllChannelLiveCells(ctx)
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
