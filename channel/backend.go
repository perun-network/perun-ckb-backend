package channel

import (
	"fmt"
	"log"
	"math"
	"math/big"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/encoding"
)

const CKBBackendID = 3

func init() {
	channel.SetBackend(Backend, CKBBackendID)
}

type backend struct{}

func (b backend) NewAppID() (channel.AppID, error) {
	// panic("no app channels")
	return NewDefaultTempAppID(), nil
}

var Backend = backend{}

func (b backend) CalcID(params *channel.Params) (channel.ID, error) {
	cp, err := encoding.PackChannelParameters(params)
	if err != nil {
		return channel.ID{}, errors.WithMessage(err, "could not encode params")
	}
	return blake2b.Sum256(cp.AsSlice()), nil
}

// We add the appData to the starting of the state slice
// Encoding: appData(32 bytes) + channel state (molecule)
func (b backend) Sign(account wallet.Account, state *channel.State) (wallet.Sig, error) {
	s, err := encoding.PackChannelState(state)
	if err != nil {
		return nil, fmt.Errorf("unable to encode channel state: %w", err)
	}
	appData, ok := state.Data.(*TempChannelID)
	if !ok {
		randomChannelID, err := NewRandomTempChannelID()
		if err != nil {
			return nil, fmt.Errorf("generating random TempChannelID: %w", err)
		}
		appData = &randomChannelID
	}
	if len(appData) != TempChannelIDLength {
		return nil, fmt.Errorf("appData(tempChannelID) length is not 32 bytes, got %d", len(appData))
	}
	slice := s.AsSlice()
	extraInfo := []byte{}
	extraInfo = append(extraInfo, appData[:]...)
	log.Println(len(extraInfo), len(slice))
	toSign := append(extraInfo, slice[:]...)
	return account.SignData(toSign)
}

func (b backend) Verify(addr wallet.Address, state *channel.State, sig wallet.Sig) (bool, error) {
	// Re-encode state deterministically.
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
