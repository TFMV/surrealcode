package analysis

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"

	"go/ast"
	"go/parser"
	"go/token"

	"github.com/TFMV/surrealcode/db"
	"github.com/TFMV/surrealcode/expr"
	surrealcode "github.com/TFMV/surrealcode/parser"
	"github.com/TFMV/surrealcode/types"
)

// Analyzer provides a high-level interface for code analysis and storage
type Analyzer struct {
	DB        db.DB
	ExprCache *expr.ExprCache
	Parser    *surrealcode.Parser
	Metrics   *MetricsAnalyzer
	Report    types.AnalysisReport
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
	fset := token.NewFileSet()
	readability := ComputeReadabilityMetrics(fn, fset)
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

// NewAnalyzerWithoutDB creates an analyzer without database connection
func NewAnalyzerWithoutDB() *Analyzer {
	cache := expr.NewExprCache(10000)
	return &Analyzer{
		ExprCache: cache,
		Parser:    surrealcode.NewParser(cache),
		Metrics:   NewMetricsAnalyzer(),
	}
}

// Initialize sets up the database connection and schema
func (a *Analyzer) Initialize(ctx context.Context) error {
	return a.DB.Initialize(ctx)
}

// AnalyzeDirectory scans a directory tree and stores analysis results
func (a *Analyzer) AnalyzeDirectory(ctx context.Context, dir string) error {
	fmt.Println("Starting analysis...")
	report, err := a.GetAnalysis(ctx, dir)
	if err != nil {
		return fmt.Errorf("failed to analyze directory: %w", err)
	}

	fmt.Println("Analysis complete, storing results...")
	if err := a.DB.StoreAnalysis(ctx, report); err != nil {
		return fmt.Errorf("failed to store analysis results: %w", err)
	}
	fmt.Println("Results stored successfully")

	return nil
}

// GetAnalysis performs code analysis without storing results
func (a *Analyzer) GetAnalysis(ctx context.Context, dir string) (types.AnalysisReport, error) {
	fmt.Println("Scanning directory:", dir)
	var filePaths []string
	if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			filePaths = append(filePaths, path)
		}
		return err
	}); err != nil {
		return types.AnalysisReport{}, err
	}

	fmt.Printf("Found %d Go files\n", len(filePaths))
	var report types.AnalysisReport
	functionMap := make(map[string]types.FunctionCall)
	fset := token.NewFileSet()

	// Process files sequentially
	for _, path := range filePaths {
		fmt.Printf("Processing file: %s\n", path)
		// Parse for analysis
		analysis, err := a.Parser.ParseFile(path)
		if err != nil {
			return types.AnalysisReport{}, err
		}

		// Parse for AST
		file, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
		if err != nil {
			return types.AnalysisReport{}, err
		}

		// Process functions
		for _, fn := range analysis.Functions {
			if funcDecl := findFunction(file, fn.Caller); funcDecl != nil {
				fn.Metrics = computeMetrics(a.Metrics, funcDecl, fset)
			}
			functionMap[fn.Caller] = fn
		}

		// Collect other data
		report.Structs = append(report.Structs, analysis.Structs...)
		report.Interfaces = append(report.Interfaces, analysis.Interfaces...)
		report.Globals = append(report.Globals, analysis.Globals...)
		report.Imports = append(report.Imports, analysis.Imports...)
	}

	// Post-process
	functionMap = DetectRecursion(functionMap)
	deadCode := DetectDeadCode(functionMap, []string{"main", "complex"})

	// Convert to final report
	for _, fn := range functionMap {
		fn.Metrics.IsUnused = slices.Contains(deadCode.UnusedFunctions, fn.Caller)
		report.Functions = append(report.Functions, fn)
	}

	fmt.Println("Post-processing results...")
	a.Report = report
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

// GenerateCodeSummary creates a summary report from analysis results
func (a *Analyzer) GenerateCodeSummary(report types.AnalysisReport) types.CodeSummary {
	summary := types.CodeSummary{
		ComplexityDistribution: make(map[string]int),
	}

	var totalComplexity, totalMaintainability, totalNesting float64

	for _, fn := range report.Functions {
		// Count totals
		summary.TotalFunctions++
		summary.TotalLines += fn.Metrics.LinesOfCode

		if fn.Metrics.IsUnused {
			summary.UnusedFunctions++
		}
		if fn.IsRecursive {
			summary.RecursiveFunctions++
		}
		if fn.Metrics.IsDuplicate {
			summary.DuplicateCode++
		}

		// Calculate averages
		totalComplexity += float64(fn.Metrics.CyclomaticComplexity)
		totalMaintainability += fn.Metrics.Maintainability
		totalNesting += float64(fn.Metrics.Readability.NestingDepth)

		// Categorize complexity
		switch {
		case fn.Metrics.CyclomaticComplexity <= 5:
			summary.ComplexityDistribution["Low"]++
		case fn.Metrics.CyclomaticComplexity <= 10:
			summary.ComplexityDistribution["Medium"]++
		default:
			summary.ComplexityDistribution["High"]++
		}

		// Identify hotspots
		if isHotspot(fn.Metrics) {
			issues := identifyIssues(fn.Metrics)
			hotspot := types.HotspotFunction{
				Name:            fn.Caller,
				File:            fn.File,
				Complexity:      fn.Metrics.CyclomaticComplexity,
				Maintainability: fn.Metrics.Maintainability,
				Issues:          issues,
			}
			summary.Hotspots = append(summary.Hotspots, hotspot)
		}
	}

	// Calculate averages
	if summary.TotalFunctions > 0 {
		summary.AvgComplexity = totalComplexity / float64(summary.TotalFunctions)
		summary.AvgMaintainability = totalMaintainability / float64(summary.TotalFunctions)
		summary.AvgNestingDepth = totalNesting / float64(summary.TotalFunctions)
	}

	// Sort hotspots by complexity
	sort.Slice(summary.Hotspots, func(i, j int) bool {
		return summary.Hotspots[i].Complexity > summary.Hotspots[j].Complexity
	})

	return summary
}

func isHotspot(metrics types.FunctionMetrics) bool {
	return metrics.CyclomaticComplexity > 10 ||
		metrics.Readability.NestingDepth > 4 ||
		metrics.Maintainability < 50
}

func identifyIssues(metrics types.FunctionMetrics) []string {
	var issues []string

	if metrics.CyclomaticComplexity > 10 {
		issues = append(issues, "High cyclomatic complexity")
	}
	if metrics.Readability.NestingDepth > 4 {
		issues = append(issues, "Deep nesting")
	}
	if metrics.Maintainability < 50 {
		issues = append(issues, "Low maintainability")
	}
	if metrics.IsDuplicate {
		issues = append(issues, "Code duplication")
	}
	if metrics.CognitiveComplexity.Score > 15 {
		issues = append(issues, "High cognitive complexity")
	}

	return issues
}

// CodeDuplicationDetector with thread safety
func (c *CodeDuplicationDetector) DetectDuplication(fn *ast.FuncDecl) bool {
	body := extractFunctionBody(fn)
	hash := rabinKarpHash(body)

	c.mu.RLock()
	_, exists := c.seen[hash]
	c.mu.RUnlock()

	if exists {
		return true
	}

	c.mu.Lock()
	c.seen[hash] = body
	c.mu.Unlock()
	return false
}

func computeMetrics(metricsAnalyzer *MetricsAnalyzer, fn *ast.FuncDecl, fset *token.FileSet) types.FunctionMetrics {
	halstead := ComputeHalsteadMetrics(fn)
	readability := ComputeReadabilityMetrics(fn, fset)
	cognitive := ComputeCognitiveComplexity(fn)
	isDup := metricsAnalyzer.duplicationDetector.DetectDuplication(fn)

	return types.FunctionMetrics{
		CyclomaticComplexity: cognitive.BranchingScore + 1,
		LinesOfCode:          CountLines(fn, fset),
		IsDuplicate:          isDup,
		HalsteadMetrics: struct {
			Volume     float64 `json:"volume"`
			Difficulty float64 `json:"difficulty"`
			Effort     float64 `json:"effort"`
		}{
			Volume:     halstead.Volume,
			Difficulty: halstead.Difficulty,
			Effort:     halstead.Effort,
		},
		CognitiveComplexity: struct {
			Score          int `json:"score"`
			NestedDepth    int `json:"nested_depth"`
			LogicalOps     int `json:"logical_ops"`
			BranchingScore int `json:"branching_score"`
		}{
			Score:          cognitive.Score,
			NestedDepth:    cognitive.NestedDepth,
			LogicalOps:     cognitive.LogicalOps,
			BranchingScore: cognitive.BranchingScore,
		},
		Readability: struct {
			NestingDepth   int     `json:"nesting_depth"`
			CommentDensity float64 `json:"comment_density"`
			BranchDensity  float64 `json:"branch_density"`
		}{
			NestingDepth:   readability.NestingDepth,
			CommentDensity: readability.CommentDensity,
			BranchDensity:  readability.BranchDensity,
		},
		Maintainability: MaintainabilityIndex(readability, cognitive.BranchingScore, isDup),
	}
}

// ComputeReadabilityMetrics analyzes a function for readability heuristics.
func ComputeReadabilityMetrics(fn *ast.FuncDecl, fset *token.FileSet) CodeReadabilityMetrics {
	loc := CountLines(fn, fset)
	nestingDepth := 0
	commentCount := 0
	branchCount := 0
	maxNesting := 0
	currentNesting := 0

	ast.Inspect(fn, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.BlockStmt:
			currentNesting++
			if currentNesting > maxNesting {
				maxNesting = currentNesting
			}
			nestingDepth++
		case *ast.Comment:
			commentCount++
		case *ast.IfStmt, *ast.ForStmt, *ast.SwitchStmt, *ast.SelectStmt:
			branchCount++
		}
		return true
	})

	commentDensity := float64(commentCount) / float64(loc)
	branchDensity := float64(branchCount) / float64(loc)

	return CodeReadabilityMetrics{
		FunctionLength:   loc,
		NestingDepth:     maxNesting,
		CommentDensity:   commentDensity,
		CyclomaticPoints: branchCount + 1, // Base complexity + branches
		BranchDensity:    branchDensity,
	}
}

// MaintainabilityIndex computes a maintainability score.
func MaintainabilityIndex(metrics CodeReadabilityMetrics, complexity int, duplication bool) float64 {
	// Normalize inputs
	normalizedLOC := math.Min(float64(metrics.FunctionLength), 100) / 100
	normalizedComplexity := math.Min(float64(complexity), 50) / 50
	normalizedCommentDensity := math.Min(metrics.CommentDensity, 0.4)

	// Base score starts at 100
	score := 100.0

	// Deduct points based on normalized metrics
	score -= 20 * normalizedLOC            // Up to 20 points for length
	score -= 30 * normalizedComplexity     // Up to 30 points for complexity
	score += 10 * normalizedCommentDensity // Up to 10 points bonus for comments

	// Apply penalties
	if duplication {
		score *= 0.8 // 20% penalty for duplication
	}

	if metrics.NestingDepth > 3 {
		score *= 0.9 // 10% penalty for deep nesting
	}

	if metrics.BranchDensity > 0.5 {
		score *= 0.9 // 10% penalty for high branch density
	}

	return score
}

// CountLines counts the actual number of lines in a function
func CountLines(fn *ast.FuncDecl, fset *token.FileSet) int {
	if fset == nil {
		return 0
	}

	startLine := fset.Position(fn.Pos()).Line
	endLine := fset.Position(fn.End()).Line
	return endLine - startLine + 1
}

// ComputeCognitiveComplexity analyzes the cognitive complexity of a function
func ComputeCognitiveComplexity(fn *ast.FuncDecl) CognitiveComplexity {
	var cc CognitiveComplexity

	// visit recursively traverses the AST, passing along the current nesting level.
	var visit func(n ast.Node, nesting int)
	visit = func(n ast.Node, nesting int) {
		if n == nil {
			return
		}
		// Update maximum nesting depth if needed.
		if nesting > cc.NestedDepth {
			cc.NestedDepth = nesting
		}
		switch node := n.(type) {
		case *ast.IfStmt:
			cc.BranchingScore++
			cc.Score += 1               // if statement adds 1
			visit(node.Cond, nesting)   // condition: same nesting
			visit(node.Body, nesting+1) // body: deeper nesting
			visit(node.Else, nesting+1) // else branch: deeper nesting
			return
		case *ast.ForStmt:
			cc.BranchingScore++
			cc.Score += 2 // for loop adds 2
			if node.Init != nil {
				visit(node.Init, nesting)
			}
			if node.Cond != nil {
				visit(node.Cond, nesting)
			}
			if node.Post != nil {
				visit(node.Post, nesting)
			}
			visit(node.Body, nesting+1)
			return
		case *ast.RangeStmt:
			cc.BranchingScore++
			cc.Score += 2 // range loop adds 2
			visit(node.Body, nesting+1)
			return
		case *ast.SwitchStmt:
			cc.BranchingScore++
			cc.Score += 1 // switch adds 1
			if node.Tag != nil {
				visit(node.Tag, nesting)
			}
			visit(node.Body, nesting+1)
			return
		case *ast.BinaryExpr:
			if node.Op == token.LAND || node.Op == token.LOR {
				cc.LogicalOps++
				cc.Score += 1 // each && or || adds 1
			}
			visit(node.X, nesting)
			visit(node.Y, nesting)
			return
		}
		// For any other node, traverse its children.
		ast.Inspect(n, func(child ast.Node) bool {
			// Skip the current node to avoid infinite recursion.
			if child == n {
				return true
			}
			visit(child, nesting)
			return false
		})
	}

	visit(fn, 0)
	return cc
}

// DetectDeadCode analyzes the call graph to find unused functions
func DetectDeadCode(functions map[string]types.FunctionCall, entryPoints []string) DeadCodeInfo {
	var info DeadCodeInfo
	info.Reachable = make(map[string]bool)

	// First mark entry points as reachable
	for _, entry := range entryPoints {
		markReachable(entry, functions, info.Reachable)
	}

	// Then mark exported functions as reachable
	for fname := range functions {
		if isExported(fname) {
			markReachable(fname, functions, info.Reachable)
		}
	}

	// Find unused functions (only unexported and unreachable)
	for fname := range functions {
		if !info.Reachable[fname] && !isExported(fname) && !slices.Contains(entryPoints, fname) {
			info.UnusedFunctions = append(info.UnusedFunctions, fname)
		}
	}

	return info
}

// markReachable recursively marks all functions reachable from the given function
func markReachable(fname string, functions map[string]types.FunctionCall, reachable map[string]bool) {
	if reachable[fname] {
		return // Already visited
	}
	reachable[fname] = true

	// Mark all called functions as reachable
	if fn, exists := functions[fname]; exists {
		for _, callee := range fn.Callees {
			if !strings.Contains(callee, ".") { // Skip package-qualified calls
				markReachable(callee, functions, reachable)
			}
		}
	}
}

// isExported returns true if the function name starts with an uppercase letter
func isExported(fname string) bool {
	if len(fname) == 0 {
		return false
	}
	return unicode.IsUpper(rune(fname[0]))
}
