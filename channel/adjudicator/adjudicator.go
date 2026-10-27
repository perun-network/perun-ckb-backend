package adjudicator

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pkg/errors"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/client"
)

type Adjudicator struct {
	client          client.CKBClient
	pollingInterval time.Duration
}

// Option configures an Adjudicator.
type Option func(*Adjudicator)

// WithPollingInterval sets how often subscriptions created by Subscribe poll
// the chain for channel updates. It panics if interval is not positive.
func WithPollingInterval(interval time.Duration) Option {
	if interval <= 0 {
		panic(fmt.Sprintf("adjudicator: polling interval must be positive, got %v", interval))
	}
	return func(a *Adjudicator) {
		a.pollingInterval = interval
	}
}

func NewAdjudicator(client client.CKBClient, opts ...Option) *Adjudicator {
	a := &Adjudicator{client: client, pollingInterval: DefaultSubscriptionPollingInterval}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a Adjudicator) Register(ctx context.Context, req channel.AdjudicatorReq, states []channel.SignedState) error {
	// If sub-states are present, register them first
	if len(states) > 0 {
		vcstate := states[0]
		indexMap := req.Tx.Locked[0].IndexMap

		if err := a.client.DisputeVC(ctx, vcstate.State.ID, req.Tx.ID, vcstate.State, req.Tx.State, vcstate.Params, req.Params, vcstate.Sigs, req.Tx.Sigs, indexMap); err != nil {
			return errors.WithMessage(err, "failed to dispute virtual channel")
		}
		return nil // Only one virtual channel is supported

	}
	return a.client.Dispute(ctx, req.Tx.ID, req.Tx.State, req.Tx.Sigs, req.Params)
}

func (a Adjudicator) Withdraw(ctx context.Context, req channel.AdjudicatorReq, stateMap channel.StateMap) error {
	if req.Secondary { // Secondary withdraw already handled by first withdraw.
		log.Println("Adjudicator: Secondary withdraw already handled by first withdraw.")
		return nil
	}
	if req.Tx.IsFinal {
		return a.client.Close(ctx, req.Tx.ID, req.Tx.State, req.Tx.Sigs, req.Params)
	} else {
		// Check length of state map: currentyl only one virtual channel is supported
		if len(stateMap) > 0 {
			if len(stateMap) > 1 {
				return errors.New("only one virtual channel is supported")
			}

			// Force Close with Virtual Channel.
			for _, vcstate := range stateMap {
				indexMap := req.Tx.Locked[0].IndexMap
				return a.client.ForceCloseWithVC(ctx, req.Tx.ID, vcstate.State.ID, req.Tx.State, vcstate.State, req.Tx.Sigs, req.Params, indexMap)
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
	return NewAdjudicatorSubFromChannelID(ctx, a.client, id, WithSubscriptionPollingInterval(a.pollingInterval)), nil
}
