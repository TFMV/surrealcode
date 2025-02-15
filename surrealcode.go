// Package surrealcode provides static code analysis for Go projects with SurrealDB storage.
package surrealcode

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/TFMV/surrealcode/schema"
	"github.com/golang/groupcache/lru"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"golang.org/x/sync/errgroup"
)

// ---------------------------
// Data Structures for Analysis
// ---------------------------

// FunctionCall represents a function's metadata and its relationships with other code elements.
type FunctionCall struct {
	ID                   *models.RecordID `json:"id,omitempty"`
	Caller               string           `json:"caller"`
	Callees              []string         `json:"callees"`
	File                 string           `json:"file"`
	Package              string           `json:"package"`
	Params               []string         `json:"params"`
	Returns              []string         `json:"returns"`
	IsMethod             bool             `json:"is_method"`
	Struct               string           `json:"struct"`
	IsRecursive          bool             `json:"is_recursive"`
	CyclomaticComplexity int              `json:"cyclomatic_complexity"`
	LinesOfCode          int              `json:"lines_of_code"`
}

// StructDefinition represents a struct declaration and its metadata.
type StructDefinition struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Name    string           `json:"name"`
	File    string           `json:"file"`
	Package string           `json:"package"`
}

// InterfaceDefinition represents an interface declaration and its method set.
type InterfaceDefinition struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Name    string           `json:"name"`
	File    string           `json:"file"`
	Package string           `json:"package"`
	Methods []string         `json:"methods"`
}

// GlobalVariable represents a global variable declaration and its metadata.
type GlobalVariable struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Name    string           `json:"name"`
	Type    string           `json:"type"`
	Value   string           `json:"value,omitempty"`
	File    string           `json:"file"`
	Package string           `json:"package"`
}

// ImportDefinition represents an import statement and its context.
type ImportDefinition struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Path    string           `json:"path"`
	File    string           `json:"file"`
	Package string           `json:"package"`
}

// AnalysisReport contains the complete analysis results for a Go codebase.
type AnalysisReport struct {
	Functions  []FunctionCall        `json:"functions"`
	Structs    []StructDefinition    `json:"structs"`
	Interfaces []InterfaceDefinition `json:"interfaces"`
	Globals    []GlobalVariable      `json:"globals"`
	Imports    []ImportDefinition    `json:"imports"`
}

// ---------------------------
// Expression Cache (thread-safe)
// ---------------------------

// ExprCache provides thread-safe caching of AST expression string representations
type ExprCache struct {
	cache *lru.Cache
}

// NewExprCache creates a new expression cache with the specified maximum size.
func NewExprCache(size int) *ExprCache {
	return &ExprCache{
		cache: lru.New(size),
	}
}

// Get retrieves a cached expression string if it exists.
func (c *ExprCache) Get(expr ast.Expr) (string, bool) {
	if val, ok := c.cache.Get(expr); ok {
		return val.(string), true
	}
	return "", false
}

// Put adds an expression string to the cache.
func (c *ExprCache) Put(expr ast.Expr, str string) {
	c.cache.Add(expr, str)
}

// ---------------------------
// Analyzer & Database Integration
// ---------------------------

// Analyzer provides a high-level interface for code analysis and storage in SurrealDB.
type Analyzer struct {
	DB        *surrealdb.DB
	Namespace string
	Database  string
	Username  string
	Password  string
	exprCache *ExprCache
}

// NewAnalyzer creates a new Analyzer given the database credentials.
func NewAnalyzer(dbURL, namespace, database, username, password string) (*Analyzer, error) {
	db, err := surrealdb.New(dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	cache := NewExprCache(10000) // Limit to 10k expressions

	return &Analyzer{
		DB:        db,
		Namespace: namespace,
		Database:  database,
		Username:  username,
		Password:  password,
		exprCache: cache,
	}, nil
}

// Initialize sets up the database connection and schema for code analysis storage.
func (a *Analyzer) Initialize() error {
	if err := a.DB.Use(a.Namespace, a.Database); err != nil {
		return fmt.Errorf("failed to set namespace/database: %w", err)
	}

	authData := &surrealdb.Auth{
		Username: a.Username,
		Password: a.Password,
	}
	token, err := a.DB.SignIn(authData)
	if err != nil {
		return fmt.Errorf("failed to sign in: %w", err)
	}

	if err := a.DB.Authenticate(token); err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	// Initialize schema
	if err := schema.InitializeSchema(context.Background(), a.DB); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}

// AnalyzeDirectory scans a directory tree for Go files and stores analysis results in SurrealDB.
func (a *Analyzer) AnalyzeDirectory(ctx context.Context, dir string) error {
	report, err := a.GetAnalysis(ctx, dir)
	if err != nil {
		return fmt.Errorf("failed to analyze directory: %w", err)
	}
	return StoreInSurrealDBBatch(ctx, a.DB, report)
}

// GetAnalysis performs code analysis on a directory without storing the results.
func (a *Analyzer) GetAnalysis(ctx context.Context, dir string) (AnalysisReport, error) {
	return a.scanDirectory(ctx, dir)
}

// Add method to Analyzer
func (a *Analyzer) exprToString(expr ast.Expr) string {
	if str, ok := a.exprCache.Get(expr); ok {
		return str
	}

	var result string
	switch e := expr.(type) {
	case *ast.Ident:
		result = e.Name
	case *ast.StarExpr:
		result = "*" + a.exprToString(e.X)
	case *ast.SelectorExpr:
		result = a.exprToString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		result = "[]" + a.exprToString(e.Elt)
	case *ast.MapType:
		result = fmt.Sprintf("map[%s]%s", a.exprToString(e.Key), a.exprToString(e.Value))
	case *ast.ChanType:
		result = "chan " + a.exprToString(e.Value)
	case *ast.FuncType:
		params := make([]string, 0, len(e.Params.List))
		for _, p := range e.Params.List {
			params = append(params, a.exprToString(p.Type))
		}
		results := []string{}
		if e.Results != nil {
			results = make([]string, 0, len(e.Results.List))
			for _, r := range e.Results.List {
				results = append(results, a.exprToString(r.Type))
			}
		}
		result = fmt.Sprintf("func(%s)", strings.Join(params, ", "))
		if len(results) > 0 {
			result += " (" + strings.Join(results, ", ") + ")"
		}
	case *ast.InterfaceType:
		methods := make([]string, 0, len(e.Methods.List))
		for _, m := range e.Methods.List {
			methods = append(methods, a.exprToString(m.Type))
		}
		result = "interface{" + strings.Join(methods, "; ") + "}"
	case *ast.StructType:
		fields := make([]string, 0, len(e.Fields.List))
		for _, f := range e.Fields.List {
			fields = append(fields, a.exprToString(f.Type))
		}
		result = "struct{" + strings.Join(fields, "; ") + "}"
	case *ast.BasicLit:
		result = e.Value
	default:
		// More informative fallback
		result = fmt.Sprintf("<%T>", expr)
	}

	a.exprCache.Put(expr, result)
	return result
}

// Update parseGoFile to be a method of Analyzer
func (a *Analyzer) parseGoFile(filename string) ([]FunctionCall, []StructDefinition, []InterfaceDefinition, []GlobalVariable, []ImportDefinition, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.AllErrors)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	packageName := file.Name.Name

	// Containers for definitions
	structsMap := map[string]StructDefinition{}
	var interfacesMap = map[string]InterfaceDefinition{}
	var globals []GlobalVariable
	var importRecords []ImportDefinition
	functionSignatures := map[string]FunctionCall{}

	// Iterate through declarations.
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.ImportSpec:
					path := strings.Trim(s.Path.Value, `"`)
					importRecords = append(importRecords, ImportDefinition{
						Path:    path,
						File:    filepath.Base(filename),
						Package: packageName,
					})
				case *ast.ValueSpec:
					for _, name := range s.Names {
						value := ""
						if len(s.Values) > 0 {
							value = a.exprToString(s.Values[0])
						}
						typeStr := ""
						if s.Type != nil {
							typeStr = a.exprToString(s.Type)
						}
						globals = append(globals, GlobalVariable{
							Name:    name.Name,
							Type:    typeStr,
							Value:   value,
							File:    filepath.Base(filename),
							Package: packageName,
						})
					}
				case *ast.TypeSpec:
					switch typ := s.Type.(type) {
					case *ast.StructType:
						structsMap[s.Name.Name] = StructDefinition{
							Name:    s.Name.Name,
							File:    filepath.Base(filename),
							Package: packageName,
						}
					case *ast.InterfaceType:
						var methods []string
						for _, field := range typ.Methods.List {
							// Methods may have multiple names or be embedded interfaces.
							if len(field.Names) > 0 {
								for _, n := range field.Names {
									methods = append(methods, n.Name)
								}
							} else {
								// Embedded interface (represented by type).
								methods = append(methods, a.exprToString(field.Type))
							}
						}
						interfacesMap[s.Name.Name] = InterfaceDefinition{
							Name:    s.Name.Name,
							File:    filepath.Base(filename),
							Package: packageName,
							Methods: methods,
						}
					}
				}
			}
		case *ast.FuncDecl:
			// Determine if this is a method and set up full function name.
			isMethod := d.Recv != nil
			structName := ""
			if isMethod {
				for _, recv := range d.Recv.List {
					if ident, ok := recv.Type.(*ast.Ident); ok {
						structName = ident.Name
					} else if star, ok := recv.Type.(*ast.StarExpr); ok {
						structName = a.exprToString(star.X)
					}
				}
			}

			funcName := d.Name.Name
			var fullName string
			if isMethod && structName != "" {
				fullName = fmt.Sprintf("%s.%s.%s", packageName, structName, funcName)
			} else {
				fullName = fmt.Sprintf("%s.%s", packageName, funcName)
			}

			// Process parameters.
			var params []string
			if d.Type.Params != nil {
				for _, param := range d.Type.Params.List {
					typeStr := a.exprToString(param.Type)
					for _, name := range param.Names {
						params = append(params, fmt.Sprintf("%s %s", name.Name, typeStr))
					}
				}
			}

			// Process return types.
			var returns []string
			if d.Type.Results != nil {
				for _, result := range d.Type.Results.List {
					returns = append(returns, a.exprToString(result.Type))
				}
			}

			// Initialize callee list.
			var callees []string
			// Compute cyclomatic complexity.
			complexity := 1 // Base complexity.
			loc := 0
			if d.Body != nil {
				// Compute LOC using the positions of { and }.
				start := fset.Position(d.Body.Lbrace).Line
				end := fset.Position(d.Body.Rbrace).Line
				loc = end - start + 1

				ast.Inspect(d.Body, func(n ast.Node) bool {
					switch n.(type) {
					case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause, *ast.SelectStmt:
						complexity++
					case *ast.BinaryExpr:
						// Count short-circuit operators.
						bin := n.(*ast.BinaryExpr)
						if bin.Op.String() == "&&" || bin.Op.String() == "||" {
							complexity++
						}
					}
					// Also inspect call expressions to build the call graph.
					if call, ok := n.(*ast.CallExpr); ok {
						var calleeName string
						switch fun := call.Fun.(type) {
						case *ast.Ident:
							calleeName = fun.Name
						case *ast.SelectorExpr:
							calleeName = a.exprToString(fun)
						}
						// If the callee name is not qualified, assume it belongs to the same package.
						if !strings.Contains(calleeName, ".") {
							calleeName = fmt.Sprintf("%s.%s", packageName, calleeName)
						}
						callees = append(callees, calleeName)
					}
					return true
				})
			}

			functionSignatures[fullName] = FunctionCall{
				Caller:               fullName,
				Callees:              callees,
				File:                 filepath.Base(filename),
				Package:              packageName,
				Params:               params,
				Returns:              returns,
				IsMethod:             isMethod,
				Struct:               structName,
				CyclomaticComplexity: complexity,
				LinesOfCode:          loc,
			}
		}
	}

	// Detect recursive calls using Tarjan's SCC algorithm.
	functionSignatures = a.detectRecursion(functionSignatures)

	// Convert maps to slices.
	var functions []FunctionCall
	for _, fn := range functionSignatures {
		// Ensure slices are non-nil.
		if fn.Callees == nil {
			fn.Callees = []string{}
		}
		if fn.Params == nil {
			fn.Params = []string{}
		}
		if fn.Returns == nil {
			fn.Returns = []string{}
		}
		functions = append(functions, fn)
	}

	var structs []StructDefinition
	for _, s := range structsMap {
		structs = append(structs, s)
	}

	var interfaces []InterfaceDefinition
	for _, iface := range interfacesMap {
		interfaces = append(interfaces, iface)
	}

	return functions, structs, interfaces, globals, importRecords, nil
}

// recursionData tracks node state during Tarjan's algorithm
type recursionData struct {
	depth     int  // Discovery time
	lowlink   int  // Lowest reachable vertex
	inStack   bool // Whether node is in current SCC stack
	recursive bool // Whether node is part of a recursive cycle
}

func (a *Analyzer) detectRecursion(functions map[string]FunctionCall) map[string]FunctionCall {
	index := 0
	stack := []string{}
	recData := map[string]*recursionData{}

	var tarjan func(caller string, depth int)
	tarjan = func(caller string, depth int) {
		rec := &recursionData{
			depth:   index,
			lowlink: index,
			inStack: true,
		}
		recData[caller] = rec
		index++
		stack = append(stack, caller)

		if fn, exists := functions[caller]; exists {
			for _, callee := range fn.Callees {
				// Check for direct recursion first
				if callee == caller {
					fn.IsRecursive = true
					functions[caller] = fn
					continue
				}

				if _, found := recData[callee]; !found {
					tarjan(callee, depth+1)
					rec.lowlink = min(rec.lowlink, recData[callee].lowlink)
				} else if recData[callee].inStack {
					rec.lowlink = min(rec.lowlink, recData[callee].depth)
				}
			}
		}

		// Root of an SCC
		if rec.lowlink == rec.depth {
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
			// Mark all nodes in cycle if SCC size > 1
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

	// Process all nodes
	for caller := range functions {
		if _, found := recData[caller]; !found {
			tarjan(caller, 0)
		}
	}

	return functions
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---------------------------
// Concurrent Directory Scanning
// ---------------------------

// analysisResult is used internally to combine file results.
type analysisResult struct {
	functions  []FunctionCall
	structs    []StructDefinition
	interfaces []InterfaceDefinition
	globals    []GlobalVariable
	imports    []ImportDefinition
}

// scanDirectory with improved error handling using errgroup
func (a *Analyzer) scanDirectory(ctx context.Context, dir string) (AnalysisReport, error) {
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
		return AnalysisReport{}, fmt.Errorf("failed to scan directory %s: %w", dir, err)
	}

	g, ctx := errgroup.WithContext(ctx)
	resultCh := make(chan analysisResult, len(filePaths))

	// Process files concurrently
	for _, path := range filePaths {
		path := path // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			funcs, structs, ifaces, globals, imports, err := a.parseGoFile(path)
			if err != nil {
				return fmt.Errorf("error parsing %s: %w", path, err)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case resultCh <- analysisResult{
				functions:  funcs,
				structs:    structs,
				interfaces: ifaces,
				globals:    globals,
				imports:    imports,
			}:
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
	var report AnalysisReport
	for res := range resultCh {
		report.Functions = append(report.Functions, res.functions...)
		report.Structs = append(report.Structs, res.structs...)
		report.Interfaces = append(report.Interfaces, res.interfaces...)
		report.Globals = append(report.Globals, res.globals...)
		report.Imports = append(report.Imports, res.imports...)
	}

	// Wait for all goroutines and check for errors
	if err := g.Wait(); err != nil {
		return AnalysisReport{}, err
	}

	return report, nil
}

// ---------------------------
// Data Sanitization Helpers
// ---------------------------

// sanitizeFunctionCalls ensures all slices are non-nil for JSON serialization
func sanitizeFunctionCalls(fns []FunctionCall) []FunctionCall {
	for i := range fns {
		if fns[i].Callees == nil {
			fns[i].Callees = []string{}
		}
		if fns[i].Params == nil {
			fns[i].Params = []string{}
		}
		if fns[i].Returns == nil {
			fns[i].Returns = []string{}
		}
	}
	return fns
}

// sanitizeInterfaceDefinitions ensures all method lists are non-nil
func sanitizeInterfaceDefinitions(ifaces []InterfaceDefinition) []InterfaceDefinition {
	for i := range ifaces {
		if ifaces[i].Methods == nil {
			ifaces[i].Methods = []string{}
		}
	}
	return ifaces
}

// ---------------------------
// SurrealDB Integration
// ---------------------------

// StoreInSurrealDBBatch stores the complete analysis report in SurrealDB with relationships.
func StoreInSurrealDBBatch(ctx context.Context, db *surrealdb.DB, report AnalysisReport) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Sanitize data before storage
	report.Functions = sanitizeFunctionCalls(report.Functions)
	report.Interfaces = sanitizeInterfaceDefinitions(report.Interfaces)

	// Store functions and create call relationships.
	for _, fn := range report.Functions {
		if _, err := surrealdb.Create[FunctionCall](db, models.Table("functions"), fn); err != nil {
			return fmt.Errorf("error storing function %s: %v", fn.Caller, err)
		}

		// Create call relationships.
		for _, callee := range fn.Callees {
			query := fmt.Sprintf(`
				LET $caller = SELECT * FROM functions WHERE caller = '%s';
				LET $callee = SELECT * FROM functions WHERE caller = '%s';
				CREATE calls SET 
					from = $caller[0].id,
					to = $callee[0].id,
					file = '%s',
					package = '%s'`,
				fn.Caller, callee, fn.File, fn.Package,
			)
			if _, err := surrealdb.Query[any](db, query, map[string]interface{}{}); err != nil {
				return fmt.Errorf("error creating call relationship %s->%s: %v", fn.Caller, callee, err)
			}
		}
	}

	// Store structs.
	for _, s := range report.Structs {
		if _, err := surrealdb.Create[StructDefinition](db, models.Table("structs"), s); err != nil {
			return fmt.Errorf("error storing struct %s: %v", s.Name, err)
		}
	}

	// Store interfaces.
	for _, iface := range report.Interfaces {
		if _, err := surrealdb.Create[InterfaceDefinition](db, models.Table("interfaces"), iface); err != nil {
			return fmt.Errorf("error storing interface %s: %v", iface.Name, err)
		}
	}

	// Store globals.
	for _, g := range report.Globals {
		if _, err := surrealdb.Create[GlobalVariable](db, models.Table("globals"), g); err != nil {
			return fmt.Errorf("error storing global %s: %v", g.Name, err)
		}
	}

	// Store imports.
	for _, imp := range report.Imports {
		if _, err := surrealdb.Create[ImportDefinition](db, models.Table("imports"), imp); err != nil {
			return fmt.Errorf("error storing import %s: %v", imp.Path, err)
		}
	}

	return nil
}
