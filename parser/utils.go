package parser

import (
	"go/ast"
	"go/token"
)

func computeComplexity(node ast.Node) int {
	complexity := 1 // Base complexity
	ast.Inspect(node, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause, *ast.SelectStmt:
			complexity++
		case *ast.BinaryExpr:
			bin := n.(*ast.BinaryExpr)
			if bin.Op == token.LAND || bin.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	return complexity
}

func computeLOC(fset *token.FileSet, node *ast.BlockStmt) int {
	if node == nil {
		return 0
	}
	return fset.Position(node.End()).Line - fset.Position(node.Pos()).Line + 1
}
