package result

import (
	"cmp"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/sourcegraph/go-lsp"
)

// Symbol is a code symbol.
type Symbol struct {
	Name string

	// TODO (@camdencheek): remove path since it's duplicated
	// in the file reference of symbol match. Alternatively,
	// merge Symbol and SymbolMatch.
	Path       string
	Line       int
	Character  int
	Kind       string
	Language   string
	Parent     string
	ParentKind string
	Signature  string

	FileLimited bool
}

// NewSymbolMatch returns a new SymbolMatch. Passing -1 as the character will make NewSymbolMatch infer
// the column from the line and symbol name.
func NewSymbolMatch(file *File, lineNumber, character int, name, kind, parent, parentKind, language, line string, fileLimited bool) *SymbolMatch {
	if character == -1 {
		// The caller is requesting we infer the character position.
		character = strings.Index(line, name)

		if character == -1 {
			// We couldn't find the symbol in the line, so set the column to 0. ctags doesn't always
			// return the right line.
			character = 0
		}
	}

	return &SymbolMatch{
		Symbol: Symbol{
			Name:        name,
			Kind:        kind,
			Parent:      parent,
			ParentKind:  parentKind,
			Path:        file.Path,
			Line:        lineNumber,
			Character:   character,
			Language:    language,
			FileLimited: fileLimited,
		},
		File: file,
	}
}

func (s Symbol) LSPKind() lsp.SymbolKind {
	// Ctags kinds are determined by the parser and do not (in general) match LSP symbol kinds.
	switch strings.ToLower(s.Kind) {
	case "file":
		return lsp.SKFile
	case "module":
		return lsp.SKModule
	case "namespace":
		return lsp.SKNamespace
	case "package", "packagename", "subprogspec":
		return lsp.SKPackage
	case "class", "classes", "type", "service", "typedef", "union", "section", "subtype", "component":
		return lsp.SKClass
	case "method", "methodspec":
		return lsp.SKMethod
	case "property":
		return lsp.SKProperty
	case "field", "member", "anonmember", "recordfield":
		return lsp.SKField
	case "constructor":
		return lsp.SKConstructor
	case "enum", "enumerator":
		return lsp.SKEnum
	case "interface":
		return lsp.SKInterface
	case "function", "func", "subroutine", "macro", "subprogram", "procedure", "command", "singletonmethod":
		return lsp.SKFunction
	case "variable", "var", "functionvar", "define", "alias", "val":
		return lsp.SKVariable
	case "constant", "const":
		return lsp.SKConstant
	case "string", "message", "heredoc":
		return lsp.SKString
	case "number":
		return lsp.SKNumber
	case "bool", "boolean":
		return lsp.SKBoolean
	case "array":
		return lsp.SKArray
	case "object", "literal", "map":
		return lsp.SKObject
	case "key", "label", "target", "selector", "id", "tag":
		return lsp.SKKey
	case "null":
		return lsp.SKNull
	case "enum member", "enumconstant":
		return lsp.SKEnumMember
	case "struct":
		return lsp.SKStruct
	case "event":
		return lsp.SKEvent
	case "operator":
		return lsp.SKOperator
	case "type parameter", "annotation":
		return lsp.SKTypeParameter
	}
	return 0
}

func (s Symbol) Range() lsp.Range {
	// TODO(keegancsmith) For results from zoekt s.Character is not the start
	// of the symbol, but the start of the match. So doing s.Character +
	// len(s.Name) is incorrect.
	return lsp.Range{
		Start: lsp.Position{Line: s.Line - 1, Character: s.Character},
		End:   lsp.Position{Line: s.Line - 1, Character: s.Character + len(s.Name)},
	}
}

// Symbols is the result of a search on the symbols service.
type Symbols = []Symbol

// SymbolMatch is a symbol search result decorated with extra metadata in the frontend.
type SymbolMatch struct {
	Symbol Symbol
	File   *File
}

func (s *SymbolMatch) URL() *url.URL {
	base := s.File.URL()
	base.RawQuery = urlFragmentFromRange(s.Symbol.Range())
	return base
}

func compareSymbolMatches(a, b *SymbolMatch) int {
	if v := cmp.Compare(a.Symbol.Line, b.Symbol.Line); v != 0 {
		return v
	}

	if v := cmp.Compare(a.Symbol.Name, b.Symbol.Name); v != 0 {
		return v
	}

	if v := cmp.Compare(a.Symbol.Kind, b.Symbol.Kind); v != 0 {
		return v
	}

	if v := cmp.Compare(a.Symbol.Parent, b.Symbol.Parent); v != 0 {
		return v
	}

	return cmp.Compare(a.Symbol.ParentKind, b.Symbol.ParentKind)
}

// DedupSymbols removes duplicate symbols from the list. We use a heuristic to
// determine duplicate matches. We regard a match as the same if they have the
// same symbol info and appear on the same line. I am unaware of a language
// where you can break that assumption.
func DedupSymbols(symbols []*SymbolMatch) []*SymbolMatch {
	if len(symbols) <= 1 {
		return symbols
	}
	dedup := symbols[:1]
	for _, sym := range symbols[1:] {
		last := dedup[len(dedup)-1]
		if compareSymbolMatches(sym, last) == 0 {
			continue
		}
		dedup = append(dedup, sym)
	}
	return dedup
}

func urlFragmentFromRange(lspRange lsp.Range) string {
	if lspRange.Start == lspRange.End {
		return "L" + lineSpecFromPosition(lspRange.Start, false)
	}

	hasCharacter := lspRange.Start.Character != 0 || lspRange.End.Character != 0
	return "L" + lineSpecFromPosition(lspRange.Start, hasCharacter) + "-" + lineSpecFromPosition(lspRange.End, hasCharacter)
}

func lineSpecFromPosition(pos lsp.Position, forceIncludeCharacter bool) string {
	if !forceIncludeCharacter && pos.Character == 0 {
		return strconv.Itoa(pos.Line + 1)
	}
	return fmt.Sprintf("%d:%d", pos.Line+1, pos.Character+1)
}

// toSelectKind maps an internal symbol kind (cf. ctagsKind) to a corresponding
// symbol selector kind value in select.go. The single selector value `kind`
// corresponds 1-to-1 with LSP symbol kinds.
var ToSelectKind = map[string]string{
	"file":            "file",
	"module":          "module",
	"namespace":       "namespace",
	"package":         "package",
	"packagename":     "package",
	"subprogspec":     "package",
	"class":           "class",
	"classes":         "class",
	"type":            "class",
	"service":         "class",
	"typedef":         "class",
	"union":           "class",
	"section":         "class",
	"subtype":         "class",
	"component":       "class",
	"method":          "method",
	"methodspec":      "method",
	"property":        "property",
	"field":           "field",
	"member":          "field",
	"anonmember":      "field",
	"recordfield":     "field",
	"constructor":     "constructor",
	"interface":       "interface",
	"function":        "function",
	"func":            "function",
	"subroutine":      "function",
	"macro":           "function",
	"subprogram":      "function",
	"procedure":       "function",
	"command":         "function",
	"singletonmethod": "function",
	"variable":        "variable",
	"var":             "variable",
	"functionvar":     "variable",
	"define":          "variable",
	"alias":           "variable",
	"val":             "variable",
	"constant":        "constant",
	"const":           "constant",
	"string":          "string",
	"message":         "string",
	"heredoc":         "string",
	"number":          "number",
	"boolean":         "boolean",
	"bool":            "boolean",
	"array":           "array",
	"object":          "object",
	"literal":         "object",
	"map":             "object",
	"key":             "key",
	"label":           "key",
	"target":          "key",
	"selector":        "key",
	"id":              "key",
	"tag":             "key",
	"null":            "null",
	"enum member":     "enum-member",
	"enumconstant":    "enum-member",
	"struct":          "struct",
	"event":           "event",
	"operator":        "operator",
	"type parameter":  "type-parameter",
	"annotation":      "type-parameter",
}

func pick(symbols []*SymbolMatch, satisfy func(*SymbolMatch) bool) []*SymbolMatch {
	var result []*SymbolMatch
	for _, symbol := range symbols {
		if satisfy(symbol) {
			result = append(result, symbol)
		}
	}
	return result
}

func SelectSymbolKind(symbols []*SymbolMatch, field string) []*SymbolMatch {
	return pick(symbols, func(s *SymbolMatch) bool {
		return field == ToSelectKind[strings.ToLower(s.Symbol.Kind)]
	})
}
