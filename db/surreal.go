package db

import (
	"context"
	"fmt"

	"github.com/TFMV/surrealcode/types"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

type Config struct {
	URL       string
	Namespace string
	Database  string
	Username  string
	Password  string
}

type SurrealDB struct {
	db     *surrealdb.DB
	config Config
}

func NewSurrealDB(config Config) (*SurrealDB, error) {
	db, err := surrealdb.New(config.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &SurrealDB{
		db:     db,
		config: config,
	}, nil
}

func (s *SurrealDB) Initialize(ctx context.Context) error {
	if err := s.db.Use(s.config.Namespace, s.config.Database); err != nil {
		return fmt.Errorf("failed to set namespace/database: %w", err)
	}

	authData := &surrealdb.Auth{
		Username: s.config.Username,
		Password: s.config.Password,
	}
	token, err := s.db.SignIn(authData)
	if err != nil {
		return fmt.Errorf("failed to sign in: %w", err)
	}

	if err := s.db.Authenticate(token); err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	return nil
}

func (s *SurrealDB) StoreAnalysis(ctx context.Context, report types.AnalysisReport) error {
	// Store functions
	for _, fn := range report.Functions {
		if _, err := surrealdb.Create[types.FunctionCall](s.db, models.Table("functions"), fn); err != nil {
			return fmt.Errorf("error storing function %s: %v", fn.Caller, err)
		}

		// Create call relationships
		for _, callee := range fn.Callees {
			query := fmt.Sprintf(`
				LET $caller = SELECT * FROM functions WHERE caller = '%s';
				LET $callee = SELECT * FROM functions WHERE caller = '%s';
				CREATE calls SET 
					from = $caller[0].id,
					to = $callee[0].id,
					file = '%s',
					package = '%s'`,
				fn.Caller, callee, fn.File, fn.Package,
			)
			if _, err := surrealdb.Query[any](s.db, query, map[string]interface{}{}); err != nil {
				return fmt.Errorf("error creating call relationship %s->%s: %v", fn.Caller, callee, err)
			}
		}
	}

	// Store other entities...
	// (structs, interfaces, globals, imports)
	// Similar pattern as functions

	return nil
}
