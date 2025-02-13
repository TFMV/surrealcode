package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"path/filepath"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

/*
Start SurrealDB:
surreal start --user root --pass root --bind 0.0.0.0:8000 memory
*/

// FunctionCall represents a function and its dependencies
type FunctionCall struct {
	ID          *models.RecordID `json:"id,omitempty"`
	Caller      string           `json:"caller"`
	Callees     []string         `json:"callees"`
	File        string           `json:"file"`
	Package     string           `json:"package"`
	Params      []string         `json:"params"`
	Returns     []string         `json:"returns"`
	IsMethod    bool             `json:"is_method"`
	Struct      string           `json:"struct,omitempty"`
	IsRecursive bool             `json:"is_recursive"`
}

// StructDefinition represents a struct and its methods
type StructDefinition struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Name    string           `json:"name"`
	File    string           `json:"file"`
	Package string           `json:"package"`
}

// GlobalVariable represents global variables/constants
type GlobalVariable struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Name    string           `json:"name"`
	Type    string           `json:"type"`
	Value   string           `json:"value,omitempty"`
	File    string           `json:"file"`
	Package string           `json:"package"`
}

// Parses a Go file and extracts function calls, structs, imports, and globals
func parseFile(filename string) ([]FunctionCall, []StructDefinition, []GlobalVariable, []string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.AllErrors)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	packageName := file.Name.Name
	structs := make(map[string]StructDefinition)
	globals := []GlobalVariable{}
	imports := []string{}

	// Track function signatures for parameters and return types
	functionSignatures := map[string]FunctionCall{}

	// Extract struct definitions, function calls, and global variables
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl: // Handles imports, global variables, and constants
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.ImportSpec:
					imports = append(imports, s.Path.Value)
				case *ast.ValueSpec: // Global vars/constants
					for _, name := range s.Names {
						globals = append(globals, GlobalVariable{
							Name:    name.Name,
							Type:    fmt.Sprintf("%T", s.Type),
							File:    filepath.Base(filename),
							Package: packageName,
						})
					}
				case *ast.TypeSpec: // Structs
					if _, ok := s.Type.(*ast.StructType); ok {
						structs[s.Name.Name] = StructDefinition{
							Name:    s.Name.Name,
							File:    filepath.Base(filename),
							Package: packageName,
						}
					}
				}
			}

		case *ast.FuncDecl: // Functions & Methods
			caller := d.Name.Name
			var callees []string
			var params []string
			var returns []string
			isMethod := false
			structName := ""

			// If function has a receiver, it's a method
			if d.Recv != nil {
				isMethod = true
				for _, recv := range d.Recv.List {
					if ident, ok := recv.Type.(*ast.Ident); ok {
						structName = ident.Name
					}
				}
			}

			// Extract parameters
			for _, param := range d.Type.Params.List {
				for _, name := range param.Names {
					params = append(params, fmt.Sprintf("%s %s", name.Name, param.Type))
				}
			}

			// Extract return types
			if d.Type.Results != nil {
				for _, result := range d.Type.Results.List {
					returns = append(returns, fmt.Sprintf("%s", result.Type))
				}
			}

			// Extract function calls within the function body
			ast.Inspect(d.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if ident, ok := call.Fun.(*ast.Ident); ok {
						callees = append(callees, ident.Name)
					}
				}
				return true
			})

			// Store function information
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

	// Detect recursive calls
	for caller, call := range functionSignatures {
		if contains(call.Callees, caller) {
			call.IsRecursive = true
		}
		functionSignatures[caller] = call
	}

	// Convert function call map to a slice
	var result []FunctionCall
	for _, call := range functionSignatures {
		result = append(result, call)
	}

	// Convert struct map to a slice
	var structList []StructDefinition
	for _, s := range structs {
		structList = append(structList, s)
	}

	return result, structList, globals, imports, nil
}

// Utility function to check if a slice contains a value
func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// Store enhanced data in SurrealDB
func storeInSurrealDB(db *surrealdb.DB, calls []FunctionCall, structs []StructDefinition, globals []GlobalVariable, imports []string) error {
	// Store function calls
	for _, call := range calls {
		_, err := surrealdb.Create[FunctionCall](db, models.Table("functions"), call)
		if err != nil {
			return err
		}
	}

	// Store structs
	for _, s := range structs {
		_, err := surrealdb.Create[StructDefinition](db, models.Table("structs"), s)
		if err != nil {
			return err
		}
	}

	// Store global variables
	for _, g := range globals {
		_, err := surrealdb.Create[GlobalVariable](db, models.Table("globals"), g)
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	if err != nil {
		log.Fatal(err)
	}

	// Set namespace and database
	if err = db.Use("test", "test"); err != nil {
		log.Fatal(err)
	}

	// Sign in as root
	authData := &surrealdb.Auth{
		Username: "root",
		Password: "root",
	}
	token, err := db.SignIn(authData)
	if err != nil {
		log.Fatal(err)
	}

	// Authenticate with token
	if err := db.Authenticate(token); err != nil {
		log.Fatal(err)
	}

	calls, structs, globals, imports, err := parseFile("demo/example.go")
	if err != nil {
		log.Fatal(err)
	}

	if err := storeInSurrealDB(db, calls, structs, globals, imports); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Code analysis data stored successfully")
}
