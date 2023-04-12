package wallet

import (
	"errors"
	"perun.network/go-perun/wallet"
	"sync"
)

type Wallet struct {
	lock     sync.Mutex
	accounts map[string]*Account
}

func (w *Wallet) Unlock(address wallet.Address) (wallet.Account, error) {
	addr, ok := address.(*Address)
	if !ok {
		return nil, errors.New("address is not of type Address")
	}
	w.lock.Lock()
	defer w.lock.Unlock()
	account, ok := w.accounts[addr.String()]
	if !ok {
		return nil, errors.New("account not found")
	}
	return account, nil
}

func (w *Wallet) LockAll() {}

func (w *Wallet) IncrementUsage(address wallet.Address) {}

func (w *Wallet) DecrementUsage(address wallet.Address) {}

func (w *Wallet) AddNewAccount() (wallet.Account, error) {
	acc, err := NewAccount()
	if err != nil {
		return nil, err
	}
	w.lock.Lock()
	defer w.lock.Unlock()
	_, ok := w.accounts[acc.Address().String()]
	if ok {
		return nil, errors.New("account already exists")
	}
	w.accounts[acc.Address().String()] = acc
	return acc, nil
}

func NewWallet() *Wallet {
	return &Wallet{
		accounts: make(map[string]*Account),
	}
}
