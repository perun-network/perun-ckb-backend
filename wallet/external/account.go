package external

import (
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/wallet/address"
)

type Account struct {
	client      Client
	Participant address.Participant
}

func (a Account) Address() wallet.Address {
	return &a.Participant
}

func (a Account) SignData(data []byte) ([]byte, error) {
	// TODO: Figure out signature encoding.
	return a.client.SignData(a.Participant, data)
}
