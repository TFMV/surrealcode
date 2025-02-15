package analysis_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/TFMV/surrealcode/analysis"
	"github.com/TFMV/surrealcode/db"
	"github.com/TFMV/surrealcode/expr"
	"github.com/TFMV/surrealcode/parser"
	"github.com/TFMV/surrealcode/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzer_ParseGoFile(t *testing.T) {
	analyzer := &analysis.Analyzer{
		ExprCache: expr.NewExprCache(100),
		Parser:    parser.NewParser(expr.NewExprCache(100)),
	}

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
		{
			name:    "invalid syntax",
			input:   `package main func`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "test.go")
			require.NoError(t, os.WriteFile(tmpFile, []byte(tt.input), 0644))

			analysis, err := analyzer.Parser.ParseFile(tmpFile)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, analysis.Functions, tt.wantFunc)
			assert.Len(t, analysis.Structs, tt.wantStr)
			assert.Len(t, analysis.Interfaces, 0)
			assert.Len(t, analysis.Globals, tt.wantGlob)
			assert.Len(t, analysis.Imports, tt.wantImp)
		})
	}
}

func TestDetectRecursion(t *testing.T) {
	tests := []struct {
		name      string
		functions map[string]types.FunctionCall
		want      map[string]bool
	}{
		{
			name: "direct recursion",
			functions: map[string]types.FunctionCall{
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
			functions: map[string]types.FunctionCall{
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
			functions: map[string]types.FunctionCall{
				"main": {Caller: "main", Callees: []string{"fmt.Println"}},
			},
			want: map[string]bool{
				"main": false,
			},
		},
		{
			name:      "empty functions",
			functions: map[string]types.FunctionCall{},
			want:      map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analysis.DetectRecursion(tt.functions)
			for fname, want := range tt.want {
				assert.Equal(t, want, result[fname].IsRecursive)
			}
		})
	}
}

func TestAnalyzer_Initialize(t *testing.T) {
	analyzer := &analysis.Analyzer{
		ExprCache: expr.NewExprCache(100),
		DB:        db.NewMockDB(),
	}

	err := analyzer.Initialize(context.Background())
	assert.NoError(t, err)
}

func TestAnalyzer_GetAnalysis(t *testing.T) {
	analyzer := &analysis.Analyzer{
		ExprCache: expr.NewExprCache(100),
		Parser:    parser.NewParser(expr.NewExprCache(100)),
		Metrics:   analysis.NewMetricsAnalyzer(),
	}

	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main
		func main() {}`), 0644)
	require.NoError(t, err)

	report, err := analyzer.GetAnalysis(context.Background(), dir)
	assert.NoError(t, err)
	assert.Len(t, report.Functions, 1)
}

func BenchmarkDetectRecursion(b *testing.B) {
	functions := map[string]types.FunctionCall{
		"a": {Caller: "a", Callees: []string{"b"}},
		"b": {Caller: "b", Callees: []string{"c"}},
		"c": {Caller: "c", Callees: []string{"d"}},
		"d": {Caller: "d", Callees: []string{"a"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analysis.DetectRecursion(functions)
	}
}

func TestAnalyzerMetrics(t *testing.T) {
	analyzer := &analysis.Analyzer{
		ExprCache: expr.NewExprCache(100),
		Parser:    parser.NewParser(expr.NewExprCache(100)),
		Metrics:   analysis.NewMetricsAnalyzer(),
	}

	src := `package test
		func complex(x, y int) int {
			a := x + y
			b := x * y
			if a > b {
				return a - b
			}
			return b / a
		}`

	dir := t.TempDir()
	tmpFile := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(tmpFile, []byte(src), 0644))

	report, err := analyzer.GetAnalysis(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, report.Functions, 1)

	fn := report.Functions[0]
	assert.Greater(t, fn.CyclomaticComplexity, 0)
	assert.Greater(t, fn.LinesOfCode, 0)
	assert.False(t, fn.IsDuplicate)
}
