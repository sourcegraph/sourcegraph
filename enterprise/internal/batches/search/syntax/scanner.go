package syntax

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// TokenType is the set of lexical tokens in the query syntax.
type TokenType int

//go:generate env GOBIN=$PWD/.bin GO111MODULE=on go install golang.org/x/tools/cmd/stringer
//go:generate env $PWD/.bin/stringer -type=TokenType

// All TokenType values.
const (
	TokenEOF TokenType = iota
	TokenError
	TokenLiteral
	TokenQuoted
	TokenPattern
	TokenColon
	TokenMinus
	TokenSep // separator (like a semicolon)
)

var singleCharTokens = map[rune]TokenType{
	':': TokenColon,
	'-': TokenMinus,
}

// Token is a token in a query.
type Token struct {
	Type  TokenType // type of token
	Value string    // string value
	Pos   int       // starting character position
}

// Scan scans the query and returns a list of tokens.
func Scan(input string) []Token {
	s := &scanner{input: input}

	for state := scanDefault; state != nil; {
		state = state(s)
	}
	return s.tokens
}

type stateFn func(*scanner) stateFn

type scanner struct {
	input   string
	tokens  []Token
	pos     int
	prevPos int
	start   int
}

func (s *scanner) next() rune {
	s.prevPos = s.pos
	if s.eof() {
		// All callers of (*scanner).next should check for EOF first.
		panic("eof")
	}
	r, w := utf8.DecodeRuneInString(s.input[s.pos:])
	s.pos += w
	return r
}

func (s *scanner) eof() bool { return s.pos >= len(s.input) }

func (s *scanner) ignore() { s.start = s.pos }

func (s *scanner) backup() { s.pos = s.prevPos }

func (s *scanner) peek() rune {
	r := s.next()
	s.backup()
	return r
}

func (s *scanner) emit(typ TokenType) {
	s.tokens = append(s.tokens, Token{
		Type:  typ,
		Value: s.input[s.start:s.pos],
		Pos:   s.start,
	})
	s.start = s.pos
}

func (s *scanner) emitError(msg string) {
	s.tokens = append(s.tokens, Token{
		Type:  TokenError,
		Value: msg,
		Pos:   s.start,
	})
	s.start = s.pos
}

func scanDefault(s *scanner) stateFn {
	if s.eof() {
		s.emit(TokenEOF)
		return nil
	}
	r := s.next()
	if !unicode.IsSpace(r) {
		s.backup()
		s.ignore()
		if typ, ok := singleCharTokens[r]; ok {
			s.next()
			s.emit(typ)
			return scanDefault
		}

		if r == '"' || r == '\'' {
			return scanQuoted
		}
		if r == '/' {
			return scanPattern
		}

		return scanText
	}
	return scanSpace
}

func scanText(s *scanner) stateFn {
	// Characters that may come before a ':' (TokenColon) in a TokenLiteral.
	preColonChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	escaped := false
	for {
		if s.eof() {
			break
		}
		r := s.next()
		if !escaped {
			if r == '\\' {
				escaped = true
				continue
			}

			if unicode.IsSpace(r) {
				s.backup()
				break
			}
		}
		escaped = false
		if r == ':' {
			// Start of value.
			s.backup()
			s.emit(TokenLiteral)
			s.next()
			s.emit(TokenColon)
			return scanValue
		}
		if !strings.ContainsRune(preColonChars, r) {
			return scanLiteral
		}
	}

	s.emit(TokenLiteral)
	return scanDefault
}

func scanValue(s *scanner) stateFn {
	if s.eof() {
		return scanDefault
	}
	r := s.peek()
	if unicode.IsSpace(r) {
		return scanDefault
	}
	if r == '"' || r == '\'' {
		return scanQuoted
	}
	return scanLiteral
}

func scanLiteral(s *scanner) stateFn {
	escaped := false
	for {
		if s.eof() {
			break
		}
		r := s.next()
		if !escaped {
			if r == '\\' {
				escaped = true
				continue
			}

			if unicode.IsSpace(r) {
				s.backup()
				break
			}
		}
		escaped = false
	}

	s.emit(TokenLiteral)
	return scanDefault
}

func scanQuoted(s *scanner) stateFn {
	q := s.next()
	escaped := false
	for {
		if s.eof() {
			if escaped {
				s.emitError("unterminated escape sequence")
			} else {
				s.emitError("unclosed quoted string")
			}
			return nil
		}
		r := s.next()
		if !escaped {
			if r == '\\' {
				escaped = true
				continue
			}
			if r == q {
				break
			}
		}
		escaped = false
	}
	s.emit(TokenQuoted)
	return scanDefault
}

func scanPattern(s *scanner) stateFn {
	slash := s.next()
	s.ignore()
	escaped := false
	for {
		if s.eof() {
			break
		}
		r := s.next()
		if !escaped {
			if r == '\\' {
				escaped = true
				continue
			}
			if r == slash {
				s.backup()
				defer s.ignore()
				defer s.next()
				break
			}
		}
		escaped = false
	}
	if escaped {
		s.emitError("unterminated escape sequence")
		return nil
	}
	s.emit(TokenPattern)
	return scanDefault
}

func scanSpace(s *scanner) stateFn {
	for {
		if s.eof() {
			return nil
		}
		r := s.next()
		if !unicode.IsSpace(r) {
			s.backup()
			break
		}
	}
	s.emit(TokenSep)
	return scanDefault
}
