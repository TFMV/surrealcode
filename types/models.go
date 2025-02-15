package types

import "github.com/surrealdb/surrealdb.go/pkg/models"

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
