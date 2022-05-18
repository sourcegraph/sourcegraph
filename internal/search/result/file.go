package result

import (
	"net/url"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/search/filter"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// File represents all the information we need to identify a file in a repository
type File struct {
	// InputRev is the Git revspec that the user originally requested to search. It is used to
	// preserve the original revision specifier from the user instead of navigating them to the
	// absolute commit ID when they select a result.
	InputRev *string           `json:"-"`
	Repo     types.MinimalRepo `json:"-"`
	CommitID api.CommitID      `json:"-"`
	Path     string
}

func (f *File) URL() *url.URL {
	var path strings.Builder
	path.Grow(len("/@/-/blob/") + len(f.Repo.Name) + len(f.Path) + 20)
	path.WriteRune('/')
	path.WriteString(string(f.Repo.Name))
	if f.InputRev != nil && len(*f.InputRev) > 0 {
		path.WriteRune('@')
		path.WriteString(*f.InputRev)
	}
	path.WriteString("/-/blob/")
	path.WriteString(f.Path)
	return &url.URL{Path: path.String()}
}

// FileMatch represents either:
// - A collection of symbol results (len(Symbols) > 0)
// - A collection of text content results (len(LineMatches) > 0)
// - A result representing the whole file (len(Symbols) == 0 && len(LineMatches) == 0)
type FileMatch struct {
	File

	LineMatches []*LineMatch
	Symbols     []*SymbolMatch `json:"-"`

	LimitHit bool
}

func (fm *FileMatch) RepoName() types.MinimalRepo {
	return fm.File.Repo
}

func (fm *FileMatch) searchResultMarker() {}

func (fm *FileMatch) ResultCount() int {
	rc := len(fm.Symbols)
	for _, m := range fm.LineMatches {
		rc += len(m.OffsetAndLengths)
	}
	if rc == 0 {
		return 1 // 1 to count "empty" results like type:path results
	}
	return rc
}

// IsPathMatch returns true if a `FileMatch` has no line or symbol matches. In
// the absence of a true `PathMatch` type, we use this function as a proxy
// signal to drive `select:file` logic that deduplicates path results.
func (fm *FileMatch) IsPathMatch() bool {
	return fm.LineMatches == nil && fm.Symbols == nil
}

func (fm *FileMatch) Select(selectPath filter.SelectPath) Match {
	switch selectPath.Root() {
	case filter.Repository:
		return &RepoMatch{
			Name: fm.Repo.Name,
			ID:   fm.Repo.ID,
		}
	case filter.File:
		fm.LineMatches = nil
		fm.Symbols = nil
		if len(selectPath) > 1 && selectPath[1] == "directory" {
			fm.Path = path.Clean(path.Dir(fm.Path)) + "/" // Add trailing slash for clarity.
		}
		return fm
	case filter.Symbol:
		if len(fm.Symbols) > 0 {
			fm.LineMatches = nil // Only return symbol match if symbols exist
			if len(selectPath) > 1 {
				filteredSymbols := SelectSymbolKind(fm.Symbols, selectPath[1])
				if len(filteredSymbols) == 0 {
					return nil // Remove file match if there are no symbol results after filtering
				}
				fm.Symbols = filteredSymbols
			}
			return fm
		}
		return nil
	case filter.Content:
		// Only return file match if line matches exist
		if len(fm.LineMatches) > 0 {
			fm.Symbols = nil
			return fm
		}
		return nil
	case filter.Commit:
		return nil
	}
	return nil
}

// AppendMatches appends the line matches from src as well as updating match
// counts and limit.
func (fm *FileMatch) AppendMatches(src *FileMatch) {
	fm.LineMatches = append(fm.LineMatches, src.LineMatches...)
	fm.Symbols = append(fm.Symbols, src.Symbols...)
	fm.LimitHit = fm.LimitHit || src.LimitHit
}

// Limit will mutate fm such that it only has limit results. limit is a number
// greater than 0.
//
//   if limit >= ResultCount then nothing is done and we return limit - ResultCount.
//   if limit < ResultCount then ResultCount becomes limit and we return 0.
func (fm *FileMatch) Limit(limit int) int {
	// Check if we need to limit.
	if after := limit - fm.ResultCount(); after >= 0 {
		return after
	}

	// Invariant: limit > 0
	for i, m := range fm.LineMatches {
		after := limit - len(m.OffsetAndLengths)
		if after <= 0 {
			fm.Symbols = nil
			fm.LineMatches = fm.LineMatches[:i+1]
			m.OffsetAndLengths = m.OffsetAndLengths[:limit]
			return 0
		}
		limit = after
	}

	fm.Symbols = fm.Symbols[:limit]
	return 0
}

func (fm *FileMatch) Key() Key {
	k := Key{
		TypeRank: rankFileMatch,
		Repo:     fm.Repo.Name,
		Commit:   fm.CommitID,
		Path:     fm.Path,
	}

	if fm.InputRev != nil {
		k.Rev = *fm.InputRev
	}

	return k
}

// LineColumn is a subset of the fields on Location because we don't
// have the rune offset necessary to build a full Location yet.
// Eventually, the two structs should be merged.
type LineColumn struct {
	// Line is the count of newlines before the offset in the matched text
	Line int32

	// Column is the count of unicode code points after the last newline in the matched text
	Column int32
}

type MultilineMatch struct {
	// Preview is a possibly-multiline string that contains all the
	// lines that the match overlaps.
	// The number of lines in Preview should be End.Line - Start.Line + 1
	Preview string
	Start   LineColumn
	End     LineColumn
}

func (m MultilineMatch) AsLineMatches() []LineMatch {
	lines := strings.Split(m.Preview, "\n")
	lineMatches := make([]LineMatch, 0, len(lines))
	for i, line := range lines {
		// TODO include the newline character?
		offset := int32(0)
		if i == 0 {
			offset = m.Start.Column
		}
		length := int32(utf8.RuneCountInString(line)) - offset
		if i == len(lines)-1 {
			length = m.End.Column - offset
		}
		lineMatches = append(lineMatches, LineMatch{
			Preview:          line,
			LineNumber:       m.Start.Line + int32(i),
			OffsetAndLengths: [][2]int32{{offset, length}},
		})
	}
	return lineMatches
}

type LineMatch struct {
	// Preview is the full single line these offsets belong to
	Preview          string
	OffsetAndLengths [][2]int32
	LineNumber       int32
}

func (m LineMatch) AsMultilineMatches() []MultilineMatch {
	multilineMatches := make([]MultilineMatch, 0, len(m.OffsetAndLengths))
	for _, offsetAndLength := range m.OffsetAndLengths {
		offset, length := offsetAndLength[0], offsetAndLength[1]
		multilineMatches = append(multilineMatches, MultilineMatch{
			Preview: m.Preview,
			Start:   LineColumn{Line: m.LineNumber, Column: offset},
			End:     LineColumn{Line: m.LineNumber, Column: offset + length},
		})
	}
	return multilineMatches
}
