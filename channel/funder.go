package channel

import (
	"bytes"
	"context"
	"errors"
	"github.com/nervosnetwork/ckb-sdk-go/v2/systemscript"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/channel/defaults"
	"perun.network/perun-ckb-backend/client"
	"perun.network/perun-ckb-backend/encoding"
	"perun.network/perun-ckb-backend/wallet/address"
	"time"
)

type KnownPayoutScripts int

const (
	Secp256k1Blake160SighashAll KnownPayoutScripts = iota
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
	token, err := f.client.Start(ctx, req.Params, req.State)
	if err != nil {
		return err
	}
polling:
	for i := 0; i < f.MaxIterationsUntilAbort; i++ {
		select {
		case <-ctx.Done():
			return f.client.Abort(ctx, token)
		case <-time.After(f.PollingInterval):
			cs, err := f.client.GetChannelStatus(ctx, token)
			if err != nil {
				continue polling
			}
			if encoding.ToBool(*cs.Funded()) {
				return nil
			}
		}
	}
	return f.client.Abort(ctx, token)
}

func (f Funder) fundPartyB(ctx context.Context, req channel.FundingReq) error {
polling:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(f.PollingInterval):
			channelConstants, channelStatus, err := f.client.GetChannelWithID(ctx, req.Params.ID())
			if err != nil {
				continue polling
			}
			token, err := f.verifyChannelIntegrity(req, channelConstants, channelStatus)
			if err != nil {
				return err
			}
			if encoding.ToBool(*channelStatus.Funded()) {
				return nil
			}
			return f.client.Fund(ctx, token)
		}
	}
}

// Fund funds the channel with the given funding request.
func (f Funder) Fund(ctx context.Context, req channel.FundingReq) error {
	_, err := f.knownPayoutPreimage(req.Params.Parts[0])
	if err != nil {
		return err
	}
	_, err = f.knownPayoutPreimage(req.Params.Parts[1])
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
func (f Funder) verifyChannelIntegrity(req channel.FundingReq, constants *molecule.ChannelConstants, status *molecule.ChannelStatus) (*molecule.ChannelToken, error) {
	// Verify everything in channel constants besides Params.
	// The Params are already implicitly verified because:
	// 1. We queried for a channel with the given channel ID.
	// 2. The pcts does not allow creation of a channel where the channel id is not the hash of the channel params.
	onchainPCLSHashType, err := encoding.ToHashType(constants.PclsHashType())
	if err != nil {
		return nil, err
	}
	onchainPFLSHashType, err := encoding.ToHashType(constants.PflsHashType())
	if err != nil {
		return nil, err
	}
	if types.UnpackHash(constants.PclsCodeHash()) != f.Constants.PCLSCodeHash ||
		onchainPCLSHashType != f.Constants.PCLSHashType ||
		types.UnpackHash(constants.PflsCodeHash()) != f.Constants.PFLSCodeHash ||
		onchainPFLSHashType != f.Constants.PFLSHashType ||
		encoding.UnpackUint64(constants.PflsMinCapacity()) != f.Constants.PFLSMinCapacity {
		return nil, errors.New("invalid channel constants")
	}

	// Now we verify the integrity of the channel state in the channel status.
	// All other parameters in the channel status are implicitly verified by the pcts.
	reqState, err := encoding.PackChannelState(req.State)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(reqState.AsSlice(), status.State().AsSlice()) {
		return nil, errors.New("invalid channel state")
	}
	return constants.ThreadToken(), nil
}

// knownPayoutPreimage verifies that we know a preimage of the payout script hash. We currently only support
// secp256k1_blake160_sighash_all, so we can just check if the payment script for the given participant is a
// secp256k1_blake160_sighash_all script to their public key.
func (f Funder) knownPayoutPreimage(addr wallet.Address) (KnownPayoutScripts, error) {
	participant, ok := addr.(*address.Participant)
	if !ok {
		return -1, errors.New("invalid participant")
	}
	pubkey := participant.GetCompressedSEC1()
	script, err := systemscript.Secp256K1Blake160SignhashAllByPublicKey(pubkey[:])
	if err != nil {
		return -1, err
	}
	if script.Hash() == participant.PaymentScriptHash {
		return Secp256k1Blake160SighashAll, nil
	}
	if script.OccupiedCapacity() != participant.PaymentMinCapacity {
		return -1, errors.New("invalid payment script capacity")
	}

	return -1, errors.New("unknown payout script")
}
