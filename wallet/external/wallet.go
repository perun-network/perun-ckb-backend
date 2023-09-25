package external

import (
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/wallet/address"
)

type Wallet struct {
	client Client
}

func (w Wallet) Unlock(a wallet.Address) (wallet.Account, error) {
	p, err := address.IsParticipant(a)
	if err != nil {
		return nil, err
	}
	err = w.client.Unlock(*p)
	if err != nil {
		return nil, err
	}
	return Account{client: w.client, Participant: *p}, nil
}

func (w Wallet) LockAll() {}

func (w Wallet) IncrementUsage(address wallet.Address) {}

func (w Wallet) DecrementUsage(address wallet.Address) {}
