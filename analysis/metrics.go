package analysis

import (
	"go/ast"
	"math"
	"strings"
)

// HalsteadMetrics represents the computed Halstead complexity metrics.
type HalsteadMetrics struct {
	Operators       int     // Total occurrences of operators
	Operands        int     // Total occurrences of operands
	UniqueOperators int     // Unique operators
	UniqueOperands  int     // Unique operands
	Volume          float64 // Volume = (N1 + N2) * log2(n1 + n2)
	Difficulty      float64 // Difficulty = (n1/2) * (N2/n2)
	Effort          float64 // Effort = Difficulty * Volume
}

// ComputeHalsteadMetrics analyzes a function's AST to compute Halstead complexity metrics.
func ComputeHalsteadMetrics(fn *ast.FuncDecl) HalsteadMetrics {
	operatorSet := map[string]struct{}{}
	operandSet := map[string]struct{}{}
	operatorCount := 0
	oprandCount := 0

	ast.Inspect(fn, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BinaryExpr:
			operatorSet[x.Op.String()] = struct{}{}
			operatorCount++
		case *ast.UnaryExpr:
			operatorSet[x.Op.String()] = struct{}{}
			operatorCount++
		case *ast.Ident:
			operandSet[x.Name] = struct{}{}
			oprandCount++
		case *ast.BasicLit:
			operandSet[x.Value] = struct{}{}
			oprandCount++
		}
		return true
	})

	n1 := len(operatorSet)
	n2 := len(operandSet)
	N1 := operatorCount
	N2 := oprandCount
	volume := float64(N1+N2) * math.Log2(float64(n1+n2))
	difficulty := (float64(n1) / 2.0) * (float64(N2) / float64(n2))
	effort := difficulty * volume

	return HalsteadMetrics{
		Operators:       N1,
		Operands:        N2,
		UniqueOperators: n1,
		UniqueOperands:  n2,
		Volume:          volume,
		Difficulty:      difficulty,
		Effort:          effort,
	}
}

// CodeDuplicationDetector detects duplicated code using Rabin-Karp hashing.
type CodeDuplicationDetector struct {
	seen map[uint64]string // Stores hash -> code snippet mapping
}

// NewCodeDuplicationDetector initializes the detector.
func NewCodeDuplicationDetector() *CodeDuplicationDetector {
	return &CodeDuplicationDetector{
		seen: make(map[uint64]string),
	}
}

// DetectDuplication checks if a function body has been seen before.
func (c *CodeDuplicationDetector) DetectDuplication(fn *ast.FuncDecl) bool {
	body := extractFunctionBody(fn)
	hash := rabinKarpHash(body)
	if _, exists := c.seen[hash]; exists {
		return true // Duplicate detected
	}
	c.seen[hash] = body
	return false
}

// extractFunctionBody returns a function's body as a string.
func extractFunctionBody(fn *ast.FuncDecl) string {
	var builder strings.Builder
	ast.Inspect(fn, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			builder.WriteString(ident.Name)
		}
		return true
	})
	return builder.String()
}

// rabinKarpHash generates a hash for a given string.
func rabinKarpHash(code string) uint64 {
	const primeBase = 31
	var hash uint64
	for i := 0; i < len(code); i++ {
		hash = hash*primeBase + uint64(code[i])
	}
	return hash
}

// CodeReadabilityMetrics evaluates readability based on heuristics.
type CodeReadabilityMetrics struct {
	FunctionLength int
	NestingDepth   int
	CommentDensity float64
}

// ComputeReadabilityMetrics analyzes a function for readability heuristics.
func ComputeReadabilityMetrics(fn *ast.FuncDecl) CodeReadabilityMetrics {
	loc := countLines(fn)
	nestingDepth := 0
	commentCount := 0
	ast.Inspect(fn, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.BlockStmt:
			nestingDepth++
		case *ast.Comment:
			commentCount++
		}
		return true
	})

	commentDensity := float64(commentCount) / float64(loc)
	return CodeReadabilityMetrics{
		FunctionLength: loc,
		NestingDepth:   nestingDepth,
		CommentDensity: commentDensity,
	}
}

// MaintainabilityIndex computes a maintainability score.
func MaintainabilityIndex(metrics CodeReadabilityMetrics, complexity int, duplication bool) float64 {
	duplicationPenalty := 1.0
	if duplication {
		duplicationPenalty = 0.5
	}
	return (171 - 5.2*float64(metrics.FunctionLength) - 0.23*float64(complexity) - 16.2*metrics.CommentDensity) * duplicationPenalty
}

// countLines estimates the number of lines in a function.
func countLines(fn *ast.FuncDecl) int {
	start := fn.Pos()
	end := fn.End()
	return int(end - start) // Convert token.Pos to int
}
