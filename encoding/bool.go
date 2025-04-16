package encoding

import "github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"

// ToBool converts a molecule.Bool to a Go bool.
func ToBool(b molecule.Bool) bool {
	switch b.ToUnion().ItemName() {
	case "True":
		return true
	case "False":
		return false
	default:
		panic("invalid bool")
	}
}

// FromBool converts a Go bool to a molecule.Bool.
func FromBool(b bool) molecule.Bool {
	if b {
		return True
	} else {
		return False
	}
}

var (
	// True is a molecule.Bool representing true.
	True = molecule.NewBoolBuilder().Set(molecule.BoolUnionFromTrue(molecule.TrueDefault())).Build()
	// False is a molecule.Bool representing false.
	False = molecule.NewBoolBuilder().Set(molecule.BoolUnionFromFalse(molecule.FalseDefault())).Build()
)
