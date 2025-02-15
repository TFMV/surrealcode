package db

import (
	"context"

	"github.com/TFMV/surrealcode/types"
)

type DB interface {
	Initialize(ctx context.Context) error
	StoreAnalysis(ctx context.Context, report types.AnalysisReport) error
}
