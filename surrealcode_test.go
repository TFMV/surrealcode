package surrealcode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
)

func TestMain(m *testing.M) {
	// Setup test environment
	setupTestDB()
	code := m.Run()
	// Cleanup
	teardownTestDB()
	os.Exit(code)
}

func setupTestDB() *surrealdb.DB {
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	if err := db.Use("test", "test"); err != nil {
		panic(err)
	}
	return db
}

func teardownTestDB() {
	// Cleanup test database
}

func TestAnalyzer_ParseGoFile(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantFunc int
		wantStr  int
		wantGlob int
		wantImp  int
		wantErr  bool
	}{
		{
			name: "basic function",
			input: `package main
				func main() { println("hello") }`,
			wantFunc: 1,
			wantErr:  false,
		},
		{
			name: "recursive function",
			input: `package main
				func factorial(n int) int {
					if n <= 1 { return 1 }
					return n * factorial(n-1)
				}`,
			wantFunc: 1,
			wantErr:  false,
		},
		{
			name: "struct with method",
			input: `package main
				type Person struct { Name string }
				func (p Person) Greet() string { return "Hello, " + p.Name }`,
			wantFunc: 1,
			wantStr:  1,
			wantErr:  false,
		},
		{
			name: "global variables",
			input: `package main
				var (
					Debug = false
					Version = "1.0.0"
				)`,
			wantGlob: 2,
			wantErr:  false,
		},
		{
			name: "imports",
			input: `package main
				import (
					"fmt"
					"strings"
				)`,
			wantImp: 2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test file
			tmpFile := filepath.Join(t.TempDir(), "test.go")
			require.NoError(t, os.WriteFile(tmpFile, []byte(tt.input), 0644))

			// Parse file
			funcs, structs, globals, imports, err := parseGoFile(tmpFile)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, funcs, tt.wantFunc)
			assert.Len(t, structs, tt.wantStr)
			assert.Len(t, globals, tt.wantGlob)
			assert.Len(t, imports, tt.wantImp)
		})
	}
}

func TestDetectRecursion(t *testing.T) {
	tests := []struct {
		name      string
		functions map[string]FunctionCall
		want      map[string]bool // function name -> expected recursive status
	}{
		{
			name: "direct recursion",
			functions: map[string]FunctionCall{
				"factorial": {
					Caller:  "factorial",
					Callees: []string{"factorial"},
				},
			},
			want: map[string]bool{
				"factorial": true,
			},
		},
		{
			name: "indirect recursion",
			functions: map[string]FunctionCall{
				"a": {Caller: "a", Callees: []string{"b"}},
				"b": {Caller: "b", Callees: []string{"c"}},
				"c": {Caller: "c", Callees: []string{"a"}},
			},
			want: map[string]bool{
				"a": true,
				"b": true,
				"c": true,
			},
		},
		{
			name: "no recursion",
			functions: map[string]FunctionCall{
				"main": {Caller: "main", Callees: []string{"fmt.Println"}},
			},
			want: map[string]bool{
				"main": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectRecursion(tt.functions)
			for fname, want := range tt.want {
				assert.Equal(t, want, result[fname].IsRecursive)
			}
		})
	}
}

// Helper function to create a test project
func setupTestProject(t *testing.T) string {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": `package main
			import "fmt"
			func main() { fmt.Println("hello") }`,
		"util/helper.go": `package util
			type Helper struct{}
			func (h Helper) DoWork() {}`,
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}

	return dir
}

func TestExprToString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple type",
			input: "type X struct { F int }",
			want:  "int",
		},
		{
			name:  "pointer type",
			input: "type X struct { F *string }",
			want:  "*string",
		},
		{
			name:  "selector type",
			input: "type X struct { F fmt.Stringer }",
			want:  "fmt.Stringer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the input to get an ast.Expr
			tmpFile := filepath.Join(t.TempDir(), "test.go")
			content := "package test\n" + tt.input
			require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

			// Test the expression conversion
			// Note: This needs to be implemented based on how you get the ast.Expr
		})
	}
}

func BenchmarkDetectRecursion(b *testing.B) {
	functions := map[string]FunctionCall{
		"a": {Caller: "a", Callees: []string{"b"}},
		"b": {Caller: "b", Callees: []string{"c"}},
		"c": {Caller: "c", Callees: []string{"d"}},
		"d": {Caller: "d", Callees: []string{"a"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detectRecursion(functions)
	}
}
