package search

import (
	"bytes"
	"unicode/utf8"

	"github.com/sourcegraph/sourcegraph/internal/byteutils"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/search/casetransform"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// ToMatchTree converts a protocol.SearchQuery into its equivalent MatchTree.
// We don't send a match tree directly over the wire so using the protocol
// package doesn't pull in all the dependencies that the match tree needs.
func ToMatchTree(q protocol.Node) (MatchTree, error) {
	switch v := q.(type) {
	case *protocol.CommitBefore:
		return &CommitBefore{*v}, nil
	case *protocol.CommitAfter:
		return &CommitAfter{*v}, nil
	case *protocol.AuthorMatches:
		re, err := casetransform.CompileRegexp(v.Expr, v.IgnoreCase)
		return &AuthorMatches{re}, err
	case *protocol.CommitterMatches:
		re, err := casetransform.CompileRegexp(v.Expr, v.IgnoreCase)
		return &CommitterMatches{re}, err
	case *protocol.MessageMatches:
		re, err := casetransform.CompileRegexp(v.Expr, v.IgnoreCase)
		return &MessageMatches{re}, err
	case *protocol.DiffMatches:
		re, err := casetransform.CompileRegexp(v.Expr, v.IgnoreCase)
		return &DiffMatches{re}, err
	case *protocol.DiffModifiesFile:
		re, err := casetransform.CompileRegexp(v.Expr, v.IgnoreCase)
		return &DiffModifiesFile{re}, err
	case *protocol.Boolean:
		return &Constant{v.Value}, nil
	case *protocol.Operator:
		operands := make([]MatchTree, 0, len(v.Operands))
		for _, operand := range v.Operands {
			sub, err := ToMatchTree(operand)
			if err != nil {
				return nil, err
			}
			operands = append(operands, sub)
		}
		return &Operator{Kind: v.Kind, Operands: operands}, nil
	default:
		return nil, errors.Errorf("unknown protocol query type %T", q)
	}
}

// Visit performs a preorder traversal over the match tree, calling f on each node
func Visit(mt MatchTree, f func(MatchTree)) {
	switch v := mt.(type) {
	case *Operator:
		f(mt)
		for _, child := range v.Operands {
			Visit(child, f)
		}
	default:
		f(mt)
	}
}

// MatchTree is an interface representing the queries we can run against a commit.
type MatchTree interface {
	// Match returns whether the given predicate matches a commit and, if it does,
	// the portions of the commit that match in the form of *CommitHighlights
	Match(*LazyCommit) (CommitFilterResult, MatchedCommit, error)
}

// AuthorMatches is a predicate that matches if the author's name or email address
// matches the regex pattern.
type AuthorMatches struct {
	*casetransform.Regexp
}

func (a *AuthorMatches) Match(lc *LazyCommit) (CommitFilterResult, MatchedCommit, error) {
	if a.Regexp.Match(lc.AuthorName, &lc.LowerBuf) || a.Regexp.Match(lc.AuthorEmail, &lc.LowerBuf) {
		return filterResult(true), MatchedCommit{}, nil
	}
	return filterResult(false), MatchedCommit{}, nil
}

// CommitterMatches is a predicate that matches if the author's name or email address
// matches the regex pattern.
type CommitterMatches struct {
	*casetransform.Regexp
}

func (c *CommitterMatches) Match(lc *LazyCommit) (CommitFilterResult, MatchedCommit, error) {
	if c.Regexp.Match(lc.CommitterName, &lc.LowerBuf) || c.Regexp.Match(lc.CommitterEmail, &lc.LowerBuf) {
		return filterResult(true), MatchedCommit{}, nil
	}
	return filterResult(false), MatchedCommit{}, nil
}

// CommitBefore is a predicate that matches if the commit is before the given date
type CommitBefore struct {
	protocol.CommitBefore
}

func (c *CommitBefore) Match(lc *LazyCommit) (CommitFilterResult, MatchedCommit, error) {
	committerDate, err := lc.CommitterDate()
	if err != nil {
		return filterResult(false), MatchedCommit{}, err
	}
	return filterResult(committerDate.Before(c.Time)), MatchedCommit{}, nil
}

// CommitAfter is a predicate that matches if the commit is after the given date
type CommitAfter struct {
	protocol.CommitAfter
}

func (c *CommitAfter) Match(lc *LazyCommit) (CommitFilterResult, MatchedCommit, error) {
	committerDate, err := lc.CommitterDate()
	if err != nil {
		return filterResult(false), MatchedCommit{}, err
	}
	return filterResult(committerDate.After(c.Time)), MatchedCommit{}, nil
}

// MessageMatches is a predicate that matches if the commit message matches
// the provided regex pattern.
type MessageMatches struct {
	*casetransform.Regexp
}

func (m *MessageMatches) Match(lc *LazyCommit) (CommitFilterResult, MatchedCommit, error) {
	results := m.FindAllIndex(lc.Message, -1, &lc.LowerBuf)
	if results == nil {
		return filterResult(false), MatchedCommit{}, nil
	}

	return filterResult(true), MatchedCommit{
		Message: matchesToRanges(lc.Message, results),
	}, nil
}

// DiffMatches is a a predicate that matches if any of the lines changed by
// the commit match the given regex pattern.
type DiffMatches struct {
	*casetransform.Regexp
}

func (dm *DiffMatches) Match(lc *LazyCommit) (CommitFilterResult, MatchedCommit, error) {
	diff, err := lc.Diff()
	if err != nil {
		return filterResult(false), MatchedCommit{}, err
	}

	var fileDiffHighlights map[int]MatchedFileDiff
	matchedFileDiffs := make(map[int]struct{})
	for fileIdx, fileDiff := range diff {
		var hunkHighlights map[int]MatchedHunk
		for hunkIdx, hunk := range fileDiff.Hunks {
			var lineHighlights map[int]result.Ranges
			lr := byteutils.NewLineReader(hunk.Body)
			lineIdx := -1
			for lr.Scan() {
				line := lr.Line()
				lineIdx++

				if len(line) == 0 {
					continue
				}

				origin, lineWithoutPrefix := line[0], line[1:]
				switch origin {
				case '+', '-':
				default:
					continue
				}

				matches := dm.FindAllIndex(lineWithoutPrefix, -1, &lc.LowerBuf)
				if matches != nil {
					if lineHighlights == nil {
						lineHighlights = make(map[int]result.Ranges, 1)
					}
					lineHighlights[lineIdx] = matchesToRanges(lineWithoutPrefix, matches)
				}
			}

			if len(lineHighlights) > 0 {
				if hunkHighlights == nil {
					hunkHighlights = make(map[int]MatchedHunk, 1)
				}
				hunkHighlights[hunkIdx] = MatchedHunk{lineHighlights}
			}
		}
		if len(hunkHighlights) > 0 {
			if fileDiffHighlights == nil {
				fileDiffHighlights = make(map[int]MatchedFileDiff)
			}
			fileDiffHighlights[fileIdx] = MatchedFileDiff{MatchedHunks: hunkHighlights}
			matchedFileDiffs[fileIdx] = struct{}{}
		}
	}

	return CommitFilterResult{MatchedFileDiffs: matchedFileDiffs}, MatchedCommit{Diff: fileDiffHighlights}, nil
}

// DiffModifiesFile is a predicate that matches if the commit modifies any files
// that match the given regex pattern.
type DiffModifiesFile struct {
	*casetransform.Regexp
}

func (dmf *DiffModifiesFile) Match(lc *LazyCommit) (CommitFilterResult, MatchedCommit, error) {
	{
		// This block pre-filters a commit based on the output of the `--name-status` output.
		// It is significantly cheaper to get the changed file names compared to generating the full
		// diff, so we try to short-circuit when possible.

		foundMatch := false
		for _, fileName := range lc.ModifiedFiles() {
			if dmf.Regexp.Match([]byte(fileName), &lc.LowerBuf) {
				foundMatch = true
				break
			}
		}
		if !foundMatch {
			return filterResult(false), MatchedCommit{}, nil
		}
	}

	diff, err := lc.Diff()
	if err != nil {
		return filterResult(false), MatchedCommit{}, err
	}

	var fileDiffHighlights map[int]MatchedFileDiff
	matchedFileDiffs := make(map[int]struct{})
	for fileIdx, fileDiff := range diff {
		oldFileMatches := dmf.FindAllIndex([]byte(fileDiff.OrigName), -1, &lc.LowerBuf)
		newFileMatches := dmf.FindAllIndex([]byte(fileDiff.NewName), -1, &lc.LowerBuf)
		if oldFileMatches != nil || newFileMatches != nil {
			if fileDiffHighlights == nil {
				fileDiffHighlights = make(map[int]MatchedFileDiff)
			}
			fileDiffHighlights[fileIdx] = MatchedFileDiff{
				OldFile: matchesToRanges([]byte(fileDiff.OrigName), oldFileMatches),
				NewFile: matchesToRanges([]byte(fileDiff.NewName), newFileMatches),
			}
			matchedFileDiffs[fileIdx] = struct{}{}
		}
	}

	return CommitFilterResult{MatchedFileDiffs: matchedFileDiffs}, MatchedCommit{Diff: fileDiffHighlights}, nil
}

type Constant struct {
	Value bool
}

func (c *Constant) Match(*LazyCommit) (CommitFilterResult, MatchedCommit, error) {
	return filterResult(c.Value), MatchedCommit{}, nil
}

type Operator struct {
	Kind     protocol.OperatorKind
	Operands []MatchTree
}

func (o *Operator) Match(commit *LazyCommit) (CommitFilterResult, MatchedCommit, error) {
	switch o.Kind {
	case protocol.Not:
		cfr, _, err := o.Operands[0].Match(commit)
		if err != nil {
			return filterResult(false), MatchedCommit{}, err
		}
		cfr.Invert(commit)
		return cfr, MatchedCommit{}, nil
	case protocol.And:
		resultMatches := MatchedCommit{}

		// Start with everything matching, then intersect
		mergedCFR := CommitFilterResult{CommitMatched: true, MatchedFileDiffs: nil}
		for _, operand := range o.Operands {
			cfr, matches, err := operand.Match(commit)
			if err != nil {
				return filterResult(false), MatchedCommit{}, err
			}
			mergedCFR.Intersect(cfr)
			if !mergedCFR.Satisfies() {
				return filterResult(false), MatchedCommit{}, err
			}
			resultMatches = resultMatches.Merge(matches)
		}
		resultMatches.ConstrainToMatched(mergedCFR.MatchedFileDiffs)
		return mergedCFR, resultMatches, nil
	case protocol.Or:
		resultMatches := MatchedCommit{}

		// Start with no matches, then union
		mergedCFR := CommitFilterResult{CommitMatched: false, MatchedFileDiffs: make(map[int]struct{})}
		for _, operand := range o.Operands {
			cfr, matches, err := operand.Match(commit)
			if err != nil {
				return filterResult(false), MatchedCommit{}, err
			}
			mergedCFR.Union(cfr)
			if mergedCFR.Satisfies() {
				resultMatches = resultMatches.Merge(matches)
			}
		}
		resultMatches.ConstrainToMatched(mergedCFR.MatchedFileDiffs)
		return mergedCFR, resultMatches, nil
	default:
		panic("invalid operator kind")
	}
}

// matchesToRanges is a helper that takes the return value of regexp.FindAllStringIndex()
// and converts it to Ranges.
// INVARIANT: matches must be ordered and non-overlapping,
// which is guaranteed by regexp.FindAllIndex()
func matchesToRanges(content []byte, matches [][]int) result.Ranges {
	var (
		unscannedOffset          = 0
		scannedNewlines          = 0
		lastScannedNewlineOffset = -1
	)

	lineColumnOffset := func(byteOffset int) (line, column int) {
		unscanned := content[unscannedOffset:byteOffset]
		lastUnscannedNewlineOffset := bytes.LastIndexByte(unscanned, '\n')
		if lastUnscannedNewlineOffset != -1 {
			lastScannedNewlineOffset = unscannedOffset + lastUnscannedNewlineOffset
			scannedNewlines += bytes.Count(unscanned, []byte("\n"))
		}
		column = utf8.RuneCount(content[lastScannedNewlineOffset+1 : byteOffset])
		unscannedOffset = byteOffset
		return scannedNewlines, column
	}

	res := make(result.Ranges, 0, len(matches))
	for _, match := range matches {
		startLine, startColumn := lineColumnOffset(match[0])
		endLine, endColumn := lineColumnOffset(match[1])
		res = append(res, result.Range{
			Start: result.Location{Line: startLine, Column: startColumn, Offset: match[0]},
			End:   result.Location{Line: endLine, Column: endColumn, Offset: match[1]},
		})
	}
	return res
}

// CommitFilterResult represents constraints to answer whether a diff satisfies a query.
// It maintains a list of the indices of the single file diffs within the full diff that
// matched query nodes that apply to single file diffs such as "DiffModifiesFile" and "DiffMatches".
// We do this because a query like `file:a b` will be translated to
// `DiffModifiesFile{a} AND DiffMatches{b}`, which will match a diff that contains one
// single file diff that matches `DiffModifiesFile{a}` and a different single file diff that matches
// `DiffMatches{b}` when in reality, when a user writes `file:a b`, they probably
// want content matches that occur in file `a`, not just content matches that occur
// in a diff that modifies file `a` elsewhere.
type CommitFilterResult struct {
	// CommitMatched indicates whether a commit field matched (i.e. Author, Committer, etc.)
	CommitMatched bool

	// MatchedFileDiffs is the set of indices of single file diffs that matched the node.
	// We use the convention that nil means "unevaluated", which is treated as "all match"
	// during merges, but not when calling HasMatch().
	MatchedFileDiffs map[int]struct{}
}

// Satisfies returns whether constraint is satisfied -- either a commit field match or a single file diff.
func (c CommitFilterResult) Satisfies() bool {
	if c.CommitMatched {
		return true
	}
	return len(c.MatchedFileDiffs) > 0
}

// Invert inverts the filter result. It inverts whether any commit fields matched, as well
// as inverts the indices of single file diffs that match. We pass `LazyCommit` in so we can get
// the number of single file diffs in the commit's diff.
func (c *CommitFilterResult) Invert(lc *LazyCommit) {
	c.CommitMatched = !c.CommitMatched
	if c.MatchedFileDiffs == nil {
		c.MatchedFileDiffs = make(map[int]struct{})
		return
	} else if len(c.MatchedFileDiffs) == 0 {
		c.MatchedFileDiffs = nil
		return
	}
	diff, err := lc.Diff() // error already checked
	if err != nil {
		panic("unexpected error: " + err.Error())
	}
	for i := range len(diff) {
		if _, ok := c.MatchedFileDiffs[i]; ok {
			delete(c.MatchedFileDiffs, i)
		} else {
			c.MatchedFileDiffs[i] = struct{}{}
		}
	}
}

// Union merges other into the receiver, unioning the single file diff indices
func (c *CommitFilterResult) Union(other CommitFilterResult) {
	c.CommitMatched = c.CommitMatched || other.CommitMatched
	if c.MatchedFileDiffs == nil || other.MatchedFileDiffs == nil {
		c.MatchedFileDiffs = nil
		return
	}
	for i := range other.MatchedFileDiffs {
		c.MatchedFileDiffs[i] = struct{}{}
	}
}

// Intersect merges other into the receiver, computing the intersection of the single file diff indices
func (c *CommitFilterResult) Intersect(other CommitFilterResult) {
	c.CommitMatched = c.CommitMatched && other.CommitMatched
	if c.MatchedFileDiffs == nil {
		c.MatchedFileDiffs = other.MatchedFileDiffs
		return
	} else if other.MatchedFileDiffs == nil {
		return
	}
	for i := range c.MatchedFileDiffs {
		if _, ok := other.MatchedFileDiffs[i]; !ok {
			delete(c.MatchedFileDiffs, i)
		}
	}
}

// filterResult is a helper method that constructs a CommitFilterResult for the simple
// case of a commit field matching or failing to match.
func filterResult(val bool) CommitFilterResult {
	cfr := CommitFilterResult{CommitMatched: val}
	if !val {
		cfr.MatchedFileDiffs = make(map[int]struct{})
	}
	return cfr
}
