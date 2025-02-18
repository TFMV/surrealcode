package analysis

import (
	"context"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/TFMV/surrealcode/db"
	"github.com/TFMV/surrealcode/expr"
	surrealtypes "github.com/TFMV/surrealcode/types"
)

// -----------------------------------------------------------------------------
// Analyzer and Metrics Types
// -----------------------------------------------------------------------------

// Analyzer provides a high-level interface for code analysis and storage.
type Analyzer struct {
	DB        db.DB
	ExprCache *expr.ExprCache
	Metrics   *MetricsAnalyzer
	Report    surrealtypes.AnalysisReport
}

// MetricsAnalyzer handles all metrics computation.
type MetricsAnalyzer struct {
	duplicationDetector *CodeDuplicationDetector
}

// NewMetricsAnalyzer creates a new metrics analyzer.
func NewMetricsAnalyzer() *MetricsAnalyzer {
	return &MetricsAnalyzer{
		duplicationDetector: NewCodeDuplicationDetector(),
	}
}

// NewAnalyzer creates a new Analyzer with the given configuration.
func NewAnalyzer(config db.Config) (*Analyzer, error) {
	sdb, err := db.NewSurrealDB(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create database connection: %w", err)
	}
	cache := expr.NewExprCache(10000)
	analyzer := &Analyzer{
		DB:        sdb,
		ExprCache: cache,
		Metrics:   NewMetricsAnalyzer(),
	}

	// Add cleanup for database connection
	runtime.AddCleanup(analyzer, func(db db.DB) {
		if closer, ok := db.(interface{ Close() error }); ok {
			closer.Close()
		}
	}, analyzer.DB)

	return analyzer, nil
}

// NewAnalyzerWithoutDB creates an analyzer without a database connection.
func NewAnalyzerWithoutDB() *Analyzer {
	cache := expr.NewExprCache(10000)
	return &Analyzer{
		ExprCache: cache,
		Metrics:   NewMetricsAnalyzer(),
	}
}

// Initialize sets up the database connection and schema.
func (a *Analyzer) Initialize(ctx context.Context) error {
	return a.DB.Initialize(ctx)
}

// -----------------------------------------------------------------------------
// File Analysis (Formerly in parser.go)
// -----------------------------------------------------------------------------

// FileAnalysis represents the analysis results of a single file.
type FileAnalysis struct {
	Functions  []surrealtypes.FunctionCall
	Structs    []surrealtypes.StructDefinition
	Interfaces []surrealtypes.InterfaceDefinition
	Globals    []surrealtypes.GlobalVariable
	Imports    []surrealtypes.ImportDefinition
	Implements []surrealtypes.InterfaceImplementation
}

type HalsteadMetrics struct {
	Operators       int     // Total occurrences of operators
	Operands        int     // Total occurrences of operands
	UniqueOperators int     // Unique operators
	UniqueOperands  int     // Unique operands
	Volume          float64 // Volume = (N1 + N2) * log2(n1 + n2)
	Difficulty      float64 // Difficulty = (n1/2) * (N2/n2)
	Effort          float64 // Effort = Difficulty * Volume
}

// AnalyzeFile parses and analyzes a single file.
// This method merges the logic formerly in your SurrealParser.
func (a *Analyzer) AnalyzeFile(path string) (FileAnalysis, error) {
	fset := token.NewFileSet()

	// Parse file using go/parser.
	file, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
	if err != nil {
		return FileAnalysis{}, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	pkgName := file.Name.Name

	var functions []surrealtypes.FunctionCall
	var structs []surrealtypes.StructDefinition
	var interfaces []surrealtypes.InterfaceDefinition
	var globals []surrealtypes.GlobalVariable
	var imports []surrealtypes.ImportDefinition
	var implements []surrealtypes.InterfaceImplementation

	// Maps for type checking.
	structIdents := make(map[string]*ast.Ident)
	ifaceIdents := make(map[string]*ast.Ident)

	// Process top-level declarations.
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			// Build full function/method name.
			methodName := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				recvType := simpleTypeString(d.Recv.List[0].Type)
				methodName = fmt.Sprintf("%s.%s", recvType, d.Name.Name)
			}
			fn := surrealtypes.FunctionCall{
				Caller:            methodName,
				File:              path,
				Package:           pkgName,
				Params:            []string{},
				Returns:           []string{},
				Callees:           []string{},
				ReferencedGlobals: []string{},
				Dependencies:      []string{},
			}
			// Extract parameter types.
			if d.Type.Params != nil {
				for _, param := range d.Type.Params.List {
					fn.Params = append(fn.Params, simpleTypeString(param.Type))
				}
			}
			// Extract return types.
			if d.Type.Results != nil {
				for _, ret := range d.Type.Results.List {
					fn.Returns = append(fn.Returns, simpleTypeString(ret.Type))
				}
			}
			if d.Recv != nil {
				fn.IsMethod = true
				if len(d.Recv.List) > 0 {
					fn.Struct = simpleTypeString(d.Recv.List[0].Type)
				}
			}
			// Track globals and dependencies via a simple AST inspection.
			ast.Inspect(d.Body, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.SelectorExpr:
					if ident, ok := node.X.(*ast.Ident); ok {
						for _, imp := range imports {
							if strings.HasSuffix(imp.Path, ident.Name) {
								fn.Dependencies = append(fn.Dependencies, imp.Path)
							}
						}
					}
				case *ast.Ident:
					// If the identifier matches a global name, record it.
					for _, global := range globals {
						if node.Name == global.Name {
							fn.ReferencedGlobals = append(fn.ReferencedGlobals, global.Name)
						}
					}
				}
				return true
			})
			functions = append(functions, fn)

		case *ast.GenDecl:
			switch d.Tok {
			case token.IMPORT:
				for _, spec := range d.Specs {
					if impSpec, ok := spec.(*ast.ImportSpec); ok {
						imports = append(imports, surrealtypes.ImportDefinition{
							Path:    strings.Trim(impSpec.Path.Value, `"`),
							File:    path,
							Package: pkgName,
						})
					}
				}
			case token.VAR, token.CONST:
				for _, spec := range d.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for i, name := range vs.Names {
							var valueStr string
							if i < len(vs.Values) {
								valueStr = a.ExprCache.ToString(vs.Values[i])
							}
							globals = append(globals, surrealtypes.GlobalVariable{
								Name:    name.Name,
								Type:    a.ExprCache.ToString(vs.Type),
								Value:   valueStr,
								File:    path,
								Package: pkgName,
							})
						}
					}
				}
			case token.TYPE:
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						switch t := ts.Type.(type) {
						case *ast.StructType:
							structs = append(structs, surrealtypes.StructDefinition{
								Name:    ts.Name.Name,
								File:    path,
								Package: pkgName,
							})
							structIdents[ts.Name.Name] = ts.Name
						case *ast.InterfaceType:
							var methods []string
							for _, m := range t.Methods.List {
								if m.Names != nil {
									for _, n := range m.Names {
										methods = append(methods, n.Name)
									}
								}
							}
							interfaces = append(interfaces, surrealtypes.InterfaceDefinition{
								Name:    ts.Name.Name,
								File:    path,
								Package: pkgName,
								Methods: methods,
							})
							ifaceIdents[ts.Name.Name] = ts.Name
						}
					}
				}
			}
		}
	}

	// Perform type checking using Go's type checker.
	conf := types.Config{
		Importer: importer.ForCompiler(fset, "source", nil),
		Error:    func(err error) {}, // ignore errors
	}
	info := &types.Info{
		Defs: make(map[*ast.Ident]types.Object),
	}
	pkgInfo := types.NewPackage(pkgName, "")
	_, err = conf.Check(pkgInfo.Path(), fset, []*ast.File{file}, info)
	if err != nil {
		fmt.Printf("Type checking skipped for %s: %v\n", path, err)
		// Continue with AST-based analysis
	} else {
		// For each struct and interface, check for implementations.
		for _, st := range structs {
			ident, ok := structIdents[st.Name]
			if !ok {
				continue
			}
			obj := info.Defs[ident]
			if obj == nil {
				continue
			}
			structType := obj.Type()
			for _, iface := range interfaces {
				iident, ok := ifaceIdents[iface.Name]
				if !ok {
					continue
				}
				iobj := info.Defs[iident]
				if iobj == nil {
					continue
				}
				ifaceType := iobj.Type()
				if ifaceType == nil {
					continue
				}
				ifaceUnderlying, ok := ifaceType.Underlying().(*types.Interface)
				if !ok {
					continue
				}
				if types.Implements(structType, ifaceUnderlying) || types.Implements(types.NewPointer(structType), ifaceUnderlying) {
					implements = append(implements, surrealtypes.InterfaceImplementation{
						Struct:    st.Name,
						Interface: iface.Name,
					})
				}
			}
		}
	}

	// Process functions further to calculate metrics.
	// (We loop again over our functions slice and try to find their AST node.)
	detector := NewCodeDuplicationDetector()
	for i := range functions {
		funcDecl := findFunctionDecl(file, functions[i].Caller)
		if funcDecl != nil {
			// Check for duplication before the first function
			if detector.DetectDuplication(funcDecl) {
				functions[i].IsDuplicate = true
			}
			// Calculate metrics after duplication check
			complexity := ComputeComplexity(funcDecl)
			loc := ComputeLOC(fset, funcDecl.Body)
			readability := ComputeReadabilityMetrics(funcDecl, fset)
			halstead := ComputeHalsteadMetrics(funcDecl)
			cognitive := ComputeCognitiveComplexity(funcDecl)

			functions[i].Metrics = surrealtypes.FunctionMetrics{
				CyclomaticComplexity: complexity,
				LinesOfCode:          loc,
				HalsteadMetrics:      halstead,
				CognitiveComplexity:  cognitive,
				Readability: surrealtypes.ReadabilityMetrics{
					NestingDepth:   readability.NestingDepth,
					CommentDensity: readability.CommentDensity,
					BranchDensity:  readability.BranchDensity,
				},
				Maintainability: calculateMaintainability(readability, complexity),
			}
		}
	}

	return FileAnalysis{
		Functions:  functions,
		Structs:    structs,
		Interfaces: interfaces,
		Globals:    globals,
		Imports:    imports,
		Implements: implements,
	}, nil
}

// simpleTypeString converts an AST expression representing a type into a string.
func simpleTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + simpleTypeString(t.X)
	case *ast.SelectorExpr:
		return simpleTypeString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + simpleTypeString(t.Elt)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// -----------------------------------------------------------------------------
// Analyzer Workflow
// -----------------------------------------------------------------------------

// AnalyzeDirectory scans a directory tree and stores analysis results.
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

// GetAnalysis performs code analysis without storing results.
func (a *Analyzer) GetAnalysis(ctx context.Context, dir string) (surrealtypes.AnalysisReport, error) {
	fmt.Println("Scanning directory:", dir)
	var filePaths []string
	if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			filePaths = append(filePaths, path)
		}
		return err
	}); err != nil {
		return surrealtypes.AnalysisReport{}, err
	}
	fmt.Printf("Found %d Go files\n", len(filePaths))
	var report surrealtypes.AnalysisReport
	functionMap := make(map[string]surrealtypes.FunctionCall)

	// Process each file.
	for _, path := range filePaths {
		fmt.Printf("Processing file: %s\n", path)
		analysis, err := a.AnalyzeFile(path)
		if err != nil {
			return surrealtypes.AnalysisReport{}, err
		}
		// Merge functions from this file.
		for _, fn := range analysis.Functions {
			functionMap[fn.Caller] = fn
		}
		// Merge other collected types.
		report.Structs = append(report.Structs, analysis.Structs...)
		report.Interfaces = append(report.Interfaces, analysis.Interfaces...)
		report.Globals = append(report.Globals, analysis.Globals...)
		report.Imports = append(report.Imports, analysis.Imports...)
		report.Implements = append(report.Implements, analysis.Implements...)
	}

	// Add recursion detection
	functionMap = detectRecursion(functionMap)

	// Build the final report
	report = surrealtypes.AnalysisReport{
		Functions:  make([]surrealtypes.FunctionCall, 0, len(functionMap)),
		Structs:    report.Structs,
		Interfaces: report.Interfaces,
		Globals:    report.Globals,
		Imports:    report.Imports,
		Implements: report.Implements,
	}

	// Convert map to slice
	for _, fn := range functionMap {
		report.Functions = append(report.Functions, fn)
	}

	// Post-process: detect dead code.
	deadCode := DetectDeadCode(functionMap, []string{"main", "complex"})
	for i := range report.Functions {
		report.Functions[i].Metrics.IsUnused = slices.Contains(deadCode.UnusedFunctions, report.Functions[i].Caller)
	}
	fmt.Println("Post-processing results...")
	a.Report = report
	return report, nil
}

// GenerateCodeSummary creates a summary report from analysis results.
func (a *Analyzer) GenerateCodeSummary(report surrealtypes.AnalysisReport) surrealtypes.CodeSummary {
	summary := surrealtypes.CodeSummary{
		ComplexityDistribution: make(map[string]int),
	}
	var totalComplexity, totalMaintainability, totalNesting float64
	for _, fn := range report.Functions {
		summary.TotalFunctions++
		summary.TotalLines += fn.Metrics.LinesOfCode
		if fn.Metrics.IsUnused {
			summary.UnusedFunctions++
		}
		if fn.IsRecursive {
			summary.RecursiveFunctions++
		}
		if fn.IsDuplicate {
			summary.DuplicateCode++
		}
		totalComplexity += float64(fn.Metrics.CyclomaticComplexity)
		totalMaintainability += fn.Metrics.Maintainability
		totalNesting += float64(fn.Metrics.Readability.NestingDepth)
		switch {
		case fn.Metrics.CyclomaticComplexity <= 5:
			summary.ComplexityDistribution["Low"]++
		case fn.Metrics.CyclomaticComplexity <= 10:
			summary.ComplexityDistribution["Medium"]++
		default:
			summary.ComplexityDistribution["High"]++
		}
		if isHotspot(fn.Metrics) {
			issues := identifyIssues(fn.Metrics)
			hotspot := surrealtypes.HotspotFunction{
				Name:            fn.Caller,
				File:            fn.File,
				Complexity:      fn.Metrics.CyclomaticComplexity,
				Maintainability: fn.Metrics.Maintainability,
				Issues:          issues,
			}
			summary.Hotspots = append(summary.Hotspots, hotspot)
		}
	}
	if summary.TotalFunctions > 0 {
		sf := float64(summary.TotalFunctions)
		summary.AvgComplexity = totalComplexity / sf
		summary.AvgMaintainability = totalMaintainability / sf
		summary.AvgNestingDepth = totalNesting / sf
	}
	sort.Slice(summary.Hotspots, func(i, j int) bool {
		return summary.Hotspots[i].Complexity > summary.Hotspots[j].Complexity
	})
	return summary
}

// -----------------------------------------------------------------------------
// Helper Functions and Metrics Computation
// -----------------------------------------------------------------------------

type DeadCodeInfo struct {
	Reachable       map[string]bool
	UnusedFunctions []string
}

func DetectDeadCode(functions map[string]surrealtypes.FunctionCall, entryPoints []string) DeadCodeInfo {
	var info DeadCodeInfo
	info.Reachable = make(map[string]bool)
	for _, entry := range entryPoints {
		markReachable(entry, functions, info.Reachable)
	}
	for fname := range functions {
		if isExported(fname) {
			markReachable(fname, functions, info.Reachable)
		}
	}
	for fname := range functions {
		if !info.Reachable[fname] && !isExported(fname) && !slices.Contains(entryPoints, fname) {
			info.UnusedFunctions = append(info.UnusedFunctions, fname)
		}
	}
	return info
}

func markReachable(fname string, functions map[string]surrealtypes.FunctionCall, reachable map[string]bool) {
	if reachable[fname] {
		return
	}
	reachable[fname] = true
	if fn, exists := functions[fname]; exists {
		for _, callee := range fn.Callees {
			if !strings.Contains(callee, ".") {
				markReachable(callee, functions, reachable)
			}
		}
	}
}

func isExported(fname string) bool {
	if len(fname) == 0 {
		return false
	}
	return unicode.IsUpper(rune(fname[0]))
}

func isHotspot(metrics surrealtypes.FunctionMetrics) bool {
	return metrics.CyclomaticComplexity > 10 ||
		metrics.Readability.NestingDepth > 4 ||
		metrics.Maintainability < 50
}

func identifyIssues(metrics surrealtypes.FunctionMetrics) []string {
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
	if metrics.CognitiveComplexity.Score > 15 {
		issues = append(issues, "High cognitive complexity")
	}
	return issues
}

func ComputeComplexity(node ast.Node) int {
	complexity := 1
	ast.Inspect(node, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause, *ast.SelectStmt:
			complexity++
		case *ast.BinaryExpr:
			bin := n.(*ast.BinaryExpr)
			if bin.Op == token.LAND || bin.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	return complexity
}

func ComputeLOC(fset *token.FileSet, node *ast.BlockStmt) int {
	if node == nil {
		return 0
	}
	return fset.Position(node.End()).Line - fset.Position(node.Pos()).Line + 1
}

func CountLines(fn *ast.FuncDecl, fset *token.FileSet) int {
	if fset == nil {
		return 0
	}
	startLine := fset.Position(fn.Pos()).Line
	endLine := fset.Position(fn.End()).Line
	return endLine - startLine + 1
}

type CodeReadabilityMetrics struct {
	FunctionLength   int
	NestingDepth     int
	CommentDensity   float64
	CyclomaticPoints int
	BranchDensity    float64
}

func ComputeReadabilityMetrics(fn *ast.FuncDecl, fset *token.FileSet) CodeReadabilityMetrics {
	loc := CountLines(fn, fset)
	acc := struct {
		branchCount  int
		commentCount int
		maxNesting   int
	}{}
	var recReadability func(n ast.Node, currentNesting int)
	recReadability = func(n ast.Node, currentNesting int) {
		if n == nil {
			return
		}
		if currentNesting > acc.maxNesting {
			acc.maxNesting = currentNesting
		}
		switch node := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.SwitchStmt, *ast.SelectStmt:
			acc.branchCount++
			if ifNode, ok := node.(*ast.IfStmt); ok {
				recReadability(ifNode.Cond, currentNesting)
				recReadability(ifNode.Body, currentNesting+1)
				if ifNode.Else != nil {
					recReadability(ifNode.Else, currentNesting+1)
				}
				return
			}
			if forNode, ok := node.(*ast.ForStmt); ok {
				recReadability(forNode.Init, currentNesting)
				recReadability(forNode.Cond, currentNesting)
				recReadability(forNode.Post, currentNesting)
				recReadability(forNode.Body, currentNesting+1)
				return
			}
			if rangeNode, ok := node.(*ast.RangeStmt); ok {
				recReadability(rangeNode.X, currentNesting)
				recReadability(rangeNode.Body, currentNesting+1)
				return
			}
			if switchNode, ok := node.(*ast.SwitchStmt); ok {
				recReadability(switchNode.Tag, currentNesting)
				recReadability(switchNode.Body, currentNesting+1)
				return
			}
		case *ast.Comment:
			acc.commentCount++
		}
		for _, child := range children(n) {
			recReadability(child, currentNesting)
		}
	}
	recReadability(fn, 0)
	commentDensity, branchDensity := 0.0, 0.0
	if loc > 0 {
		commentDensity = float64(acc.commentCount) / float64(loc)
		branchDensity = float64(acc.branchCount) / float64(loc)
	}
	return CodeReadabilityMetrics{
		FunctionLength:   loc,
		NestingDepth:     acc.maxNesting,
		CommentDensity:   commentDensity,
		CyclomaticPoints: acc.branchCount + 1,
		BranchDensity:    branchDensity,
	}
}

func children(n ast.Node) []ast.Node {
	var out []ast.Node
	ast.Inspect(n, func(child ast.Node) bool {
		if child != n && child != nil {
			out = append(out, child)
			return false
		}
		return true
	})
	return out
}

func calculateMaintainability(r CodeReadabilityMetrics, complexity int) float64 {
	return 171 - 5.2*math.Log(float64(complexity)) - 0.23*float64(r.NestingDepth)
}

// CodeDuplicationDetector with thread safety.
type CodeDuplicationDetector struct {
	mu   sync.RWMutex
	seen map[uint64]string
}

func NewCodeDuplicationDetector() *CodeDuplicationDetector {
	return &CodeDuplicationDetector{
		seen: make(map[uint64]string),
	}
}

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

func extractFunctionBody(fn *ast.FuncDecl) string {
	if fn.Body == nil {
		return ""
	}
	// Remove comments and normalize whitespace
	var buf strings.Builder
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.Ident:
			buf.WriteString(node.Name)
			buf.WriteString(" ")
		case *ast.BasicLit:
			buf.WriteString(node.Value)
			buf.WriteString(" ")
		case *ast.BinaryExpr:
			buf.WriteString(node.Op.String())
			buf.WriteString(" ")
		case *ast.ReturnStmt:
			buf.WriteString("return ")
		}
		return true
	})
	return strings.TrimSpace(buf.String())
}

func rabinKarpHash(s string) uint64 {
	// Placeholder hash function (replace with a proper implementation as needed)
	var hash uint64
	for i := 0; i < len(s); i++ {
		hash = hash*31 + uint64(s[i])
	}
	return hash
}

// ComputeHalsteadMetrics is a stub for Halstead metric calculation.
func ComputeHalsteadMetrics(fn *ast.FuncDecl) surrealtypes.HalsteadMetrics {
	operators := make(map[string]int)
	operands := make(map[string]int)

	ast.Inspect(fn, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.BinaryExpr:
			operators[node.Op.String()]++
		case *ast.UnaryExpr:
			operators[node.Op.String()]++
		case *ast.Ident:
			operands[node.Name]++
		case *ast.BasicLit:
			operands[node.Value]++
		}
		return true
	})

	n1 := len(operators) // unique operators
	n2 := len(operands)  // unique operands
	N1 := 0              // total operators
	N2 := 0              // total operands

	for _, count := range operators {
		N1 += count
	}
	for _, count := range operands {
		N2 += count
	}

	volume := float64(N1+N2) * math.Log2(float64(n1+n2))
	difficulty := float64(n1) * float64(N2) / (2.0 * float64(n2))
	effort := difficulty * volume

	return surrealtypes.HalsteadMetrics{
		Volume:     volume,
		Difficulty: difficulty,
		Effort:     effort,
	}
}

// ComputeCognitiveComplexity computes cognitive complexity via a recursive AST traversal.
type CognitiveComplexity struct {
	Score          int
	NestedDepth    int
	LogicalOps     int
	BranchingScore int
}

func ComputeCognitiveComplexity(fn *ast.FuncDecl) surrealtypes.CognitiveComplexityMetrics {
	cc := CognitiveComplexity{}
	maxDepth := 0
	var recursiveVisit func(n ast.Node, depth int)
	recursiveVisit = func(n ast.Node, depth int) {
		if n == nil {
			return
		}
		if depth > maxDepth {
			maxDepth = depth
		}
		switch node := n.(type) {
		case *ast.IfStmt:
			recursiveVisit(node.Cond, depth)
			cc.BranchingScore++
			cc.Score++
			recursiveVisit(node.Body, depth+1)
			if node.Else != nil {
				recursiveVisit(node.Else, depth+1)
			}
			return
		case *ast.ForStmt:
			recursiveVisit(node.Init, depth)
			recursiveVisit(node.Cond, depth)
			recursiveVisit(node.Post, depth)
			cc.BranchingScore++
			cc.Score++
			recursiveVisit(node.Body, depth+1)
			return
		case *ast.RangeStmt:
			recursiveVisit(node.X, depth)
			cc.BranchingScore++
			cc.Score++
			recursiveVisit(node.Body, depth+1)
			return
		case *ast.SwitchStmt:
			recursiveVisit(node.Tag, depth)
			cc.BranchingScore++
			cc.Score++
			recursiveVisit(node.Body, depth+1)
			return
		case *ast.BinaryExpr:
			if node.Op == token.LAND || node.Op == token.LOR {
				cc.LogicalOps++
				cc.Score++
			}
			recursiveVisit(node.X, depth)
			recursiveVisit(node.Y, depth)
			return
		}
		for _, child := range children(n) {
			recursiveVisit(child, depth)
		}
	}
	recursiveVisit(fn.Body, 0)
	if cc.BranchingScore > 0 {
		cc.Score++
	}
	cc.NestedDepth = maxDepth

	return surrealtypes.CognitiveComplexityMetrics{
		Score:          cc.Score,
		NestedDepth:    cc.NestedDepth,
		LogicalOps:     cc.LogicalOps,
		BranchingScore: cc.BranchingScore,
	}
}

func findFunctionDecl(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			fullName := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				recvType := simpleTypeString(fn.Recv.List[0].Type)
				fullName = fmt.Sprintf("%s.%s", recvType, fn.Name.Name)
			}
			if fullName == name {
				return fn
			}
		}
	}
	return nil
}

type functionNode struct {
	name    string
	index   int
	lowlink int
	inStack bool
}

func detectRecursion(functions map[string]surrealtypes.FunctionCall) map[string]surrealtypes.FunctionCall {
	index := 0
	stack := []string{}
	recData := map[string]*functionNode{}

	var tarjan func(caller string)
	tarjan = func(caller string) {
		rec := &functionNode{
			name:    caller,
			index:   index,
			lowlink: index,
			inStack: true,
		}
		recData[caller] = rec
		index++
		stack = append(stack, caller)

		if fn, exists := functions[caller]; exists {
			for _, callee := range fn.Callees {
				if callee == caller {
					fn.IsRecursive = true
					functions[caller] = fn
					continue
				}

				if data, found := recData[callee]; !found {
					tarjan(callee)
					rec.lowlink = min(rec.lowlink, recData[callee].lowlink)
				} else if data.inStack {
					rec.lowlink = min(rec.lowlink, data.index)
				}
			}
		}

		if rec.lowlink == rec.index {
			var sccNodes []string
			for {
				n := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				recData[n].inStack = false
				sccNodes = append(sccNodes, n)
				if n == caller {
					break
				}
			}
			if len(sccNodes) > 1 {
				for _, n := range sccNodes {
					if fn, exists := functions[n]; exists {
						fn.IsRecursive = true
						functions[n] = fn
					}
				}
			}
		}
	}

	for caller := range functions {
		if _, found := recData[caller]; !found {
			tarjan(caller)
		}
	}

	return functions
}
