package codenav

import (
	"context"
	"strings"
	"testing"

	genslices "github.com/life4/genesis/slices"
	"github.com/sourcegraph/scip/bindings/go/scip"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/internal/lsifstore"
	lsifstoremocks "github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/internal/lsifstore/mocks"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/core"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/search"
	searchClient "github.com/sourcegraph/sourcegraph/internal/search/client"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var uploadIDSupply = 0

func newUploadID() int {
	uploadIDSupply += 1
	return uploadIDSupply
}

// Generates a fake scip.Range that is easy to tell from other ranges
func testRange(r int) scip.Range {
	return scip.NewRangeUnchecked([]int32{int32(r), int32(r), int32(r)})
}

// Generates fake scip.Range's for the given range [hi, lo)
func testRanges(lo int, hi int) []scip.Range {
	if hi < lo {
		panic("hi must be greater than lo")
	}
	ranges := make([]scip.Range, 0, hi-lo+1)
	for i := lo; i < hi; i++ {
		ranges = append(ranges, testRange(i))
	}
	return ranges
}

type fakeOccurrence struct {
	symbol       string
	isDefinition bool
	range_       scip.Range
}

type fakeDocument struct {
	path        core.UploadRelPath
	occurrences []fakeOccurrence
}

func (d fakeDocument) Occurrences() []*scip.Occurrence {
	occs := make([]*scip.Occurrence, 0, len(d.occurrences))
	for _, occ := range d.occurrences {
		var symbolRoles scip.SymbolRole = 0
		if occ.isDefinition {
			symbolRoles = scip.SymbolRole_Definition
		}
		occs = append(occs, &scip.Occurrence{
			Range:       occ.range_.SCIPRange(),
			Symbol:      occ.symbol,
			SymbolRoles: int32(symbolRoles),
		})
	}
	return occs
}

func sym(name string) string {
	return "test . . . " + name + "."
}

func ref(symbol string, range_ scip.Range) fakeOccurrence {
	return fakeOccurrence{
		symbol:       sym(symbol),
		isDefinition: false,
		range_:       range_,
	}
}

func def(symbol string, range_ scip.Range) fakeOccurrence {
	return fakeOccurrence{
		symbol:       sym(symbol),
		isDefinition: true,
		range_:       range_,
	}
}

func local(symbol string, range_ scip.Range) fakeOccurrence {
	return fakeOccurrence{
		symbol:       "local " + symbol,
		isDefinition: false,
		range_:       range_,
	}
}

func doc(path string, occurrences ...fakeOccurrence) fakeDocument {
	return fakeDocument{
		path:        core.NewUploadRelPathUnchecked(path),
		occurrences: occurrences,
	}
}

// Set up uploads + lsifstore
func setupUpload(commit api.CommitID, root string, documents ...fakeDocument) (uploadsshared.CompletedUpload, lsifstore.LsifStore) {
	id := newUploadID()
	lsifStore := lsifstoremocks.NewMockLsifStore()
	lsifStore.SCIPDocumentFunc.SetDefaultHook(func(ctx context.Context, uploadId int, path core.UploadRelPath) (core.Option[*scip.Document], error) {
		if id != uploadId {
			return core.None[*scip.Document](), errors.New("unknown upload id")
		}
		for _, document := range documents {
			if document.path.Equal(path) {
				return core.Some(&scip.Document{
					RelativePath: document.path.RawValue(),
					Occurrences:  document.Occurrences(),
				}), nil
			}
		}
		return core.None[*scip.Document](), nil
	})

	lsifStore.SCIPDocumentsFunc.SetDefaultHook(func(ctx context.Context, uploadId int, paths []core.UploadRelPath) (map[core.UploadRelPath]*scip.Document, error) {
		if id != uploadId {
			return nil, errors.New("unknown upload id")
		}
		results := make(map[core.UploadRelPath]*scip.Document)
		for _, path := range paths {
			for _, document := range documents {
				if document.path.Equal(path) {
					results[path] = &scip.Document{
						RelativePath: document.path.RawValue(),
						Occurrences:  document.Occurrences(),
					}
				}
			}
		}
		return results, nil
	})

	return uploadsshared.CompletedUpload{
		ID:     id,
		Commit: string(commit),
		Root:   root,
	}, lsifStore
}

func shiftSCIPRange(r scip.Range, numLines int) scip.Range {
	return scip.NewRangeUnchecked([]int32{
		r.Start.Line + int32(numLines),
		r.Start.Character,
		r.End.Line + int32(numLines),
		r.End.Character,
	})
}

func shiftPos(pos scip.Position, numLines int32) scip.Position {
	return scip.Position{
		Line:      pos.Line + numLines,
		Character: pos.Character,
	}
}

// A GitTreeTranslator that returns positions and ranges shifted by numLines
// and returns failed translations for path/range pairs if shouldFail returns true
type fakeTranslator struct {
	from       api.CommitID
	to         api.CommitID
	numLines   int
	shouldFail func(core.RepoRelPath, scip.Range) bool
}

func (t fakeTranslator) TranslatePosition(
	ctx context.Context, from, to api.CommitID, path core.RepoRelPath, pos scip.Position,
) (core.Option[scip.Position], error) {
	numLines := t.numLines
	if from == t.to && to == t.from {
		numLines = -numLines
	}
	if t.shouldFail(path, scip.Range{Start: pos, End: pos}) {
		return core.None[scip.Position](), nil
	}
	return core.Some(shiftPos(pos, int32(numLines))), nil
}

func (t fakeTranslator) TranslateRange(
	ctx context.Context, from, to api.CommitID, path core.RepoRelPath, range_ scip.Range,
) (core.Option[scip.Range], error) {
	numLines := t.numLines
	if from == t.to && to == t.from {
		numLines = -numLines
	}
	if t.shouldFail(path, range_) {
		return core.None[scip.Range](), nil
	}
	return core.Some(shiftSCIPRange(range_, numLines)), nil
}

func (t fakeTranslator) Prefetch(ctx context.Context, from api.CommitID, to api.CommitID, paths []core.RepoRelPath) {
	return
}

func NewFakeTranslator(
	from, to api.CommitID,
	numLines int,
	shouldFail func(core.RepoRelPath, scip.Range) bool,
) GitTreeTranslator {
	return fakeTranslator{
		from:       from,
		to:         to,
		numLines:   numLines,
		shouldFail: shouldFail,
	}
}

// A GitTreeTranslator that returns all positions and ranges shifted by numLines.
func shiftAllTranslator(from, to api.CommitID, numLines int) GitTreeTranslator {
	return NewFakeTranslator(from, to, numLines, func(path core.RepoRelPath, range_ scip.Range) bool { return false })
}

// A GitTreeTranslator that returns all positions and ranges unchanged
func noopTranslator() GitTreeTranslator {
	return shiftAllTranslator("a", "b", 0)
}

type MatchLike interface {
	GetRange() scip.Range
	GetIsDefinition() bool
	GetSurroundingContent() string
}

func expectContent[T MatchLike](t *testing.T, matches []T, range_ scip.Range, content string) {
	t.Helper()
	for _, match := range matches {
		if match.GetRange().CompareStrict(range_) == 0 {
			require.Equal(t, content, match.GetSurroundingContent())
		}
	}
}

func expectRanges[T MatchLike](t *testing.T, matches []T, ranges ...scip.Range) {
	t.Helper()
	for _, match := range matches {
		_, err := genslices.Find(ranges, func(r scip.Range) bool {
			return match.GetRange().CompareStrict(r) == 0
		})
		require.NoErrorf(t, err, "Did not expect match at %q", match.GetRange().String())
	}

	for _, r := range ranges {
		_, err := genslices.Find(matches, func(match T) bool {
			return match.GetRange().CompareStrict(r) == 0
		})
		require.NoErrorf(t, err, "Expected match at %q", r.String())
	}
}

func expectDefinitionRanges[T MatchLike](t *testing.T, matches []T, ranges ...scip.Range) {
	t.Helper()
	for _, match := range matches {
		_, err := genslices.Find(ranges, func(r scip.Range) bool {
			return match.GetRange().CompareStrict(r) == 0
		})
		if match.GetIsDefinition() && err != nil {
			t.Errorf("Did not expect match at %q to be a definition", match.GetRange().String())
			return
		} else if !match.GetIsDefinition() && err == nil {
			t.Errorf("Expected match at %q to be a definition", match.GetRange().String())
			return
		}
	}
}

func expectSome[A any](t *testing.T, opt core.Option[A]) {
	t.Helper()
	require.True(t, opt.IsSome(), "Expected Some but got None")
}

func expectNone[A any](t *testing.T, opt core.Option[A]) {
	t.Helper()
	require.True(t, opt.IsNone(), "Expected None but got Some")
}

func scipToResultPosition(p scip.Position) result.Location {
	return result.Location{
		Line:   int(p.Line),
		Column: int(p.Character),
	}
}

func scipToResultRange(r scip.Range) result.Range {
	return result.Range{
		Start: scipToResultPosition(r.Start),
		End:   scipToResultPosition(r.End),
	}
}

// scipToSymbolMatch "reverse engineers" the lsp.Range function on result.Symbol
func scipToSymbolMatch(r scip.Range) *result.SymbolMatch {
	return &result.SymbolMatch{
		Symbol: result.Symbol{
			Line:      int(r.Start.Line + 1),
			Character: int(r.Start.Character),
			Name:      strings.Repeat("a", int(r.End.Character-r.Start.Character)),
		}}
}

type FakeSearchBuilder struct {
	fileMatches   []result.Match
	symbolMatches []result.Match
}

func FakeSearchClient() FakeSearchBuilder {
	return FakeSearchBuilder{
		fileMatches:   []result.Match{},
		symbolMatches: make([]result.Match, 0),
	}
}

func ChunkMatchWithLine(range_ scip.Range, line string) result.ChunkMatch {
	return result.ChunkMatch{
		Ranges:  []result.Range{scipToResultRange(range_)},
		Content: line,
		ContentStart: result.Location{
			Line:   int(range_.Start.Line),
			Column: 0,
		},
	}
}

func ChunkMatch(range_ scip.Range) result.ChunkMatch {
	return ChunkMatchWithLine(range_, "chonky")
}

func ChunkMatches(ranges ...scip.Range) []result.ChunkMatch {
	return genslices.Map(ranges, ChunkMatch)
}

func (b FakeSearchBuilder) WithFile(file string, matches ...result.ChunkMatch) FakeSearchBuilder {
	b.fileMatches = append(b.fileMatches, &result.FileMatch{
		File:         result.File{Path: file},
		ChunkMatches: matches,
	})
	return b
}

func (b FakeSearchBuilder) WithSymbols(file string, ranges ...scip.Range) FakeSearchBuilder {
	b.symbolMatches = append(b.symbolMatches, &result.FileMatch{
		File:    result.File{Path: file},
		Symbols: genslices.Map(ranges, scipToSymbolMatch),
	})
	return b
}

func (b FakeSearchBuilder) Build() searchClient.SearchClient {
	mockSearchClient := searchClient.NewMockSearchClient()
	mockSearchClient.PlanFunc.SetDefaultHook(func(_ context.Context, _ string, _ *string, query string, _ search.Mode, _ search.Protocol, _ *int32) (*search.Inputs, error) {
		return &search.Inputs{OriginalQuery: query}, nil
	})
	mockSearchClient.ExecuteFunc.SetDefaultHook(func(_ context.Context, s streaming.Sender, i *search.Inputs) (*search.Alert, error) {
		if strings.Contains(i.OriginalQuery, "type:file") {
			s.Send(streaming.SearchEvent{Results: b.fileMatches})
		} else if strings.Contains(i.OriginalQuery, "type:symbol") {
			s.Send(streaming.SearchEvent{Results: b.symbolMatches})
		}
		return nil, nil
	})
	return mockSearchClient
}
