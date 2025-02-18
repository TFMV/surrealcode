package analysis_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TFMV/surrealcode/analysis"
	"github.com/TFMV/surrealcode/expr"
	"github.com/TFMV/surrealcode/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAnalyzer(t *testing.T, src string) (*analysis.Analyzer, []types.FunctionCall) {
	analyzer := &analysis.Analyzer{
		ExprCache: expr.NewExprCache(100),
		Metrics:   analysis.NewMetricsAnalyzer(),
	}

	tmpFile := filepath.Join(t.TempDir(), "test.go")
	require.NoError(t, os.WriteFile(tmpFile, []byte(src), 0644))

	analysis, err := analyzer.AnalyzeFile(tmpFile)
	require.NoError(t, err)
	require.NotEmpty(t, analysis.Functions)

	return analyzer, analysis.Functions
}

func TestComputeHalsteadMetrics(t *testing.T) {
	src := `package test
        func example(n int) int {
            if n <= 1 { return 1 }
            return n * example(n-1)
        }`

	_, functions := setupAnalyzer(t, src)
	require.Len(t, functions, 1)

	fn := functions[0]
	metrics := fn.Metrics.HalsteadMetrics
	assert.Greater(t, metrics.Volume, 0.0)
	assert.Greater(t, metrics.Effort, 0.0)
}

func TestDetectDuplication(t *testing.T) {
	src := `package test
        func example1(x int) int {
            return x + 1
        }
        func example2(x int) int {
            return x + 1
        }`

	_, functions := setupAnalyzer(t, src)
	require.Len(t, functions, 2)

	assert.False(t, functions[0].IsDuplicate)
	assert.True(t, functions[1].IsDuplicate)
}

func TestComputeReadabilityMetrics(t *testing.T) {
	src := `package test
        func example(n int) int {
            if n <= 1 { return 1 }
            return n * example(n-1)
        }`

	_, functions := setupAnalyzer(t, src)
	metrics := functions[0].Metrics.Readability

	assert.Greater(t, metrics.NestingDepth, 0)
	assert.GreaterOrEqual(t, metrics.CommentDensity, 0.0)
	assert.GreaterOrEqual(t, metrics.BranchDensity, 0.0)
}

func TestMaintainabilityIndex(t *testing.T) {
	src := `package test
        func example(n int) int {
            if n <= 1 { return 1 }
            return n * example(n-1)
        }`

	_, functions := setupAnalyzer(t, src)
	maintainability := functions[0].Metrics.Maintainability

	assert.Less(t, maintainability, 171.0)
	assert.Greater(t, maintainability, -200.0)
}

func TestCountLines(t *testing.T) {
	src := `package test
        func example(n int) int {
            if n <= 1 { return 1 }
            return n * example(n-1)
        }`

	_, functions := setupAnalyzer(t, src)
	require.Len(t, functions, 1)

	lines := functions[0].Metrics.LinesOfCode
	assert.Greater(t, lines, 0)
}

func TestHalsteadMetricsComplexFunction(t *testing.T) {
	src := `package test
        func complex(x, y int) int {
            a := x + y
            b := x * y
            if a > b {
                return a - b
            }
            return b / a
        }`

	_, functions := setupAnalyzer(t, src)
	metrics := functions[0].Metrics.HalsteadMetrics

	assert.Greater(t, metrics.Volume, 0.0)
	assert.Greater(t, metrics.Effort, 0.0)
	assert.Greater(t, metrics.Difficulty, 0.0)
}

func TestDuplicationWithComments(t *testing.T) {
	src := `package test
        func example1(x int) int {
            // This is a comment
            return x + 1
        }
        func example2(x int) int {
            // Different comment
            return x + 1
        }`

	_, functions := setupAnalyzer(t, src)
	require.Len(t, functions, 2)

	assert.False(t, functions[0].IsDuplicate, "First function should not be marked as duplicate")
	assert.True(t, functions[1].IsDuplicate, "Second function should be marked as duplicate")
}

func TestReadabilityWithNestedBlocks(t *testing.T) {
	src := `package test
        func nested() {
            if true {
                for i := 0; i < 10; i++ {
                    if i > 5 {
                        // Deep nesting
                    }
                }
            }
        }`

	_, functions := setupAnalyzer(t, src)
	require.Len(t, functions, 1)

	fn := functions[0]
	metrics := fn.Metrics.Readability
	assert.GreaterOrEqual(t, metrics.NestingDepth, 3)
}

func TestCognitiveComplexity(t *testing.T) {
	tests := []struct {
		name           string
		src            string
		wantScore      int
		wantNesting    int
		wantLogicalOps int
	}{
		{
			name: "simple function",
			src: `package test
				func simple(x int) int {
					return x + 1
				}`,
			wantScore:      0,
			wantNesting:    0,
			wantLogicalOps: 0,
		},
		{
			name: "nested conditions",
			src: `package test
				func complex(x int) int {
					if x > 0 {
						if x > 10 && x < 20 {
							for i := 0; i < x; i++ {
								if i%2 == 0 {
									x++
								}
							}
						}
					}
					return x
				}`,
			wantScore:      6,
			wantNesting:    4,
			wantLogicalOps: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, functions := setupAnalyzer(t, tt.src)
			cc := functions[0].Metrics.CognitiveComplexity

			assert.Equal(t, tt.wantScore, cc.Score)
			assert.Equal(t, tt.wantNesting, cc.NestedDepth)
			assert.Equal(t, tt.wantLogicalOps, cc.LogicalOps)
		})
	}
}

func TestDetectDeadCode(t *testing.T) {
	tests := []struct {
		name        string
		functions   map[string]types.FunctionCall
		entryPoints []string
		wantUnused  []string
	}{
		{
			name: "simple dead code",
			functions: map[string]types.FunctionCall{
				"main": {
					Caller:  "main",
					Callees: []string{"used"},
				},
				"used": {
					Caller:  "used",
					Callees: []string{},
				},
				"unused": {
					Caller:  "unused",
					Callees: []string{},
				},
			},
			entryPoints: []string{"main"},
			wantUnused:  []string{"unused"},
		},
		{
			name: "exported functions not dead",
			functions: map[string]types.FunctionCall{
				"main": {
					Caller:  "main",
					Callees: []string{},
				},
				"Exported": {
					Caller:  "Exported",
					Callees: []string{},
				},
			},
			entryPoints: []string{"main"},
			wantUnused:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := analysis.DetectDeadCode(tt.functions, tt.entryPoints)
			assert.ElementsMatch(t, tt.wantUnused, info.UnusedFunctions)
		})
	}
}
