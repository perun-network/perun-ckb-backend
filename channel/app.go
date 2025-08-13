package channel

import (
	"crypto/rand"
	"errors"
	"fmt"

	"perun.network/go-perun/channel"
)

const (
	TempChannelIDLength = 32
)

// TempChannelID implements the channel.Data interface
type TempChannelID [TempChannelIDLength]byte

func NewRandomTempChannelID() (TempChannelID, error) {
	temp := TempChannelID{}
	_, err := rand.Read(temp[:])
	if err != nil {
		return TempChannelID{}, errors.New("failed to generate random TempChannelID: " + err.Error())
	}
	return temp, nil
}

func NewTempChannlID(s string) (TempChannelID, error) {
	if len(s) != 32 {
		return TempChannelID{}, errors.New("invalid string. string length for TempChannelID, must be 32 bytes")
	}
	temp := TempChannelID{}
	copy(temp[:], s[:32])
	return temp, nil
}

// NewTempChannelIDFromBytes creates a new TempChannelID from byte data
func NewTempChannelIDFromBytes(data []byte) (TempChannelID, error) {
	if len(data) != 32 {
		return TempChannelID{}, errors.New("invalid data. data length for TempChannelID, must be 32 bytes")
	}
	temp := TempChannelID{}
	copy(temp[:], data[:])
	return temp, nil
}

func (t *TempChannelID) String() string {
	return string(t[:])
}

func (t TempChannelID) MarshalBinary() ([]byte, error) {
	b := make([]byte, 32)
	copy(b, t[:])
	return b, nil
}

func (t *TempChannelID) UnmarshalBinary(data []byte) error {
	if len(data) != 32 {
		return errors.New("invalid data. data length for TempChannelID, must be 32 bytes")
	}
	copy(t[:], data[:])
	return nil
}

func (t TempChannelID) Clone() channel.Data {
	clone := &TempChannelID{}
	copy(clone[:], t[:])
	return clone
}

// TempAppID is a AppID
type TempAppID [32]byte

func NewDefaultTempAppID() channel.AppID {
	temp := TempAppID{}
	copy(temp[:], "tempAppID00000000000000000000000")
	return &temp
}

func (t TempAppID) MarshalBinary() ([]byte, error) {
	return t[:], nil
}

func (t *TempAppID) UnmarshalBinary(data []byte) error {
	if len(data) != 32 {
		return fmt.Errorf("invalid data length: expected 32 bytes, got %d", len(data))
	}
	copy(t[:], data)
	return nil
}

func (t *TempAppID) Equal(other channel.AppID) bool {
	if otherTemp, ok := other.(*TempAppID); ok {
		return *t == *otherTemp
	}
	return false
}

func (t *TempAppID) Key() channel.AppIDKey {
	return "TempAppIDKey"
}

// TempApp implements the channel.StateApp interface
// It must do so as the params expect the app to either implement channel.StateApp or channel.ActionApp
type TempApp struct {
	id TempAppID
}

func NewDefaultTempApp() channel.App {
	id := TempAppID{}
	copy(id[:], "temp_app_00000000000000000000000000000000")

	return &TempApp{
		id: id,
	}
}

func (t *TempApp) Def() channel.AppID {
	return &t.id
}

func (t *TempApp) NewData() channel.Data {
	return &TempChannelID{}
}

func (t *TempApp) ValidTransition(parameters *channel.Params, from, to *channel.State, actor channel.Index) error {
	return nil
}

func (t *TempApp) ValidInit(parameters *channel.Params, state *channel.State) error {
	if state == nil {
		return fmt.Errorf("invalid initial state: state is nil")
	}
	if parameters == nil {
		return fmt.Errorf("invalid parameters: parameters are nil")
	}
	return nil
}
