package graphqlbackend

import (
	"strings"
)

// toSelectKind maps an internal symbol kind (cf. ctagsKind) to a corresponding
// symbol selector kind value in select.go. The single selector value `kind`
// corresponds 1-to-1 with LSP symbol kinds.
var toSelectKind = map[string]string{
	"file":            "file",
	"module":          "module",
	"namespace":       "namespace",
	"package":         "package",
	"packagename":     "package",
	"subprogspec":     "package",
	"class":           "class",
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

func pick(symbols []*SearchSymbolResult, satisfy func(*SearchSymbolResult) bool) []*SearchSymbolResult {
	var result []*SearchSymbolResult
	for _, symbol := range symbols {
		if satisfy(symbol) {
			result = append(result, symbol)
		}
	}
	return result
}

func SelectKind(symbols []*SearchSymbolResult, field string) []*SearchSymbolResult {
	return pick(symbols, func(s *SearchSymbolResult) bool {
		if field == toSelectKind[strings.ToLower(s.symbol.Kind)] {
			return true
		}
		return false
	})
}
