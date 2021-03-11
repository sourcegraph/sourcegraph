package search

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/search/syntax"
)

// ErrExpr is a base type for errors that occur in a specific expression
// within a parse tree, and is intended to be embedded within other error types.
type ErrExpr struct {
	Pos   int
	Input string
}

func createErrExpr(input string, expr *syntax.Expr) ErrExpr {
	return ErrExpr{
		Pos:   expr.Pos,
		Input: input,
	}
}

func (e ErrExpr) Error() string {
	preceding := ""
	if e.Pos > 0 {
		preceding = e.Input[0:e.Pos]
		if len(preceding) > 10 {
			preceding = "..." + preceding[len(preceding)-10:]
		}
	}

	succeeding := ""
	if e.Pos < len(e.Input)-1 {
		succeeding = e.Input[e.Pos+1:]
	}

	return fmt.Sprintf("The error started at character %d: <code>%s<strong>%c</strong>%s</code>", e.Pos+1, preceding, e.Input[e.Pos], succeeding)
}

type ErrUnsupportedField struct {
	ErrExpr
	Field string
}

func (e ErrUnsupportedField) Error() string {
	return fmt.Sprintf("Fields of type `%s` are unsupported. %s", e.Field, e.ErrExpr.Error())
}

type ErrUnsupportedValueType struct {
	ErrExpr
	ValueType syntax.TokenType
}

func (e ErrUnsupportedValueType) Error() string {
	switch e.ValueType {
	case syntax.TokenPattern:
		return fmt.Sprintf("Regular expressions are unsupported. %s", e.ErrExpr.Error())
	default:
		return fmt.Sprintf("Values of type `%s` are unsupported. %s", e.ValueType.String(), e.ErrExpr.Error())
	}
}
