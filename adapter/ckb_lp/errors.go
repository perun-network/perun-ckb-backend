package ckblp

import "errors"

var (
	ErrZeroPrice                 = errors.New("price_x64 must be non-zero")
	ErrInvalidLPCell             = errors.New("invalid LP cell data")
	ErrInvalidWitness            = errors.New("invalid LP witness")
	ErrNotImplemented            = errors.New("not implemented")
	ErrInvalidLPCellArg          = errors.New("invalid LP cell argument")
	ErrInvalidChannelID          = errors.New("invalid channel id")
	ErrInvalidContributionID     = errors.New("invalid contribution id")
	ErrScriptHashMismatch        = errors.New("script hash mismatch")
	ErrInsufficientOperatorFunds = errors.New("insufficient operator funding")
)

// ErrorKind classifies adapter errors for hub cleanup behavior.
type ErrorKind uint8

const (
	ErrorKindDeterministic ErrorKind = iota + 1
	ErrorKindRetriable
)

// ClassifiedError wraps an error with its classification.
type ClassifiedError struct {
	Kind ErrorKind
	Err  error
}

func (e ClassifiedError) Error() string {
	if e.Err == nil {
		return "classified error"
	}
	if e.Kind == ErrorKindDeterministic {
		return "deterministic: " + e.Err.Error()
	}
	return "retriable: " + e.Err.Error()
}

func (e ClassifiedError) Unwrap() error {
	return e.Err
}

// Deterministic marks an error as deterministic.
func Deterministic(err error) error {
	if err == nil {
		return nil
	}
	return ClassifiedError{Kind: ErrorKindDeterministic, Err: err}
}

// Retriable marks an error as retriable.
func Retriable(err error) error {
	if err == nil {
		return nil
	}
	return ClassifiedError{Kind: ErrorKindRetriable, Err: err}
}

// IsDeterministic reports whether err is classified as deterministic.
func IsDeterministic(err error) bool {
	var classified ClassifiedError
	if errors.As(err, &classified) {
		return classified.Kind == ErrorKindDeterministic
	}
	return false
}

// IsRetriable reports whether err is classified as retriable.
func IsRetriable(err error) bool {
	var classified ClassifiedError
	if errors.As(err, &classified) {
		return classified.Kind == ErrorKindRetriable
	}
	return false
}
