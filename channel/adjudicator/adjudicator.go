package adjudicator

import (
	"context"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/client"
)

type Adjudicator struct {
	client client.CKBClient
}

func (a Adjudicator) Register(ctx context.Context, req channel.AdjudicatorReq, states []channel.SignedState) error {
	return a.client.Dispute(ctx, req.Tx.ID, req.Tx.State, req.Tx.Sigs, req.Params)
}

func (a Adjudicator) Withdraw(ctx context.Context, req channel.AdjudicatorReq, stateMap channel.StateMap) error {
	if req.Tx.State.IsFinal {
		return a.client.Close(ctx, req.Tx.ID, req.Tx.State, req.Tx.Sigs, req.Params)
	} else {
		return a.client.ForceClose(ctx, req.Tx.ID, req.Tx.State, req.Params)
	}

}

func (a Adjudicator) Progress(ctx context.Context, req channel.ProgressReq) error {
	// Progress only needed for state channels
	panic("unimplemented: Progress only needed for state channels")
}

func (a Adjudicator) Subscribe(ctx context.Context, id channel.ID) (channel.AdjudicatorSubscription, error) {
	return NewAdjudicatorSubFromChannelID(ctx, a.client, id), nil
}
