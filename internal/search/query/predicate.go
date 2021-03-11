package query

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type Predicate interface {
	// Field is the name of the field that the predicate applies to.
	// For example, with `file:contains()`, Field returns "file".
	Field() string

	// Name is the name of the predicate.
	// For example, with `file:contains()`, Name returns "contains".
	Name() string

	// UnmarshalParams unmarshals the contents of the predicate arguments
	// into the predicate object.
	UnmarshalParams(string) error

	// Query returns a Q that, when evaluated, returns a list of results
	// that can replace the predicate
	Query(parent Q) Q
}

var DefaultPredicateRegistry = predicateRegistry{
	FieldRepo: {
		"contains": func() Predicate {
			return &RepoContainsPredicate{}
		},
	},
}

type predicateRegistry map[string]map[string]func() Predicate

// Get returns a predicate for the given field with the given name. If no such predicate
// exists, or the params provided are invalid, it returns an error.
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
	if err := predicate.UnmarshalParams(params); err != nil {
		return nil, fmt.Errorf("failed to parse params: %s", err)
	}
	return predicate, nil
}

var (
	predicateRegexp = regexp.MustCompile(`^(?P<name>[a-z]+)\((?P<params>.*)\)$`)
	nameIndex       = predicateRegexp.SubexpIndex("name")
	paramsIndex     = predicateRegexp.SubexpIndex("params")
)

// ParseAsPredicate attempts to parse a value as a predicate. It does not validate
// that the parsed predicate is a defined predicate.
func ParseAsPredicate(value string) (name, params string, err error) {
	match := predicateRegexp.FindStringSubmatch(value)
	if match == nil {
		return "", "", fmt.Errorf("value '%s' is not a predicate", value)
	}

	name = match[nameIndex]
	params = match[paramsIndex]
	return name, params, nil
}

// RepoContainsPredicate represents the `repo:contains()` predicate,
// which filters to repos that contain either a file or content
type RepoContainsPredicate struct {
	File    string
	Content string
}

func (f *RepoContainsPredicate) UnmarshalParams(params string) error {
	nodes, err := ParseAndOr(params, SearchTypeRegex)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		switch v := node.(type) {
		case Parameter:
			switch strings.ToLower(v.Field) {
			case "file":
				if f.File != "" {
					return errors.New("cannot specify file multiple times")
				}
				f.File = v.Value
			case "content":
				if f.Content != "" {
					return errors.New("cannot specify content multiple times")
				}
				f.Content = v.Value
			default:
				return fmt.Errorf("unsupported option %q", v.Field)
			}
		case Pattern:
			if f.Content != "" {
				return errors.New("cannot specify content multiple times")
			}
			f.Content = v.Value
		default:
			return fmt.Errorf("unsupported node type %T", node)
		}
	}

	if f.File == "" && f.Content == "" {
		return errors.New("one of file or content must be set")
	}

	return nil
}

func (f *RepoContainsPredicate) Field() string { return FieldRepo }
func (f *RepoContainsPredicate) Name() string  { return "contains" }
func (f *RepoContainsPredicate) Query(parent Q) Q {
	nodes := make([]Node, 0, 3)
	nodes = append(nodes, Parameter{
		Field: FieldSelect,
		Value: "repo",
	}, Parameter{
		Field: FieldCount,
		Value: "99999",
	})

	if f.File != "" {
		nodes = append(nodes, Parameter{
			Field: FieldFile,
			Value: f.File,
		})
	}

	if f.Content != "" {
		nodes = append(nodes, Pattern{
			Value: f.Content,
		})
	}

	nodes = append(nodes, nonPredicateRepos(parent)...)
	return nodes
}

// nonPredicateRepos returns the repo nodes in a query that aren't predicates
func nonPredicateRepos(q Q) []Node {
	var res []Node
	VisitField(q, FieldRepo, func(value string, negated bool, ann Annotation) {
		if _, _, err := ParseAsPredicate(value); err == nil {
			// Skip predicates
			return
		}

		res = append(res, Parameter{
			Field:      FieldRepo,
			Value:      value,
			Negated:    negated,
			Annotation: ann,
		})
	})
	return res
}
