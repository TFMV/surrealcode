package expr

import (
	"fmt"
	"go/ast"
	"strings"
	"sync"

	"github.com/golang/groupcache/lru"
)

type ExprCache struct {
	cache *lru.Cache
	mu    sync.RWMutex // Add mutex for thread safety
}

func NewExprCache(size int) *ExprCache {
	return &ExprCache{
		cache: lru.New(size),
	}
}

func (c *ExprCache) Get(expr ast.Expr) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if val, ok := c.cache.Get(expr); ok {
		return val.(string), true
	}
	return "", false
}

func (c *ExprCache) Put(expr ast.Expr, str string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Add(expr, str)
}

func (c *ExprCache) ToString(expr ast.Expr) string {
	// First try with read lock
	c.mu.RLock()
	if val, ok := c.cache.Get(expr); ok {
		c.mu.RUnlock()
		return val.(string)
	}
	c.mu.RUnlock()

	// If not found, acquire write lock and compute
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check again in case another goroutine computed it
	if val, ok := c.cache.Get(expr); ok {
		return val.(string)
	}

	// Generate string representation
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

func (c *ExprCache) Clear() {
	c.cache.Clear()
}
