package test

import (
	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

type MockHandler struct {
	group *types.Script
}

type MockContext struct{}

func NewMockHandler(g *types.Script) *MockHandler {
	return &MockHandler{
		group: g,
	}
}

// BuildTransaction implements collector.ScriptHandler
func (*MockHandler) BuildTransaction(builder collector.TransactionBuilder, group *transaction.ScriptGroup, context interface{}) (bool, error) {
	return true, nil
}

var _ collector.ScriptHandler = (*MockHandler)(nil)
