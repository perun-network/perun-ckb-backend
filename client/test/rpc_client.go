package test

import (
	"context"

	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

type MockRPCClient struct {
	addNode                          func(ctx context.Context, peerId string, address string) error
	batchLiveCells                   func(ctx context.Context, batch []types.BatchLiveCellItem) error
	batchTransactions                func(ctx context.Context, batch []types.BatchTransactionItem) error
	calculateDaoMaximumWithdraw      func(ctx context.Context, point *types.OutPoint, hash types.Hash) (uint64, error)
	callContext                      func(ctx context.Context, result interface{}, method string, args ...interface{}) error
	clearBannedAddresses             func(ctx context.Context) error
	clearTxPool                      func(ctx context.Context) error
	close                            func()
	dryRunTransaction                func(ctx context.Context, transaction *types.Transaction) (*types.DryRunTransactionResult, error)
	estimateCycles                   func(ctx context.Context, transaction *types.Transaction) (*types.EstimateCycles, error)
	generateEpochs                   func(ctx context.Context, number uint64) (uint64, error)
	getBannedAddresses               func(ctx context.Context) ([]*types.BannedAddress, error)
	getBlock                         func(ctx context.Context, hash types.Hash) (*types.Block, error)
	getBlockByNumber                 func(ctx context.Context, number uint64) (*types.Block, error)
	getBlockByNumberWithCycles       func(ctx context.Context, number uint64) (*types.BlockWithCycles, error)
	getBlockEconomicState            func(ctx context.Context, hash types.Hash) (*types.BlockEconomicState, error)
	getBlockHash                     func(ctx context.Context, number uint64) (*types.Hash, error)
	getBlockMedianTime               func(ctx context.Context, blockHash types.Hash) (uint64, error)
	getBlockWithCycles               func(ctx context.Context, hash types.Hash) (*types.BlockWithCycles, error)
	getBlockchainInfo                func(ctx context.Context) (*types.BlockchainInfo, error)
	getCells                         func(ctx context.Context, searchKey *indexer.SearchKey, order indexer.SearchOrder, limit uint64, afterCursor string) (*indexer.LiveCells, error)
	getCellsCapacity                 func(ctx context.Context, searchKey *indexer.SearchKey) (*indexer.Capacity, error)
	getConsensus                     func(ctx context.Context) (*types.Consensus, error)
	getCurrentEpoch                  func(ctx context.Context) (*types.Epoch, error)
	getDeploymentsInfo               func(ctx context.Context) (*types.DeploymentsInfo, error)
	getEpochByNumber                 func(ctx context.Context, number uint64) (*types.Epoch, error)
	getFeeRateStatics                func(ctx context.Context, target interface{}) (*types.FeeRateStatics, error)
	getFeeRateStatistics             func(ctx context.Context, target interface{}) (*types.FeeRateStatistics, error)
	getForkBlock                     func(ctx context.Context, blockHash types.Hash) (*types.Block, error)
	getHeader                        func(ctx context.Context, hash types.Hash) (*types.Header, error)
	getHeaderByNumber                func(ctx context.Context, number uint64) (*types.Header, error)
	getIndexerTip                    func(ctx context.Context) (*indexer.TipHeader, error)
	getLiveCell                      func(ctx context.Context, outPoint *types.OutPoint, withData bool, includeTxPool *bool) (*types.CellWithStatus, error)
	getPackedBlock                   func(ctx context.Context, hash types.Hash) (*types.Block, error)
	getPackedBlockWithCycles         func(ctx context.Context, hash types.Hash) (*types.BlockWithCycles, error)
	getPackedHeader                  func(ctx context.Context, hash types.Hash) (*types.Header, error)
	getPackedHeaderByNumber          func(ctx context.Context, number uint64) (*types.Header, error)
	getPeers                         func(ctx context.Context) ([]*types.RemoteNode, error)
	getPoolTxDetailInfo              func(ctx context.Context, hash types.Hash) (*types.PoolTxDetailInfo, error)
	getRawTxPool                     func(ctx context.Context) (*types.RawTxPool, error)
	getTipBlockNumber                func(ctx context.Context) (uint64, error)
	getTipHeader                     func(ctx context.Context) (*types.Header, error)
	getTransaction                   func(ctx context.Context, hash types.Hash, onlyCommited *bool) (*types.TransactionWithStatus, error)
	getTransactionAndWitnessProof    func(ctx context.Context, txHashes []string, blockHash *types.Hash) (*types.TransactionAndWitnessProof, error)
	getTransactionProof              func(ctx context.Context, txHashes []string, blockHash *types.Hash) (*types.TransactionProof, error)
	getTransactions                  func(ctx context.Context, searchKey *indexer.SearchKey, order indexer.SearchOrder, limit uint64, afterCursor string) (*indexer.TxsWithCell, error)
	getTransactionsGrouped           func(ctx context.Context, searchKey *indexer.SearchKey, order indexer.SearchOrder, limit uint64, afterCursor string) (*indexer.TxsWithCells, error)
	localNodeInfo                    func(ctx context.Context) (*types.LocalNode, error)
	pingPeers                        func(ctx context.Context) error
	removeNode                       func(ctx context.Context, peerId string) error
	sendTransaction                  func(ctx context.Context, tx *types.Transaction) (*types.Hash, error)
	setBan                           func(ctx context.Context, address string, command string, banTime uint64, absolute bool, reason string) error
	setNetworkActive                 func(ctx context.Context, state bool) error
	syncState                        func(ctx context.Context) (*types.SyncState, error)
	testTxPoolAccept                 func(ctx context.Context, tx *types.Transaction) (*types.EntryCompleted, error)
	txPoolInfo                       func(ctx context.Context) (*types.TxPoolInfo, error)
	verifyTransactionAndWitnessProof func(ctx context.Context, proof *types.TransactionAndWitnessProof) ([]*types.Hash, error)
	verifyTransactionProof           func(ctx context.Context, proof *types.TransactionProof) ([]*types.Hash, error)
}

var _ rpc.Client = (*MockRPCClient)(nil)

type MockRPCClientOption func(*MockRPCClient) *MockRPCClient

func NewMockRPCClient() *MockRPCClient {
	return &MockRPCClient{
		// For each member of this struct generate a function that panics with "unimplemented"
		addNode: func(ctx context.Context, peerId string, address string) error {
			panic("unimplemented")
		},
		batchLiveCells: func(ctx context.Context, batch []types.BatchLiveCellItem) error {
			panic("unimplemented")
		},
		batchTransactions: func(ctx context.Context, batch []types.BatchTransactionItem) error {
			panic("unimplemented")
		},
		calculateDaoMaximumWithdraw: func(ctx context.Context, point *types.OutPoint, hash types.Hash) (uint64, error) {
			panic("unimplemented")
		},
		callContext: func(ctx context.Context, result interface{}, method string, args ...interface{}) error {
			panic("unimplemented")
		},
		clearBannedAddresses: func(ctx context.Context) error {
			panic("unimplemented")
		},
		clearTxPool: func(ctx context.Context) error {
			panic("unimplemented")
		},
		close: func() {
			panic("unimplemented")
		},
		dryRunTransaction: func(ctx context.Context, transaction *types.Transaction) (*types.DryRunTransactionResult, error) {
			panic("unimplemented")
		},
		estimateCycles: func(ctx context.Context, transaction *types.Transaction) (*types.EstimateCycles, error) {
			panic("unimplemented")
		},
		generateEpochs: func(ctx context.Context, number uint64) (uint64, error) {
			panic("unimplemented")
		},
		getBannedAddresses: func(ctx context.Context) ([]*types.BannedAddress, error) {
			panic("unimplemented")
		},
		getBlock: func(ctx context.Context, hash types.Hash) (*types.Block, error) {
			panic("unimplemented")
		},
		getBlockByNumber: func(ctx context.Context, number uint64) (*types.Block, error) {
			panic("unimplemented")
		},
		getBlockByNumberWithCycles: func(ctx context.Context, number uint64) (*types.BlockWithCycles, error) {
			panic("unimplemented")
		},
		getBlockEconomicState: func(ctx context.Context, hash types.Hash) (*types.BlockEconomicState, error) {
			panic("unimplemented")
		},
		getBlockHash: func(ctx context.Context, number uint64) (*types.Hash, error) {
			panic("unimplemented")
		},
		getBlockMedianTime: func(ctx context.Context, blockHash types.Hash) (uint64, error) {
			panic("unimplemented")
		},
		getBlockWithCycles: func(ctx context.Context, hash types.Hash) (*types.BlockWithCycles, error) {
			panic("unimplemented")
		},
		getBlockchainInfo: func(ctx context.Context) (*types.BlockchainInfo, error) {
			panic("unimplemented")
		},
		getCells: func(ctx context.Context, searchKey *indexer.SearchKey, order indexer.SearchOrder, limit uint64, afterCursor string) (*indexer.LiveCells, error) {
			panic("unimplemented")
		},
		getCellsCapacity: func(ctx context.Context, searchKey *indexer.SearchKey) (*indexer.Capacity, error) {
			panic("unimplemented")
		},
		getConsensus: func(ctx context.Context) (*types.Consensus, error) {
			panic("unimplemented")
		},
		getCurrentEpoch: func(ctx context.Context) (*types.Epoch, error) {
			panic("unimplemented")
		},
		getDeploymentsInfo: func(ctx context.Context) (*types.DeploymentsInfo, error) {
			panic("unimplemented")
		},
		getEpochByNumber: func(ctx context.Context, number uint64) (*types.Epoch, error) {
			panic("unimplemented")
		},
		getFeeRateStatics: func(ctx context.Context, target interface{}) (*types.FeeRateStatics, error) {
			panic("unimplemented")
		},
		getForkBlock: func(ctx context.Context, blockHash types.Hash) (*types.Block, error) {
			panic("unimplemented")
		},
		getHeader: func(ctx context.Context, hash types.Hash) (*types.Header, error) {
			panic("unimplemented")
		},
		getHeaderByNumber: func(ctx context.Context, number uint64) (*types.Header, error) {
			panic("unimplemented")
		},
		getIndexerTip: func(ctx context.Context) (*indexer.TipHeader, error) {
			panic("unimplemented")
		},
		getLiveCell: func(ctx context.Context, outPoint *types.OutPoint, withData bool, includeTxPool *bool) (*types.CellWithStatus, error) {
			panic("unimplemented")
		},
		getPackedBlock: func(ctx context.Context, hash types.Hash) (*types.Block, error) {
			panic("unimplemented")
		},
		getPackedBlockWithCycles: func(ctx context.Context, hash types.Hash) (*types.BlockWithCycles, error) {
			panic("unimplemented")
		},
		getPackedHeader: func(ctx context.Context, hash types.Hash) (*types.Header, error) {
			panic("unimplemented")
		},
		getPackedHeaderByNumber: func(ctx context.Context, number uint64) (*types.Header, error) {
			panic("unimplemented")
		},
		getPeers: func(ctx context.Context) ([]*types.RemoteNode, error) {
			panic("unimplemented")
		},
		getPoolTxDetailInfo: func(ctx context.Context, hash types.Hash) (*types.PoolTxDetailInfo, error) {
			panic("unimplemented")
		},
		getRawTxPool: func(ctx context.Context) (*types.RawTxPool, error) {
			panic("unimplemented")
		},
		getTipBlockNumber: func(ctx context.Context) (uint64, error) {
			panic("unimplemented")
		},
		getTipHeader: func(ctx context.Context) (*types.Header, error) {
			panic("unimplemented")
		},
		getTransaction: func(ctx context.Context, hash types.Hash, onlyCommited *bool) (*types.TransactionWithStatus, error) {
			panic("unimplemented")
		},
		getTransactionAndWitnessProof: func(ctx context.Context, txHashes []string, blockHash *types.Hash) (*types.TransactionAndWitnessProof, error) {
			panic("unimplemented")
		},
		getTransactionProof: func(ctx context.Context, txHashes []string, blockHash *types.Hash) (*types.TransactionProof, error) {
			panic("unimplemented")
		},
		getTransactions: func(ctx context.Context, searchKey *indexer.SearchKey, order indexer.SearchOrder, limit uint64, afterCursor string) (*indexer.TxsWithCell, error) {
			panic("unimplemented")
		},
		getTransactionsGrouped: func(ctx context.Context, searchKey *indexer.SearchKey, order indexer.SearchOrder, limit uint64, afterCursor string) (*indexer.TxsWithCells, error) {
			panic("unimplemented")
		},
		localNodeInfo: func(ctx context.Context) (*types.LocalNode, error) {
			panic("unimplemented")
		},
		pingPeers: func(ctx context.Context) error {
			panic("unimplemented")
		},
		removeNode: func(ctx context.Context, peerId string) error {
			panic("unimplemented")
		},
		sendTransaction: func(ctx context.Context, tx *types.Transaction) (*types.Hash, error) {
			panic("unimplemented")
		},
		setBan: func(ctx context.Context, address string, command string, banTime uint64, absolute bool, reason string) error {
			panic("unimplemented")
		},
		setNetworkActive: func(ctx context.Context, state bool) error {
			panic("unimplemented")
		},
		syncState: func(ctx context.Context) (*types.SyncState, error) {
			panic("unimplemented")
		},
		txPoolInfo: func(ctx context.Context) (*types.TxPoolInfo, error) {
			panic("unimplemented")
		},
		testTxPoolAccept: func(ctx context.Context, tx *types.Transaction) (*types.EntryCompleted, error) {
			panic("unimplemented")
		},
		verifyTransactionAndWitnessProof: func(ctx context.Context, proof *types.TransactionAndWitnessProof) ([]*types.Hash, error) {
			panic("unimplemented")
		},
		verifyTransactionProof: func(ctx context.Context, proof *types.TransactionProof) ([]*types.Hash, error) {
			panic("unimplemented")
		},
	}
}

func (m *MockRPCClient) SetAddNode(f func(ctx context.Context, peerId string, address string) error) {
	m.addNode = f
}
func (m *MockRPCClient) SetBatchLiveCells(f func(ctx context.Context, batch []types.BatchLiveCellItem) error) {
	m.batchLiveCells = f
}
func (m *MockRPCClient) SetBatchTransactions(f func(ctx context.Context, batch []types.BatchTransactionItem) error) {
	m.batchTransactions = f
}
func (m *MockRPCClient) SetCalculateDaoMaximumWithdraw(f func(ctx context.Context, point *types.OutPoint, hash types.Hash) (uint64, error)) {
	m.calculateDaoMaximumWithdraw = f
}
func (m *MockRPCClient) SetCallContext(f func(ctx context.Context, result interface{}, method string, args ...interface{}) error) {
	m.callContext = f
}
func (m *MockRPCClient) SetClearBannedAddresses(f func(ctx context.Context) error) {
	m.clearBannedAddresses = f
}
func (m *MockRPCClient) SetClearTxPool(f func(ctx context.Context) error) {
	m.clearTxPool = f
}
func (m *MockRPCClient) SetClose(f func()) {
	m.close = f
}
func (m *MockRPCClient) SetDryRunTransaction(f func(ctx context.Context, transaction *types.Transaction) (*types.DryRunTransactionResult, error)) {
	m.dryRunTransaction = f
}
func (m *MockRPCClient) SetEstimateCycles(f func(ctx context.Context, transaction *types.Transaction) (*types.EstimateCycles, error)) {
	m.estimateCycles = f
}
func (m *MockRPCClient) SetGetBannedAddresses(f func(ctx context.Context) ([]*types.BannedAddress, error)) {
	m.getBannedAddresses = f
}
func (m *MockRPCClient) SetGetBlock(f func(ctx context.Context, hash types.Hash) (*types.Block, error)) {
	m.getBlock = f
}
func (m *MockRPCClient) SetGetBlockByNumber(f func(ctx context.Context, number uint64) (*types.Block, error)) {
	m.getBlockByNumber = f
}
func (m *MockRPCClient) SetGetBlockByNumberWithCycles(f func(ctx context.Context, number uint64) (*types.BlockWithCycles, error)) {
	m.getBlockByNumberWithCycles = f
}
func (m *MockRPCClient) SetGetBlockEconomicState(f func(ctx context.Context, hash types.Hash) (*types.BlockEconomicState, error)) {
	m.getBlockEconomicState = f
}
func (m *MockRPCClient) SetGetBlockHash(f func(ctx context.Context, number uint64) (*types.Hash, error)) {
	m.getBlockHash = f
}
func (m *MockRPCClient) SetGetBlockMedianTime(f func(ctx context.Context, blockHash types.Hash) (uint64, error)) {
	m.getBlockMedianTime = f
}
func (m *MockRPCClient) SetGetBlockWithCycles(f func(ctx context.Context, hash types.Hash) (*types.BlockWithCycles, error)) {
	m.getBlockWithCycles = f
}
func (m *MockRPCClient) SetGetBlockchainInfo(f func(ctx context.Context) (*types.BlockchainInfo, error)) {
	m.getBlockchainInfo = f
}
func (m *MockRPCClient) SetGetCells(f func(ctx context.Context, searchKey *indexer.SearchKey, order indexer.SearchOrder, limit uint64, afterCursor string) (*indexer.LiveCells, error)) {
	m.getCells = f
}
func (m *MockRPCClient) SetGetCellsCapacity(f func(ctx context.Context, searchKey *indexer.SearchKey) (*indexer.Capacity, error)) {
	m.getCellsCapacity = f
}
func (m *MockRPCClient) SetGetConsensus(f func(ctx context.Context) (*types.Consensus, error)) {
	m.getConsensus = f
}
func (m *MockRPCClient) SetGetCurrentEpoch(f func(ctx context.Context) (*types.Epoch, error)) {
	m.getCurrentEpoch = f
}
func (m *MockRPCClient) SetGetEpochByNumber(f func(ctx context.Context, number uint64) (*types.Epoch, error)) {
	m.getEpochByNumber = f
}
func (m *MockRPCClient) SetGetFeeRateStatics(f func(ctx context.Context, target interface{}) (*types.FeeRateStatics, error)) {
	m.getFeeRateStatics = f
}
func (m *MockRPCClient) SetGetFeeRateStatistics(f func(ctx context.Context, target interface{}) (*types.FeeRateStatistics, error)) {
	m.getFeeRateStatistics = f
}
func (m *MockRPCClient) SetGetForkBlock(f func(ctx context.Context, blockHash types.Hash) (*types.Block, error)) {
	m.getForkBlock = f
}
func (m *MockRPCClient) SetGetHeader(f func(ctx context.Context, hash types.Hash) (*types.Header, error)) {
	m.getHeader = f
}
func (m *MockRPCClient) SetGetHeaderByNumber(f func(ctx context.Context, number uint64) (*types.Header, error)) {
	m.getHeaderByNumber = f
}
func (m *MockRPCClient) SetGetIndexerTip(f func(ctx context.Context) (*indexer.TipHeader, error)) {
	m.getIndexerTip = f
}
func (m *MockRPCClient) SetGetLiveCell(f func(ctx context.Context, outPoint *types.OutPoint, withData bool, includeTxPool *bool) (*types.CellWithStatus, error)) {
	m.getLiveCell = f
}
func (m *MockRPCClient) SetGetPackedBlock(f func(ctx context.Context, hash types.Hash) (*types.Block, error)) {
	m.getPackedBlock = f
}
func (m *MockRPCClient) SetGetPackedBlockWithCycles(f func(ctx context.Context, hash types.Hash) (*types.BlockWithCycles, error)) {
	m.getPackedBlockWithCycles = f
}
func (m *MockRPCClient) SetGetPackedHeader(f func(ctx context.Context, hash types.Hash) (*types.Header, error)) {
	m.getPackedHeader = f
}
func (m *MockRPCClient) SetGetPackedHeaderByNumber(f func(ctx context.Context, number uint64) (*types.Header, error)) {
	m.getPackedHeaderByNumber = f
}
func (m *MockRPCClient) SetGetPeers(f func(ctx context.Context) ([]*types.RemoteNode, error)) {
	m.getPeers = f
}
func (m *MockRPCClient) SetGetRawTxPool(f func(ctx context.Context) (*types.RawTxPool, error)) {
	m.getRawTxPool = f
}
func (m *MockRPCClient) SetGetTipBlockNumber(f func(ctx context.Context) (uint64, error)) {
	m.getTipBlockNumber = f
}
func (m *MockRPCClient) SetGetTipHeader(f func(ctx context.Context) (*types.Header, error)) {
	m.getTipHeader = f
}
func (m *MockRPCClient) SetGetTransaction(f func(ctx context.Context, hash types.Hash, onlyCommited *bool) (*types.TransactionWithStatus, error)) {
	m.getTransaction = f
}
func (m *MockRPCClient) SetGetTransactionAndWitnessProof(f func(ctx context.Context, txHashes []string, blockHash *types.Hash) (*types.TransactionAndWitnessProof, error)) {
	m.getTransactionAndWitnessProof = f
}
func (m *MockRPCClient) SetGetTransactionProof(f func(ctx context.Context, txHashes []string, blockHash *types.Hash) (*types.TransactionProof, error)) {
	m.getTransactionProof = f
}
func (m *MockRPCClient) SetGetTransactions(f func(ctx context.Context, searchKey *indexer.SearchKey, order indexer.SearchOrder, limit uint64, afterCursor string) (*indexer.TxsWithCell, error)) {
	m.getTransactions = f
}
func (m *MockRPCClient) SetGetTransactionsGrouped(f func(ctx context.Context, searchKey *indexer.SearchKey, order indexer.SearchOrder, limit uint64, afterCursor string) (*indexer.TxsWithCells, error)) {
	m.getTransactionsGrouped = f
}
func (m *MockRPCClient) SetLocalNodeInfo(f func(ctx context.Context) (*types.LocalNode, error)) {
	m.localNodeInfo = f
}
func (m *MockRPCClient) SetPingPeers(f func(ctx context.Context) error) {
	m.pingPeers = f
}
func (m *MockRPCClient) SetRemoveNode(f func(ctx context.Context, peerId string) error) {
	m.removeNode = f
}
func (m *MockRPCClient) SetSendTransaction(f func(ctx context.Context, tx *types.Transaction) (*types.Hash, error)) {
	m.sendTransaction = f
}
func (m *MockRPCClient) SetSetBan(f func(ctx context.Context, address string, command string, banTime uint64, absolute bool, reason string) error) {
	m.setBan = f
}
func (m *MockRPCClient) SetSetNetworkActive(f func(ctx context.Context, state bool) error) {
	m.setNetworkActive = f
}
func (m *MockRPCClient) SetSyncState(f func(ctx context.Context) (*types.SyncState, error)) {
	m.syncState = f
}
func (m *MockRPCClient) SetTxPoolInfo(f func(ctx context.Context) (*types.TxPoolInfo, error)) {
	m.txPoolInfo = f
}
func (m *MockRPCClient) SetVerifyTransactionAndWitnessProof(f func(ctx context.Context, proof *types.TransactionAndWitnessProof) ([]*types.Hash, error)) {
	m.verifyTransactionAndWitnessProof = f
}
func (m *MockRPCClient) SetVerifyTransactionProof(f func(ctx context.Context, proof *types.TransactionProof) ([]*types.Hash, error)) {
	m.verifyTransactionProof = f
}

// AddNode implements rpc.Client
func (m *MockRPCClient) AddNode(ctx context.Context, peerId string, address string) error {
	return m.addNode(ctx, peerId, address)
}

// BatchLiveCells implements rpc.Client
func (m *MockRPCClient) BatchLiveCells(ctx context.Context, batch []types.BatchLiveCellItem) error {
	return m.batchLiveCells(ctx, batch)
}

// BatchTransactions implements rpc.Client
func (m *MockRPCClient) BatchTransactions(ctx context.Context, batch []types.BatchTransactionItem) error {
	return m.batchTransactions(ctx, batch)
}

// CalculateDaoMaximumWithdraw implements rpc.Client
func (m *MockRPCClient) CalculateDaoMaximumWithdraw(ctx context.Context, point *types.OutPoint, hash types.Hash) (uint64, error) {
	return m.calculateDaoMaximumWithdraw(ctx, point, hash)
}

// CallContext implements rpc.Client
func (m *MockRPCClient) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	return m.callContext(ctx, result, method, args...)
}

// ClearBannedAddresses implements rpc.Client
func (m *MockRPCClient) ClearBannedAddresses(ctx context.Context) error {
	return m.clearBannedAddresses(ctx)
}

// ClearTxPool implements rpc.Client
func (m *MockRPCClient) ClearTxPool(ctx context.Context) error {
	return m.clearTxPool(ctx)
}

// Close implements rpc.Client
func (m *MockRPCClient) Close() {
	m.close()
}

// DryRunTransaction implements rpc.Client
func (m *MockRPCClient) DryRunTransaction(ctx context.Context, transaction *types.Transaction) (*types.DryRunTransactionResult, error) {
	return m.dryRunTransaction(ctx, transaction)
}

// EstimateCycles implements rpc.Client
func (m *MockRPCClient) EstimateCycles(ctx context.Context, transaction *types.Transaction) (*types.EstimateCycles, error) {
	return m.estimateCycles(ctx, transaction)
}

func (m *MockRPCClient) GenerateEpochs(ctx context.Context, numEpochs uint64) (uint64, error) {
	// Implement the method logic here
	return m.generateEpochs(ctx, numEpochs)
}

// GetBannedAddresses implements rpc.Client
func (m *MockRPCClient) GetBannedAddresses(ctx context.Context) ([]*types.BannedAddress, error) {
	return m.getBannedAddresses(ctx)
}

// GetBlock implements rpc.Client
func (m *MockRPCClient) GetBlock(ctx context.Context, hash types.Hash) (*types.Block, error) {
	return m.getBlock(ctx, hash)
}

// GetBlockByNumber implements rpc.Client
func (m *MockRPCClient) GetBlockByNumber(ctx context.Context, number uint64) (*types.Block, error) {
	return m.getBlockByNumber(ctx, number)
}

// GetBlockByNumberWithCycles implements rpc.Client
func (m *MockRPCClient) GetBlockByNumberWithCycles(ctx context.Context, number uint64) (*types.BlockWithCycles, error) {
	return m.getBlockByNumberWithCycles(ctx, number)
}

// GetBlockEconomicState implements rpc.Client
func (m *MockRPCClient) GetBlockEconomicState(ctx context.Context, hash types.Hash) (*types.BlockEconomicState, error) {
	return m.getBlockEconomicState(ctx, hash)
}

// GetBlockHash implements rpc.Client
func (m *MockRPCClient) GetBlockHash(ctx context.Context, number uint64) (*types.Hash, error) {
	return m.getBlockHash(ctx, number)
}

// GetBlockMedianTime implements rpc.Client
func (m *MockRPCClient) GetBlockMedianTime(ctx context.Context, blockHash types.Hash) (uint64, error) {
	return m.getBlockMedianTime(ctx, blockHash)
}

// GetBlockWithCycles implements rpc.Client
func (m *MockRPCClient) GetBlockWithCycles(ctx context.Context, hash types.Hash) (*types.BlockWithCycles, error) {
	return m.getBlockWithCycles(ctx, hash)
}

// GetBlockchainInfo implements rpc.Client
func (m *MockRPCClient) GetBlockchainInfo(ctx context.Context) (*types.BlockchainInfo, error) {
	return m.getBlockchainInfo(ctx)
}

// GetCells implements rpc.Client
func (m *MockRPCClient) GetCells(ctx context.Context, searchKey *indexer.SearchKey, order indexer.SearchOrder, limit uint64, afterCursor string) (*indexer.LiveCells, error) {
	return m.getCells(ctx, searchKey, order, limit, afterCursor)
}

// GetCellsCapacity implements rpc.Client
func (m *MockRPCClient) GetCellsCapacity(ctx context.Context, searchKey *indexer.SearchKey) (*indexer.Capacity, error) {
	return m.getCellsCapacity(ctx, searchKey)
}

// GetConsensus implements rpc.Client
func (m *MockRPCClient) GetConsensus(ctx context.Context) (*types.Consensus, error) {
	return m.getConsensus(ctx)
}

// GetCurrentEpoch implements rpc.Client
func (m *MockRPCClient) GetCurrentEpoch(ctx context.Context) (*types.Epoch, error) {
	return m.getCurrentEpoch(ctx)
}

func (m *MockRPCClient) GetDeploymentsInfo(ctx context.Context) (*types.DeploymentsInfo, error) {
	return m.getDeploymentsInfo(ctx)
}

// GetEpochByNumber implements rpc.Client
func (m *MockRPCClient) GetEpochByNumber(ctx context.Context, number uint64) (*types.Epoch, error) {
	return m.getEpochByNumber(ctx, number)
}

// GetFeeRateStatics implements rpc.Client
func (m *MockRPCClient) GetFeeRateStatics(ctx context.Context, target interface{}) (*types.FeeRateStatics, error) {
	return m.getFeeRateStatics(ctx, target)
}

func (m *MockRPCClient) GetFeeRateStatistics(ctx context.Context, target interface{}) (*types.FeeRateStatistics, error) {
	return m.getFeeRateStatistics(ctx, target)
}

// GetForkBlock implements rpc.Client
func (m *MockRPCClient) GetForkBlock(ctx context.Context, blockHash types.Hash) (*types.Block, error) {
	return m.getForkBlock(ctx, blockHash)
}

// GetHeader implements rpc.Client
func (m *MockRPCClient) GetHeader(ctx context.Context, hash types.Hash) (*types.Header, error) {
	return m.getHeader(ctx, hash)
}

// GetHeaderByNumber implements rpc.Client
func (m *MockRPCClient) GetHeaderByNumber(ctx context.Context, number uint64) (*types.Header, error) {
	return m.getHeaderByNumber(ctx, number)
}

// GetIndexerTip implements rpc.Client
func (m *MockRPCClient) GetIndexerTip(ctx context.Context) (*indexer.TipHeader, error) {
	return m.getIndexerTip(ctx)
}

// GetLiveCell implements rpc.Client
func (m *MockRPCClient) GetLiveCell(ctx context.Context, outPoint *types.OutPoint, withData bool, includeTxPool *bool) (*types.CellWithStatus, error) {
	return m.getLiveCell(ctx, outPoint, withData, includeTxPool)
}

// GetPackedBlock implements rpc.Client
func (m *MockRPCClient) GetPackedBlock(ctx context.Context, hash types.Hash) (*types.Block, error) {
	return m.getPackedBlock(ctx, hash)
}

// GetPackedBlockWithCycles implements rpc.Client
func (m *MockRPCClient) GetPackedBlockWithCycles(ctx context.Context, hash types.Hash) (*types.BlockWithCycles, error) {
	return m.getPackedBlockWithCycles(ctx, hash)
}

// GetPackedHeader implements rpc.Client
func (m *MockRPCClient) GetPackedHeader(ctx context.Context, hash types.Hash) (*types.Header, error) {
	return m.getPackedHeader(ctx, hash)
}

// GetPackedHeaderByNumber implements rpc.Client
func (m *MockRPCClient) GetPackedHeaderByNumber(ctx context.Context, number uint64) (*types.Header, error) {
	return m.getPackedHeaderByNumber(ctx, number)
}

// GetPeers implements rpc.Client
func (m *MockRPCClient) GetPeers(ctx context.Context) ([]*types.RemoteNode, error) {
	return m.getPeers(ctx)
}

func (m *MockRPCClient) GetPoolTxDetailInfo(ctx context.Context, hash types.Hash) (*types.PoolTxDetailInfo, error) {
	return m.getPoolTxDetailInfo(ctx, hash)
}

// GetRawTxPool implements rpc.Client
func (m *MockRPCClient) GetRawTxPool(ctx context.Context) (*types.RawTxPool, error) {
	return m.getRawTxPool(ctx)
}

// GetTipBlockNumber implements rpc.Client
func (m *MockRPCClient) GetTipBlockNumber(ctx context.Context) (uint64, error) {
	return m.getTipBlockNumber(ctx)
}

// GetTipHeader implements rpc.Client
func (m *MockRPCClient) GetTipHeader(ctx context.Context) (*types.Header, error) {
	return m.getTipHeader(ctx)
}

// GetTransaction implements rpc.Client
func (m *MockRPCClient) GetTransaction(ctx context.Context, hash types.Hash, onlyCommited *bool) (*types.TransactionWithStatus, error) {
	return m.getTransaction(ctx, hash, onlyCommited)
}

// GetTransactionAndWitnessProof implements rpc.Client
func (m *MockRPCClient) GetTransactionAndWitnessProof(ctx context.Context, txHashes []string, blockHash *types.Hash) (*types.TransactionAndWitnessProof, error) {
	return m.getTransactionAndWitnessProof(ctx, txHashes, blockHash)
}

// GetTransactionProof implements rpc.Client
func (m *MockRPCClient) GetTransactionProof(ctx context.Context, txHashes []string, blockHash *types.Hash) (*types.TransactionProof, error) {
	return m.getTransactionProof(ctx, txHashes, blockHash)
}

// GetTransactions implements rpc.Client
func (m *MockRPCClient) GetTransactions(ctx context.Context, searchKey *indexer.SearchKey, order indexer.SearchOrder, limit uint64, afterCursor string) (*indexer.TxsWithCell, error) {
	return m.getTransactions(ctx, searchKey, order, limit, afterCursor)
}

// GetTransactionsGrouped implements rpc.Client
func (m *MockRPCClient) GetTransactionsGrouped(ctx context.Context, searchKey *indexer.SearchKey, order indexer.SearchOrder, limit uint64, afterCursor string) (*indexer.TxsWithCells, error) {
	return m.getTransactionsGrouped(ctx, searchKey, order, limit, afterCursor)
}

// LocalNodeInfo implements rpc.Client
func (m *MockRPCClient) LocalNodeInfo(ctx context.Context) (*types.LocalNode, error) {
	return m.localNodeInfo(ctx)
}

// PingPeers implements rpc.Client
func (m *MockRPCClient) PingPeers(ctx context.Context) error {
	return m.pingPeers(ctx)
}

// RemoveNode implements rpc.Client
func (m *MockRPCClient) RemoveNode(ctx context.Context, peerId string) error {
	return m.removeNode(ctx, peerId)
}

// SendTransaction implements rpc.Client
func (m *MockRPCClient) SendTransaction(ctx context.Context, tx *types.Transaction) (*types.Hash, error) {
	return m.sendTransaction(ctx, tx)
}

// SetBan implements rpc.Client
func (m *MockRPCClient) SetBan(ctx context.Context, address string, command string, banTime uint64, absolute bool, reason string) error {
	return m.setBan(ctx, address, command, banTime, absolute, reason)
}

// SetNetworkActive implements rpc.Client
func (m *MockRPCClient) SetNetworkActive(ctx context.Context, state bool) error {
	return m.setNetworkActive(ctx, state)
}

// SyncState implements rpc.Client
func (m *MockRPCClient) SyncState(ctx context.Context) (*types.SyncState, error) {
	return m.syncState(ctx)
}

func (m *MockRPCClient) TestTxPoolAccept(ctx context.Context, tx *types.Transaction) (*types.EntryCompleted, error) {
	return m.testTxPoolAccept(ctx, tx)
}

// TxPoolInfo implements rpc.Client
func (m *MockRPCClient) TxPoolInfo(ctx context.Context) (*types.TxPoolInfo, error) {
	return m.txPoolInfo(ctx)
}

// VerifyTransactionAndWitnessProof implements rpc.Client
func (m *MockRPCClient) VerifyTransactionAndWitnessProof(ctx context.Context, proof *types.TransactionAndWitnessProof) ([]*types.Hash, error) {
	return m.verifyTransactionAndWitnessProof(ctx, proof)
}

// VerifyTransactionProof implements rpc.Client
func (m *MockRPCClient) VerifyTransactionProof(ctx context.Context, proof *types.TransactionProof) ([]*types.Hash, error) {
	return m.verifyTransactionProof(ctx, proof)
}
