package types

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// -----------------------------------------------------------------------------
// Domain (Full) Types
// -----------------------------------------------------------------------------

type FunctionCall struct {
	ID                *models.RecordID `json:"id,omitempty"`
	Caller            string           `json:"caller"`
	Callees           []string         `json:"callees"`
	File              string           `json:"file"`
	Package           string           `json:"package"`
	Params            []string         `json:"params"`
	Returns           []string         `json:"returns"`
	IsMethod          bool             `json:"is_method"`
	IsRecursive       bool             `json:"is_recursive"`
	IsDuplicate       bool             `json:"is_duplicate"`
	IsInterface       bool             `json:"is_interface"`
	IsStruct          bool             `json:"is_struct"`
	IsGlobal          bool             `json:"is_global"`
	Struct            string           `json:"struct"`
	Metrics           FunctionMetrics  `json:"metrics"`
	ReferencedGlobals []string         `json:"referenced_globals"`
	Dependencies      []string         `json:"dependencies"`
}

type StructDefinition struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Name    string           `json:"name"`
	File    string           `json:"file"`
	Package string           `json:"package"`
}

type InterfaceDefinition struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Name    string           `json:"name"`
	File    string           `json:"file"`
	Package string           `json:"package"`
	Methods []string         `json:"methods"`
}

type GlobalVariable struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Name    string           `json:"name"`
	Type    string           `json:"type"`
	Value   string           `json:"value,omitempty"`
	File    string           `json:"file"`
	Package string           `json:"package"`
}

type ImportDefinition struct {
	ID      *models.RecordID `json:"id,omitempty"`
	Path    string           `json:"path"`
	File    string           `json:"file"`
	Package string           `json:"package"`
}

type InterfaceImplementation struct {
	Struct    string `json:"struct"`
	Interface string `json:"interface"`
}

type AnalysisReport struct {
	Functions  []FunctionCall
	Structs    []StructDefinition
	Interfaces []InterfaceDefinition
	Globals    []GlobalVariable
	Imports    []ImportDefinition
	Implements []InterfaceImplementation
}

// -----------------------------------------------------------------------------
// Metrics Types
// -----------------------------------------------------------------------------

type FunctionMetrics struct {
	CyclomaticComplexity int                        `json:"cyclomatic_complexity"`
	LinesOfCode          int                        `json:"lines_of_code"`
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

// -----------------------------------------------------------------------------
// Summary Types
// -----------------------------------------------------------------------------

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

type StructSummary struct {
	Name    string `json:"name"`
	File    string `json:"file"`
	Package string `json:"package"`
}

type InterfaceSummary struct {
	Name    string `json:"name"`
	File    string `json:"file"`
	Package string `json:"package"`
}

type GlobalSummary struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	File    string `json:"file"`
	Package string `json:"package"`
}

type ImportSummary struct {
	Path    string `json:"path"`
	File    string `json:"file"`
	Package string `json:"package"`
}

type ImplementationSummary struct {
	Struct    string `json:"struct"`
	Interface string `json:"interface"`
}

// -----------------------------------------------------------------------------
// Internal Conversion Helpers
// -----------------------------------------------------------------------------

// FunctionSummary is the "compressed" version of FunctionCall
type FunctionSummary struct {
	Name            string  `json:"name"`
	File            string  `json:"file"`
	Complexity      int     `json:"complexity"`
	Lines           int     `json:"lines"`
	Maintainability float64 `json:"maintainability"`
	NestingDepth    int     `json:"nesting_depth"`
	IsUnused        bool    `json:"is_unused"`
	IsDuplicate     bool    `json:"is_duplicate"`
	IsRecursive     bool    `json:"is_recursive"`
	IsMethod        bool    `json:"is_method"`
	IsInterface     bool    `json:"is_interface"`
	IsStruct        bool    `json:"is_struct"`
	IsGlobal        bool    `json:"is_global"`
}

// ToFunctionSummary converts a FunctionCall into its summary representation.
func (fn FunctionCall) ToFunctionSummary() FunctionSummary {
	return FunctionSummary{
		Name:            fn.Caller,
		File:            fn.File,
		Complexity:      fn.Metrics.CyclomaticComplexity,
		Lines:           fn.Metrics.LinesOfCode,
		Maintainability: fn.Metrics.Maintainability,
		NestingDepth:    fn.Metrics.Readability.NestingDepth,
		IsUnused:        fn.Metrics.IsUnused,

		// Merge or choose from either FunctionCall's IsDuplicate or Metrics.IsDuplicate
		IsDuplicate: fn.IsDuplicate,

		IsRecursive: fn.IsRecursive,
		IsMethod:    fn.IsMethod,
		IsInterface: fn.IsInterface,
		IsStruct:    fn.IsStruct,
		IsGlobal:    fn.IsGlobal,
	}
}

// ToStructSummary converts a StructDefinition into its summary representation.
func (s StructDefinition) ToStructSummary() StructSummary {
	return StructSummary{
		Name:    s.Name,
		File:    s.File,
		Package: s.Package,
	}
}

// ToInterfaceSummary converts an InterfaceDefinition to an InterfaceSummary.
func (i InterfaceDefinition) ToInterfaceSummary() InterfaceSummary {
	return InterfaceSummary{
		Name:    i.Name,
		File:    i.File,
		Package: i.Package,
	}
}

// ToGlobalSummary converts a GlobalVariable to a GlobalSummary.
func (g GlobalVariable) ToGlobalSummary() GlobalSummary {
	return GlobalSummary{
		Name:    g.Name,
		Type:    g.Type,
		Value:   g.Value,
		File:    g.File,
		Package: g.Package,
	}
}

// ToImportSummary converts an ImportDefinition to an ImportSummary.
func (i ImportDefinition) ToImportSummary() ImportSummary {
	return ImportSummary{
		Path:    i.Path,
		File:    i.File,
		Package: i.Package,
	}
}

// ToImplementationSummary converts an InterfaceImplementation to a summary.
func (impl InterfaceImplementation) ToImplementationSummary() ImplementationSummary {
	return ImplementationSummary{
		Struct:    impl.Struct,
		Interface: impl.Interface,
	}
}

// -----------------------------------------------------------------------------
// PrettyPrint Logic
// -----------------------------------------------------------------------------

// Summary is the overall JSON structure we marshal in PrettyPrint.
type Summary struct {
	TotalFunctions int                     `json:"total_functions"`
	TotalStructs   int                     `json:"total_structs"`
	TotalImports   int                     `json:"total_imports"`
	Functions      []FunctionSummary       `json:"functions"`
	Structs        []StructSummary         `json:"structs"`
	Interfaces     []InterfaceSummary      `json:"interfaces"`
	Globals        []GlobalSummary         `json:"globals"`
	Imports        []ImportSummary         `json:"imports"`
	Implements     []ImplementationSummary `json:"implements"`
}

// BuildSummary constructs the summary object from the AnalysisReport.
func (r AnalysisReport) BuildSummary() Summary {
	// Convert full objects to "summary" objects
	fnSummaries := make([]FunctionSummary, 0, len(r.Functions))
	for _, fn := range r.Functions {
		fnSummaries = append(fnSummaries, fn.ToFunctionSummary())
	}

	structSummaries := make([]StructSummary, 0, len(r.Structs))
	for _, st := range r.Structs {
		structSummaries = append(structSummaries, st.ToStructSummary())
	}

	ifaceSummaries := make([]InterfaceSummary, 0, len(r.Interfaces))
	for _, iface := range r.Interfaces {
		ifaceSummaries = append(ifaceSummaries, iface.ToInterfaceSummary())
	}

	globalSummaries := make([]GlobalSummary, 0, len(r.Globals))
	for _, g := range r.Globals {
		globalSummaries = append(globalSummaries, g.ToGlobalSummary())
	}

	importSummaries := make([]ImportSummary, 0, len(r.Imports))
	for _, imp := range r.Imports {
		importSummaries = append(importSummaries, imp.ToImportSummary())
	}

	implSummaries := make([]ImplementationSummary, 0, len(r.Implements))
	for _, impl := range r.Implements {
		implSummaries = append(implSummaries, impl.ToImplementationSummary())
	}

	// Build initial summary object
	summary := Summary{
		TotalFunctions: len(r.Functions),
		TotalStructs:   len(r.Structs),
		TotalImports:   len(r.Imports),
		Functions:      fnSummaries,
		Structs:        structSummaries,
		Interfaces:     ifaceSummaries,
		Globals:        globalSummaries,
		Imports:        importSummaries,
		Implements:     implSummaries,
	}

	// Sort them as needed
	sort.Slice(summary.Functions, func(i, j int) bool {
		return summary.Functions[i].Name < summary.Functions[j].Name
	})
	sort.Slice(summary.Structs, func(i, j int) bool {
		return summary.Structs[i].Name < summary.Structs[j].Name
	})
	sort.Slice(summary.Interfaces, func(i, j int) bool {
		return summary.Interfaces[i].Name < summary.Interfaces[j].Name
	})
	sort.Slice(summary.Globals, func(i, j int) bool {
		return summary.Globals[i].Name < summary.Globals[j].Name
	})
	sort.Slice(summary.Imports, func(i, j int) bool {
		return summary.Imports[i].Path < summary.Imports[j].Path
	})
	sort.Slice(summary.Implements, func(i, j int) bool {
		if summary.Implements[i].Struct != summary.Implements[j].Struct {
			return summary.Implements[i].Struct < summary.Implements[j].Struct
		}
		return summary.Implements[i].Interface < summary.Implements[j].Interface
	})

	return summary
}

// PrettyPrint returns a JSON-formatted summary of the analysis.
func (r AnalysisReport) PrettyPrint() string {
	summary := r.BuildSummary()

	jsonBytes, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error generating summary: %v", err)
	}
	return string(jsonBytes)
}
