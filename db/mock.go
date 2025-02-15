package db

import (
	"context"

	"github.com/TFMV/surrealcode/types"
)

type MockDB struct {
	InitializeFunc    func(ctx context.Context) error
	StoreAnalysisFunc func(ctx context.Context, report types.AnalysisReport) error
}

func NewMockDB() *MockDB {
	return &MockDB{
		InitializeFunc: func(ctx context.Context) error {
			return nil
		},
	}
}

func (m *MockDB) Initialize(ctx context.Context) error {
	return m.InitializeFunc(ctx)
}

func (m *MockDB) StoreAnalysis(ctx context.Context, report types.AnalysisReport) error {
	if m.StoreAnalysisFunc != nil {
		return m.StoreAnalysisFunc(ctx, report)
	}
	return nil
}
