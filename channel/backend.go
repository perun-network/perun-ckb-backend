package channel

import (
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
	"math"
	"math/big"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/channel/asset"
)

const CKBBackendID = 3

func init() {
	channel.SetBackend(Backend, CKBBackendID)
}

type backend struct{}

func (b backend) NewAppID() (channel.AppID, error) {
	panic("no app channels")
}

var Backend = backend{}

func (b backend) CalcID(params *channel.Params) (channel.ID, error) {
	p, err := ToEthParams(params)
	if err != nil {
		return channel.ID{}, errors.WithMessage(err, "could not convert params")
	}
	bytes, err := EncodeChannelParams(&p)
	if err != nil {
		return channel.ID{}, errors.WithMessage(err, "could not encode params")
	}
	return crypto.Keccak256Hash(bytes), nil
}

func (b backend) CalcVCID(params *channel.Params) channel.ID {
	panic("no virtual channels")
}

func (b backend) Sign(account wallet.Account, state *channel.State) (wallet.Sig, error) {
	if err := checkBackends(state.Backends); err != nil {
		return nil, errors.New("invalid backends in state allocation: " + err.Error())
	}

	ethState := ToEthState(state)

	bytes, err := EncodeEthState(&ethState)
	if err != nil {
		return nil, err
	}
	sig, err := account.SignData(bytes)
	if err != nil {
		return nil, err
	}
	return sig, err
}

func (b backend) Verify(addr wallet.Address, state *channel.State, sig wallet.Sig) (bool, error) {
	ethState := ToEthState(state)
	bytes, err := EncodeEthState(&ethState)
	if err != nil {
		return false, err
	}
	return wallet.VerifySignature(bytes, sig, addr)
}

// NewAsset returns an empty (and thus invalid) asset for unmarshalling into.
func (b backend) NewAsset() channel.Asset {
	return asset.NewInvalidAsset()
}

var MaxBalance = new(big.Int).SetUint64(math.MaxUint64)

func checkBackends(backends []wallet.BackendID) error {
	if len(backends) == 0 {
		return errors.New("backends slice is empty")
	}

	hasCKBBackend := false

	for _, backend := range backends {
		if backend == CKBBackendID {
			hasCKBBackend = true
		}
	}

	if !hasCKBBackend {
		return errors.New("CKBBackendID not found in backends")
	}

	return nil
}
