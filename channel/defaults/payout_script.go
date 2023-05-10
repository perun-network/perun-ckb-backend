package defaults

import (
	"errors"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/wallet/address"
)

type KnownPayoutScripts int

const (
	Secp256k1Blake160SighashAll KnownPayoutScripts = iota
)

// KnownPayoutPreimage verifies that we know a preimage of the payout script hash. We currently only support
// secp256k1_blake160_sighash_all, so we can just check if the payment script for the given participant is a
// secp256k1_blake160_sighash_all script to their public key.
func KnownPayoutPreimage(addr wallet.Address) (KnownPayoutScripts, error) {
	participant, ok := addr.(*address.Participant)
	if !ok {
		return -1, errors.New("invalid participant")
	}
	script, err := participant.GetSecp256k1Blake160SighashAll()
	if err != nil {
		return -1, err
	}
	if script.Hash() == participant.PaymentScriptHash {
		return Secp256k1Blake160SighashAll, nil
	}
	if script.OccupiedCapacity() != participant.PaymentMinCapacity {
		return -1, errors.New("invalid payment script capacity")
	}

	return -1, errors.New("unknown payout script")
}
