package adjudicator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"perun.network/go-perun/channel"
)

func TestNewAdjudicatorPollingInterval(t *testing.T) {
	require.Equal(t, DefaultSubscriptionPollingInterval, NewAdjudicator(nil).pollingInterval)
	require.Equal(t, time.Second, NewAdjudicator(nil, WithPollingInterval(time.Second)).pollingInterval)
	require.Panics(t, func() { WithPollingInterval(0) })
	require.Panics(t, func() { WithSubscriptionPollingInterval(-time.Second) })
}

func TestSubscribeUsesPollingInterval(t *testing.T) {
	// A cancelled context stops the polling loop before it polls the nil client.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sub, err := NewAdjudicator(nil, WithPollingInterval(time.Second)).Subscribe(ctx, channel.ID{})
	require.NoError(t, err)
	require.Equal(t, time.Second, sub.(*PollingSubscription).PollingInterval)
}
