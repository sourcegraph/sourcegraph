package query

import (
	"fmt"
	"regexp"
)

type Predicate interface {
	// Field is the name of the field that the predicate applies to.
	// For example, with `file:contains()`, Field returns "file".
	Field() string

	// Name is the name of the predicate.
	// For example, with `file:contains()`, Name returns "contains".
	Name() string

	// String returns the query representation of the predicate
	String() string

	// UnmarshalText unmarshals the contents of the predicate arguments
	// into the predicate object.
	UnmarshalText([]byte) error

	// Query returns a Q that, when evaluated, returns a list of results
	// that can replace the predicate
	Query() Q
}

var DefaultPredicateRegistry = predicateRegistry{
	FieldFile: {
		"contains": func() Predicate {
			return &FileContainsPredicate{}
		},
	},
}

type predicateFactory func() Predicate

type predicateRegistry map[string]map[string]predicateFactory

func (pr predicateRegistry) Get(field, name, params string) (Predicate, error) {
	fieldPredicates, ok := pr[field]
	if !ok {
		return nil, fmt.Errorf("no predicates registered for field %s", field)
	}

	newPredicateFunc, ok := fieldPredicates[name]
	if !ok {
		return nil, fmt.Errorf("field '%s' has no predicate named '%s'", field, name)
	}

	predicate := newPredicateFunc()
	if err := predicate.UnmarshalText([]byte(params)); err != nil {
		return nil, fmt.Errorf("failed to parse params: %s", err)
	}
	return predicate, nil
}

var (
	predicateRegexp = regexp.MustCompile(`^(?P<name>[a-z]+)\((?P<params>.*)\)$`)
	nameIndex       = predicateRegexp.SubexpIndex("name")
	paramsIndex     = predicateRegexp.SubexpIndex("params")
)

func ParseAsPredicate(value string) (name, params string, err error) {
	match := predicateRegexp.FindStringSubmatch(value)
	if match == nil {
		return "", "", fmt.Errorf("value '%s' is not a predicate", value)
	}

	name = match[nameIndex]
	params = match[paramsIndex]
	return name, params, nil
}

// FileContainsPredicate represents the `file:contains(regexp)` predicate,
// which filters to files that contain a string literal
type FileContainsPredicate struct {
	Pattern string
}

func (f *FileContainsPredicate) UnmarshalText(text []byte) error {
	f.Pattern = string(text)
	return nil
}

// TODO (@camdencheek): should we have Field and PredicateName types, or is string alright?
func (f *FileContainsPredicate) Field() string { return FieldFile }
func (f *FileContainsPredicate) Name() string  { return "contains" }
func (f *FileContainsPredicate) String() string {
	return fmt.Sprintf("%s:%s(%s)", f.Field(), f.Name(), f.Pattern)
}
func (f *FileContainsPredicate) Query() Q {
	return []Node{
		Pattern{
			Value: f.Pattern,
			Annotation: Annotation{
				Labels: Regexp,
			},
		},
		Parameter{
			Field: "patterntype",
			Value: "regexp",
		},
		Parameter{
			Field: "select",
			Value: "file",
		},
		// TODO this runs into the same issues that AND queries do, in that we can't be sure
		// that we're getting enough to get results.
		Parameter{
			Field: "count",
			Value: "10000",
		},
	}
}

type PredicateWithRepos struct {
	Predicate
	RepoNodes []Node
}

func (p PredicateWithRepos) Query() Q {
	return append(p.RepoNodes, p.Predicate.Query()...)
}
