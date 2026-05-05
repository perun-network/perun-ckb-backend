package ckblp

import "errors"

var (
	ErrZeroPrice        = errors.New("price_x64 must be non-zero")
	ErrInvalidLPCell    = errors.New("invalid LP cell data")
	ErrInvalidWitness   = errors.New("invalid LP witness")
	ErrNotImplemented   = errors.New("not implemented")
	ErrInvalidLPCellArg = errors.New("invalid LP cell argument")
)
