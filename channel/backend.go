package channel

import (
	"fmt"
	"math"
	"math/big"

	"golang.org/x/crypto/blake2b"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/encoding"
	"perun.network/perun-ckb-backend/wallet/address"
)

func init() {
	channel.SetBackend(Backend, int(address.BackendIDValue))
}

type backend struct{}

func (b backend) NewAppID() (channel.AppID, error) {
	return nil, fmt.Errorf("no app channels")
}

var Backend = backend{}

func (b backend) CalcID(params *channel.Params) (channel.ID, error) {
	cp, err := encoding.PackChannelParameters(params)
	if err != nil {
		return channel.ID{}, err
	}
	return blake2b.Sum256(cp.AsSlice()), nil
}

func (b backend) CalcVCID(params *channel.Params) channel.ID {
	panic("no virtual channels")
}

func (b backend) Sign(account wallet.Account, state *channel.State) (wallet.Sig, error) {
	s, err := encoding.PackChannelState(state)
	if err != nil {
		return nil, fmt.Errorf("unable to encode channel state: %w", err)
	}
	return account.SignData(s.AsSlice())
}

func (b backend) Verify(addr wallet.Address, state *channel.State, sig wallet.Sig) (bool, error) {
	s, err := encoding.PackChannelState(state)
	if err != nil {
		return false, fmt.Errorf("unable to encode channel state: %w", err)
	}
	return wallet.VerifySignature(s.AsSlice(), sig, addr)
}

// NewAsset returns an empty (and thus invalid) asset for unmarshalling into.
func (b backend) NewAsset() channel.Asset {
	return asset.NewInvalidAsset()
}

var MaxBalance = new(big.Int).SetUint64(math.MaxUint64)
