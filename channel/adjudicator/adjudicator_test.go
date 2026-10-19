package adjudicator

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/client"
)

// pollCountingClient counts channel status polls and always answers with a
// transient error, so the subscription keeps polling without emitting events.
// Every other CKBClient method is left nil and must not be called.
type pollCountingClient struct {
	client.CKBClient
	polls atomic.Int32
}

var errTransient = errors.New("transient RPC error")

func (c *pollCountingClient) GetChannelWithID(context.Context, channel.ID) (client.BlockNumber, *types.Script, *molecule.ChannelConstants, *molecule.ChannelStatus, error) {
	c.polls.Add(1)
	return 0, nil, nil, nil, errTransient
}

func (c *pollCountingClient) GetChannelWithExactPCTS(context.Context, *types.Script) (client.BlockNumber, *molecule.ChannelStatus, error) {
	c.polls.Add(1)
	return 0, nil, errTransient
}

func TestNewAdjudicatorPollingInterval(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want time.Duration
	}{
		{"default", nil, DefaultSubscriptionPollingInterval},
		{"custom", []Option{WithPollingInterval(250 * time.Millisecond)}, 250 * time.Millisecond},
		{"zero keeps default", []Option{WithPollingInterval(0)}, DefaultSubscriptionPollingInterval},
		{"negative keeps default", []Option{WithPollingInterval(-time.Second)}, DefaultSubscriptionPollingInterval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewAdjudicator(nil, tt.opts...).PollingInterval(); got != tt.want {
				t.Errorf("PollingInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscribeUsesConfiguredPollingInterval(t *testing.T) {
	ckb := &pollCountingClient{}
	adj := NewAdjudicator(ckb, WithPollingInterval(10*time.Millisecond))

	sub, err := adj.Subscribe(context.Background(), channel.ID{})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer sub.Close()

	if got := sub.(*PollingSubscription).PollingInterval; got != 10*time.Millisecond {
		t.Fatalf("subscription PollingInterval = %v, want 10ms", got)
	}
	// With a 10ms interval, three polls arrive well within the deadline; with
	// the 4s default, none would.
	deadline := time.Now().Add(2 * time.Second)
	for ckb.polls.Load() < 3 {
		if time.Now().After(deadline) {
			t.Fatalf("got %d polls in 2s, want at least 3 at a 10ms interval", ckb.polls.Load())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestSubscriptionDefaultsToDefaultPollingInterval(t *testing.T) {
	ckb := &pollCountingClient{}
	sub := NewAdjudicatorSubFromChannelID(context.Background(), ckb, channel.ID{}, WithSubscriptionPollingInterval(0))
	defer sub.Close()

	if sub.PollingInterval != DefaultSubscriptionPollingInterval {
		t.Fatalf("PollingInterval = %v, want default %v", sub.PollingInterval, DefaultSubscriptionPollingInterval)
	}
	time.Sleep(100 * time.Millisecond)
	if n := ckb.polls.Load(); n != 0 {
		t.Fatalf("got %d polls within 100ms, want none at the 4s default", n)
	}
}
