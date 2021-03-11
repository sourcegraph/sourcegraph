package result

import (
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

type FileMatch struct {
	Path        string
	LineMatches []*LineMatch
	LimitHit    bool

	Symbols  []*SymbolMatch  `json:"-"`
	URI      string          `json:"-"`
	Repo     *types.RepoName `json:"-"`
	CommitID api.CommitID    `json:"-"`

	// InputRev is the Git revspec that the user originally requested to search. It is used to
	// preserve the original revision specifier from the user instead of navigating them to the
	// absolute commit ID when they select a result.
	InputRev *string `json:"-"`
}

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

// LineMatch is the struct used by vscode to receive search results for a line
type LineMatch struct {
	Preview          string
	OffsetAndLengths [][2]int32
	LineNumber       int32
	LimitHit         bool
}
