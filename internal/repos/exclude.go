package repos

import (
	"strings"

	"github.com/grafana/regexp"
)

// excludeFunc takes a string and returns true if it should be excluded. In
// the case of repo sourcing it will take a repository name or ID as input.
type excludeFunc func(input any) bool

// excludeBuilder builds an excludeFunc.
type excludeBuilder struct {
	exact    map[string]struct{}
	patterns []*regexp.Regexp
	generic  []func(repo interface{}) bool
	err      error
}

// Exact will case-insensitively exclude the string name.
func (e *excludeBuilder) Exact(name string) {
	if e.exact == nil {
		e.exact = map[string]struct{}{}
	}
	if name == "" {
		return
	}
	e.exact[strings.ToLower(name)] = struct{}{}
}

// Pattern will exclude strings matching the regex pattern.
func (e *excludeBuilder) Pattern(pattern string) {
	if pattern == "" {
		return
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		e.err = err
		return
	}
	e.patterns = append(e.patterns, re)
}

// Generic registers the passed in generic exclude function that will be used to determine whether a repo
// should be excluded.
func (e *excludeBuilder) Generic(excludeFunc func(repo interface{}) bool) {
	if excludeFunc == nil {
		return
	}

	if e.generic == nil {
		e.generic = []func(interface{}) bool{}
	}

	e.generic = append(e.generic, excludeFunc)
}

// Build will return an excludeFunc based on the previous calls to Exact, Pattern, and
// Generic.
func (e *excludeBuilder) Build() (excludeFunc, error) {
	return func(input any) bool {
		if inputString, ok := input.(string); ok {
			if _, ok := e.exact[strings.ToLower(inputString)]; ok {
				return true
			}

			for _, re := range e.patterns {
				if re.MatchString(inputString) {
					return true
				}
			}
		} else {
			for _, excludeFunc := range e.generic {
				if excludeFunc(input) {
					return true
				}
			}
		}

		return false
	}, e.err
}
