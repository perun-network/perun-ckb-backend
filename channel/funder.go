package channel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"math"
	"perun.network/go-perun/channel"
	"perun.network/perun-ckb-backend/channel/defaults"
	"perun.network/perun-ckb-backend/client"
	"perun.network/perun-ckb-backend/encoding"
	"time"
)

const DefaultPollingInterval = time.Duration(5) * time.Second
const DefaultMaxIterationsUntilAbort = 12

type ValidChannelConstants struct {
	PCLSCodeHash    types.Hash
	PCLSHashType    types.ScriptHashType
	PFLSCodeHash    types.Hash
	PFLSHashType    types.ScriptHashType
	PFLSMinCapacity uint64
}

type Funder struct {
	client                  client.CKBClient
	PollingInterval         time.Duration
	MaxIterationsUntilAbort int
	Constants               ValidChannelConstants
}

func NewDefaultFunder(client client.CKBClient) *Funder {
	return &Funder{
		client:                  client,
		PollingInterval:         DefaultPollingInterval,
		MaxIterationsUntilAbort: DefaultMaxIterationsUntilAbort,
		Constants: ValidChannelConstants{
			PCLSCodeHash:    defaults.DefaultPCLSCodeHash,
			PCLSHashType:    defaults.DefaultPCLSHashType,
			PFLSCodeHash:    defaults.DefaultPFLSCodeHash,
			PFLSHashType:    defaults.DefaultPFLSHashType,
			PFLSMinCapacity: defaults.DefaultPFLSMinCapacity,
		},
	}
}

func (f Funder) fundPartyA(ctx context.Context, req channel.FundingReq) error {
	script, err := f.client.Start(ctx, req.Params, req.State)
	if err != nil {
		return err
	}
polling:
	for i := 0; i < f.MaxIterationsUntilAbort; i++ {
		select {
		case <-ctx.Done():
			return f.client.Abort(ctx, script)
		case <-time.After(f.PollingInterval):
			_, cs, err := f.client.GetChannelWithExactPCTS(ctx, script)
			if err != nil {
				continue polling
			}
			if encoding.ToBool(*cs.Funded()) {
				return nil
			}
		}
	}
	return f.client.Abort(ctx, script)
}

func (f Funder) fundPartyB(ctx context.Context, req channel.FundingReq) error {
polling:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(f.PollingInterval):
			_, script, channelConstants, channelStatus, err := f.client.GetChannelWithID(ctx, req.Params.ID())
			if err != nil {
				continue polling
			}
			err = f.verifyChannelIntegrity(req, channelConstants, channelStatus)
			if err != nil {
				return err
			}
			if encoding.ToBool(*channelStatus.Funded()) {
				return nil
			}
			return f.client.Fund(ctx, script)
		}
	}
}

// Fund funds the channel with the given funding request.
func (f Funder) Fund(ctx context.Context, req channel.FundingReq) error {
	_, _, err := defaults.VerifyAndGetPayoutScript(req.Params.Parts[0])
	if err != nil {
		return err
	}
	_, _, err = defaults.VerifyAndGetPayoutScript(req.Params.Parts[1])
	if err != nil {
		return err
	}
	switch req.Idx {
	case 0:
		return f.fundPartyA(ctx, req)
	case 1:
		return f.fundPartyB(ctx, req)
	default:
		return errors.New("invalid index")
	}
}

// verifyChannelIntegrity needs to verify everything that is not covered by the channel id.
func (f Funder) verifyChannelIntegrity(req channel.FundingReq, constants *molecule.ChannelConstants, status *molecule.ChannelStatus) error {
	// Verify everything in channel constants besides Params.
	// The Params are already implicitly verified because:
	// 1. We queried for a channel with the given channel ID.
	// 2. The pcts does not allow creation of a channel where the channel id is not the hash of the channel params.
	onchainPCLSHashType, err := encoding.ToHashType(constants.PclsHashType())
	if err != nil {
		return err
	}
	onchainPFLSHashType, err := encoding.ToHashType(constants.PflsHashType())
	if err != nil {
		return err
	}
	if types.UnpackHash(constants.PclsCodeHash()) != f.Constants.PCLSCodeHash ||
		onchainPCLSHashType != f.Constants.PCLSHashType ||
		types.UnpackHash(constants.PflsCodeHash()) != f.Constants.PFLSCodeHash ||
		onchainPFLSHashType != f.Constants.PFLSHashType ||
		encoding.UnpackUint64(constants.PflsMinCapacity()) != f.Constants.PFLSMinCapacity {
		return errors.New("invalid channel constants")
	}
	challengeDuration := encoding.UnpackUint64(constants.Params().ChallengeDuration())
	if challengeDuration > math.MaxInt64 {
		return fmt.Errorf("challenge duration %d is too large, max: %d", challengeDuration, math.MaxInt64)
	}

	// Now we verify the integrity of the channel state in the channel status.
	// All other parameters in the channel status are implicitly verified by the pcts.
	reqState, err := encoding.PackChannelState(req.State)
	if err != nil {
		return err
	}
	if !bytes.Equal(reqState.AsSlice(), status.State().AsSlice()) {
		return errors.New("invalid channel state")
	}
	return nil
}
