package codenav

import (
	"context"
	"testing"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/scip/bindings/go/scip"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/core"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
)

func TestSearchBasedUsages_ResultWithoutSymbols(t *testing.T) {
	refRange := testRange(1)
	refRange2 := testRange(2)
	refRangeLineContent := "refRangeContent"

	mockSearchClient := FakeSearchClient().
		WithFile("path.java", ChunkMatchWithLine(refRange, refRangeLineContent), ChunkMatch(refRange2)).
		Build()

	searchResult, err := searchBasedUsagesImpl(
		context.Background(), observation.TestTraceLogger(log.NoOp()), mockSearchClient,
		UsagesForSymbolArgs{Limit: 100}, "symbol", "Java", core.None[MappedIndex](),
	)
	require.NoError(t, err)
	expectRanges(t, searchResult.Matches, refRange, refRange2)
	expectContent(t, searchResult.Matches, refRange, refRangeLineContent)
}

func TestSearchBasedUsages_ResultWithSymbols(t *testing.T) {
	refRange := testRange(1)
	defRange := testRange(2)
	refRange2 := testRange(3)

	mockSearchClient := FakeSearchClient().
		WithFile("path.java", ChunkMatches(refRange, refRange2, defRange)...).
		WithSymbols("path.java", defRange).
		Build()

	searchResult, err := searchBasedUsagesImpl(
		context.Background(), observation.TestTraceLogger(log.NoOp()), mockSearchClient,
		UsagesForSymbolArgs{Limit: 100}, "symbol", "Java", core.None[MappedIndex](),
	)
	require.NoError(t, err)
	expectRanges(t, searchResult.Matches, refRange, refRange2, defRange)
	expectDefinitionRanges(t, searchResult.Matches, defRange)
}

func TestSearchBasedUsages_SyntacticMatchesGetRemovedFromSearchBasedResults(t *testing.T) {
	commentRange := testRange(1)
	syntacticRange := testRange(2)

	commit := api.CommitID("deadbeef")
	mockSearchClient := FakeSearchClient().
		WithFile("path.java", ChunkMatches(commentRange, syntacticRange)...).
		Build()
	upload, lsifStore := setupUpload(commit, "", doc("path.java", ref("ref", syntacticRange)))
	fakeMappedIndex := NewMappedIndexFromTranslator(lsifStore, noopTranslator(), upload, commit)

	searchResult, err := searchBasedUsagesImpl(
		context.Background(), observation.TestTraceLogger(log.NoOp()), mockSearchClient,
		UsagesForSymbolArgs{Limit: 100}, "symbol", "Java", core.Some(fakeMappedIndex),
	)
	require.NoError(t, err)
	expectRanges(t, searchResult.Matches, commentRange)
}

func TestSearchBasedUsages_CountLimitExcludesFiles(t *testing.T) {
	ranges1 := testRanges(0, 5)
	ranges2 := testRanges(5, 10)
	mockSearchClient := FakeSearchClient().
		WithFile("a.java", ChunkMatches(ranges1...)...).
		WithFile("b.java", ChunkMatches(ranges2...)...).
		Build()

	searchResult, err := searchBasedUsagesImpl(
		context.Background(), observation.TestTraceLogger(log.NoOp()), mockSearchClient,
		UsagesForSymbolArgs{Limit: 5}, "symbol", "Java", core.None[MappedIndex](),
	)
	require.NoError(t, err)
	expectRanges(t, searchResult.Matches, ranges1...)
	expectSome(t, searchResult.NextCursor)
}

func TestSearchBasedUsages_CountLimitOnFileBoundary(t *testing.T) {
	ranges1 := testRanges(0, 5)
	ranges2 := testRanges(5, 10)
	mockSearchClient := FakeSearchClient().
		WithFile("a.java", ChunkMatches(ranges1...)...).
		WithFile("b.java", ChunkMatches(ranges2...)...).
		Build()
	searchResult, err := searchBasedUsagesImpl(
		context.Background(), observation.TestTraceLogger(log.NoOp()), mockSearchClient,
		UsagesForSymbolArgs{Limit: 6}, "symbol", "Java", core.None[MappedIndex](),
	)
	require.NoError(t, err)
	expectRanges(t, searchResult.Matches, testRanges(0, 10)...)
	expectSome(t, searchResult.NextCursor)
}

func TestSearchBasedUsages_CountLimitLargerThanMatchCount(t *testing.T) {
	ranges1 := testRanges(0, 5)
	ranges2 := testRanges(5, 10)
	mockSearchClient := FakeSearchClient().
		WithFile("a.java", ChunkMatches(ranges1...)...).
		WithFile("b.java", ChunkMatches(ranges2...)...).
		Build()
	searchResult, err := searchBasedUsagesImpl(
		context.Background(), observation.TestTraceLogger(log.NoOp()), mockSearchClient,
		UsagesForSymbolArgs{Limit: 11}, "symbol", "Java", core.None[MappedIndex](),
	)
	require.NoError(t, err)
	expectRanges(t, searchResult.Matches, testRanges(0, 10)...)
	expectNone(t, searchResult.NextCursor)
}

func TestSyntacticUsages(t *testing.T) {
	initialRange := testRange(10)
	refRange := testRange(1)
	defRange := testRange(2)
	commentRange := testRange(3)
	localRange := testRange(4)
	lineContent := "initialRangeContent"
	commit := api.CommitID("deadbeef")

	mockSearchClient := FakeSearchClient().
		WithFile("path.java", ChunkMatches(refRange, defRange, commentRange, localRange)...).
		WithFile("initial.java", ChunkMatchWithLine(initialRange, lineContent)).
		Build()
	upload, lsifStore := setupUpload(commit, "",
		doc("path.java",
			ref("ref", refRange),
			def("def", defRange),
			local("lcl", localRange)),
		doc("initial.java",
			ref("initial", initialRange)))
	fakeMappedIndex := NewMappedIndexFromTranslator(lsifStore, noopTranslator(), upload, commit)

	syntacticUsages, err := syntacticUsagesImpl(
		context.Background(), observation.TestTraceLogger(log.NoOp()),
		mockSearchClient, fakeMappedIndex, UsagesForSymbolArgs{
			Commit:      commit,
			Path:        core.NewRepoRelPathUnchecked("initial.java"),
			SymbolRange: initialRange,
			Limit:       100,
		},
	)
	if err != nil {
		t.Error(t, err)
	}
	// We expect syntactic usages to filter both the comment range that was included in the search result,
	// but not in the index as well as the range referencing the local symbol.
	expectRanges(t, syntacticUsages.Matches, initialRange, refRange, defRange)
	expectDefinitionRanges(t, syntacticUsages.Matches, defRange)
	expectContent(t, syntacticUsages.Matches, initialRange, lineContent)
}

func TestSyntacticUsages_DocumentNotInIndex(t *testing.T) {
	initialRange := testRange(1)
	refRange := testRange(2)
	commit := api.CommitID("deadbeef")

	mockSearchClient := FakeSearchClient().WithFile("not-in-index.java", ChunkMatch(refRange)).Build()
	upload, lsifStore := setupUpload(commit, "",
		doc("initial.java",
			ref("initial", initialRange)))
	fakeMappedIndex := NewMappedIndexFromTranslator(lsifStore, noopTranslator(), upload, commit)
	syntacticUsages, err := syntacticUsagesImpl(
		context.Background(), observation.TestTraceLogger(log.NoOp()),
		mockSearchClient, fakeMappedIndex, UsagesForSymbolArgs{
			Commit:      commit,
			Path:        core.NewRepoRelPathUnchecked("initial.java"),
			SymbolRange: initialRange,
			Limit:       100,
		},
	)
	if err != nil {
		t.Error(t, err)
	}
	expectRanges(t, syntacticUsages.Matches)
}

func TestSyntacticUsages_IndexCommitTranslated(t *testing.T) {
	initialRange := testRange(10)
	refRange := testRange(1)
	editedRange := testRange(2)
	noMatchRange := testRange(3)
	indexCommit := api.CommitID("deadbeef")
	targetCommit := api.CommitID("beefdead")

	mockSearchClient := FakeSearchClient().
		WithFile("path.java", ChunkMatches(refRange, editedRange, noMatchRange)...).
		Build()
	upload, lsifStore := setupUpload(indexCommit, "",
		doc("initial.java",
			ref("initial", shiftSCIPRange(initialRange, 2))),
		doc("path.java",
			ref("ref", shiftSCIPRange(refRange, 2)),
			ref("edited", shiftSCIPRange(editedRange, 2)),
			ref("noMatch", noMatchRange)))
	// Ranges in the index are shifted by +2, so the translator needs to shift by -2 to match up with the search results.
	fakeMappedIndex := NewMappedIndexFromTranslator(lsifStore, NewFakeTranslator(upload.GetCommit(), targetCommit, -2,
		func(_ core.RepoRelPath, r scip.Range) bool {
			// When a line was edited in a diff we invalidate all occurrences on that line.
			return r.CompareStrict(editedRange) == 0
		}), upload, targetCommit)

	syntacticUsages, err := syntacticUsagesImpl(
		context.Background(), observation.TestTraceLogger(log.NoOp()),
		mockSearchClient, fakeMappedIndex, UsagesForSymbolArgs{
			Commit:      targetCommit,
			Path:        core.NewRepoRelPathUnchecked("initial.java"),
			SymbolRange: initialRange,
			Limit:       100,
		},
	)
	if err != nil {
		t.Error(t, err)
	}
	expectRanges(t, syntacticUsages.Matches, refRange)
}

func TestCandidateStream(t *testing.T) {
	fakeMatches := func(count int, path string) result.Matches {
		matches := make(result.Matches, count)
		for i := 0; i < count; i++ {
			matches[i] = &result.FileMatch{File: result.File{Path: path}}
		}
		return matches
	}
	testCtx := context.Background()
	searchCtx, cancelFn := context.WithCancel(testCtx)
	stream := NewCandidateStream([]string{"a.go"}, 5, cancelFn)

	stream.Send(streaming.SearchEvent{
		Results: fakeMatches(2, "a.go"),
		Stats:   streaming.Stats{},
	})
	stream.Send(streaming.SearchEvent{
		Results: fakeMatches(3, "b.go"),
		Stats:   streaming.Stats{},
	})
	require.NoError(t, searchCtx.Err())
	require.Equal(t, 3, stream.Results.ResultCount())

	stream.Send(streaming.SearchEvent{
		Results: fakeMatches(4, "c.go"),
		Stats:   streaming.Stats{},
	})
	require.ErrorIs(t, searchCtx.Err(), context.Canceled)
	require.Equal(t, 7, stream.Results.ResultCount())

	stream.Send(streaming.SearchEvent{
		Results: fakeMatches(5, "d.go"),
		Stats:   streaming.Stats{},
	})
	require.ErrorIs(t, searchCtx.Err(), context.Canceled)
	require.Equal(t, 7, stream.Results.ResultCount())
}

func TestCandidateStream_FiltersFilesWithLimitHit(t *testing.T) {
	fakeMatches := func(count int, limitHit bool) result.Matches {
		matches := make(result.Matches, count)
		for i := 0; i < count; i++ {
			matches[i] = &result.FileMatch{LimitHit: limitHit}
		}
		return matches
	}
	testCtx := context.Background()
	searchCtx, cancelFn := context.WithCancel(testCtx)
	stream := NewCandidateStream([]string{}, 5, cancelFn)

	stream.Send(streaming.SearchEvent{
		Results: fakeMatches(10, true),
		Stats:   streaming.Stats{},
	})
	require.NoError(t, searchCtx.Err())
	require.Equal(t, 0, stream.Results.ResultCount())

	stream.Send(streaming.SearchEvent{
		Results: fakeMatches(6, false),
		Stats:   streaming.Stats{},
	})
	require.ErrorIs(t, searchCtx.Err(), context.Canceled)
	require.Equal(t, 6, stream.Results.ResultCount())
}
