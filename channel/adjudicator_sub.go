package channel

import (
	"bytes"
	"context"
	"fmt"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/client"
	"time"
)

const (
	DefaultBufferSize                  = 3
	DefaultSubscriptionPollingInterval = time.Duration(4) * time.Second
)

type AdjudicatorSubscription struct {
	PollingInterval time.Duration
	client          client.CKBClient
	id              channel.ID
	pcts            *types.Script
	events          chan channel.AdjudicatorEvent
	err             error
}

func NewAdjudicatorSubFromChannelID(ctx context.Context, ckbClient client.CKBClient, id channel.ID) *AdjudicatorSubscription {
	return &AdjudicatorSubscription{
		PollingInterval: DefaultSubscriptionPollingInterval,
		client:          ckbClient,
		id:              id,
		events:          make(chan channel.AdjudicatorEvent, DefaultBufferSize),
	}
}

func (a *AdjudicatorSubscription) run(ctx context.Context) {
	finish := func() {
		a.err = fmt.Errorf("subscription closed by context: %w", ctx.Err())
		close(a.events)
	}
	var oldStatus *molecule.ChannelStatus
	for {
		select {
		case <-ctx.Done():
			finish()
			return
		case <-time.After(a.PollingInterval):
			newStatus, err := a.pollStatus(ctx)
			if err != nil {
				// TODO: What happens if the channel is closed on-chain?
				continue
			}
			didEmitEvent := a.emitEventIfNecessary(oldStatus, newStatus)
			if didEmitEvent {
				oldStatus = newStatus
			}
		}

	}
}

func (a *AdjudicatorSubscription) pollStatus(ctx context.Context) (*molecule.ChannelStatus, error) {
	if a.pcts != nil {
		return a.client.GetChannelWithExactPCTS(ctx, a.pcts)
	}
	pcts, _, status, err := a.client.GetChannelWithID(ctx, a.id)
	if err != nil {
		return nil, err
	}
	a.pcts = pcts
	return status, nil
}

func (a *AdjudicatorSubscription) emitEventIfNecessary(oldStatus *molecule.ChannelStatus, newStatus *molecule.ChannelStatus) bool {
	if newStatus == nil {
		// TODO: How can this happen?
		return false
	}
	if oldStatus != nil && bytes.Equal(oldStatus.AsSlice(), newStatus.AsSlice()) {
		return false
	}
	// TODO: Decode event
	panic("decode and push event")
}

func (a *AdjudicatorSubscription) EventStream() <-chan channel.AdjudicatorEvent {
	return a.events
}

func (a *AdjudicatorSubscription) Err() error {
	return a.err
}
