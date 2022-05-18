package query

var empty = struct{}{}

// All field names.
const (
	FieldDefault            = ""
	FieldCase               = "case"
	FieldRepo               = "repo"
	FieldFile               = "file"
	FieldFork               = "fork"
	FieldArchived           = "archived"
	FieldLang               = "lang"
	FieldType               = "type"
	FieldRepoHasFile        = "repohasfile"
	FieldRepoHasCommitAfter = "repohascommitafter"
	FieldPatternType        = "patterntype"
	FieldContent            = "content"
	FieldVisibility         = "visibility"
	FieldRev                = "rev"
	FieldContext            = "context"

	// For diff and commit search only:
	FieldBefore    = "before"
	FieldAfter     = "after"
	FieldAuthor    = "author"
	FieldCommitter = "committer"
	FieldMessage   = "message"

	// Temporary experimental fields:
	FieldIndex     = "index"
	FieldCount     = "count" // Searches that specify `count:` will fetch at least that number of results, or the full result set
	FieldTimeout   = "timeout"
	FieldCombyRule = "rule"
	FieldSelect    = "select"
	FieldSymbolAt  = "symbolat"
)

var allFields = map[string]struct{}{
	FieldCase:               empty,
	FieldRepo:               empty,
	"r":                     empty,
	FieldContext:            empty,
	"g":                     empty,
	FieldFile:               empty,
	"f":                     empty,
	"path":                  empty,
	FieldFork:               empty,
	FieldArchived:           empty,
	FieldLang:               empty,
	"l":                     empty,
	"language":              empty,
	FieldType:               empty,
	FieldPatternType:        empty,
	FieldContent:            empty,
	FieldVisibility:         empty,
	FieldRepoHasFile:        empty,
	FieldRepoHasCommitAfter: empty,
	FieldBefore:             empty,
	"until":                 empty,
	FieldAfter:              empty,
	"since":                 empty,
	FieldAuthor:             empty,
	FieldCommitter:          empty,
	FieldMessage:            empty,
	"m":                     empty,
	"msg":                   empty,
	FieldIndex:              empty,
	FieldCount:              empty,
	FieldTimeout:            empty,
	FieldCombyRule:          empty,
	FieldRev:                empty,
	"revision":              empty,
	FieldSelect:             empty,
	FieldSymbolAt:           empty,
}

var aliases = map[string]string{
	"r":        FieldRepo,
	"f":        FieldFile,
	"path":     FieldFile,
	"l":        FieldLang,
	"language": FieldLang,
	"since":    FieldAfter,
	"until":    FieldBefore,
	"m":        FieldMessage,
	"msg":      FieldMessage,
	"revision": FieldRev,
}

// resolveFieldAlias resolves an aliased field like `r:` to its canonical name
// like `repo:`.
func resolveFieldAlias(field string) string {
	if canonical, ok := aliases[field]; ok {
		return canonical
	}
	return field
}
