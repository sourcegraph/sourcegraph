package search

import (
	"strings"

	"github.com/grafana/regexp"

	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/searcher/protocol"
)

type pathMatcher struct {
	Include []*regexp.Regexp
	Exclude *regexp.Regexp
}

func (pm *pathMatcher) Matches(path string) bool {
	for _, re := range pm.Include {
		if !re.MatchString(path) {
			return false
		}
	}
	return pm.Exclude == nil || !pm.Exclude.MatchString(path)
}

func (pm *pathMatcher) String() string {
	var parts []string
	for _, re := range pm.Include {
		parts = append(parts, re.String())
	}
	if pm.Exclude != nil {
		parts = append(parts, "!"+pm.Exclude.String())
	}
	return strings.Join(parts, " ")
}

// toPathMatcher returns a pathMatcher that matches a path iff:
// * all the includePatterns match the path; AND
// * the excludePattern does NOT match the path.
func toPathMatcher(p *protocol.PatternInfo) (*pathMatcher, error) {
	// set err once if non-nil. This simplifies our many calls to compilePattern.
	var err error
	compile := func(pattern string) *regexp.Regexp {
		if !p.PathPatternsAreCaseSensitive {
			// Respect the CaseSensitive option. However, if the pattern already contains
			// (?i:...), then don't clear that 'i' flag (because we assume that behavior
			// is desirable in more cases).
			pattern = "(?i:" + pattern + ")"
		}
		re, innerErr := regexp.Compile(pattern)
		if innerErr != nil {
			err = innerErr
		}
		return re
	}

	var include []*regexp.Regexp
	for _, pattern := range p.IncludePaths {
		include = append(include, compile(pattern))
	}

	// As an optimization, add the language filters as path patterns since they're
	// faster to check than calling go-enry. This is not necessary for correctness.
	for _, lang := range p.IncludeLangs {
		pattern := query.LangToFileRegexp(lang)
		include = append(include, compile(pattern))
	}

	var exclude *regexp.Regexp
	if p.ExcludePaths != "" {
		exclude = compile(p.ExcludePaths)
	}

	return &pathMatcher{
		Include: include,
		Exclude: exclude,
	}, err
}
