package search

import (
	"sort"

	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
)

type CommitHighlights struct {
	Message protocol.Ranges
	Diff    map[int]FileDiffHighlight
}

func (c *CommitHighlights) Merge(other *CommitHighlights) *CommitHighlights {
	if c == nil {
		return other
	}

	if other == nil {
		return c
	}

	c.Message = c.Message.Merge(other.Message)

	if c.Diff == nil {
		c.Diff = other.Diff
	} else {
		for i, fdh := range other.Diff {
			c.Diff[i] = c.Diff[i].Merge(fdh)
		}
	}

	return c
}

type FileDiffHighlight struct {
	OldFile        protocol.Ranges
	NewFile        protocol.Ranges
	HunkHighlights map[int]HunkHighlight
}

func (f FileDiffHighlight) Merge(other FileDiffHighlight) FileDiffHighlight {
	f.OldFile = append(f.OldFile, other.OldFile...)
	sort.Sort(f.OldFile)

	f.NewFile = append(f.NewFile, other.NewFile...)
	sort.Sort(f.NewFile)

	if f.HunkHighlights == nil {
		f.HunkHighlights = other.HunkHighlights
	} else {
		for i, hh := range other.HunkHighlights {
			f.HunkHighlights[i] = f.HunkHighlights[i].Merge(hh)
		}
	}
	return f
}

type HunkHighlight struct {
	LineHighlights map[int]protocol.Ranges
}

func (h HunkHighlight) Merge(other HunkHighlight) HunkHighlight {
	if h.LineHighlights == nil {
		h.LineHighlights = other.LineHighlights
	} else {
		for i, lh := range other.LineHighlights {
			h.LineHighlights[i] = h.LineHighlights[i].Merge(lh)
		}
	}
	return h
}
