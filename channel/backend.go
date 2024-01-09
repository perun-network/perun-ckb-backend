package channel

import (
	"fmt"
	"golang.org/x/crypto/blake2b"
	"math"
	"math/big"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/encoding"
)

func init() {
	channel.SetBackend(Backend)
}

type backend struct{}

func (b backend) NewAppID() channel.AppID {
	panic("no app channels")
}

var Backend = backend{}

func (b backend) CalcID(params *channel.Params) channel.ID {
	cp, err := encoding.PackChannelParameters(params)
	if err != nil {
		panic(err)
	}
	return blake2b.Sum256(cp.AsSlice())
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

// NewAsset returns an empty asset.SUDTAsset.
func (b backend) NewAsset() channel.Asset {
	// FIXME: Unite SUDTAsset and native asset in same type.
	return asset.CKBAsset
}

var MaxBalance = new(big.Int).SetUint64(math.MaxUint64)
