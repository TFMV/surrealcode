package schema

import (
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
)

// InitializeSchema sets up the database schema and indexes
func InitializeSchema(db *surrealdb.DB) error {
	schemas := []string{
		// Define functions table
		`DEFINE TABLE functions SCHEMAFULL;
		 DEFINE FIELD caller ON functions TYPE string;
		 DEFINE FIELD callees ON functions TYPE array;
		 DEFINE FIELD file ON functions TYPE string;
		 DEFINE FIELD package ON functions TYPE string;
		 DEFINE FIELD params ON functions TYPE array;
		 DEFINE FIELD returns ON functions TYPE array;
		 DEFINE FIELD is_method ON functions TYPE bool;
		 DEFINE FIELD struct ON functions TYPE option<string>;
		 DEFINE FIELD is_recursive ON functions TYPE bool;
		 DEFINE INDEX func_caller ON functions FIELDS caller;
		 DEFINE INDEX func_file ON functions FIELDS file;
		 DEFINE INDEX func_package ON functions FIELDS package;`,

		// Define structs table
		`DEFINE TABLE structs SCHEMAFULL;
		 DEFINE FIELD name ON structs TYPE string;
		 DEFINE FIELD file ON structs TYPE string;
		 DEFINE FIELD package ON structs TYPE string;
		 DEFINE INDEX struct_name ON structs FIELDS name;
		 DEFINE INDEX struct_file ON structs FIELDS file;`,

		// Define globals table
		`DEFINE TABLE globals SCHEMAFULL;
		 DEFINE FIELD name ON globals TYPE string;
		 DEFINE FIELD type ON globals TYPE string;
		 DEFINE FIELD value ON globals TYPE string;
		 DEFINE FIELD file ON globals TYPE string;
		 DEFINE FIELD package ON globals TYPE string;
		 DEFINE INDEX global_name ON globals FIELDS name;
		 DEFINE INDEX global_file ON globals FIELDS file;`,

		// Define relationships
		`DEFINE TABLE calls SCHEMAFULL;
		 DEFINE FIELD in ON calls TYPE string;
		 DEFINE FIELD out ON calls TYPE string;
		 DEFINE FIELD file ON calls TYPE string;
		 DEFINE FIELD package ON calls TYPE string;
		 DEFINE INDEX call_relation ON calls FIELDS in, out;`,
	}

	// Execute each schema definition
	for _, schema := range schemas {
		if _, err := surrealdb.Query[any](db, schema, map[string]interface{}{}); err != nil {
			return fmt.Errorf("schema initialization error: %v", err)
		}
	}

	return nil
}
