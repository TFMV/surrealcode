package types

import (
	"encoding/json"
	"fmt"

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
	ReferencedGlobals    []string         `json:"referenced_globals"`
	Dependencies         []string         `json:"dependencies"`
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

// InterfaceImplementation represents an implementation of an interface
type InterfaceImplementation struct {
	Struct    string `json:"struct"`
	Interface string `json:"interface"`
}

// AnalysisReport contains the complete analysis results
type AnalysisReport struct {
	Functions  []FunctionCall
	Structs    []StructDefinition
	Interfaces []InterfaceDefinition
	Globals    []GlobalVariable
	Imports    []ImportDefinition
	Implements []InterfaceImplementation
}

type FunctionMetrics struct {
	CyclomaticComplexity int                        `json:"cyclomatic_complexity"`
	LinesOfCode          int                        `json:"lines_of_code"`
	IsDuplicate          bool                       `json:"is_duplicate"`
	HalsteadMetrics      HalsteadMetrics            `json:"halstead_metrics"`
	CognitiveComplexity  CognitiveComplexityMetrics `json:"cognitive_complexity"`
	Readability          ReadabilityMetrics         `json:"readability"`
	Maintainability      float64                    `json:"maintainability_index"`
	IsUnused             bool                       `json:"is_unused"`
}

type HalsteadMetrics struct {
	Volume     float64 `json:"volume"`
	Difficulty float64 `json:"difficulty"`
	Effort     float64 `json:"effort"`
}

type CognitiveComplexityMetrics struct {
	Score          int `json:"score"`
	NestedDepth    int `json:"nested_depth"`
	LogicalOps     int `json:"logical_ops"`
	BranchingScore int `json:"branching_score"`
}

type ReadabilityMetrics struct {
	NestingDepth   int     `json:"nesting_depth"`
	CommentDensity float64 `json:"comment_density"`
	BranchDensity  float64 `json:"branch_density"`
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
	type FunctionSummary struct {
		Name            string  `json:"name"`
		File            string  `json:"file"`
		Complexity      int     `json:"complexity"`
		Lines           int     `json:"lines"`
		Maintainability float64 `json:"maintainability"`
		NestingDepth    int     `json:"nesting_depth"`
		IsUnused        bool    `json:"is_unused"`
	}

	type Summary struct {
		TotalFunctions int               `json:"total_functions"`
		TotalStructs   int               `json:"total_structs"`
		TotalImports   int               `json:"total_imports"`
		Functions      []FunctionSummary `json:"functions"`
	}

	summary := Summary{
		TotalFunctions: len(r.Functions),
		TotalStructs:   len(r.Structs),
		TotalImports:   len(r.Imports),
		Functions:      make([]FunctionSummary, 0, len(r.Functions)),
	}

	for _, fn := range r.Functions {
		summary.Functions = append(summary.Functions, FunctionSummary{
			Name:            fn.Caller,
			File:            fn.File,
			Complexity:      fn.Metrics.CyclomaticComplexity,
			Lines:           fn.Metrics.LinesOfCode,
			Maintainability: fn.Metrics.Maintainability,
			NestingDepth:    fn.Metrics.Readability.NestingDepth,
			IsUnused:        fn.Metrics.IsUnused,
		})
	}

	jsonBytes, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error generating summary: %v", err)
	}

	return string(jsonBytes)
}
