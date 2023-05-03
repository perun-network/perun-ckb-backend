package client

import (
	"context"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
)

type CKBClient interface {
	// Start starts a new channel on-chain with the given parameters and initial state.
	// It returns the resulting channel token or an error.
	// Start should block until the starting transaction is committed on-chain.
	// The implementation can assume that Start will only ever be performed by Party A.
	Start(ctx context.Context, params *channel.Params, state *channel.State) (*molecule.ChannelToken, error)

	// Abort aborts the channel with the given channel token.
	Abort(ctx context.Context, token *molecule.ChannelToken) error

	// Fund funds the channel with the given channel token. The implementation can assume that Fund will only ever
	// be performed by Party B.
	Fund(ctx context.Context, token *molecule.ChannelToken) error

	// GetChannelStatus returns the on-chain status of the channel with the given channel token or an error.
	GetChannelStatus(ctx context.Context, token *molecule.ChannelToken) (*molecule.ChannelStatus, error)

	// GetChannelWithID returns an on-chain channel with the given channel ID.
	// Note: Only the channel ID field in the state must be verified checked, as the pcts verifies the integrity of said
	// field upon channel start (i.e. that it is equal to the hash of the channel parameters).
	// If there are multiple channels with the same ID, the implementation can return any of them, but the returned
	// constants and status must belong to the same channel.
	GetChannelWithID(ctx context.Context, id channel.ID) (*molecule.ChannelConstants, *molecule.ChannelStatus, error)
}
