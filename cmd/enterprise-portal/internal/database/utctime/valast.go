package utctime

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/hexops/valast"
)

// Register custom representation for autogold.
func init() {
	valast.RegisterType(func(ut Time) ast.Expr {
		t := ut.AsTime()
		return &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   &ast.Ident{Name: "utctime"},
				Sel: &ast.Ident{Name: "Date"},
			},
			Args: []ast.Expr{
				&ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", t.Year())},
				&ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", t.Month())},
				&ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", t.Day())},
				&ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", t.Hour())},
				&ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", t.Minute())},
				&ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", t.Second())},
				&ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", t.Nanosecond())},
			},
		}
	})
}
