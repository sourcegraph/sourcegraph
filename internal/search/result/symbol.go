package result

import (
	"github.com/sourcegraph/sourcegraph/internal/gituri"
)

// Symbol is a code symbol.
type Symbol struct {
	Name       string
	Path       string
	Line       int
	Kind       string
	Language   string
	Parent     string
	ParentKind string
	Signature  string
	Pattern    string

	FileLimited bool
}

// Symbols is the result of a search on the symbols service.
type Symbols = []Symbol

// SymbolMatch is a symbol search result decorated with extra metadata in the frontend.
type SymbolMatch struct {
	Symbol  Symbol
	BaseURI *gituri.URI
	Lang    string
}

func (s *SymbolMatch) URI() *gituri.URI {
	return s.BaseURI.WithFilePath(s.Symbol.Path)
}
