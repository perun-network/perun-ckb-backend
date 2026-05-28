// Copyright 2025 PolyCrypt GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//nolint:golint
package channel

import (
	"log"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
	"math/big"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/multi"
	ckbaddress "perun.network/perun-ckb-backend/wallet/address"

	ckbasset "perun.network/perun-ckb-backend/channel/asset"
)

const EthBackendID = 1

// ToEthState converts a channel.State to a ChannelState struct.
func ToEthState(s *channel.State) EthChannelState {
	numAssets := len(s.Assets)
	backends := make([]*big.Int, numAssets)
	for i := range s.Assets {
		backends[i] = big.NewInt(int64(s.Backends[i]))
	}
	locked := make([]ChannelSubAlloc, len(s.Locked))
	for i, sub := range s.Locked {
		// Create index map.
		indexMap := make([]uint16, s.NumParts())
		if len(sub.IndexMap) == 0 {
			for i := range indexMap {
				indexMap[i] = uint16(i)
			}
		} else {
			for i, x := range sub.IndexMap {
				indexMap[i] = uint16(x)
			}
		}

		locked[i] = ChannelSubAlloc{ID: [][32]byte{sub.ID}, Balances: sub.Bals, IndexMap: indexMap}
	}

	assets := make([]ChannelAsset, numAssets)

	for i, backendID := range s.Backends {
		switch backendID {
		case EthBackendID:
			assets[i] = assetToEthAsset(s.Assets[i])

		case CKBBackendID:
			assets[i] = assetToCKBAsset(s.Assets[i])

		default:
			log.Panicf("wrong backend ID: %d", backendID)
		}
	}

	outcome := ChannelAllocation{
		Assets:   assets,
		Backends: backends,
		Balances: s.Balances,
		Locked:   locked,
	}
	// Check allocation dimensions
	if len(outcome.Assets) != len(outcome.Balances) || len(s.Balances) != len(outcome.Balances) {
		log.Panic("invalid allocation dimensions")
	}
	appData, err := s.Data.MarshalBinary()
	if err != nil {
		log.Panicf("error encoding app data: %v", err)
	}
	return EthChannelState{
		ChannelID: s.ID,
		Version:   s.Version,
		Outcome:   outcome,
		AppData:   appData,
		IsFinal:   s.IsFinal,
	}
}

func assetToEthAsset(asset channel.Asset) ChannelAsset {
	multiAsset, ok := asset.(multi.Asset)
	if !ok {
		log.Panicf("expected asset of type MultiLedgerAsset, but got wrong asset type: %T", asset)
	}
	id := new(big.Int)
	_, ok = id.SetString(string(multiAsset.LedgerBackendID().LedgerID().MapKey()), 10)
	if !ok {
		log.Panicf("Error: Failed to parse string into big.Int")
	}
	ethAddress := common.Address{}
	ethAddress.SetBytes(multiAsset.Address())
	return ChannelAsset{
		ChainID:  id,
		EthAsset: ethAddress,
		CCAsset:  make([]byte, 32),
	}
}

func assetToCKBAsset(asset channel.Asset) ChannelAsset {
	var assetBytes []byte
	var err error

	switch v := asset.(type) {
	case *ckbasset.NervosAsset:
		assetBytes, err = v.MarshalBinary()
		if err != nil {
			log.Panicf("Could not encode NervosAsset: %v", err)
		}
	default:
		log.Panicf("expected asset of type NervosAsset, but got: %T", asset)
	}

	return ChannelAsset{
		ChainID:  big.NewInt(CKBBackendID),
		EthAsset: common.HexToAddress("0x0000000000000000000000000000000000000000"),
		CCAsset:  assetBytes,
	}
}

func ToEthParams(params *channel.Params) (ChannelParams, error) {
	participants := make([]ChannelParticipant, len(params.Parts))
	for i, p := range params.Parts {
		var ccAddress []byte
		var ethAddress common.Address
		ethExplicit := false
		if add, ok := p[EthBackendID]; ok {
			ethAddress = common.Address{}
			var err error
			ethBytes, err := add.MarshalBinary()
			if err != nil {
				return ChannelParams{}, errors.WithMessage(err, "could not encode eth address")
			}
			ethAddress.SetBytes(ethBytes)
			ethExplicit = true
		}
		if add, ok := p[CKBBackendID]; ok {
			participant, ok := add.(*ckbaddress.Participant)
			if !ok {
				return ChannelParams{}, errors.New("participant is not *address.Participant")
			}
			onchainParticipant, err := participant.PackOnChainParticipant()
			if err != nil {
				return ChannelParams{}, errors.New("error packing onchain participant: " + err.Error())
			}
			ccAddress, err = participant.MarshalBinary()
			if err != nil {
				return ChannelParams{}, errors.New("error marshalling participant: " + err.Error())
			}
			// Only derive the Ethereum address from the CKB pubkey if no
			// explicit Ethereum address was provided. Otherwise the explicit
			// address would be silently overwritten, breaking cross-chain
			// channels with distinct CKB and Ethereum keys.
			if !ethExplicit {
				pubKey, err := ckbaddress.UnpackSEC1EncodedPubKey(onchainParticipant.PubKey())
				if err != nil {
					return ChannelParams{}, errors.New("error unpacking sec1 encoded pubkey: " + err.Error())
				}
				pubKeyUncompressed := pubKey.SerializeUncompressed()
				pubKeyNoPrefix := pubKeyUncompressed[1:]
				ethHash := crypto.Keccak256(pubKeyNoPrefix)
				copy(ethAddress[:], ethHash[12:32])
			}
		}
		participants[i] = ChannelParticipant{
			EthAddress: ethAddress,
			CcAddress:  ccAddress,
		}
	}
	var app common.Address
	app.SetBytes(make([]byte, 20))
	return ChannelParams{
		ChallengeDuration: new(big.Int).SetUint64(params.ChallengeDuration),
		Nonce:             params.Nonce,
		Participants:      participants,
		App:               app,
		LedgerChannel:     params.LedgerChannel,
		VirtualChannel:    params.VirtualChannel,
	}, nil
}

// EncodeEthState encodes the state as with abi.encode() in the smart contracts.
//
//nolint:funlen
func EncodeEthState(state *EthChannelState) ([]byte, error) {
	// Define the top-level ABI type for the state struct.
	stateType, err := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "channelID", Type: "bytes32"},
		{Name: "version", Type: "uint64"},
		{Name: "outcome", Type: "tuple", Components: []abi.ArgumentMarshaling{
			{Name: "assets", Type: "tuple[]", Components: []abi.ArgumentMarshaling{
				{Name: "chainID", Type: "uint256"},
				{Name: "ethHolder", Type: "address"},
				{Name: "ccHolder", Type: "bytes"},
			}},
			{Name: "backends", Type: "uint256[]"},
			{Name: "balances", Type: "uint256[][]"},
			{Name: "locked", Type: "tuple[]", Components: []abi.ArgumentMarshaling{
				{Name: "ID", Type: "bytes32[]"},
				{Name: "balances", Type: "uint256[]"},
				{Name: "indexMap", Type: "uint16[]"},
			}},
		}},
		{Name: "appData", Type: "bytes"},
		{Name: "isFinal", Type: "bool"},
	})
	if err != nil {
		return nil, err
	}

	// Define the Arguments.
	args := abi.Arguments{
		{Type: stateType},
	}

	// Pack the data for encoding.
	return args.Pack(
		struct {
			ChannelID [32]byte
			Version   uint64
			Outcome   struct {
				Assets []struct {
					ChainID   *big.Int
					EthHolder common.Address
					CcHolder  []byte
				}
				Backends []*big.Int
				Balances [][]*big.Int
				Locked   []struct {
					ID       [][32]byte
					Balances []*big.Int
					IndexMap []uint16
				}
			}
			AppData []byte
			IsFinal bool
		}{
			ChannelID: state.ChannelID,
			Version:   state.Version,
			Outcome: struct {
				Assets []struct {
					ChainID   *big.Int
					EthHolder common.Address
					CcHolder  []byte
				}
				Backends []*big.Int
				Balances [][]*big.Int
				Locked   []struct {
					ID       [][32]byte
					Balances []*big.Int
					IndexMap []uint16
				}
			}{
				Assets: func() []struct {
					ChainID   *big.Int
					EthHolder common.Address
					CcHolder  []byte
				} {
					var assets []struct {
						ChainID   *big.Int
						EthHolder common.Address
						CcHolder  []byte
					}
					for _, asset := range state.Outcome.Assets {
						assets = append(assets, struct {
							ChainID   *big.Int
							EthHolder common.Address
							CcHolder  []byte
						}{
							ChainID:   asset.ChainID,
							EthHolder: asset.EthAsset,
							CcHolder:  asset.CCAsset,
						})
					}
					return assets
				}(),
				Backends: state.Outcome.Backends,
				Balances: state.Outcome.Balances,
				Locked: func() []struct {
					ID       [][32]byte
					Balances []*big.Int
					IndexMap []uint16
				} {
					var locked []struct {
						ID       [][32]byte
						Balances []*big.Int
						IndexMap []uint16
					}
					for _, lock := range state.Outcome.Locked {
						locked = append(locked, struct {
							ID       [][32]byte
							Balances []*big.Int
							IndexMap []uint16
						}{
							ID:       lock.ID,
							Balances: lock.Balances,
							IndexMap: lock.IndexMap,
						})
					}
					return locked
				}(),
			},
			AppData: state.AppData,
			IsFinal: state.IsFinal,
		},
	)
}

// here we have ethereum methods

// EthChannelState is an auto generated low-level Go binding around a user-defined struct.
type EthChannelState struct {
	ChannelID [32]byte
	Version   uint64
	Outcome   ChannelAllocation
	AppData   []byte
	IsFinal   bool
}

// ChannelAllocation is an auto generated low-level Go binding around a user-defined struct.
type ChannelAllocation struct {
	Assets   []ChannelAsset
	Backends []*big.Int
	Balances [][]*big.Int
	Locked   []ChannelSubAlloc
}

// ChannelAsset is an auto generated low-level Go binding around a user-defined struct.
type ChannelAsset struct {
	ChainID  *big.Int
	EthAsset common.Address
	CCAsset  []byte
}

// ChannelSubAlloc is an auto generated low-level Go binding around a user-defined struct.
type ChannelSubAlloc struct {
	ID       [][32]byte
	Balances []*big.Int
	IndexMap []uint16
}

// EncodeChannelParams encodes the ChannelParams struct using the ABI encoding.
func EncodeChannelParams(params *ChannelParams) ([]byte, error) {
	// Define the top-level ABI type for the ChannelParams struct.
	paramsType, err := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "challengeDuration", Type: "uint256"},
		{Name: "nonce", Type: "uint256"},
		{Name: "participants", Type: "tuple[]", Components: []abi.ArgumentMarshaling{
			{Name: "ethAddress", Type: "address"},
			{Name: "ccAddress", Type: "bytes"},
		}},
		{Name: "app", Type: "address"},
		{Name: "ledgerChannel", Type: "bool"},
		{Name: "virtualChannel", Type: "bool"},
	})
	if err != nil {
		return nil, err
	}

	// Define the Arguments.
	args := abi.Arguments{
		{Type: paramsType},
	}

	// Pack the data for encoding.
	return args.Pack(
		struct {
			ChallengeDuration *big.Int
			Nonce             *big.Int
			Participants      []struct {
				EthAddress common.Address
				CcAddress  []byte
			}
			App            common.Address
			LedgerChannel  bool
			VirtualChannel bool
		}{
			ChallengeDuration: params.ChallengeDuration,
			Nonce:             params.Nonce,
			Participants: func() []struct {
				EthAddress common.Address
				CcAddress  []byte
			} {
				var participants []struct {
					EthAddress common.Address
					CcAddress  []byte
				}
				for _, participant := range params.Participants {
					participants = append(participants, struct {
						EthAddress common.Address
						CcAddress  []byte
					}{
						EthAddress: participant.EthAddress,
						CcAddress:  participant.CcAddress,
					})
				}
				return participants
			}(),
			App:            params.App,
			LedgerChannel:  params.LedgerChannel,
			VirtualChannel: params.VirtualChannel,
		},
	)
}

// ChannelParams is an auto generated low-level Go binding around an user-defined struct.
type ChannelParams struct {
	ChallengeDuration *big.Int
	Nonce             *big.Int
	Participants      []ChannelParticipant
	App               common.Address
	LedgerChannel     bool
	VirtualChannel    bool
}

// ChannelParticipant is an auto generated low-level Go binding around an user-defined struct.
type ChannelParticipant struct {
	EthAddress common.Address
	CcAddress  []byte
}
