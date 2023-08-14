package context

import (
	"strings"

	"github.com/sourcegraph/scip/bindings/go/scip"
)

type DocumentAndText struct {
	Content        string
	NewlineOffsets []int
	SCIP           *scip.Document
}

func NewDocumentAndText(content string, syntectDocs *scip.Document) DocumentAndText {
	return DocumentAndText{
		Content:        content,
		NewlineOffsets: buildNewlineOffsets(content),
		SCIP:           syntectDocs,
	}
}

func (d DocumentAndText) Extract(r *scip.Range) string {
	// Sanity check (lines are validated via `lineBounds` below)
	if r.Start.Character < 0 || r.End.Character < 0 {
		return ""
	}

	// Find the byte offsets for the start and end lines in the underlying text
	startLineLower, startLineUpper, startOk := d.lineBounds(int(r.Start.Line))
	endLineLower, endLineUpper, endOk := d.lineBounds(int(r.End.Line))
	if !startOk || !endOk {
		return ""
	}

	lo := startLineLower + int(r.Start.Character)
	if lo > startLineUpper {
		// Start character exceeds line; start from the next line
		lo = startLineUpper + 1
	}

	hi := endLineLower + int(r.End.Character)
	if hi > endLineUpper {
		// End character exceeds line; cut it back to true end of line
		hi = endLineUpper
	}

	// Sanity check
	if hi <= lo {
		return ""
	}

	return d.Content[lo:hi]
}

// lineBounds returns the lower and upper offsets for a particular zero-indexed
// line number from the underlying document text. The resulting offsets include
// the trailing newline (if one exists).
func (d DocumentAndText) lineBounds(line int) (lower, upper int, _ bool) {
	if line < 0 || line > len(d.NewlineOffsets) {
		return 0, 0, false
	}

	if line == 0 {
		// first line, offset is buffer start
		lower = 0
	} else {
		// skip preceding newline
		lower = d.NewlineOffsets[line-1] + 1
	}

	if line == len(d.NewlineOffsets) {
		// last line, offset is end of buffer
		upper = len(d.Content)
	} else {
		// offset is the trailing newline separator
		upper = d.NewlineOffsets[line]
	}

	return lower, upper, true
}

// buildNewlineOffsets returns an ordered slice of the byte offsets of each
// newline character in the given text.
func buildNewlineOffsets(s string) []int {
	newlineOffsets := make([]int, 0, strings.Count(s, "\n"))
	for start := 0; start < len(s); {
		index := strings.IndexByte(s[start:], '\n')
		if index == -1 {
			break
		}

		start += index
		newlineOffsets = append(newlineOffsets, start)
		start += 1
	}

	return newlineOffsets
}
