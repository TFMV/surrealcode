package types

// FunctionMetrics contains code analysis metrics
type FunctionMetrics struct {
	Halstead    HalsteadMetrics
	Readability CodeReadabilityMetrics
	IsDuplicate bool
}

type HalsteadMetrics struct {
	Operators       int
	Operands        int
	UniqueOperators int
	UniqueOperands  int
	Volume          float64
	Difficulty      float64
	Effort          float64
}

type CodeReadabilityMetrics struct {
	FunctionLength int
	NestingDepth   int
	CommentDensity float64
}
