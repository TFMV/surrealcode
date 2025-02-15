package surrealcode

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Data structures for analysis
type FunctionCall struct {
	ID          *models.RecordID `json:"id,omitempty"`
	Caller      string           `json:"caller"`
	Callees     []string         `json:"callees"`
	File        string           `json:"file"`
	Package     string           `json:"package"`
	Params      []string         `json:"params"`
	Returns     []string         `json:"returns"`
	IsMethod    bool             `json:"is_method"`
	Struct      string           `json:"struct"`
	IsRecursive bool             `json:"is_recursive"`
}

type StructDefinition struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Name    string           `json:"name"`
	File    string           `json:"file"`
	Package string           `json:"package"`
}

type GlobalVariable struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Name    string           `json:"name"`
	Type    string           `json:"type"`
	Value   string           `json:"value,omitempty"`
	File    string           `json:"file"`
	Package string           `json:"package"`
}

// Add a new type for imports
type ImportDefinition struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Path    string           `json:"path"`
	File    string           `json:"file"`
	Package string           `json:"package"`
}

// Cache for expression string representations
var exprCache = map[ast.Expr]string{}

// Analyzer provides a high-level interface for code analysis and storage
type Analyzer struct {
	DB        *surrealdb.DB
	Namespace string
	Database  string
	Username  string
	Password  string
}

// NewAnalyzer creates and initializes a new Analyzer
func NewAnalyzer(dbURL, namespace, database, username, password string) (*Analyzer, error) {
	db, err := surrealdb.New(dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &Analyzer{
		DB:        db,
		Namespace: namespace,
		Database:  database,
		Username:  username,
		Password:  password,
	}, nil
}

// Initialize sets up the database connection and schema
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

	return nil
}

// AnalyzeDirectory scans and stores code analysis data
func (a *Analyzer) AnalyzeDirectory(dir string) error {
	functions, structs, globals, imports, err := scanDirectory(dir)
	if err != nil {
		return fmt.Errorf("failed to analyze directory: %w", err)
	}

	if err := StoreInSurrealDBBatch(a.DB, functions, structs, globals, imports); err != nil {
		return fmt.Errorf("failed to store analysis data: %w", err)
	}

	return nil
}

// GetAnalysis returns the analysis data without storing it
func (a *Analyzer) GetAnalysis(dir string) ([]FunctionCall, []StructDefinition, []GlobalVariable, []ImportDefinition, error) {
	return scanDirectory(dir)
}

// ---------------------
// Parsing & Analysis
// ---------------------

// parseGoFile parses a single Go file and extracts functions, structs, globals and imports.
func parseGoFile(filename string) ([]FunctionCall, []StructDefinition, []GlobalVariable, []ImportDefinition, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.AllErrors)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	packageName := file.Name.Name
	structsMap := map[string]StructDefinition{}
	var globals []GlobalVariable
	var importRecords []ImportDefinition
	functionSignatures := map[string]FunctionCall{}

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
							value = exprToString(s.Values[0]) // Convert value expression to string
						}
						typeStr := ""
						if s.Type != nil {
							typeStr = exprToString(s.Type)
						}
						globals = append(globals, GlobalVariable{
							Name:    name.Name,
							Type:    typeStr,
							Value:   value, // Ensure non-nil value
							File:    filepath.Base(filename),
							Package: packageName,
						})
					}
				case *ast.TypeSpec:
					if _, ok := s.Type.(*ast.StructType); ok {
						structsMap[s.Name.Name] = StructDefinition{
							Name:    s.Name.Name,
							File:    filepath.Base(filename),
							Package: packageName,
						}
					}
				}
			}
		case *ast.FuncDecl:
			caller := d.Name.Name
			var callees []string
			var params []string
			var returns []string
			isMethod := false
			structName := ""

			if d.Recv != nil {
				isMethod = true
				for _, recv := range d.Recv.List {
					if ident, ok := recv.Type.(*ast.Ident); ok {
						structName = ident.Name
					}
				}
			}

			// Get parameters
			if d.Type.Params != nil {
				for _, param := range d.Type.Params.List {
					for _, name := range param.Names {
						params = append(params, fmt.Sprintf("%s %s", name.Name, exprToString(param.Type)))
					}
				}
			}

			// Get return types
			if d.Type.Results != nil {
				for _, result := range d.Type.Results.List {
					returns = append(returns, exprToString(result.Type))
				}
			}

			// Inspect function body for calls
			ast.Inspect(d.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					switch fun := call.Fun.(type) {
					case *ast.Ident:
						callees = append(callees, fun.Name)
					case *ast.SelectorExpr:
						callees = append(callees, exprToString(fun))
					}
				}
				return true
			})

			functionSignatures[caller] = FunctionCall{
				Caller:   caller,
				Callees:  callees,
				File:     filepath.Base(filename),
				Package:  packageName,
				Params:   params,
				Returns:  returns,
				IsMethod: isMethod,
				Struct:   structName,
			}
		}
	}

	// Detect recursive calls using graph-based detection
	functionSignatures = detectRecursion(functionSignatures)

	// Convert maps to slices
	var functions []FunctionCall
	for _, fc := range functionSignatures {
		functions = append(functions, fc)
	}

	var structs []StructDefinition
	for _, s := range structsMap {
		structs = append(structs, s)
	}

	return functions, structs, globals, importRecords, nil
}

// exprToString converts an AST expression to a string representation.
func exprToString(expr ast.Expr) string {
	if cached, exists := exprCache[expr]; exists {
		return cached
	}

	var result string
	switch e := expr.(type) {
	case *ast.Ident:
		result = e.Name
	case *ast.StarExpr:
		result = "*" + exprToString(e.X)
	case *ast.SelectorExpr:
		result = exprToString(e.X) + "." + e.Sel.Name
	default:
		result = fmt.Sprintf("%T", expr)
	}

	exprCache[expr] = result
	return result
}

// contains checks if a slice contains a given string.
func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// scanDirectory recursively scans a directory for *.go files.
func scanDirectory(dir string) ([]FunctionCall, []StructDefinition, []GlobalVariable, []ImportDefinition, error) {
	var allFunctions []FunctionCall
	var allStructs []StructDefinition
	var allGlobals []GlobalVariable
	var allImports []ImportDefinition

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk directory: %w", err)
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			funcs, structs, globals, imports, err := parseGoFile(path)
			if err != nil {
				log.Printf("Error parsing %s: %v", path, err)
				return nil
			}
			allFunctions = append(allFunctions, funcs...)
			allStructs = append(allStructs, structs...)
			allGlobals = append(allGlobals, globals...)
			allImports = append(allImports, imports...)
		}
		return nil
	})

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to scan directory %s: %w", dir, err)
	}

	return allFunctions, allStructs, allGlobals, allImports, nil
}

// ---------------------
// SurrealDB Integration
// ---------------------

// StoreInSurrealDBBatch performs batch insertion into SurrealDB.
func StoreInSurrealDBBatch(db *surrealdb.DB, functions []FunctionCall, structs []StructDefinition, globals []GlobalVariable, imports []ImportDefinition) error {
	// Store functions
	for _, fn := range functions {
		// Ensure all fields are initialized
		if fn.Callees == nil {
			fn.Callees = []string{}
		}
		if fn.Params == nil {
			fn.Params = []string{}
		}
		if fn.Returns == nil {
			fn.Returns = []string{}
		}

		if _, err := surrealdb.Create[FunctionCall](db, models.Table("functions"), fn); err != nil {
			return fmt.Errorf("error storing function %s: %v", fn.Caller, err)
		}

		// Create call relationships
		for _, callee := range fn.Callees {
			query := fmt.Sprintf(`
				CREATE calls SET 
					in = functions:%s,
					out = functions:%s,
					file = "%s",
					package = "%s"`,
				fn.Caller, callee,
				fn.File, fn.Package,
			)
			if _, err := surrealdb.Query[any](db, query, map[string]interface{}{}); err != nil {
				return fmt.Errorf("error creating call relationship %s->%s: %v", fn.Caller, callee, err)
			}
		}
	}

	// Store structs
	for _, s := range structs {
		if _, err := surrealdb.Create[StructDefinition](db, models.Table("structs"), s); err != nil {
			return fmt.Errorf("error storing struct %s: %v", s.Name, err)
		}
	}

	// Store globals
	for _, g := range globals {
		if _, err := surrealdb.Create[GlobalVariable](db, models.Table("globals"), g); err != nil {
			return fmt.Errorf("error storing global %s: %v", g.Name, err)
		}
	}

	// Store imports
	for _, imp := range imports {
		if _, err := surrealdb.Create[ImportDefinition](db, models.Table("imports"), imp); err != nil {
			return fmt.Errorf("error storing import %s: %v", imp.Path, err)
		}
	}

	return nil
}

// detectRecursion identifies recursive calls using Tarjan's SCC algorithm
func detectRecursion(functions map[string]FunctionCall) map[string]FunctionCall {
	index := 0
	stack := []string{}
	indices := map[string]int{}
	lowlink := map[string]int{}
	inStack := map[string]bool{}

	var tarjan func(caller string)
	tarjan = func(caller string) {
		indices[caller] = index
		lowlink[caller] = index
		index++
		stack = append(stack, caller)
		inStack[caller] = true

		if fn, exists := functions[caller]; exists {
			for _, callee := range fn.Callees {
				if _, found := indices[callee]; !found {
					tarjan(callee)
					lowlink[caller] = min(lowlink[caller], lowlink[callee])
				} else if inStack[callee] {
					lowlink[caller] = min(lowlink[caller], indices[callee])
					// Mark direct recursion
					if callee == caller {
						fn.IsRecursive = true
						functions[caller] = fn
					}
				}
			}
		}

		// Found a strongly connected component
		if lowlink[caller] == indices[caller] {
			var sccSize int
			var sccNodes []string
			for {
				n := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				inStack[n] = false
				sccNodes = append(sccNodes, n)
				sccSize++
				if n == caller {
					break
				}
			}

			// If SCC has more than one node or contains self-recursion, mark all nodes as recursive
			if sccSize > 1 {
				for _, n := range sccNodes {
					if fn, exists := functions[n]; exists {
						fn.IsRecursive = true
						functions[n] = fn
					}
				}
			}
		}
	}

	// Run Tarjan's algorithm on all nodes
	for caller := range functions {
		if _, found := indices[caller]; !found {
			tarjan(caller)
		}
	}

	return functions
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
