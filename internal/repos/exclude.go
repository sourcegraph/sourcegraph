package repos

import (
	"strconv"
	"strings"

	"github.com/grafana/regexp"

	"github.com/sourcegraph/sourcegraph/internal/bytesize"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// excludeFunc takes either a generic object and returns true if the repo should be excluded. In
// the case of repo sourcing it will take a repository name, ID, or the repo itself as input.
type excludeFunc func(input any) bool

// repoExcluder is made up of several rules that can be chained together to
// exclude a repository.
//
// Rules can be added by calling `repoExcluder.AddRule()`.
//
// After rules have been added, the caller uses `ShouldExclude` to check if a
// repo should be excluded. In that call, all the rules are OR'd together. If
// one rule excludes, the repo is excluded.
type repoExcluder struct {
	exactRules map[string]struct{}
	rules      []*rule
}

func (e *repoExcluder) ShouldExclude(input any) bool {
	if inputString, ok := input.(string); ok {
		if _, exists := e.exactRules[strings.ToLower(inputString)]; exists {
			return true
		}
	}

	for _, r := range e.rules {
		if r.Excludes(input) {
			return true
		}
	}

	return false
}

func (e *repoExcluder) AddRule(r *rule) {
	// Optimization: For rules that only have exact matches, we inline them
	// into one map for faster lookups.
	if len(r.exact) > 0 && len(r.patterns) == 0 && len(r.generic) == 0 {
		if e.exactRules == nil {
			e.exactRules = make(map[string]struct{})
		}
		for _, exact := range r.exact {
			e.exactRules[exact] = struct{}{}
		}
		return
	}
	e.rules = append(e.rules, r)
}

func NewRule() *rule {
	return &rule{}
}

func (e *repoExcluder) RuleErrors() error {
	var err errors.MultiError
	for _, r := range e.rules {
		err = errors.Append(err, r.err)
	}
	return err
}

// rule represents a single exclusion, whose conditions must all be bet in
// order to exclude a repository.
type rule struct {
	exact    []string
	patterns []*regexp.Regexp
	generic  []excludeFunc

	// err can be set during construction if any patterns failed to compile.
	err error
}

// Exact will keep track of the exact value passed in and then match it against
// the input of the `Excludes` method.
//
// If the input is an empty string, it will be ignored.
// Multiple calls to exact will be OR'd together.
func (r *rule) Exact(name string) *rule {
	if name == "" {
		return r
	}
	r.exact = append(r.exact, strings.ToLower(name))
	return r
}

// Pattern will exclude strings matching the regex pattern.
func (r *rule) Pattern(pattern string) *rule {
	if pattern == "" {
		return r
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		r.err = err
		return r
	}
	r.patterns = append(r.patterns, re)
	return r
}

// Generic registers the passed in exclude function that will be used to determine whether a repo
// should be excluded.
func (r *rule) Generic(ef excludeFunc) *rule {
	if ef == nil {
		return r
	}
	r.generic = append(r.generic, ef)
	return r
}

// Excludes returns true if the input matches ALL of the previously set
// attributes of the rule.
func (r *rule) Excludes(input any) bool {
	exclude := false

	if inputString, ok := input.(string); ok {
		// If any of the exacts match, that's a match.
		for _, exact := range r.exact {
			if exact == strings.ToLower(inputString) {
				exclude = true
			}
		}
		// Otherwise not all conditions have been met yet, return false.
		if len(r.exact) > 0 && !exclude {
			return false
		}

		for _, re := range r.patterns {
			exclude = re.MatchString(inputString)
			if !exclude {
				return false
			}
		}
	}

	for _, ef := range r.generic {
		exclude = ef(input)
		if !exclude {
			return false
		}
	}

	return exclude
}

var starsConstraintRegex = regexp.MustCompile(`([<>=]{1,2})\s*(\d+)`)

func buildStarsConstraintsExcludeFn(constraint string) (excludeFunc, error) {
	matches := starsConstraintRegex.FindStringSubmatch(constraint)
	if matches == nil {
		return nil, errors.Newf("invalid stars constraint format: %q", constraint)
	}

	operator, err := newOperator(matches[1])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to evaluate stars constraint")
	}

	count, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, err
	}

	return func(input any) bool {
		r, ok := input.(github.Repository)
		if !ok {
			return false
		}
		return operator.Eval(r.StargazerCount, count)
	}, nil
}

var sizeConstraintRegex = regexp.MustCompile(`([<>=]{1,2})\s*(\d+\s*\w+)`)

func buildSizeConstraintsExcludeFn(constraint string) (excludeFunc, error) {
	sizeMatch := sizeConstraintRegex.FindStringSubmatch(constraint)
	if sizeMatch == nil {
		return nil, errors.Newf("invalid size constraint format: %q", constraint)
	}

	operator, err := newOperator(sizeMatch[1])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to evaluate size constraint")
	}

	size, err := bytesize.Parse(sizeMatch[2])
	if err != nil {
		return nil, err
	}

	return func(input any) bool {
		r, ok := input.(github.Repository)
		if !ok {
			return false
		}

		repoSize := int(r.SizeBytes())

		// If we don't have a repository size, we don't exclude
		if repoSize == 0 {
			return false
		}

		return operator.Eval(repoSize, int(size))
	}, nil
}

type operator string

const (
	opLess           operator = "<"
	opLessOrEqual    operator = "<="
	opGreater        operator = ">"
	opGreaterOrEqual operator = ">="
)

func newOperator(input string) (operator, error) {
	if input != "<" && input != "<=" && input != ">" && input != ">=" {
		return "", errors.Newf("invalid operator %q", input)
	}
	return operator(input), nil
}

func (o operator) Eval(left, right int) bool {
	switch o {
	case "<":
		return left < right
	case ">":
		return left > right
	case "<=":
		return left <= right
	case ">=":
		return left >= right
	default:
		// I wish Go had enums
		panic(errors.Newf("unknown operator: %q", o))
	}
}
