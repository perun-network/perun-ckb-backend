package client

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/channel/defaults"
	"perun.network/perun-ckb-backend/encoding"
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
	Fund(ctx context.Context, pcts *types.Script) error

	// Dispute registers a dispute for the channel with the given channel ID on chain.
	// It should register the given state with the given signatures as witness.
	Dispute(ctx context.Context, id channel.ID, state *channel.State, sigs []wallet.Sig) error

	// Close closes the channel with the given channel ID on chain.
	// The implementation can assume that the given state is final.
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
	client       rpc.Client
	index        channel.Index
	PCTSCodeHash types.Hash
	PCTSHashType types.ScriptHashType
	PCLSCodeHash types.Hash
	PCLSHashType types.ScriptHashType
	PFLSCodeHash types.Hash
	PFLSHashType types.ScriptHashType
	lockOutPoint types.OutPoint
	cache        StableScriptCache
}

func (c Client) Dispute(ctx context.Context, id channel.ID, state *channel.State, sigs []wallet.Sig) error {
	//TODO implement me
	panic("implement me")
}

func (c Client) Close(ctx context.Context, id channel.ID, state *channel.State, sigs []wallet.Sig) error {
	//TODO implement me
	panic("implement me")
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
	cells, err := c.getExactChannelLiveCell(ctx, pcts)
	if err != nil {
		return 0, nil, err
	}
	if cells == nil {
		return 0, nil, errors.New("unable to get channel live cell")
	}
	if len(cells.Objects) == 0 {
		return 0, nil, ErrNoChannelLiveCell
	}
	if len(cells.Objects) != 1 {
		return 0, nil, fmt.Errorf("expected exactly 1 live channel cell, got: %d", len(cells.Objects))
	}
	channelStatus, err := molecule.ChannelStatusFromSlice(cells.Objects[0].OutputData, false)
	if err != nil {
		return 0, nil, err
	}
	return cells.Objects[0].BlockNumber, channelStatus, nil
}

func NewDefaultClient(rpcClient rpc.Client, index channel.Index) *Client {
	return &Client{
		client:       rpcClient,
		index:        index,
		PCTSCodeHash: defaults.DefaultPCTSCodeHash,
		PCTSHashType: defaults.DefaultPCTSHashType,
		PCLSCodeHash: defaults.DefaultPCLSCodeHash,
		PCLSHashType: defaults.DefaultPCLSHashType,
		PFLSCodeHash: defaults.DefaultPFLSCodeHash,
		PFLSHashType: defaults.DefaultPFLSHashType,
		cache:        NewStableScriptCache(),
	}
}

func (c Client) Fund(ctx context.Context, pcts *types.Script) error {
	//TODO implement me
	panic("implement me")
}

func (c Client) Start(ctx context.Context, params *channel.Params, state *channel.State) (*molecule.ChannelToken, error) {
	channelToken, err := c.createChannelToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating channel token: %w", err)
	}

	inputs, funds, change := c.mkFundingFromAllocation(ctx, state.Allocation)
	inputs = append(inputs, channelToken.AsCellInput())
	channelCell := c.mkInitialChannel(params, state, channelToken)
	ckboutputs := backend.CKBOutputs{}.Append(channelCell).Extend(funds).Extend(change)
	outputs, data := ckboutputs.AsOutputAndData()
	rawTx := molecule.NewRawTransactionBuilder().
		Inputs(molecule.NewCellInputVecBuilder().Extend(inputs).Build()).
		Outputs(outputs).
		OutputsData(data).
		Build()
	mtx := molecule.NewTransactionBuilder().Raw(rawTx).Build()
	tx := types.UnpackTransaction(&mtx)

	if err := c.sendAndAwait(ctx, tx); err != nil {
		return nil, err
	}

	return &channelToken.Token, nil
}

func (c Client) mkInitialChannel(params *channel.Params, state *channel.State, token backend.Token) backend.CKBOutput {
	channelLockScript := c.mkChannelLockScript(params, state)
	channelTypeScript := c.mkChannelTypeScript(params, state, token.Token)
	channelData := c.mkInitialChannelState(params, state)
	channelOutput := molecule.NewCellOutputBuilder().
		Lock(channelLockScript).
		Type(molecule.NewScriptOptBuilder().Set(channelTypeScript).Build()).
		Build()
	return backend.CKBOutput{
		Output: channelOutput,
		Data:   channelData,
	}
}

func (c Client) mkChannelLockScript(params *channel.Params, state *channel.State) molecule.Script {
	lockScriptCodeHash := c.PCLSCodeHash.Pack()
	lockScriptHashType := c.PCLSHashType.Pack()
	return molecule.NewScriptBuilder().CodeHash(*lockScriptCodeHash).HashType(*lockScriptHashType).Build()
}

func (c Client) mkChannelTypeScript(params *channel.Params, state *channel.State, token molecule.ChannelToken) molecule.Script {
	typeScriptCodeHash := c.PCTSCodeHash.Pack()
	typeScriptHashType := c.PCTSHashType.Pack()
	channelConstants := c.mkChannelConstants(params, token)
	channelArgs := types.PackBytes(channelConstants.AsSlice())
	return molecule.NewScriptBuilder().
		CodeHash(*typeScriptCodeHash).
		HashType(*typeScriptHashType).
		Args(*channelArgs).
		Build()
}

func (c Client) mkChannelConstants(params *channel.Params, token molecule.ChannelToken) molecule.ChannelConstants {
	chanParams, err := encoding.PackChannelParameters(params)
	if err != nil {
		panic(err)
	}

	pclsCode := c.PCLSCodeHash.Pack()
	pclsHashType := c.PCLSHashType.Pack()
	pflsCode := c.PFLSCodeHash.Pack()
	pflsHashType := c.PFLSHashType.Pack()
	pflsMinCapacity := backend.MinCapacityForPFLS()

	return molecule.NewChannelConstantsBuilder().
		Params(chanParams).
		PclsCodeHash(*pclsCode).
		PclsHashType(*pclsHashType).
		PflsCodeHash(*pflsCode).
		PflsHashType(*pflsHashType).
		PflsMinCapacity(*types.PackUint64(pflsMinCapacity)).
		ThreadToken(token).
		Build()
}

func (c Client) mkInitialChannelState(params *channel.Params, state *channel.State) molecule.Bytes {
	packedState, err := encoding.PackChannelState(state)
	if err != nil {
		panic(err)
	}
	partyA := channel.Index(0)
	balances := c.mkBalancesForParty(state, partyA)
	status := molecule.NewChannelStatusBuilder().
		State(packedState).
		Funded(encoding.False).
		Disputed(encoding.False).
		Funding(balances).
		Build()
	return *types.PackBytes(status.AsSlice())
}

func (c Client) mkBalancesForParty(state *channel.State, index channel.Index) molecule.Balances {
	balances := molecule.NewBalancesBuilder()
	if index == channel.Index(0) {
		balances = balances.Nth0(*types.PackUint64(state.Allocation.Balance(index, asset.Asset).Uint64()))
	} else { // channel.Index(1), we only have two party channels.
		balances = balances.Nth1(*types.PackUint64(state.Allocation.Balance(index, asset.Asset).Uint64()))
	}
	return balances.Build()
}

func (c Client) mkFundingFromAllocation(ctx context.Context, alloc channel.Allocation) ([]molecule.CellInput, backend.CKBOutputs, backend.CKBOutputs) {
	wanted := alloc.Balance(c.index, asset.Asset).Uint64()
	liveCells, err := c.findLiveCells(ctx, wanted)
	if err != nil {
		panic(err)
	}
	panic("implement me")
}

// findLiveCells finds one or more live cells containing at least the given
// capacity belonging to this client.
func (c Client) findLiveCells(ctx context.Context, wanted uint64) ([]molecule.CellInput, error) {
	// c.client.GetCells(ctx, searchKey, indexer.SearchOrderAsc, indexer.SearchLimit, indexer)
	panic("implement me")
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
	for txWithStatus.TxStatus.Status != types.TransactionStatusCommitted ||
		txWithStatus.TxStatus.Status != types.TransactionStatusRejected {
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

func (c Client) createChannelToken(ctx context.Context) (backend.Token, error) {
	panic("implement me")
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
		CodeHash: c.PCTSCodeHash,
		HashType: c.PCTSHashType,
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

func (c Client) getExactChannelLiveCell(ctx context.Context, pcts *types.Script) (*indexer.LiveCells, error) {
	searchKey := &indexer.SearchKey{
		Script:           pcts,
		ScriptType:       types.ScriptTypeType,
		ScriptSearchMode: types.ScriptSearchModeExact,
		Filter:           nil,
		WithData:         true,
	}
	return c.client.GetCells(ctx, searchKey, indexer.SearchOrderDesc, math.MaxUint64, "")
}

func (c Client) isValidChannelLiveCell(cell *indexer.LiveCell) bool {
	if cell.Output == nil ||
		cell.Output.Type == nil ||
		cell.Output.Type.CodeHash != c.PCTSCodeHash ||
		cell.Output.Type.HashType != c.PCTSHashType {
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
		cells, err := c.getExactChannelLiveCell(ctx, script)
		if err != nil {
			return nil, nil, err
		}
		if len(cells.Objects) > 1 {
			return nil, nil, errors.New("more than one live cell found for channel")
		}
		if len(cells.Objects) == 0 {
			return nil, nil, ErrNoChannelLiveCell
		}
		status, err := molecule.ChannelStatusFromSlice(cells.Objects[0].OutputData, false)
		return cells.Objects[0], status, nil
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

func (c Client) getFeeInputs(ctx context.Context, participant *address.Participant) ([]molecule.CellInput, backend.CKBOutput, error) {
	panic("implement me")
}
