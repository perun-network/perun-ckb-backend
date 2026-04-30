package ckblp

import (
	"context"

	"github.com/Pilatuz/bigz/uint128"
	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/client"
)

// Adapter builds LP transactions and witnesses for the hub.
type Adapter struct {
	client     client.CKBClient
	signer     backend.Signer
	transactor backend.Transactor
	deployment backend.Deployment
}

// NewAdapter creates a new LP adapter with the provided dependencies.
func NewAdapter(
	client client.CKBClient,
	signer backend.Signer,
	transactor backend.Transactor,
	deployment backend.Deployment,
) *Adapter {
	return &Adapter{
		client:     client,
		signer:     signer,
		transactor: transactor,
		deployment: deployment,
	}
}

// BuildFundChannelTx builds a FundChannelExtract transaction (not implemented yet).
func (a *Adapter) BuildFundChannelTx(
	ctx context.Context,
	channelID string,
	lpCellID string,
	amount uint64,
) error {
	return ErrNotImplemented
}

// BuildSettleChannelInsertTx builds a SettleChannelInsert transaction (not implemented yet).
func (a *Adapter) BuildSettleChannelInsertTx(
	ctx context.Context,
	channelID string,
	contributionID string,
	lpCellID string,
	principal uint64,
	feeCKB uint64,
	priceX64 uint128.Uint128,
) error {
	if priceX64 == (uint128.Uint128{}) {
		return ErrZeroPrice
	}
	return ErrNotImplemented
}

// DiscoverLPCells returns LP cells matching operator lock hash (not implemented yet).
func (a *Adapter) DiscoverLPCells(
	ctx context.Context,
	operatorLockHash byte,
) ([]LPCellInfo, error) {
	return nil, ErrNotImplemented
}

// GetLPCell fetches a single LP cell (not implemented yet).
func (a *Adapter) GetLPCell(
	ctx context.Context,
	lpCellID string,
) (LPCellInfo, error) {
	return LPCellInfo{}, ErrNotImplemented
}
