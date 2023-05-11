package defaults

import (
	"errors"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/go-perun/wallet"
	"perun.network/perun-ckb-backend/wallet/address"
)

// VerifyAndGetPayoutScript verifies that the given address provides a valid payout script for this backend.
// Currently only address.Participant with secp256k1_blake160_sighash_all is supported.
// It returns the payout script and the payment minimum capacity if the verification is successful, otherwise an error.
func VerifyAndGetPayoutScript(addr wallet.Address) (*types.Script, uint64, error) {
	participant, ok := addr.(*address.Participant)
	if !ok {
		return nil, 0, errors.New("invalid participant")
	}
	script, err := participant.GetSecp256k1Blake160SighashAll()
	if err != nil {
		return nil, 0, err
	}
	if script.Hash() != participant.PaymentScriptHash {
		return nil, 0, errors.New("invalid payment script hash (only secp256k1_blake160_sighash_all supported)")
	}
	if script.OccupiedCapacity() != participant.PaymentMinCapacity {
		return nil, 0, errors.New("invalid payment script capacity")
	}

	return script, participant.PaymentMinCapacity, nil
}
