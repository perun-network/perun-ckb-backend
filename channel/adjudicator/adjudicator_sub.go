package adjudicator

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"math"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/client"
	"perun.network/perun-ckb-backend/encoding"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
	"time"
)

const (
	DefaultBufferSize                  = 3
	DefaultSubscriptionPollingInterval = time.Duration(4) * time.Second
)

var ErrSubscriptionClosedByContext = errors.New("subscription closed by context")
var ErrChannelConcluded = errors.New("channel concluded")

type PollingSubscription struct {
	PollingInterval   time.Duration
	client            client.CKBClient
	id                channel.ID
	pcts              *types.Script
	events            chan channel.AdjudicatorEvent
	err               error
	cancel            context.CancelFunc
	foundLiveCellOnce bool
	concluded         chan struct{}
	fatalErrors       chan error
	challengeDuration *time.Duration
}

func NewAdjudicatorSubFromChannelID(ctx context.Context, ckbClient client.CKBClient, id channel.ID) *PollingSubscription {
	sub := &PollingSubscription{
		PollingInterval: DefaultSubscriptionPollingInterval,
		client:          ckbClient,
		id:              id,
		events:          make(chan channel.AdjudicatorEvent, DefaultBufferSize),
		concluded:       make(chan struct{}, 1),
		fatalErrors:     make(chan error, 1),
	}
	ctx, sub.cancel = context.WithCancel(ctx)
	go sub.run(ctx)
	return sub
}

func (a *PollingSubscription) run(ctx context.Context) {
	finish := func(err error) {
		a.err = err
		close(a.events)
	}
	var oldStatus *molecule.ChannelStatus
	for {
		select {
		case err := <-a.fatalErrors:
			finish(err)
			return
		case <-a.concluded:
			finish(ErrChannelConcluded)
			return
		case <-ctx.Done():
			finish(ErrSubscriptionClosedByContext)
			return
		case <-time.After(a.PollingInterval):
			blockNumber, newStatus, err := a.pollStatus(ctx)
			foundLiveCell := true
			if err != nil {
				if !errors.Is(err, client.ErrNoChannelLiveCell) {
					continue
				}
				foundLiveCell = false
			}
			statusDidChange := a.emitEventIfNecessary(ctx, oldStatus, newStatus, blockNumber, foundLiveCell)
			if statusDidChange {
				oldStatus = newStatus
			}
		}

	}
}

func (a *PollingSubscription) pollStatus(ctx context.Context) (client.BlockNumber, *molecule.ChannelStatus, error) {
	if a.pcts != nil {
		return a.client.GetChannelWithExactPCTS(ctx, a.pcts)
	}
	b, pcts, _, status, err := a.client.GetChannelWithID(ctx, a.id)
	if err != nil {
		return 0, nil, err
	}
	a.pcts = pcts
	return b, status, nil
}

// emitEventIfNecessary emits an event if the difference between oldStatus and newStatus indicates an event.
// It returns ture, iff the status has changed.
// Note: The status can change without warranting emission of an event!
func (a *PollingSubscription) emitEventIfNecessary(
	ctx context.Context,
	oldStatus *molecule.ChannelStatus,
	newStatus *molecule.ChannelStatus,
	newBlockNumber client.BlockNumber,
	foundLiveCell bool,
) bool {
	a.foundLiveCellOnce = a.foundLiveCellOnce || foundLiveCell
	if !a.foundLiveCellOnce {
		return false
	}
	if !foundLiveCell {
		// TODO: figure out how to set the timeout and version for concluded events.
		// TODO: Do we want to verify that the channel is actually concluded here?
		a.events <- channel.NewConcludedEvent(a.id, &channel.ElapsedTimeout{}, 0)
		close(a.concluded)
		return true
	}
	if newStatus == nil {
		a.fatalErrors <- fmt.Errorf("a live cell was found but newStatus is nil")
		return false
	}

	// If oldStatus is nil, then this is the first live cell we ever found, so we signal a status change but do not emit
	// an event.
	if oldStatus == nil {
		return true
	}

	// If the status has not changed, we do not emit an event and signal that the status has not changed.
	if bytes.Equal(oldStatus.AsSlice(), newStatus.AsSlice()) {
		return false
	}
	if !encoding.ToBool(*oldStatus.Funded()) {
		return true
	}
	if !encoding.ToBool(*newStatus.Disputed()) {
		a.fatalErrors <- fmt.Errorf(
			"adjudicator_sub: channel received update but is not disputed. oldStatus: %s, newStatus: %s",
			hex.EncodeToString(oldStatus.AsSlice()),
			hex.EncodeToString(newStatus.AsSlice()),
		)
		return false
	}
	challengeDurationStart, err := a.getChallengeDurationStart(ctx, newBlockNumber)
	if err != nil {
		a.fatalErrors <- fmt.Errorf("could not get challenge duration start: %v", err)
		return false
	}

	challengeDuration, err := a.getChallengeDuration()
	if err != nil {
		a.fatalErrors <- fmt.Errorf("could not get challenge duration: %v", err)
		return false
	}

	event := channel.NewRegisteredEvent(
		a.id,
		&channel.TimeTimeout{Time: challengeDurationStart.Add(challengeDuration)},
		molecule2.UnpackUint64(newStatus.State().Version()),
		nil, // only needed for virtual channels
		nil, // only needed for virtual channels
	)
	a.events <- event
	return true
}

// Next returns the next event from the subscription.
// It blocks until an event is available or the subscription is closed.
// It returns nil if the subscription is closed. or an error occurs
func (a *PollingSubscription) Next() channel.AdjudicatorEvent {
	return <-a.events
}

// Err returns the error that caused the subscription to close.
// The returned error is ErrChannelConcluded, iff the channel was concluded.
// The returned error is ErrSubscriptionClosedByContext, iff the subscription was closed by the context or via Close.
func (a *PollingSubscription) Err() error {
	return a.err
}

// Close closes the subscription.
func (a *PollingSubscription) Close() error {
	a.cancel()
	return nil
}

func (a *PollingSubscription) getChallengeDuration() (time.Duration, error) {
	if a.challengeDuration != nil {
		return *a.challengeDuration, nil
	}
	if a.pcts == nil {
		return 0, fmt.Errorf("cannot get challenge duration: pcts not set")
	}
	channelConstants, err := molecule.ChannelConstantsFromSlice(a.pcts.Args, false)
	if err != nil {
		return 0, err
	}

	duration := molecule2.UnpackUint64(channelConstants.Params().ChallengeDuration())
	if duration > math.MaxInt64 {
		panic(fmt.Sprintf("adjudicator_sub: challenge duration %d is too large, max: %d", duration, math.MaxInt64))
	}
	a.challengeDuration = new(time.Duration)
	*a.challengeDuration = time.Duration(duration) * time.Millisecond
	return *a.challengeDuration, nil
}

func (a *PollingSubscription) getChallengeDurationStart(ctx context.Context, blockNumber client.BlockNumber) (time.Time, error) {
	const retries = 5
	var challengeDurationStart time.Time
	var err error
	for i := 0; i < retries; i++ {
		challengeDurationStart, err = a.client.GetBlockTime(ctx, blockNumber)
		if err == nil {
			break
		}
	}
	return challengeDurationStart, err
}
