package wallet

import (
	"errors"
	"perun.network/go-perun/wallet"
	"sync"
)

type EphemeralWallet struct {
	lock     sync.Mutex
	accounts map[string]*Account
}

func (e *EphemeralWallet) Unlock(address wallet.Address) (wallet.Account, error) {
	addr, ok := address.(*Address)
	if !ok {
		return nil, errors.New("address is not of type Address")
	}
	e.lock.Lock()
	defer e.lock.Unlock()
	account, ok := e.accounts[addr.String()]
	if !ok {
		return nil, errors.New("account not found")
	}
	return account, nil
}

func (e *EphemeralWallet) LockAll() {}

func (e *EphemeralWallet) IncrementUsage(address wallet.Address) {}

func (e *EphemeralWallet) DecrementUsage(address wallet.Address) {}

func (e *EphemeralWallet) AddNewAccount() (wallet.Account, error) {
	acc, err := NewAccount()
	if err != nil {
		return nil, err
	}
	e.lock.Lock()
	defer e.lock.Unlock()
	_, ok := e.accounts[acc.Address().String()]
	if ok {
		return nil, errors.New("account already exists")
	}
	e.accounts[acc.Address().String()] = acc
	return acc, nil
}

func NewEphemeralWallet() *EphemeralWallet {
	return &EphemeralWallet{
		accounts: make(map[string]*Account),
	}
}
