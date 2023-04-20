package encoding

import "github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"

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

func FromBool(b bool) molecule.Bool {
	if b {
		return True
	} else {
		return False
	}
}

var (
	True  = molecule.NewBoolBuilder().Set(molecule.BoolUnionFromTrue(molecule.TrueDefault())).Build()
	False = molecule.NewBoolBuilder().Set(molecule.BoolUnionFromFalse(molecule.FalseDefault())).Build()
)
