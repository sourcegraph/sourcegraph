package codenav

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

func monikersToString(vs []precise.QualifiedMonikerData) string {
	strs := make([]string, 0, len(vs))
	for _, v := range vs {
		strs = append(strs, fmt.Sprintf("%s:%s:%s:%s:%s", v.Kind, v.Scheme, v.Manager, v.Identifier, v.Version))
	}

	return strings.Join(strs, ", ")
}

func sliceContains(slice []string, str string) bool {
	for _, el := range slice {
		if el == str {
			return true
		}
	}
	return false
}

func uploadIDsToString(vs []uploadsshared.CompletedUpload) string {
	ids := make([]string, 0, len(vs))
	for _, v := range vs {
		ids = append(ids, strconv.Itoa(v.ID))
	}

	return strings.Join(ids, ", ")
}

func sortRanges(ranges []shared.Range) []shared.Range {
	sort.Slice(ranges, func(i, j int) bool {
		iStart := ranges[i].Start
		jStart := ranges[j].Start

		if iStart.Line < jStart.Line {
			// iStart comes first
			return true
		} else if iStart.Line > jStart.Line {
			// jStart comes first
			return false
		}
		// otherwise, starts on same line

		if iStart.Character < jStart.Character {
			// iStart comes first
			return true
		} else if iStart.Character > jStart.Character {
			// jStart comes first
			return false
		}
		// otherwise, starts at same character

		iEnd := ranges[i].End
		jEnd := ranges[j].End

		if jEnd.Line < iEnd.Line {
			// ranges[i] encloses ranges[j] (we want smaller first)
			return false
		} else if jStart.Line < jEnd.Line {
			// ranges[j] encloses ranges[i] (we want smaller first)
			return true
		}
		// otherwise, ends on same line

		if jStart.Character < jEnd.Character {
			// ranges[j] encloses ranges[i] (we want smaller first)
			return true
		}

		return false
	})

	return ranges
}

func dedupeRanges(ranges []shared.Range) []shared.Range {
	if len(ranges) == 0 {
		return ranges
	}

	dedup := ranges[:1]
	for _, s := range ranges[1:] {
		if s != dedup[len(dedup)-1] {
			dedup = append(dedup, s)
		}
	}
	return dedup
}

type linemap struct {
	positions []int
}

func newLinemap(source string) linemap {
	// first line starts at offset 0
	l := linemap{positions: []int{0}}
	for i, char := range source {
		if char == '\n' {
			l.positions = append(l.positions, i+1)
		}
	}
	// as we want the offset of the line _following_ a symbol's line,
	// we need to add one extra here for when symbols exist on the final line
	lastNewline := l.positions[len(l.positions)-1]
	lenToEnd := len(source[lastNewline:])
	l.positions = append(l.positions, lastNewline+lenToEnd+1)
	return l
}
