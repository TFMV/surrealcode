package analysis

import (
	"go/ast"
	"go/token"
	"math"
	"strings"
	"unicode"

	"github.com/TFMV/surrealcode/types"
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

// ComputeReadabilityMetrics analyzes a function for readability heuristics.
func ComputeReadabilityMetrics(fn *ast.FuncDecl) CodeReadabilityMetrics {
	loc := CountLines(fn)
	nestingDepth := 0
	commentCount := 0
	branchCount := 0
	maxNesting := 0
	currentNesting := 0

	ast.Inspect(fn, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.BlockStmt:
			currentNesting++
			if currentNesting > maxNesting {
				maxNesting = currentNesting
			}
			nestingDepth++
		case *ast.Comment:
			commentCount++
		case *ast.IfStmt, *ast.ForStmt, *ast.SwitchStmt, *ast.SelectStmt:
			branchCount++
		}
		return true
	})

	commentDensity := float64(commentCount) / float64(loc)
	branchDensity := float64(branchCount) / float64(loc)

	return CodeReadabilityMetrics{
		FunctionLength:   loc,
		NestingDepth:     maxNesting,
		CommentDensity:   commentDensity,
		CyclomaticPoints: branchCount + 1, // Base complexity + branches
		BranchDensity:    branchDensity,
	}
}

// MaintainabilityIndex computes a maintainability score.
func MaintainabilityIndex(metrics CodeReadabilityMetrics, complexity int, duplication bool) float64 {
	duplicationPenalty := 1.0
	if duplication {
		duplicationPenalty = 0.5
	}

	nestingPenalty := 1.0
	if metrics.NestingDepth > 3 {
		nestingPenalty = 0.8
	}

	branchPenalty := 1.0
	if metrics.BranchDensity > 0.5 {
		branchPenalty = 0.9
	}

	return (171 -
		5.2*float64(metrics.FunctionLength) -
		0.23*float64(complexity) -
		16.2*metrics.CommentDensity) *
		duplicationPenalty *
		nestingPenalty *
		branchPenalty
}

// CountLines estimates the number of lines in a function.
func CountLines(fn *ast.FuncDecl) int {
	start := fn.Pos()
	end := fn.End()
	return int(end - start) // Convert token.Pos to int
}

// CognitiveComplexity represents the cognitive load of code
type CognitiveComplexity struct {
	Score          int
	NestedDepth    int
	LogicalOps     int
	BranchingScore int
}

// ComputeCognitiveComplexity analyzes the cognitive complexity of a function
func ComputeCognitiveComplexity(fn *ast.FuncDecl) CognitiveComplexity {
	var cc CognitiveComplexity

	// visit recursively traverses the AST, passing along the current nesting level.
	var visit func(n ast.Node, nesting int)
	visit = func(n ast.Node, nesting int) {
		if n == nil {
			return
		}
		// Update maximum nesting depth if needed.
		if nesting > cc.NestedDepth {
			cc.NestedDepth = nesting
		}
		switch node := n.(type) {
		case *ast.IfStmt:
			cc.BranchingScore++
			cc.Score += 1               // if statement adds 1
			visit(node.Cond, nesting)   // condition: same nesting
			visit(node.Body, nesting+1) // body: deeper nesting
			visit(node.Else, nesting+1) // else branch: deeper nesting
			return
		case *ast.ForStmt:
			cc.BranchingScore++
			cc.Score += 2 // for loop adds 2
			if node.Init != nil {
				visit(node.Init, nesting)
			}
			if node.Cond != nil {
				visit(node.Cond, nesting)
			}
			if node.Post != nil {
				visit(node.Post, nesting)
			}
			visit(node.Body, nesting+1)
			return
		case *ast.RangeStmt:
			cc.BranchingScore++
			cc.Score += 2 // range loop adds 2
			visit(node.Body, nesting+1)
			return
		case *ast.SwitchStmt:
			cc.BranchingScore++
			cc.Score += 1 // switch adds 1
			if node.Tag != nil {
				visit(node.Tag, nesting)
			}
			visit(node.Body, nesting+1)
			return
		case *ast.BinaryExpr:
			if node.Op == token.LAND || node.Op == token.LOR {
				cc.LogicalOps++
				cc.Score += 1 // each && or || adds 1
			}
			visit(node.X, nesting)
			visit(node.Y, nesting)
			return
		}
		// For any other node, traverse its children.
		ast.Inspect(n, func(child ast.Node) bool {
			// Skip the current node to avoid infinite recursion.
			if child == n {
				return true
			}
			visit(child, nesting)
			return false
		})
	}

	visit(fn, 0)
	return cc
}

// DeadCodeInfo tracks unused code elements
type DeadCodeInfo struct {
	UnusedFunctions []string
	Reachable       map[string]bool
}

// DetectDeadCode analyzes the call graph to find unused functions
func DetectDeadCode(functions map[string]types.FunctionCall, entryPoints []string) DeadCodeInfo {
	var info DeadCodeInfo
	info.Reachable = make(map[string]bool)

	// Mark all entry points as reachable
	for _, entry := range entryPoints {
		markReachable(entry, functions, info.Reachable)
	}

	// Find unused functions
	for fname := range functions {
		if !info.Reachable[fname] && !isExported(fname) {
			info.UnusedFunctions = append(info.UnusedFunctions, fname)
		}
	}

	return info
}

// markReachable recursively marks all functions reachable from the given function
func markReachable(fname string, functions map[string]types.FunctionCall, reachable map[string]bool) {
	if reachable[fname] {
		return // Already visited
	}
	reachable[fname] = true

	// Mark all called functions as reachable
	if fn, exists := functions[fname]; exists {
		for _, callee := range fn.Callees {
			if !strings.Contains(callee, ".") { // Skip package-qualified calls
				markReachable(callee, functions, reachable)
			}
		}
	}
}

// isExported returns true if the function name starts with an uppercase letter
func isExported(fname string) bool {
	if len(fname) == 0 {
		return false
	}
	return unicode.IsUpper(rune(fname[0]))
}
