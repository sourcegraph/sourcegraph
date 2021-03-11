package filter

import (
	"fmt"
	"strings"
)

type SelectType string

const (
	Commit     SelectType = "commit"
	Content    SelectType = "content"
	File       SelectType = "file"
	Repository SelectType = "repo"
	Symbol     SelectType = "symbol"
)

// SelectPath represents a parsed and validated select value and fields.
type SelectPath struct {
	Type   SelectType
	Fields []string
}

func (sp SelectPath) String() string {
	return string(sp.Type)
}

var validSelectors = map[SelectType]map[string]interface{}{
	Commit:     {},
	Content:    {},
	File:       {},
	Repository: {},
	Symbol: {
		/* cf. SymbolKind https://microsoft.github.io/language-server-protocol/specification */
		"file":           struct{}{},
		"module":         struct{}{},
		"namespace":      struct{}{},
		"package":        struct{}{},
		"class":          struct{}{},
		"method":         struct{}{},
		"property":       struct{}{},
		"field":          struct{}{},
		"constructor":    struct{}{},
		"enum":           struct{}{},
		"interface":      struct{}{},
		"function":       struct{}{},
		"variable":       struct{}{},
		"constant":       struct{}{},
		"string":         struct{}{},
		"number":         struct{}{},
		"boolean":        struct{}{},
		"array":          struct{}{},
		"object":         struct{}{},
		"key":            struct{}{},
		"null":           struct{}{},
		"enum-member":    struct{}{},
		"struct":         struct{}{},
		"event":          struct{}{},
		"operator":       struct{}{},
		"type-parameter": struct{}{},
	},
}

func splitFields(s string) (string, []string) {
	v := strings.Split(s, ".")
	return v[0], v[1:]
}

func SelectPathFromString(s string) (SelectPath, error) {
	selector, fields := splitFields(s)
	if _, ok := validSelectors[SelectType(selector)]; !ok {
		return SelectPath{}, fmt.Errorf("invalid select type '%s'", s)
	}
	if len(fields) > 0 {
		if _, ok := validSelectors[SelectType(selector)][fields[0]]; !ok {
			return SelectPath{}, fmt.Errorf("invalid field '%s' on select type '%s'", fields[0], selector)
		}
	}
	return SelectPath{Type: SelectType(selector), Fields: fields}, nil
}
