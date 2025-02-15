package types

import (
	"fmt"
	"strings"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// FunctionCall represents a function's metadata and its relationships
type FunctionCall struct {
	ID                   *models.RecordID `json:"id,omitempty"`
	Caller               string           `json:"caller"`
	Callees              []string         `json:"callees"`
	File                 string           `json:"file"`
	Package              string           `json:"package"`
	Params               []string         `json:"params"`
	Returns              []string         `json:"returns"`
	IsMethod             bool             `json:"is_method"`
	Struct               string           `json:"struct"`
	IsRecursive          bool             `json:"is_recursive"`
	CyclomaticComplexity int              `json:"cyclomatic_complexity"`
	LinesOfCode          int              `json:"lines_of_code"`
	IsDuplicate          bool             `json:"is_duplicate"`
	Metrics              FunctionMetrics  `json:"metrics"`
}

// StructDefinition represents a struct declaration
type StructDefinition struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Name    string           `json:"name"`
	File    string           `json:"file"`
	Package string           `json:"package"`
}

// InterfaceDefinition represents an interface declaration
type InterfaceDefinition struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Name    string           `json:"name"`
	File    string           `json:"file"`
	Package string           `json:"package"`
	Methods []string         `json:"methods"`
}

// GlobalVariable represents a global variable
type GlobalVariable struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Name    string           `json:"name"`
	Type    string           `json:"type"`
	Value   string           `json:"value,omitempty"`
	File    string           `json:"file"`
	Package string           `json:"package"`
}

// ImportDefinition represents an import statement
type ImportDefinition struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Path    string           `json:"path"`
	File    string           `json:"file"`
	Package string           `json:"package"`
}

// AnalysisReport contains the complete analysis results
type AnalysisReport struct {
	Functions  []FunctionCall
	Structs    []StructDefinition
	Interfaces []InterfaceDefinition
	Globals    []GlobalVariable
	Imports    []ImportDefinition
}

type FunctionMetrics struct {
	CyclomaticComplexity int  `json:"cyclomatic_complexity"`
	LinesOfCode          int  `json:"lines_of_code"`
	IsDuplicate          bool `json:"is_duplicate"`
	// Add new metrics
	HalsteadMetrics struct {
		Volume     float64 `json:"volume"`
		Difficulty float64 `json:"difficulty"`
		Effort     float64 `json:"effort"`
	} `json:"halstead_metrics"`
	CognitiveComplexity struct {
		Score          int `json:"score"`
		NestedDepth    int `json:"nested_depth"`
		LogicalOps     int `json:"logical_ops"`
		BranchingScore int `json:"branching_score"`
	} `json:"cognitive_complexity"`
	Readability struct {
		NestingDepth   int     `json:"nesting_depth"`
		CommentDensity float64 `json:"comment_density"`
		BranchDensity  float64 `json:"branch_density"`
	} `json:"readability"`
	Maintainability float64 `json:"maintainability_index"`
	IsUnused        bool    `json:"is_unused"`
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

// CodeSummary represents a high-level analysis of the codebase
type CodeSummary struct {
	TotalFunctions     int `json:"total_functions"`
	TotalLines         int `json:"total_lines"`
	UnusedFunctions    int `json:"unused_functions"`
	RecursiveFunctions int `json:"recursive_functions"`
	DuplicateCode      int `json:"duplicate_code"`

	// Averages
	AvgComplexity      float64 `json:"avg_complexity"`
	AvgMaintainability float64 `json:"avg_maintainability"`
	AvgNestingDepth    float64 `json:"avg_nesting_depth"`

	// Distribution
	ComplexityDistribution map[string]int `json:"complexity_distribution"` // Low/Medium/High

	// Hotspots (most complex/problematic functions)
	Hotspots []HotspotFunction `json:"hotspots"`
}

type HotspotFunction struct {
	Name            string   `json:"name"`
	File            string   `json:"file"`
	Complexity      int      `json:"complexity"`
	Maintainability float64  `json:"maintainability"`
	Issues          []string `json:"issues"` // e.g., "High complexity", "Deep nesting"
}

// PrettyPrint returns a formatted summary of the analysis
func (r AnalysisReport) PrettyPrint() string {
	var summary strings.Builder
	fmt.Fprintf(&summary, "Analysis Summary:\n"+
		"Total Functions: %d\n"+
		"Total Structs: %d\n"+
		"Total Interfaces: %d\n"+
		"Total Globals: %d\n"+
		"Total Imports: %d\n\n",
		len(r.Functions), len(r.Structs), len(r.Interfaces),
		len(r.Globals), len(r.Imports))

	fmt.Fprintf(&summary, "Function Metrics:\n")
	for _, fn := range r.Functions {
		fmt.Fprintf(&summary, "\n%s (%s):\n"+
			"  Complexity: %d\n"+
			"  Lines: %d\n"+
			"  Maintainability: %.2f\n"+
			"  Nesting Depth: %d\n"+
			"  Is Unused: %v\n",
			fn.Caller, fn.File,
			fn.Metrics.CyclomaticComplexity,
			fn.Metrics.LinesOfCode,
			fn.Metrics.Maintainability,
			fn.Metrics.Readability.NestingDepth,
			fn.Metrics.IsUnused)
	}

	return summary.String()
}
