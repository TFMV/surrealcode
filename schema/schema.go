package schema

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
)

// Schema contains all SurrealDB schema definitions
const Schema = `
-- Functions table (nodes)
DEFINE TABLE functions SCHEMAFULL;
DEFINE FIELD caller ON functions TYPE string ASSERT $value != NONE;
DEFINE FIELD file ON functions TYPE string;
DEFINE FIELD package ON functions TYPE string ASSERT $value != NONE;
DEFINE FIELD params ON functions TYPE array;
DEFINE FIELD returns ON functions TYPE array;
DEFINE FIELD is_method ON functions TYPE bool;
DEFINE FIELD struct ON functions TYPE option<string>;
DEFINE FIELD is_recursive ON functions TYPE bool;
DEFINE FIELD metrics ON functions TYPE object {
    cyclomatic_complexity: int,
    lines_of_code: int,
    is_duplicate: bool,
    is_unused: bool,
    halstead_metrics: {
        volume: float,
        difficulty: float,
        effort: float
    },
    cognitive_complexity: {
        score: int,
        nested_depth: int,
        logical_ops: int,
        branching_score: int
    },
    readability: {
        nesting_depth: int,
        comment_density: float,
        branch_density: float
    },
    maintainability: float
};
DEFINE FIELD created_at ON functions TYPE datetime DEFAULT time::now();
DEFINE FIELD updated_at ON functions TYPE datetime DEFAULT time::now();
DEFINE INDEX function_name ON functions FIELDS package, caller;

-- Calls table (edges: function-to-function relationships)
DEFINE TABLE calls SCHEMAFULL;
DEFINE FIELD from ON calls TYPE record<functions> ASSERT $value != NONE;
DEFINE FIELD to ON calls TYPE record<functions> ASSERT $value != NONE;
DEFINE FIELD file ON calls TYPE string;
DEFINE FIELD package ON calls TYPE string;
DEFINE INDEX call_relation ON calls FIELDS from, to;

-- Structs table
DEFINE TABLE structs SCHEMAFULL;
DEFINE FIELD name ON structs TYPE string ASSERT $value != NONE;
DEFINE FIELD file ON structs TYPE string;
DEFINE FIELD package ON structs TYPE string ASSERT $value != NONE;
DEFINE INDEX struct_name ON structs FIELDS package, name;

-- Methods relation (edges: struct-to-function)
DEFINE TABLE methods SCHEMAFULL;
DEFINE FIELD struct ON methods TYPE record<structs> ASSERT $value != NONE;
DEFINE FIELD function ON methods TYPE record<functions> ASSERT $value != NONE;

-- Interfaces table
DEFINE TABLE interfaces SCHEMAFULL;
DEFINE FIELD name ON interfaces TYPE string ASSERT $value != NONE;
DEFINE FIELD methods ON interfaces TYPE array;
DEFINE FIELD file ON interfaces TYPE string;
DEFINE FIELD package ON interfaces TYPE string ASSERT $value != NONE;
DEFINE INDEX interface_name ON interfaces FIELDS package, name;

-- Interface implementations
DEFINE TABLE implements SCHEMAFULL;
DEFINE FIELD struct ON implements TYPE record<structs> ASSERT $value != NONE;
DEFINE FIELD interface ON implements TYPE record<interfaces> ASSERT $value != NONE;

-- Globals table
DEFINE TABLE globals SCHEMAFULL;
DEFINE FIELD name ON globals TYPE string ASSERT $value != NONE;
DEFINE FIELD type ON globals TYPE string;
DEFINE FIELD value ON globals TYPE option<string>;
DEFINE FIELD file ON globals TYPE string;
DEFINE FIELD package ON globals TYPE string ASSERT $value != NONE;
DEFINE INDEX global_name ON globals FIELDS package, name;

-- References table (edges: function-to-global relationships)
DEFINE TABLE references SCHEMAFULL;
DEFINE FIELD function ON references TYPE record<functions> ASSERT $value != NONE;
DEFINE FIELD global ON references TYPE record<globals> ASSERT $value != NONE;

-- Imports table
DEFINE TABLE imports SCHEMAFULL;
DEFINE FIELD path ON imports TYPE string ASSERT $value != NONE;
DEFINE FIELD file ON imports TYPE string;
DEFINE FIELD package ON imports TYPE string ASSERT $value != NONE;

-- Dependencies table (edges: function-to-import relationships)
DEFINE TABLE dependencies SCHEMAFULL;
DEFINE FIELD function ON dependencies TYPE record<functions> ASSERT $value != NONE;
DEFINE FIELD import ON dependencies TYPE record<imports> ASSERT $value != NONE;
`

// InitializeSchema sets up the database schema
func InitializeSchema(ctx context.Context, db *surrealdb.DB) error {
	// Execute schema definition
	if _, err := surrealdb.Query[any](db, Schema, map[string]interface{}{}); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}
	return nil
}
