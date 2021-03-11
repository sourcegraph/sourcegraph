package routevar

import "github.com/sourcegraph/sourcegraph/internal/lazyregexp"

// namedToNonCapturingGroups converts named capturing groups
// `(?P<myname>...)` to non-capturing groups `(?:...)` for use in mux
// route declarations (which assume that the route patterns do not
// have any capturing groups).
func namedToNonCapturingGroups(pat string) string {
	return namedCaptureGroup.ReplaceAllLiteralString(pat, `(?:`)
}

// namedCaptureGroup matches the syntax for the opening of a regexp
// named capture group (`(?P<name>`).
var namedCaptureGroup = lazyregexp.New(`\(\?P<[^>]+>`)
