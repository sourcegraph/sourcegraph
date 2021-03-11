package query

var empty = struct{}{}

// All field names.
const (
	FieldDefault            = ""
	FieldCase               = "case"
	FieldRepo               = "repo"
	FieldRepoGroup          = "repogroup"
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
	FieldCount     = "count"  // Searches that specify `count:` will fetch at least that number of results, or the full result set
	FieldStable    = "stable" // Forces search to return a stable result ordering (currently limited to file content matches).
	FieldTimeout   = "timeout"
	FieldCombyRule = "rule"
	FieldSelect    = "select"
)

var allFields = map[string]struct{}{
	FieldCase:               empty,
	FieldRepo:               empty,
	"r":                     empty,
	FieldRepoGroup:          empty,
	FieldContext:            empty,
	"g":                     empty,
	FieldFile:               empty,
	"f":                     empty,
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
	FieldStable:             empty,
	FieldTimeout:            empty,
	FieldCombyRule:          empty,
	FieldRev:                empty,
	"revision":              empty,
	FieldSelect:             empty,
}
