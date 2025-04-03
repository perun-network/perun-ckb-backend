package adjudicator

import (
	"context"

	"github.com/pkg/errors"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/client"
)

type Adjudicator struct {
	client client.CKBClient
}

func NewAdjudicator(client client.CKBClient) *Adjudicator {
	return &Adjudicator{client: client}
}

func (a Adjudicator) Register(ctx context.Context, req channel.AdjudicatorReq, states []channel.SignedState) error {
	// If sub-states are present, register them first
	if states != nil && len(states) > 0 {
		for i, vcstate := range states {
			indexMap := req.Tx.State.Locked[i].IndexMap

			if err := a.client.DisputeVC(ctx, vcstate.State.ID, vcstate.State, vcstate.Sigs, vcstate.Params, req.Tx.Sigs, req.Tx.ID, indexMap); err != nil {
				return errors.WithMessage(err, "failed to dispute virtual channel")
			}
			return nil // Only one virtual channel is supported
		}
	}
	return a.client.Dispute(ctx, req.Tx.ID, req.Tx.State, req.Tx.Sigs, req.Params)
}

func (a Adjudicator) Withdraw(ctx context.Context, req channel.AdjudicatorReq, stateMap channel.StateMap) error {
	if req.Secondary { // Secondary withdraw already handled by first withdraw.
		return nil
	}
	if req.Tx.State.IsFinal {
		return a.client.Close(ctx, req.Tx.ID, req.Tx.State, req.Tx.Sigs, req.Params)
	} else {
		// Check length of state map: currentyl only one virtual channel is supported
		if len(stateMap) > 0 {
			if len(stateMap) > 1 {
				return errors.New("only one virtual channel is supported")
			}

			// Force Close with Virtual Channel.
			for _, vcstate := range stateMap {
				// Check if the virtual channel state is Final
				if !vcstate.State.IsFinal {
					return errors.New("virtual channel state is not final")
				}

				return a.client.ForceCloseWithVC(ctx, req.Tx.ID, vcstate.State.ID, req.Tx.State, vcstate.State, req.Tx.Sigs, vcstate.Sigs, req.Params)
			}
		}

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
