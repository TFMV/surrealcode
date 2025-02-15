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
	}

	// Store structs
	for _, st := range report.Structs {
		if _, err := surrealdb.Create[types.StructDefinition](s.db, models.Table("structs"), st); err != nil {
			return fmt.Errorf("error storing struct %s: %v", st.Name, err)
		}
	}

	// Store interfaces
	for _, iface := range report.Interfaces {
		if _, err := surrealdb.Create[types.InterfaceDefinition](s.db, models.Table("interfaces"), iface); err != nil {
			return fmt.Errorf("error storing interface %s: %v", iface.Name, err)
		}
	}

	// Store globals
	for _, global := range report.Globals {
		if _, err := surrealdb.Create[types.GlobalVariable](s.db, models.Table("globals"), global); err != nil {
			return fmt.Errorf("error storing global %s: %v", global.Name, err)
		}
	}

	// Store imports
	for _, imp := range report.Imports {
		if _, err := surrealdb.Create[types.ImportDefinition](s.db, models.Table("imports"), imp); err != nil {
			return fmt.Errorf("error storing import %s: %v", imp.Path, err)
		}
	}

	return nil
}
