package client

import (
	"context"
	"errors"
	"fmt"
	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"math"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/channel/defaults"
)

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

	// GetChannelWithID returns an on-chain channel with the given channel ID.
	// Note: Only the channel ID field in the state must be verified checked, as the pcts verifies the integrity of said
	// field upon channel start (i.e. that it is equal to the hash of the channel parameters).
	// If there are multiple channels with the same ID, the implementation can return any of them, but the returned
	// constants and status must belong to the same channel.
	GetChannelWithID(ctx context.Context, id channel.ID) (*types.Script, *molecule.ChannelConstants, *molecule.ChannelStatus, error)

	// GetChannelWithExactPCTS return the on-chain channel status for the given type script.
	GetChannelWithExactPCTS(ctx context.Context, pcts *types.Script) (*molecule.ChannelStatus, error)
}

type Client struct {
	client       rpc.Client
	PCTSCodeHash types.Hash
	PCTSHashType types.ScriptHashType
	cache        map[channel.ID]*types.Script
}

func (c Client) Start(ctx context.Context, params *channel.Params, state *channel.State) (*types.Script, error) {
	//TODO implement me
	panic("implement me")
}

func (c Client) Abort(ctx context.Context, script *types.Script) error {
	//TODO implement me
	panic("implement me")
}

func (c Client) GetChannelWithExactPCTS(ctx context.Context, pcts *types.Script) (*molecule.ChannelStatus, error) {
	cells, err := c.getExactChannelLiveCell(ctx, pcts)
	if err != nil {
		return nil, err
	}
	if cells == nil {
		return nil, errors.New("unable to get channel live cell")
	}
	if len(cells.Objects) != 1 {
		return nil, fmt.Errorf("expected exactly 1 live channel cell, got: %d", len(cells.Objects))
	}
	channelStatus, err := molecule.ChannelStatusFromSlice(cells.Objects[0].OutputData, false)
	if err != nil {
		return nil, err
	}
	return channelStatus, nil
}

func NewDefaultClient(rpcClient rpc.Client) *Client {
	return &Client{
		client:       rpcClient,
		PCTSCodeHash: defaults.DefaultPCTSCodeHash,
		PCTSHashType: defaults.DefaultPCTSHashType,
		cache:        map[channel.ID]*types.Script{},
	}
}

func (c Client) Fund(ctx context.Context, pcts *types.Script) error {
	//TODO implement me
	panic("implement me")
}

func (c Client) GetChannelWithID(ctx context.Context, id channel.ID) (*types.Script, *molecule.ChannelConstants, *molecule.ChannelStatus, error) {
	script, cached := c.cache[id]
	if cached {
		channelStatus, err := c.GetChannelWithExactPCTS(ctx, script)
		if err != nil {
			return nil, nil, nil, err
		}
		channelConstants, err := molecule.ChannelConstantsFromSlice(script.Args, false)
		return script, channelConstants, channelStatus, err
	}

	liveChannelCells, err := c.getAllChannelLiveCells(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	return c.getFirstChannelWithID(liveChannelCells, id)
}

func (c Client) getFirstChannelWithID(channels *indexer.LiveCells, id channel.ID) (*types.Script, *molecule.ChannelConstants, *molecule.ChannelStatus, error) {
	for _, cell := range channels.Objects {
		if !c.isValidChannelLiveCell(cell) {
			continue
		}
		// TODO: What does `compatible` do?
		channelStatus, err := molecule.ChannelStatusFromSlice(cell.OutputData, false)
		if err != nil {
			continue
		}
		if types.UnpackHash(channelStatus.State().ChannelId()) != id {
			continue
		}
		channelConstants, err := molecule.ChannelConstantsFromSlice(cell.Output.Type.Args, false)
		if err != nil {
			return nil, nil, nil, err
		}
		return cell.Output.Type, channelConstants, channelStatus, nil
	}
	return nil, nil, nil, errors.New("channel for channel id not found")
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
