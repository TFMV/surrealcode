package analysis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go/ast"
	"go/parser"
	"go/token"

	"github.com/TFMV/surrealcode/db"
	"github.com/TFMV/surrealcode/expr"
	surrealcode "github.com/TFMV/surrealcode/parser"
	"github.com/TFMV/surrealcode/types"
	"golang.org/x/sync/errgroup"
)

// Analyzer provides a high-level interface for code analysis and storage
type Analyzer struct {
	DB        db.DB
	ExprCache *expr.ExprCache
	Parser    *surrealcode.Parser
	Metrics   *MetricsAnalyzer
}

// MetricsAnalyzer handles all metrics computation
type MetricsAnalyzer struct {
	duplicationDetector *CodeDuplicationDetector
}

// NewMetricsAnalyzer creates a new metrics analyzer
func NewMetricsAnalyzer() *MetricsAnalyzer {
	return &MetricsAnalyzer{
		duplicationDetector: NewCodeDuplicationDetector(),
	}
}

func (m *MetricsAnalyzer) AnalyzeFunction(fn *ast.FuncDecl) (int, int, bool) {
	halstead := ComputeHalsteadMetrics(fn)
	readability := ComputeReadabilityMetrics(fn)
	isDuplicate := m.duplicationDetector.DetectDuplication(fn)

	return int(halstead.Difficulty), readability.FunctionLength, isDuplicate
}

// NewAnalyzer creates a new Analyzer with the given configuration
func NewAnalyzer(config db.Config) (*Analyzer, error) {
	sdb, err := db.NewSurrealDB(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create database connection: %w", err)
	}

	cache := expr.NewExprCache(10000)
	return &Analyzer{
		DB:        sdb,
		ExprCache: cache,
		Parser:    surrealcode.NewParser(cache),
		Metrics:   NewMetricsAnalyzer(),
	}, nil
}

// Initialize sets up the database connection and schema
func (a *Analyzer) Initialize(ctx context.Context) error {
	return a.DB.Initialize(ctx)
}

// AnalyzeDirectory scans a directory tree and stores analysis results
func (a *Analyzer) AnalyzeDirectory(ctx context.Context, dir string) error {
	report, err := a.GetAnalysis(ctx, dir)
	if err != nil {
		return fmt.Errorf("failed to analyze directory: %w", err)
	}

	if err := a.DB.StoreAnalysis(ctx, report); err != nil {
		return fmt.Errorf("failed to store analysis results: %w", err)
	}

	return nil
}

// GetAnalysis performs code analysis without storing results
func (a *Analyzer) GetAnalysis(ctx context.Context, dir string) (types.AnalysisReport, error) {
	var filePaths []string
	if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk directory: %w", err)
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			filePaths = append(filePaths, path)
		}
		return nil
	}); err != nil {
		return types.AnalysisReport{}, fmt.Errorf("failed to scan directory %s: %w", dir, err)
	}

	g, ctx := errgroup.WithContext(ctx)
	resultCh := make(chan surrealcode.FileAnalysis, len(filePaths))

	// Process files concurrently
	for _, path := range filePaths {
		path := path // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			analysis, err := a.Parser.ParseFile(path)
			if err != nil {
				return fmt.Errorf("error parsing %s: %w", path, err)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case resultCh <- analysis:
				return nil
			}
		})
	}

	// Close results channel when all goroutines complete
	go func() {
		g.Wait()
		close(resultCh)
	}()

	// Collect results
	var report types.AnalysisReport
	functionMap := make(map[string]types.FunctionCall)

	fset := token.NewFileSet()
	for res := range resultCh {
		for _, fn := range res.Functions {
			file, _ := parser.ParseFile(fset, fn.File, nil, parser.AllErrors) // Use fn.File
			if funcDecl := findFunction(file, fn.Caller); funcDecl != nil {
				complexity, loc, isDup := a.Metrics.AnalyzeFunction(funcDecl)
				fn.CyclomaticComplexity = complexity
				fn.LinesOfCode = loc
				fn.IsDuplicate = isDup
			}
			functionMap[fn.Caller] = fn
		}
		report.Structs = append(report.Structs, res.Structs...)
		report.Interfaces = append(report.Interfaces, res.Interfaces...)
		report.Globals = append(report.Globals, res.Globals...)
		report.Imports = append(report.Imports, res.Imports...)
	}

	// Detect recursion
	functionMap = DetectRecursion(functionMap)

	// Convert map to slice
	for _, fn := range functionMap {
		report.Functions = append(report.Functions, fn)
	}

	return report, nil
}

func findFunction(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}
