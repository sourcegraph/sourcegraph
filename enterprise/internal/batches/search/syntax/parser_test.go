package syntax

import (
	"reflect"
	"testing"
)

func TestParser(t *testing.T) {
	tests := map[string]struct {
		wantExpr   ParseTree
		wantString string
		wantErr    *ParseError
	}{
		"":   {wantExpr: []*Expr{}},
		" ":  {wantExpr: []*Expr{}, wantString: ""},
		"  ": {wantExpr: []*Expr{}, wantString: ""},
		"a": {
			wantExpr: []*Expr{{Value: "a", ValueType: TokenLiteral}},
		},
		"a ": {
			wantExpr:   []*Expr{{Value: "a", ValueType: TokenLiteral}},
			wantString: "a",
		},
		"a:": {wantExpr: []*Expr{{Field: "a", Value: "", ValueType: TokenLiteral}}},
		"a-": {
			wantExpr: []*Expr{{Value: "a-", ValueType: TokenLiteral}},
		},
		`"a"`: {
			wantExpr: []*Expr{{Value: `"a"`, ValueType: TokenQuoted}},
		},
		"-a": {
			wantExpr: []*Expr{{Not: true, Value: "a", ValueType: TokenLiteral}},
		},
		"a:b": {
			wantExpr: []*Expr{{Field: "a", Value: "b", ValueType: TokenLiteral}},
		},
		"a:b-:": {
			wantExpr: []*Expr{{Field: "a", Value: "b-:", ValueType: TokenLiteral}},
		},
		`a:"b"`: {
			wantExpr: []*Expr{{Field: "a", Value: `"b"`, ValueType: TokenQuoted}},
		},
		"-a:b": {
			wantExpr: []*Expr{{Not: true, Field: "a", Value: "b", ValueType: TokenLiteral}},
		},
		"/a/": {
			wantExpr: []*Expr{{Value: "a", ValueType: TokenPattern}},
		},
		`-/a/`: {
			wantExpr: []*Expr{{Not: true, Value: "a", ValueType: TokenPattern}},
		},
		"a b": {
			wantExpr: []*Expr{
				{Value: "a", ValueType: TokenLiteral},
				{Value: "b", ValueType: TokenLiteral},
			},
		},
		"a:b c:d": {
			wantExpr: []*Expr{
				{Field: "a", Value: "b", ValueType: TokenLiteral},
				{Field: "c", Value: "d", ValueType: TokenLiteral},
			},
		},
		"a: b:": {
			wantExpr: []*Expr{
				{Field: "a", Value: "", ValueType: TokenLiteral},
				{Field: "b", Value: "", ValueType: TokenLiteral},
			},
		},
		"--": {
			wantErr: &ParseError{Pos: 1, Msg: "got TokenMinus, want expr"},
		},
		`a:"b"-`: {
			wantErr: &ParseError{Pos: 5, Msg: "got TokenMinus, want separator or EOF"},
		},
		`"a"-`: {
			wantErr: &ParseError{Pos: 3, Msg: "got TokenMinus, want separator or EOF"},
		},
		`"a":b`: {
			wantErr: &ParseError{Pos: 3, Msg: "got TokenColon, want separator or EOF"},
		},
	}
	for input, test := range tests {
		t.Run(input, func(t *testing.T) {
			query, err := Parse(input)
			if err != nil && test.wantErr == nil {
				t.Fatal(err)
			} else if err == nil && test.wantErr != nil {
				t.Fatalf("got err == nil, want %q", test.wantErr)
			} else if test.wantErr != nil && !reflect.DeepEqual(err, test.wantErr) {
				t.Fatalf("got err == %q, want %q", err, test.wantErr)
			}
			if err != nil {
				return
			}
			if len(query) == 0 {
				query = []*Expr{}
			}
			for _, expr := range query {
				expr.Pos = 0
			}
			if !reflect.DeepEqual(query, test.wantExpr) {
				t.Errorf("expr: %s\ngot  %v\nwant %v", input, query, test.wantExpr)
			}
			if test.wantString == "" && len(query) > 0 {
				test.wantString = input
			}
			if exprString := query.String(); exprString != test.wantString {
				t.Errorf("expr string: %s\ngot  %s\nwant %s", input, exprString, test.wantString)
			}
		})
	}
}

func TestParseAllowingErrors(t *testing.T) {
	type args struct {
		input string
	}
	tests := []struct {
		name string
		args args
		want ParseTree
	}{
		{
			name: "empty",
			args: args{input: ""},
			want: nil,
		},
		{
			name: "a",
			args: args{input: "a"},
			want: []*Expr{
				{
					Value:     "a",
					ValueType: TokenLiteral,
				},
			},
		},
		{
			name: ":=",
			args: args{input: ":="},
			want: []*Expr{
				{
					Value:     ":=",
					ValueType: TokenError,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseAllowingErrors(tt.args.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseAllowingErrors() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
