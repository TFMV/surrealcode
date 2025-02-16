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
	// Store functions (nodes)
	for _, fn := range report.Functions {
		function := map[string]interface{}{
			"caller":       fn.Caller,
			"file":         fn.File,
			"package":      fn.Package,
			"params":       fn.Params,
			"returns":      fn.Returns,
			"is_method":    fn.IsMethod,
			"struct":       fn.Struct,
			"is_recursive": fn.IsRecursive,
			"metrics":      fn.Metrics,
		}
		if _, err := surrealdb.Create[map[string]interface{}](s.db, "functions", function); err != nil {
			return fmt.Errorf("error storing function %s: %v", fn.Caller, err)
		}
	}

	// Store function calls (edges)
	for _, fn := range report.Functions {
		for _, callee := range fn.Callees {
			call := map[string]interface{}{
				"from":    fmt.Sprintf("functions:%s", fn.Caller),
				"to":      fmt.Sprintf("functions:%s", callee),
				"file":    fn.File,
				"package": fn.Package,
			}
			if _, err := surrealdb.Create[map[string]interface{}](s.db, "calls", call); err != nil {
				return fmt.Errorf("error storing call from %s to %s: %v", fn.Caller, callee, err)
			}
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
		interfaceData := map[string]interface{}{
			"name":    iface.Name,
			"methods": iface.Methods,
			"file":    iface.File,
			"package": iface.Package,
		}
		if _, err := surrealdb.Create[map[string]interface{}](s.db, "interfaces", interfaceData); err != nil {
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

	// Store methods (struct-to-function edges)
	for _, fn := range report.Functions {
		if fn.IsMethod && fn.Struct != "" {
			method := map[string]interface{}{
				"struct":   fmt.Sprintf("structs:%s", fn.Struct),
				"function": fmt.Sprintf("functions:%s", fn.Caller),
			}
			if _, err := surrealdb.Create[map[string]interface{}](s.db, "methods", method); err != nil {
				return fmt.Errorf("error storing method %s for struct %s: %v", fn.Caller, fn.Struct, err)
			}
		}
	}

	// Store implements (struct-to-interface edges)
	for _, impl := range report.Implements {
		implData := map[string]interface{}{
			"struct":    fmt.Sprintf("structs:%s", impl.Struct),
			"interface": fmt.Sprintf("interfaces:%s", impl.Interface),
		}
		if _, err := surrealdb.Create[map[string]interface{}](s.db, "implements", implData); err != nil {
			return fmt.Errorf("error storing implementation of %s by struct %s: %v", impl.Interface, impl.Struct, err)
		}
	}

	// Store references (function-to-global edges)
	for _, fn := range report.Functions {
		for _, global := range fn.ReferencedGlobals {
			reference := map[string]interface{}{
				"function": fmt.Sprintf("functions:%s", fn.Caller),
				"global":   fmt.Sprintf("globals:%s", global),
			}
			if _, err := surrealdb.Create[map[string]interface{}](s.db, "references", reference); err != nil {
				return fmt.Errorf("error storing reference to global %s in function %s: %v", global, fn.Caller, err)
			}
		}
	}

	// Store dependencies (function-to-import edges)
	for _, fn := range report.Functions {
		for _, imp := range fn.Dependencies {
			dependency := map[string]interface{}{
				"function": fmt.Sprintf("functions:%s", fn.Caller),
				"import":   fmt.Sprintf("imports:%s", imp),
			}
			if _, err := surrealdb.Create[map[string]interface{}](s.db, "dependencies", dependency); err != nil {
				return fmt.Errorf("error storing dependency %s in function %s: %v", imp, fn.Caller, err)
			}
		}
	}

	return nil
}
