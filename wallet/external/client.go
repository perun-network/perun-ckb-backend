package external

import (
	"perun.network/perun-ckb-backend/wallet/address"
)

type Client interface {
	Unlock(address.Participant) error
	SignData(participant address.Participant, data []byte) ([]byte, error)
}
