package client

import (
	"context"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
)

type CKBClient interface {
	Start(ctx context.Context, params *channel.Params, state *channel.State) (*molecule.ChannelToken, error)
	Abort(ctx context.Context, token *molecule.ChannelToken) error
	Fund(ctx context.Context, token *molecule.ChannelToken) error
	GetChannelStatus(ctx context.Context, token *molecule.ChannelToken) (*molecule.ChannelStatus, error)
	GetChannelWithID(ctx context.Context, id channel.ID) (*molecule.ChannelConstants, *molecule.ChannelStatus, error)
}
