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
}

func (p *Parser) ParseFile(filename string) (FileAnalysis, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.AllErrors)
	if err != nil {
		return FileAnalysis{}, fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	packageName := file.Name.Name
	structsMap := make(map[string]types.StructDefinition)
	interfacesMap := make(map[string]types.InterfaceDefinition)
	var globals []types.GlobalVariable
	var imports []types.ImportDefinition
	functionMap := make(map[string]types.FunctionCall)

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			// Handle function declarations
			fn := types.FunctionCall{
				Caller:  d.Name.Name,
				File:    filename,
				Package: packageName,
				Params:  make([]string, 0),
				Returns: make([]string, 0),
				Callees: make([]string, 0),
			}

			// Parse parameters
			if d.Type.Params != nil {
				for _, param := range d.Type.Params.List {
					fn.Params = append(fn.Params, p.exprCache.ToString(param.Type))
				}
			}

			// Parse return values
			if d.Type.Results != nil {
				for _, ret := range d.Type.Results.List {
					fn.Returns = append(fn.Returns, p.exprCache.ToString(ret.Type))
				}
			}

			if d.Recv != nil {
				fn.IsMethod = true
				if len(d.Recv.List) > 0 {
					fn.Struct = p.exprCache.ToString(d.Recv.List[0].Type)
				}
			}
			functionMap[fn.Caller] = fn

		case *ast.GenDecl:
			switch d.Tok {
			case token.IMPORT:
				for _, spec := range d.Specs {
					if imp, ok := spec.(*ast.ImportSpec); ok {
						imports = append(imports, types.ImportDefinition{
							Path:    strings.Trim(imp.Path.Value, `"`),
							File:    filename,
							Package: packageName,
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
								File:    filename,
								Package: packageName,
							})
						}
					}
				}
			case token.TYPE:
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						if _, ok := ts.Type.(*ast.StructType); ok {
							structsMap[ts.Name.Name] = types.StructDefinition{
								Name:    ts.Name.Name,
								File:    filename,
								Package: packageName,
							}
						}
					}
				}
			}
		}
	}

	var functions []types.FunctionCall
	for _, fn := range functionMap {
		functions = append(functions, fn)
	}

	var structs []types.StructDefinition
	for _, s := range structsMap {
		structs = append(structs, s)
	}

	var interfaces []types.InterfaceDefinition
	for _, i := range interfacesMap {
		interfaces = append(interfaces, i)
	}

	return FileAnalysis{
		Functions:  functions,
		Structs:    structs,
		Interfaces: interfaces,
		Globals:    globals,
		Imports:    imports,
	}, nil
}
