package expr

import (
	"fmt"
	"go/ast"
	"runtime"
	"strings"
	"sync"

	"github.com/golang/groupcache/lru"
)

// ExprCache caches the string representation of AST expressions.
type ExprCache struct {
	cache *lru.Cache
	mu    sync.RWMutex // Mutex for thread safety
}

// NewExprCache creates a new ExprCache of the given size and registers a cleanup function.
func NewExprCache(size int) *ExprCache {
	ec := &ExprCache{
		cache: lru.New(size),
	}

	// Register a cleanup function that clears the cache when ec is garbage collected.
	runtime.AddCleanup(ec, func(c *lru.Cache) {
		fmt.Println("Cleaning up ExprCache: clearing underlying cache")
		c.Clear()
	}, ec.cache)

	return ec
}

// Get returns the cached string for the given expression, if available.
func (c *ExprCache) Get(expr ast.Expr) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if val, ok := c.cache.Get(expr); ok {
		return val.(string), true
	}
	return "", false
}

// Put adds the string representation for an expression into the cache.
func (c *ExprCache) Put(expr ast.Expr, str string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Add(expr, str)
}

// ToString returns the string representation for the given AST expression,
// using the cache to avoid redundant computations.
func (c *ExprCache) ToString(expr ast.Expr) string {
	// Try reading with a read lock first.
	c.mu.RLock()
	if val, ok := c.cache.Get(expr); ok {
		c.mu.RUnlock()
		return val.(string)
	}
	c.mu.RUnlock()

	// If not found, acquire a write lock and compute the string.
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check in case another goroutine computed it meanwhile.
	if val, ok := c.cache.Get(expr); ok {
		return val.(string)
	}

	// Compute string representation based on the expression type.
	var result string
	switch e := expr.(type) {
	case *ast.Ident:
		result = e.Name
	case *ast.StarExpr:
		result = "*" + c.ToString(e.X)
	case *ast.SelectorExpr:
		result = c.ToString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		result = "[]" + c.ToString(e.Elt)
	case *ast.MapType:
		result = fmt.Sprintf("map[%s]%s", c.ToString(e.Key), c.ToString(e.Value))
	case *ast.ChanType:
		result = "chan " + c.ToString(e.Value)
	case *ast.FuncType:
		params := make([]string, 0, len(e.Params.List))
		for _, p := range e.Params.List {
			params = append(params, c.ToString(p.Type))
		}
		results := []string{}
		if e.Results != nil {
			results = make([]string, 0, len(e.Results.List))
			for _, r := range e.Results.List {
				results = append(results, c.ToString(r.Type))
			}
		}
		result = fmt.Sprintf("func(%s)", strings.Join(params, ", "))
		if len(results) > 0 {
			result += " (" + strings.Join(results, ", ") + ")"
		}
	case *ast.InterfaceType:
		methods := make([]string, 0, len(e.Methods.List))
		for _, m := range e.Methods.List {
			methods = append(methods, c.ToString(m.Type))
		}
		result = "interface{" + strings.Join(methods, "; ") + "}"
	case *ast.StructType:
		fields := make([]string, 0, len(e.Fields.List))
		for _, f := range e.Fields.List {
			fields = append(fields, c.ToString(f.Type))
		}
		result = "struct{" + strings.Join(fields, "; ") + "}"
	case *ast.BasicLit:
		result = e.Value
	default:
		result = fmt.Sprintf("<%T>", expr)
	}

	c.cache.Add(expr, result)
	return result
}

// Clear clears the cache.
func (c *ExprCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Clear()
}
