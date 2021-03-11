package syntax

import (
	"reflect"
	"testing"
)

func TestScanner(t *testing.T) {
	tests := map[string]struct {
		wantTypes  []TokenType /* + implicit TokenEOF */
		wantValues []string
	}{
		"":                  {wantTypes: []TokenType{}},
		" ":                 {wantTypes: []TokenType{}},
		"\n":                {wantTypes: []TokenType{}},
		"a":                 {wantTypes: []TokenType{TokenLiteral}, wantValues: []string{"a"}},
		":":                 {wantTypes: []TokenType{TokenColon}, wantValues: []string{":"}},
		"-":                 {wantTypes: []TokenType{TokenMinus}, wantValues: []string{"-"}},
		"a:b":               {wantTypes: []TokenType{TokenLiteral, TokenColon, TokenLiteral}, wantValues: []string{"a", ":", "b"}},
		"a : b":             {wantTypes: []TokenType{TokenLiteral, TokenSep, TokenColon, TokenSep, TokenLiteral}, wantValues: []string{"a", " ", ":", " ", "b"}},
		"a: b":              {wantTypes: []TokenType{TokenLiteral, TokenColon, TokenSep, TokenLiteral}, wantValues: []string{"a", ":", " ", "b"}},
		`a:" b"`:            {wantTypes: []TokenType{TokenLiteral, TokenColon, TokenQuoted}, wantValues: []string{"a", ":", `" b"`}},
		"a :b":              {wantTypes: []TokenType{TokenLiteral, TokenSep, TokenColon, TokenLiteral}, wantValues: []string{"a", " ", ":", "b"}},
		"-a":                {wantTypes: []TokenType{TokenMinus, TokenLiteral}, wantValues: []string{"-", "a"}},
		"-a:b":              {wantTypes: []TokenType{TokenMinus, TokenLiteral, TokenColon, TokenLiteral}, wantValues: []string{"-", "a", ":", "b"}},
		"- a":               {wantTypes: []TokenType{TokenMinus, TokenSep, TokenLiteral}, wantValues: []string{"-", " ", "a"}},
		"- a:b":             {wantTypes: []TokenType{TokenMinus, TokenSep, TokenLiteral, TokenColon, TokenLiteral}, wantValues: []string{"-", " ", "a", ":", "b"}},
		"--a":               {wantTypes: []TokenType{TokenMinus, TokenMinus, TokenLiteral}, wantValues: []string{"-", "-", "a"}},
		"^a":                {wantTypes: []TokenType{TokenLiteral}, wantValues: []string{"^a"}},
		"^a .b":             {wantTypes: []TokenType{TokenLiteral, TokenSep, TokenLiteral}, wantValues: []string{"^a", " ", ".b"}},
		"a:b c:d":           {wantTypes: []TokenType{TokenLiteral, TokenColon, TokenLiteral, TokenSep, TokenLiteral, TokenColon, TokenLiteral}, wantValues: []string{"a", ":", "b", " ", "c", ":", "d"}},
		"a:b:c":             {wantTypes: []TokenType{TokenLiteral, TokenColon, TokenLiteral}, wantValues: []string{"a", ":", "b:c"}},
		`a:""`:              {wantTypes: []TokenType{TokenLiteral, TokenColon, TokenQuoted}},
		`a:"b"`:             {wantTypes: []TokenType{TokenLiteral, TokenColon, TokenQuoted}, wantValues: []string{"a", ":", `"b"`}},
		`a:'b'`:             {wantTypes: []TokenType{TokenLiteral, TokenColon, TokenQuoted}},
		`a:"b:c"`:           {wantTypes: []TokenType{TokenLiteral, TokenColon, TokenQuoted}},
		"a:'b:c'":           {wantTypes: []TokenType{TokenLiteral, TokenColon, TokenQuoted}},
		`a:b"c"`:            {wantTypes: []TokenType{TokenLiteral, TokenColon, TokenLiteral}},
		`"a"`:               {wantTypes: []TokenType{TokenQuoted}, wantValues: []string{`"a"`}},
		"'a'":               {wantTypes: []TokenType{TokenQuoted}, wantValues: []string{"'a'"}},
		`"a\"b"`:            {wantTypes: []TokenType{TokenQuoted}, wantValues: []string{`"a\"b"`}},
		`"a\\"`:             {wantTypes: []TokenType{TokenQuoted}, wantValues: []string{`"a\\"`}},
		`'a\'b'`:            {wantTypes: []TokenType{TokenQuoted}, wantValues: []string{`'a\'b'`}},
		`"\u0033"`:          {wantTypes: []TokenType{TokenQuoted}, wantValues: []string{`"\u0033"`}},
		`"\x21"`:            {wantTypes: []TokenType{TokenQuoted}, wantValues: []string{`"\x21"`}},
		`"a`:                {wantTypes: []TokenType{TokenError}},
		"'a":                {wantTypes: []TokenType{TokenError}},
		`"a\`:               {wantTypes: []TokenType{TokenError}},
		`a"`:                {wantTypes: []TokenType{TokenLiteral}, wantValues: []string{`a"`}},
		"a'":                {wantTypes: []TokenType{TokenLiteral}, wantValues: []string{"a'"}},
		`"a:b"`:             {wantTypes: []TokenType{TokenQuoted}, wantValues: []string{`"a:b"`}},
		`a"b`:               {wantTypes: []TokenType{TokenLiteral}, wantValues: []string{`a"b`}},
		`a:"b`:              {wantTypes: []TokenType{TokenLiteral, TokenColon, TokenError}, wantValues: []string{"a", ":", `unclosed quoted string`}},
		`a"b"c`:             {wantTypes: []TokenType{TokenLiteral}, wantValues: []string{`a"b"c`}},
		`a"b:c"d`:           {wantTypes: []TokenType{TokenLiteral}, wantValues: []string{`a"b:c"d`}},
		"/":                 {wantTypes: []TokenType{TokenPattern}, wantValues: []string{""}},
		"//":                {wantTypes: []TokenType{TokenPattern}, wantValues: []string{""}},
		"///":               {wantTypes: []TokenType{TokenPattern, TokenPattern}, wantValues: []string{"", ""}},
		"/a":                {wantTypes: []TokenType{TokenPattern}, wantValues: []string{"a"}},
		"-/a":               {wantTypes: []TokenType{TokenMinus, TokenPattern}, wantValues: []string{"-", "a"}},
		"a:/b":              {wantTypes: []TokenType{TokenLiteral, TokenColon, TokenLiteral}, wantValues: []string{"a", ":", "/b"}},
		`/a\`:               {wantTypes: []TokenType{TokenError}},
		`/a\/`:              {wantTypes: []TokenType{TokenPattern}, wantValues: []string{`a\/`}},
		`/a\\/`:             {wantTypes: []TokenType{TokenPattern}, wantValues: []string{`a\\`}},
		`/a\/b`:             {wantTypes: []TokenType{TokenPattern}, wantValues: []string{`a\/b`}},
		"/a/ b":             {wantTypes: []TokenType{TokenPattern, TokenSep, TokenLiteral}, wantValues: []string{"a", " ", "b"}},
		"a /b/ c":           {wantTypes: []TokenType{TokenLiteral, TokenSep, TokenPattern, TokenSep, TokenLiteral}, wantValues: []string{"a", " ", "b", " ", "c"}},
		"a /b c":            {wantTypes: []TokenType{TokenLiteral, TokenSep, TokenPattern}, wantValues: []string{"a", " ", "b c"}},
		"a /b c/":           {wantTypes: []TokenType{TokenLiteral, TokenSep, TokenPattern}, wantValues: []string{"a", " ", "b c"}},
		`foo\ bar baz`:      {wantTypes: []TokenType{TokenLiteral, TokenSep, TokenLiteral}, wantValues: []string{"foo\\ bar", " ", "baz"}},
		`\ foo\ bar\ baz\ `: {wantTypes: []TokenType{TokenLiteral}, wantValues: []string{"\\ foo\\ bar\\ baz\\ "}},
	}
	for input, test := range tests {
		t.Run(input, func(t *testing.T) {
			tokens := Scan(input)
			if len(tokens) > 0 && tokens[len(tokens)-1].Type != TokenError {
				test.wantTypes = append(test.wantTypes, TokenEOF)
				if test.wantValues != nil {
					test.wantValues = append(test.wantValues, "")
				}
			}
			if tokenTypes := tokenTypes(tokens); !reflect.DeepEqual(tokenTypes, test.wantTypes) {
				t.Errorf("token types: %s\ngot  %v\nwant %v", input, tokenTypes, test.wantTypes)
			}
			if test.wantValues != nil {
				if tokenValues := tokenValues(tokens); !reflect.DeepEqual(tokenValues, test.wantValues) {
					t.Errorf("token values: %s\ngot  %q\nwant %q", input, tokenValues, test.wantValues)
				}
			}
		})
	}
}

func tokenTypes(tokens []Token) []TokenType {
	types := make([]TokenType, len(tokens))
	for i, t := range tokens {
		types[i] = t.Type
	}
	return types
}

func tokenValues(tokens []Token) []string {
	values := make([]string, len(tokens))
	for i, t := range tokens {
		values[i] = t.Value
	}
	return values
}
