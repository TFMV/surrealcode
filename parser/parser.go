package parser

import (
	"fmt"
	"go/parser"
	"go/token"
	"strings"

	"go/ast"

	"github.com/TFMV/surrealcode/expr"
	"github.com/TFMV/surrealcode/types"
)

type Parser struct {
	exprCache *expr.ExprCache
}

func NewParser(cache *expr.ExprCache) *Parser {
	return &Parser{
		exprCache: cache,
	}
}

// FileAnalysis represents the analysis results of a single file
type FileAnalysis struct {
	Functions  []types.FunctionCall
	Structs    []types.StructDefinition
	Interfaces []types.InterfaceDefinition
	Globals    []types.GlobalVariable
	Imports    []types.ImportDefinition
	Implements []types.InterfaceImplementation
}

func (p *Parser) ParseFile(path string) (FileAnalysis, error) {
	fmt.Println("  Starting parse of:", path)
	fset := token.NewFileSet()

	fmt.Println("  Reading file...")
	file, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
	if err != nil {
		return FileAnalysis{}, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	fmt.Println("  Extracting package info...")
	pkg := file.Name.Name

	fmt.Println("  Analyzing functions...")
	var functions []types.FunctionCall
	var structs []types.StructDefinition
	var interfaces []types.InterfaceDefinition
	var globals []types.GlobalVariable
	var imports []types.ImportDefinition
	var implements []types.InterfaceImplementation

	// Process declarations first
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			fn := types.FunctionCall{
				Caller:            d.Name.Name,
				File:              path,
				Package:           pkg,
				Params:            make([]string, 0),
				Returns:           make([]string, 0),
				Callees:           make([]string, 0),
				ReferencedGlobals: make([]string, 0),
				Dependencies:      make([]string, 0),
			}

			if d.Type.Params != nil {
				for _, param := range d.Type.Params.List {
					paramType := simpleTypeString(param.Type) // Use simple type conversion
					fn.Params = append(fn.Params, paramType)
				}
			}

			if d.Type.Results != nil {
				for _, ret := range d.Type.Results.List {
					retType := simpleTypeString(ret.Type) // Use simple type conversion
					fn.Returns = append(fn.Returns, retType)
				}
			}

			if d.Recv != nil {
				fn.IsMethod = true
				if len(d.Recv.List) > 0 {
					fn.Struct = simpleTypeString(d.Recv.List[0].Type)
				}
			}

			// Track global references and dependencies
			ast.Inspect(d.Body, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.SelectorExpr:
					if ident, ok := node.X.(*ast.Ident); ok {
						// Check if it's an imported package reference
						for _, imp := range imports {
							if strings.HasSuffix(imp.Path, ident.Name) {
								fn.Dependencies = append(fn.Dependencies, imp.Path)
							}
						}
					}
				case *ast.Ident:
					// Check if identifier refers to a global
					for _, global := range globals {
						if node.Name == global.Name {
							fn.ReferencedGlobals = append(fn.ReferencedGlobals, global.Name)
						}
					}
				}
				return true
			})

			functions = append(functions, fn)
			fmt.Printf("    Function processed: %s\n", d.Name.Name)
		case *ast.GenDecl:
			switch d.Tok {
			case token.IMPORT:
				for _, spec := range d.Specs {
					if imp, ok := spec.(*ast.ImportSpec); ok {
						imports = append(imports, types.ImportDefinition{
							Path:    strings.Trim(imp.Path.Value, `"`),
							File:    path,
							Package: pkg,
						})
					}
				}
			case token.VAR, token.CONST:
				for _, spec := range d.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for i, name := range vs.Names {
							globals = append(globals, types.GlobalVariable{
								Name:    name.Name,
								Type:    p.exprCache.ToString(vs.Type),
								Value:   p.exprCache.ToString(vs.Values[i]),
								File:    path,
								Package: pkg,
							})
						}
					}
				}
			case token.TYPE:
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						if _, ok := ts.Type.(*ast.StructType); ok {
							structs = append(structs, types.StructDefinition{
								Name:    ts.Name.Name,
								File:    path,
								Package: pkg,
							})
						}
					}
				}
			}
		}
	}

	// Check interface implementations after collecting all interfaces
	for _, st := range structs {
		for _, iface := range interfaces {
			if implementsInterface(nil, iface) { // We'll improve this later
				implements = append(implements, types.InterfaceImplementation{
					Struct:    st.Name,
					Interface: iface.Name,
				})
			}
		}
	}

	fmt.Println("  Parse complete")
	return FileAnalysis{
		Functions:  functions,
		Structs:    structs,
		Interfaces: interfaces,
		Globals:    globals,
		Imports:    imports,
		Implements: implements,
	}, nil
}

// Simple type conversion without using cache
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

// Helper function to check if a struct implements an interface
func implementsInterface(st *ast.StructType, iface types.InterfaceDefinition) bool {
	// Basic implementation - in practice you'd need more sophisticated checking
	return true // For now, assume all structs implement all interfaces
}
