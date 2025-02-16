package analysis_test

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/TFMV/surrealcode/analysis"
	surrealcode "github.com/TFMV/surrealcode/parser"
	"github.com/TFMV/surrealcode/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeHalsteadMetrics(t *testing.T) {
	src := `package test
        func example(n int) int {
            if n <= 1 { return 1 }
            return n * example(n-1)
        }`

	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "", src, parser.AllErrors)
	fn := surrealcode.FindFunction(file, "example")

	metrics := analysis.ComputeHalsteadMetrics(fn)
	assert.Greater(t, metrics.Operators, 0)
	assert.Greater(t, metrics.Operands, 0)
}

func TestDetectDuplication(t *testing.T) {
	detector := analysis.NewCodeDuplicationDetector()
	src := `package test
        func example1(x int) int {
            return x + 1
        }
        func example2(x int) int {
            return x + 1
        }`

	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "", src, parser.AllErrors)

	fn1 := surrealcode.FindFunction(file, "example1")
	fn2 := surrealcode.FindFunction(file, "example2")

	// First function should not be detected as duplicate
	assert.False(t, detector.DetectDuplication(fn1))

	// Second identical function should be detected
	assert.True(t, detector.DetectDuplication(fn2))
}

func TestComputeReadabilityMetrics(t *testing.T) {
	src := `package test
        func example(n int) int {
            if n <= 1 { return 1 }
            return n * example(n-1)
        }`

	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "", src, parser.AllErrors)
	fn := surrealcode.FindFunction(file, "example")

	metrics := analysis.ComputeReadabilityMetrics(fn, fset)
	assert.Greater(t, float64(metrics.FunctionLength), 0.0)
	assert.Greater(t, float64(metrics.NestingDepth), 0.0)
	assert.GreaterOrEqual(t, metrics.CommentDensity, 0.0)
}

func TestMaintainabilityIndex(t *testing.T) {
	src := `package test
        func example(n int) int {
            if n <= 1 { return 1 }
            return n * example(n-1)
        }`
	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "", src, parser.AllErrors)
	fn := surrealcode.FindFunction(file, "example")

	metrics := analysis.ComputeReadabilityMetrics(fn, fset)
	maintainability := analysis.MaintainabilityIndex(metrics, 1, true)
	assert.Less(t, maintainability, 171.0)     // Max possible value
	assert.Greater(t, maintainability, -200.0) // Reasonable lower bound
}

func TestCountLines(t *testing.T) {
	src := `package test
        func example(n int) int {
            if n <= 1 { return 1 }
            return n * example(n-1)
        }`

	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "", src, parser.AllErrors)
	fn := surrealcode.FindFunction(file, "example")

	lines := analysis.CountLines(fn, fset)
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

	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "", src, parser.AllErrors)
	fn := surrealcode.FindFunction(file, "complex")

	metrics := analysis.ComputeHalsteadMetrics(fn)
	assert.Greater(t, metrics.Volume, 0.0)
	assert.Greater(t, metrics.Effort, 0.0)
	assert.Greater(t, metrics.UniqueOperators, 3) // Should have multiple operators
}

func TestDuplicationWithComments(t *testing.T) {
	detector := analysis.NewCodeDuplicationDetector()
	src := `package test
        func example1(x int) int {
            // This is a comment
            return x + 1
        }
        func example2(x int) int {
            // Different comment
            return x + 1
        }`

	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "", src, parser.AllErrors)

	fn1 := surrealcode.FindFunction(file, "example1")
	fn2 := surrealcode.FindFunction(file, "example2")

	assert.False(t, detector.DetectDuplication(fn1))
	assert.True(t, detector.DetectDuplication(fn2)) // Should detect despite different comments
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

	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "", src, parser.AllErrors)
	fn := surrealcode.FindFunction(file, "nested")

	metrics := analysis.ComputeReadabilityMetrics(fn, fset)
	assert.GreaterOrEqual(t, metrics.NestingDepth, 3) // Should detect deep nesting
}

func TestComputeCognitiveComplexity(t *testing.T) {
	tests := []struct {
		name           string
		src            string
		funcName       string
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
			funcName:       "simple",
			wantScore:      0,
			wantNesting:    0,
			wantLogicalOps: 0,
		},
		{
			name: "nested conditions",
			src: `package test
				func complex(x int) int {
					if x > 0 {          // +1
						if x > 10 && x < 20 {  // +2 (nesting) +1 (logical)
							for i := 0; i < x; i++ {  // +3 (nesting) +2 (loop)
								if i%2 == 0 {   // +4 (nesting)
									x++
								}
							}
						}
					}
					return x
				}`,
			funcName:       "complex",
			wantScore:      6,
			wantNesting:    4, // Updated: if(1) -> if(2) -> for(3) -> if(4)
			wantLogicalOps: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, _ := parser.ParseFile(fset, "", tt.src, parser.AllErrors)
			fn := surrealcode.FindFunction(file, tt.funcName)
			require.NotNil(t, fn, "Function not found")

			cc := analysis.ComputeCognitiveComplexity(fn)
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
