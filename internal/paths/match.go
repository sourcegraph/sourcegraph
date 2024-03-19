package paths

import (
	"strings"

	"github.com/becheran/wildmatch-go"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const separator = "/"

// patternPart implements matching for a single chunk of a glob pattern
// when separated by `/`.
type patternPart interface {
	String() string
	// Match is true if given file or directory name on the path matches
	// this part of the glob pattern.
	Match(string) bool
}

// anySubPath is indicated by ** in glob patterns, and matches arbitrary
// number of parts.
type anySubPath struct{}

func (p anySubPath) String() string      { return "**" }
func (p anySubPath) Match(_ string) bool { return true }

// exactMatch is indicated by an exact name of directory or a file within
// the glob pattern, and matches that exact part of the path only.
type exactMatch string

func (p exactMatch) String() string         { return string(p) }
func (p exactMatch) Match(part string) bool { return string(p) == part }

// anyMatch is indicated by * in a glob pattern, and matches any single file
// or directory on the path.
type anyMatch struct{}

func (p anyMatch) String() string      { return "*" }
func (p anyMatch) Match(_ string) bool { return true }

type asteriskPattern struct {
	glob     string
	compiled *wildmatch.WildMatch
}

// asteriskPattern is a pattern that may contain * glob wildcard.
func makeAsteriskPattern(pattern string) asteriskPattern {
	// TODO: This also matches `?` for single characters, which we don't need.
	// We can later switch it out by a more optimized version for our use-case
	// but for now this is giving us a good boost already.
	compiled := wildmatch.NewWildMatch(pattern)
	return asteriskPattern{glob: pattern, compiled: compiled}
}
func (p asteriskPattern) String() string { return p.glob }
func (p asteriskPattern) Match(part string) bool {
	return p.compiled.IsMatch(part)
}

// Compile translates a text representation of a glob pattern
// to an executable one that can `match` file paths.
func Compile(pattern string) (*GlobPattern, error) {
	parts := strings.Split(strings.Trim(pattern, separator), separator)
	patternParts := make([]patternPart, 0, len(parts)+2)
	isLiteral := true
	// No leading `/` is equivalent to prefixing with `/**/`.
	// The pattern matches arbitrarily down the directory tree.
	if !strings.HasPrefix(pattern, separator) {
		patternParts = append(patternParts, anySubPath{})
		isLiteral = false
	}
	for _, part := range strings.Split(strings.Trim(pattern, separator), separator) {
		switch part {
		case "":
			return nil, errors.New("two consecutive forward slashes")
		case "**":
			patternParts = append(patternParts, anySubPath{})
			isLiteral = false
		case "*":
			patternParts = append(patternParts, anyMatch{})
			isLiteral = false
		default:
			if strings.Contains(part, "*") {
				patternParts = append(patternParts, makeAsteriskPattern(part))
				isLiteral = false
			} else {
				patternParts = append(patternParts, exactMatch(part))
			}
		}
	}
	// Trailing `/` is equivalent with ending the pattern with `/**` instead.
	if strings.HasSuffix(pattern, separator) {
		patternParts = append(patternParts, anySubPath{})
		isLiteral = false
	}
	// Trailing `/**` (explicitly or implicitly like above) is necessarily
	// translated to `/**/*.
	// This is because, trailing `/**` should not match if the path finishes
	// with the part that matches up to and excluding final `**` wildcard.
	// Example: Neither `/foo/bar/**` nor `/foo/bar/` should match file `/foo/bar`.
	if len(patternParts) > 0 {
		if _, ok := patternParts[len(patternParts)-1].(anySubPath); ok {
			patternParts = append(patternParts, anyMatch{})
			isLiteral = false
		}
	}

	// initialize a matching state with positions that are
	// matches for an empty input (`/`). This is most often just bit 0, but in case
	// there are subpath wildcard **, it is expanded to all indices past the
	// wildcards, since they match empty path.
	initialState := int64(1)
	for i, globPart := range patternParts {
		if _, ok := globPart.(anySubPath); !ok {
			break
		}
		initialState = initialState | 1<<(i+1)
	}

	return &GlobPattern{
		isLiteral:    isLiteral,
		pattern:      pattern,
		parts:        patternParts,
		initialState: initialState,
		size:         len(patternParts),
	}, nil
}

// GlobPattern implements a pattern for matching file paths,
// which can use directory/file names, * and ** wildcards,
// and may or may not be anchored to the root directory.
type GlobPattern struct {
	isLiteral    bool
	pattern      string
	parts        []patternPart
	size         int
	initialState int64
}

// Match iterates over `filePath` separated by `/`. It uses a bit vector
// to track which prefixes of glob pattern match the file path prefix so far.
// Bit vector indices correspond to separators between pattern parts.
//
// Visualized matching of `/src/java/test/UnitTest.java`
// against `src/java/test/**/*Test.java`:
// / ** / src / java / test / ** / *Test.java   | Glob pattern
// 0    1     2      3      4    5            6 | Bit vector index
// X    X     -      -      -    -            - | / (starting state)
// X    X     X      -      -    -            - | /src
// X    X     -      X      -    -            - | /src/java
// X    X     -      -      X    X            - | /src/java/test
// X    X     -      -      X    X            X | /src/java/test/UnitTest.java
//
// Another example of matching `/src/app/components/Label.tsx`
// against `/src/app/components/*.tsx`:
// / src / app / components / *.tsx   | Glob pattern
// 0     1     2            3       4 | Bit vector index
// X     -     -            -       - | / (starting state)
// -     X     -            -       - | /src
// -     -     X            -       - | /src/app
// -     -     -            X       - | /src/app/components
// -     -     -            -       X | /src/app/components/Label.tsx
//
// The match is successful if after iterating through the whole file path,
// full pattern matches, that is, there is a bit at the end of the glob.
func (glob GlobPattern) Match(filePath string) bool {
	// Fast pass for literal globs, we can just string compare those.
	if glob.isLiteral {
		return glob.pattern == filePath
	}
	// If starts with ** (ie no root match), do a fast pass on the last rule first,
	// this optimizes file ending and file name matches.
	if _, ok := glob.parts[glob.size-1].(anySubPath); ok {
		l, ok := lastPart(filePath, '/')
		if ok {
			if !glob.parts[glob.size-1].Match(l) {
				return false
			}
		}
	}
	// Dirty cheap version of strings.Trim(filePath, separator)
	if len(filePath) > 0 && filePath[0] == '/' {
		filePath = filePath[1:]
	}
	if len(filePath) > 0 && filePath[len(filePath)-1] == '/' {
		filePath = filePath[:len(filePath)-1]
	}
	var (
		currentState = glob.initialState
		nextState    = int64(0)
		part         string
		hasNext      bool
	)
	for {
		part, hasNext, filePath = nextPart(filePath, '/')
		// consume advances matching algorithm by a single part of a file path.
		// The `current` bit vector is the matching state for up until, but excluding
		// given `part` of the file path. The result - next set of states - is written

		// Since `**` or `anySubPath` can match any number of times, we hold
		// an invariant: If a bit vector has 1 at the state preceding `**`,
		// then that bit vector also has 1 at the state following `**`.
		for i := range glob.size {
			if (currentState>>i)&1 == 0 {
				continue
			}
			currentPart := glob.parts[i]
			// Case 1: `currentState` matches before i-th part of the pattern,
			// so set the i+1-th position of the `next` state to whether
			// the i-th pattern matches (consumes) `part`.
			if currentPart.Match(part) {
				nextState = nextState | 1<<(i+1)
				// Keep the invariant: if there is `**` afterwards, set it
				// to the same bit. This will not be overridden in the next
				// loop turns as `**` always matches.
				if i+1 < glob.size {
					if _, ok := glob.parts[i+1].(anySubPath); ok {
						nextState = nextState | 1<<(i+2)
					}
				}
			} else {
				nextState = nextState &^ (1 << (i + 1))
				// Keep the invariant: if there is `**` afterwards, set it
				// to the same bit. This will not be overridden in the next
				// loop turns as `**` always matches.
				if i+1 < glob.size {
					if _, ok := glob.parts[i+1].(anySubPath); ok {
						nextState = nextState &^ (1 << (i + 2))
					}
				}
			}

			// Case 2: To allow `**` to consume subsequent parts of the file path,
			// we keep the i-th bit - which precedes `**` - set.
			if _, ok := currentPart.(anySubPath); ok {
				nextState = nextState | 1<<i
			}

		}

		// No matches in current state, impossible to match.
		if currentState == 0 {
			return false
		}
		currentState = nextState

		if !hasNext {
			break
		}

		nextState = 0
	}
	// Return true if given state indicates whole glob being matched.
	return (currentState>>glob.size)&1 == 1
}

// nextPart splits a string by a separator rune and returns if there's another match,
// and the remainder to recheck later. It is a lazy strings.Split, of sorts,
// allowing us to only look as far in the string as absolutely needed.
func nextPart(s string, sep rune) (string, bool, string) {
	for i, c := range s {
		if c == sep {
			return s[:i], true, s[i+1:]
		}
	}
	return s, false, s
}

// lastPart returns the last segment of s before sep. It only works with ASCII!
func lastPart(s string, sep rune) (string, bool) {
	for i := len(s) - 1; i >= 0; i-- {
		if rune(s[i]) == sep {
			return s[i+1:], true
		}
	}
	return "", false
}
