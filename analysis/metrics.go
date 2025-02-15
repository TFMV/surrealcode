package analysis

import (
	"go/ast"
	"math"
	"strings"
	"sync"
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

// NewCodeDuplicationDetector initializes the detector.
func NewCodeDuplicationDetector() *CodeDuplicationDetector {
	return &CodeDuplicationDetector{
		seen: make(map[uint64]string),
	}
}

// extractFunctionBody returns a function's body as a string.
func extractFunctionBody(fn *ast.FuncDecl) string {
	var builder strings.Builder
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			builder.WriteString(x.Name)
		case *ast.BasicLit:
			builder.WriteString(x.Value)
		case *ast.BinaryExpr:
			builder.WriteString(x.Op.String())
		case *ast.ReturnStmt:
			builder.WriteString("return")
		case *ast.IfStmt:
			builder.WriteString("if")
		case *ast.ForStmt:
			builder.WriteString("for")
		case *ast.SwitchStmt:
			builder.WriteString("switch")
		}
		return true
	})
	return builder.String()
}

// rabinKarpHash generates a hash for a given string.
func rabinKarpHash(code string) uint64 {
	const primeBase = 31
	var hash uint64
	for _, ch := range code {
		hash = hash*primeBase + uint64(ch)
	}
	return hash
}

// CodeReadabilityMetrics evaluates readability based on heuristics.
type CodeReadabilityMetrics struct {
	FunctionLength   int
	NestingDepth     int
	CommentDensity   float64
	CyclomaticPoints int     // Track complexity points
	BranchDensity    float64 // Branches per line
}

// CognitiveComplexity represents the cognitive load of code
type CognitiveComplexity struct {
	Score          int
	NestedDepth    int
	LogicalOps     int
	BranchingScore int
}

// DeadCodeInfo tracks unused code elements
type DeadCodeInfo struct {
	UnusedFunctions []string
	Reachable       map[string]bool
}

type CodeDuplicationDetector struct {
	seen map[uint64]string
	mu   sync.RWMutex
}
