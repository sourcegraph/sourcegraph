package result

import (
	"sort"
	"strings"
)

// Types represents a set of result types.
// It's a bitset corresponding to the disjunction of types it represents.
//
// For example, the set of file and repo results
// is represented as Types(TypeFile|TypeRepo)
type Types uint8

const (
	TypeEmpty Types = 0
	TypeRepo  Types = 1 << (iota - 1)
	TypeSymbol
	TypeFile
	TypePath
	TypeDiff
	TypeCommit
	TypeStructural
)

var TypeFromString = map[string]Types{
	"repo":       TypeRepo,
	"symbol":     TypeSymbol,
	"file":       TypeFile,
	"path":       TypePath,
	"diff":       TypeDiff,
	"commit":     TypeCommit,
	"structural": TypeStructural,
}

func (r Types) Has(t Types) bool {
	return r&t != 0
}

func (r Types) With(t Types) Types {
	return r | t
}

func (r Types) Without(t Types) Types {
	return r &^ t
}

func (r Types) String() string {
	var names []string
	for name, t := range TypeFromString {
		if !r.Has(t) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
