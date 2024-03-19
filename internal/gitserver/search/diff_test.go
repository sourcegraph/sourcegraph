package search

import (
	_ "embed"
	"strings"
	"testing"

	"github.com/sourcegraph/go-diff/diff"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
)

var (
	//go:embed testdata/small_diff.txt
	smallDiff string

	//go:embed testdata/large_diff.txt
	largeDiff string
)

func TestDiffSearch(t *testing.T) {
	r := diff.NewMultiFileDiffReader(strings.NewReader(smallDiff))
	fileDiffs, err := r.ReadAllFiles()
	require.NoError(t, err)

	query := &protocol.DiffMatches{Expr: "(?i)polly"}
	matchTree, err := ToMatchTree(query)
	require.NoError(t, err)

	mergedResult, highlights, err := matchTree.Match(&LazyCommit{diff: fileDiffs})
	require.NoError(t, err)
	require.True(t, mergedResult.Satisfies())

	expectedHighlights := MatchedCommit{
		Diff: map[int]MatchedFileDiff{
			1: {
				MatchedHunks: map[int]MatchedHunk{
					0: {
						MatchedLines: map[int]result.Ranges{
							3: {{
								Start: result.Location{Offset: 9, Column: 9},
								End:   result.Location{Offset: 14, Column: 14},
							}, {
								Start: result.Location{Offset: 24, Column: 24},
								End:   result.Location{Offset: 29, Column: 29},
							}},
							4: {{
								Start: result.Location{Offset: 9, Column: 9},
								End:   result.Location{Offset: 14, Column: 14},
							}, {
								Start: result.Location{Offset: 43, Column: 43},
								End:   result.Location{Offset: 48, Column: 48},
							}},
						},
					},
				},
			},
		},
	}
	require.Equal(t, expectedHighlights, highlights)

	formatted, ranges := FormatDiff(fileDiffs, highlights.Diff)
	expectedFormatted := `web/src/integration/helpers.ts web/src/integration/helpers.ts
@@ -7,3 +7,3 @@ import { createDriverForTest, Driver } from '../../../shared/src/testing/driver'
 import express from 'express'
-import { Polly } from '@pollyjs/core'
+import { Polly, Request, Response } from '@pollyjs/core'
 import { PuppeteerAdapter } from './polly/PuppeteerAdapter'
`

	expectedRanges := result.Ranges{{
		Start: result.Location{Line: 3, Column: 10, Offset: 200},
		End:   result.Location{Line: 3, Column: 15, Offset: 205},
	}, {
		Start: result.Location{Line: 3, Column: 25, Offset: 215},
		End:   result.Location{Line: 3, Column: 30, Offset: 220},
	}, {
		Start: result.Location{Line: 4, Column: 10, Offset: 239},
		End:   result.Location{Line: 4, Column: 15, Offset: 244},
	}, {
		Start: result.Location{Line: 4, Column: 44, Offset: 273},
		End:   result.Location{Line: 4, Column: 49, Offset: 278},
	}}

	require.Equal(t, expectedFormatted, formatted)
	require.Equal(t, expectedRanges, ranges)
}

func BenchmarkDiffSearchCaseInsensitiveOptimization(b *testing.B) {
	b.Run("small diff", func(b *testing.B) {
		r := diff.NewMultiFileDiffReader(strings.NewReader(smallDiff))
		fileDiffs, err := r.ReadAllFiles()
		require.NoError(b, err)

		b.Run("with optimization", func(b *testing.B) {
			query := &protocol.DiffMatches{Expr: "polly", IgnoreCase: true}
			matchTree, err := ToMatchTree(query)
			require.NoError(b, err)

			for range b.N {
				mergedResult, _, _ := matchTree.Match(&LazyCommit{diff: fileDiffs})
				require.True(b, mergedResult.Satisfies())
			}
		})

		b.Run("without optimization", func(b *testing.B) {
			query := &protocol.DiffMatches{Expr: "(?i)polly", IgnoreCase: false}
			matchTree, err := ToMatchTree(query)
			require.NoError(b, err)

			for range b.N {
				mergedResult, _, _ := matchTree.Match(&LazyCommit{diff: fileDiffs})
				require.True(b, mergedResult.Satisfies())
			}
		})
	})

	b.Run("large diff", func(b *testing.B) {
		r := diff.NewMultiFileDiffReader(strings.NewReader(largeDiff))
		fileDiffs, err := r.ReadAllFiles()
		require.NoError(b, err)

		b.Run("many matches", func(b *testing.B) {
			b.Run("with optimization", func(b *testing.B) {
				query := &protocol.DiffMatches{Expr: "suggestion", IgnoreCase: true}
				matchTree, err := ToMatchTree(query)
				require.NoError(b, err)

				for range b.N {
					mergedResult, _, _ := matchTree.Match(&LazyCommit{diff: fileDiffs})
					require.True(b, mergedResult.Satisfies())
				}
			})

			b.Run("without optimization", func(b *testing.B) {
				query := &protocol.DiffMatches{Expr: "(?i)suggestion", IgnoreCase: false}
				matchTree, err := ToMatchTree(query)
				require.NoError(b, err)

				for range b.N {
					mergedResult, _, _ := matchTree.Match(&LazyCommit{diff: fileDiffs})
					require.True(b, mergedResult.Satisfies())
				}
			})
		})

		b.Run("few matches", func(b *testing.B) {
			b.Run("with optimization", func(b *testing.B) {
				query := &protocol.DiffMatches{Expr: "limitoffset", IgnoreCase: true}
				matchTree, err := ToMatchTree(query)
				require.NoError(b, err)

				for range b.N {
					mergedResult, _, _ := matchTree.Match(&LazyCommit{diff: fileDiffs})
					require.True(b, mergedResult.Satisfies())
				}
			})

			b.Run("without optimization", func(b *testing.B) {
				query := &protocol.DiffMatches{Expr: "(?i)limitoffset", IgnoreCase: false}
				matchTree, err := ToMatchTree(query)
				require.NoError(b, err)

				for range b.N {
					mergedResult, _, _ := matchTree.Match(&LazyCommit{diff: fileDiffs})
					require.True(b, mergedResult.Satisfies())
				}
			})
		})
	})
}
