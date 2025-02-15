package schema

import (
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
)

// InitializeSchema sets up the database schema and indexes for SurrealCode
func InitializeSchema(db *surrealdb.DB) error {
	schemas := []string{
		// Define functions table
		`DEFINE TABLE functions SCHEMAFULL;
		 DEFINE FIELD name ON functions TYPE string;
		 DEFINE FIELD file ON functions TYPE string;
		 DEFINE FIELD package ON functions TYPE string;
		 DEFINE FIELD params ON functions TYPE array;
		 DEFINE FIELD returns ON functions TYPE array;
		 DEFINE FIELD is_method ON functions TYPE bool;
		 DEFINE FIELD struct ON functions TYPE option<string>;
		 DEFINE FIELD is_recursive ON functions TYPE bool;
		 DEFINE FIELD complexity ON functions TYPE int;
		 DEFINE FIELD created_at ON functions TYPE datetime DEFAULT time::now();
		 DEFINE FIELD updated_at ON functions TYPE datetime;
		 DEFINE INDEX func_name ON functions FIELDS name;
		 DEFINE INDEX func_file ON functions FIELDS file;
		 DEFINE INDEX func_package ON functions FIELDS package;`,

		// Define structs table
		`DEFINE TABLE structs SCHEMAFULL;
		 DEFINE FIELD name ON structs TYPE string;
		 DEFINE FIELD file ON structs TYPE string;
		 DEFINE FIELD package ON structs TYPE string;
		 DEFINE FIELD fields ON structs TYPE array;
		 DEFINE FIELD created_at ON structs TYPE datetime DEFAULT time::now();
		 DEFINE INDEX struct_name ON structs FIELDS name;
		 DEFINE INDEX struct_file ON structs FIELDS file;`,

		// Define globals table
		`DEFINE TABLE globals SCHEMAFULL;
		 DEFINE FIELD name ON globals TYPE string;
		 DEFINE FIELD type ON globals TYPE string;
		 DEFINE FIELD value ON globals TYPE string;
		 DEFINE FIELD file ON globals TYPE string;
		 DEFINE FIELD package ON globals TYPE string;
		 DEFINE FIELD created_at ON globals TYPE datetime DEFAULT time::now();
		 DEFINE INDEX global_name ON globals FIELDS name;
		 DEFINE INDEX global_file ON globals FIELDS file;`,

		// Define imports table
		`DEFINE TABLE imports SCHEMAFULL;
		 DEFINE FIELD path ON imports TYPE string;
		 DEFINE FIELD file ON imports TYPE string;
		 DEFINE FIELD package ON imports TYPE string;
		 DEFINE FIELD created_at ON imports TYPE datetime DEFAULT time::now();
		 DEFINE INDEX import_path ON imports FIELDS path;`,

		// Define the call relationships as an edge table
		`DEFINE TABLE calls SCHEMAFULL;
		 DEFINE FIELD from ON calls TYPE record<functions>;
		 DEFINE FIELD to ON calls TYPE record<functions>;
		 DEFINE FIELD file ON calls TYPE string;
		 DEFINE FIELD package ON calls TYPE string;
		 DEFINE INDEX call_relation ON calls FIELDS from, to;`,

		// Define struct-field relationships
		`DEFINE TABLE struct_fields SCHEMALESS AS EDGE;
		 DEFINE FIELD struct ON struct_fields TYPE record(structs);
		 DEFINE FIELD field ON struct_fields TYPE string;
		 DEFINE FIELD type ON struct_fields TYPE string;
		 DEFINE FIELD created_at ON struct_fields TYPE datetime DEFAULT time::now();
		 DEFINE INDEX struct_field_relation ON struct_fields FIELDS struct, field;`,

		// Define function-struct relationships
		`DEFINE TABLE method_of SCHEMALESS AS EDGE;
		 DEFINE FIELD function ON method_of TYPE record(functions);
		 DEFINE FIELD struct ON method_of TYPE record(structs);
		 DEFINE FIELD created_at ON method_of TYPE datetime DEFAULT time::now();
		 DEFINE INDEX method_struct_relation ON method_of FIELDS function, struct;`,

		// Define function-global variable relationships
		`DEFINE TABLE uses_global SCHEMALESS AS EDGE;
		 DEFINE FIELD function ON uses_global TYPE record(functions);
		 DEFINE FIELD global ON uses_global TYPE record(globals);
		 DEFINE FIELD created_at ON uses_global TYPE datetime DEFAULT time::now();
		 DEFINE INDEX function_global_relation ON uses_global FIELDS function, global;`,
	}

	// Execute each schema definition
	for _, schema := range schemas {
		if _, err := surrealdb.Query[any](db, schema, map[string]interface{}{}); err != nil {
			return fmt.Errorf("schema initialization error: %w", err)
		}
	}

	return nil
}
