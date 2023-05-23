package encoding

import (
	"errors"
	"fmt"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"
	"perun.network/go-perun/channel"
	gpwallet "perun.network/go-perun/wallet"
	molecule2 "perun.network/perun-ckb-backend/encoding/molecule"
	"perun.network/perun-ckb-backend/wallet/address"
)

func PackChannelParameters(params *channel.Params) (molecule.ChannelParameters, error) {
	if len(params.Parts) != 2 {
		return molecule.ChannelParameters{}, errors.New("only 2-party channels are supported")
	}
	if !params.LedgerChannel {
		return molecule.ChannelParameters{}, errors.New("only ledger channels are supported")
	}
	if params.App != channel.NoApp() {
		return molecule.ChannelParameters{}, errors.New("app channels are not supported")
	}
	if params.VirtualChannel {
		return molecule.ChannelParameters{}, errors.New("virtual channels are not supported")
	}
	a, err := PackAddressToOnChainParticipant(params.Parts[0])
	if err != nil {
		return molecule.ChannelParameters{}, fmt.Errorf("packing first party: %w", err)
	}
	b, err := PackAddressToOnChainParticipant(params.Parts[1])
	if err != nil {
		return molecule.ChannelParameters{}, fmt.Errorf("packing second party: %w", err)
	}

	nonce, err := PackNonce(params.Nonce)
	if err != nil {
		return molecule.ChannelParameters{}, fmt.Errorf("packing nonce: %w", err)
	}

	return molecule.NewChannelParametersBuilder().
		App(NoApp).
		IsLedgerChannel(True).
		IsVirtualChannel(False).
		PartyA(a).
		PartyB(b).
		Nonce(*nonce).
		ChallengeDuration(*types.PackUint64(params.ChallengeDuration)).
		Build(), nil
}

func PackAddressToOnChainParticipant(addr gpwallet.Address) (molecule.Participant, error) {
	a, ok := addr.(*address.Participant)
	if !ok {
		return molecule.Participant{}, errors.New("address is not of type wallet.Participant")
	}
	return a.PackOnChainParticipant()
}

func PackNonce(nonce channel.Nonce) (*molecule.Byte32, error) {
	bytes := nonce.Bytes()
	res := [types.HashLength]byte{}
	if len(bytes) > types.HashLength {
		return nil, fmt.Errorf(
			"nonce is too long, Expected at most %d bytes, got %d",
			types.HashLength,
			len(bytes),
		)
	}
	copy(res[types.HashLength-len(bytes):], bytes)
	return molecule2.PackByte32(res), nil
}

var NoApp = molecule.AppDefault()
