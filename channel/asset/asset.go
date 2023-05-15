package asset

import (
	pchannel "perun.network/go-perun/channel"
)

// asset is the native asset of the chain (CKBytes).
// We currently do not support UDT assets.
type asset struct{}

// Asset is the unique asset that is supported by the chain (CKBytes).
var Asset = &asset{}

func (asset) Index() pchannel.Index {
	return 0
}

// MarshalBinary does nothing and returns nil since the backend has only one asset.
func (asset) MarshalBinary() (data []byte, err error) {
	return
}

// UnmarshalBinary does nothing and returns nil since the backend has only one asset.
func (*asset) UnmarshalBinary(data []byte) error {
	return nil
}

// Equal returns true if the assets are the same.
func (asset) Equal(b pchannel.Asset) bool {
	_, ok := b.(*asset)
	return ok
}
