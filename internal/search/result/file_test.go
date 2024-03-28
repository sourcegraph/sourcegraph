package result

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func mkSymbolMatch(name string, line int) *SymbolMatch {
	return &SymbolMatch{
		Symbol: Symbol{
			Name: name,
			Line: line,
		},
	}
}

func TestAppendSymbols(t *testing.T) {
	cases := []struct {
		name   string
		input1 *FileMatch
		input2 *FileMatch
		output *FileMatch
	}{
		{
			name: "duplicate symbol",
			input1: &FileMatch{
				Symbols: []*SymbolMatch{
					mkSymbolMatch("sym2", 42),
					mkSymbolMatch("sym1", 41),
				},
			},
			input2: &FileMatch{
				Symbols: []*SymbolMatch{
					mkSymbolMatch("sym2", 42),
					mkSymbolMatch("sym3", 43),
				},
			},
			output: &FileMatch{
				Symbols: []*SymbolMatch{
					mkSymbolMatch("sym1", 41),
					mkSymbolMatch("sym2", 42),
					mkSymbolMatch("sym3", 43),
				},
			},
		},
		{
			name: "same line, different symbol",
			input1: &FileMatch{
				Symbols: []*SymbolMatch{
					mkSymbolMatch("sym1", 41),
				},
			},
			input2: &FileMatch{
				Symbols: []*SymbolMatch{
					mkSymbolMatch("sym2", 41),
				},
			},
			output: &FileMatch{
				Symbols: []*SymbolMatch{
					mkSymbolMatch("sym1", 41),
					mkSymbolMatch("sym2", 41),
				},
			},
		},
		{
			name:   "empty left side",
			input1: &FileMatch{},
			input2: &FileMatch{
				Symbols: []*SymbolMatch{
					mkSymbolMatch("sym1", 41),
				},
			},
			output: &FileMatch{
				Symbols: []*SymbolMatch{
					mkSymbolMatch("sym1", 41),
				},
			},
		},
		{
			name: "empty right side",
			input1: &FileMatch{
				Symbols: []*SymbolMatch{
					mkSymbolMatch("sym1", 41),
				},
			},
			input2: &FileMatch{},
			output: &FileMatch{
				Symbols: []*SymbolMatch{
					mkSymbolMatch("sym1", 41),
				},
			},
		},
		{
			name:   "both empty",
			input1: &FileMatch{},
			input2: &FileMatch{},
			output: &FileMatch{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.input1.mergeSymbols(tc.input2)
			require.Equal(t, tc.output, tc.input1)
		})
	}
}

func TestConvertMatches(t *testing.T) {
	t.Run("AsLineMatches", func(t *testing.T) {
		cases := []struct {
			input  ChunkMatch
			output []*LineMatch
		}{{
			input: ChunkMatch{
				Content:      "line1\nline2\nline3\n",
				ContentStart: Location{Line: 1},
				Ranges: Ranges{{
					Start: Location{1, 1, 1},
					End:   Location{13, 3, 1},
				}},
			},
			output: []*LineMatch{{
				Preview:          "line1",
				LineNumber:       1,
				OffsetAndLengths: [][2]int32{{1, 4}},
			}, {
				Preview:          "line2",
				LineNumber:       2,
				OffsetAndLengths: [][2]int32{{0, 5}},
			}, {
				Preview:          "line3",
				LineNumber:       3,
				OffsetAndLengths: [][2]int32{{0, 1}},
			}},
		}, {
			input: ChunkMatch{
				Content:      "line1\nstart 的<-multibyte\nline3",
				ContentStart: Location{Line: 1},
				Ranges: Ranges{{
					Start: Location{0, 1, 0},
					End:   Location{32, 3, 5},
				}},
			},
			output: []*LineMatch{{
				Preview:          "line1",
				LineNumber:       1,
				OffsetAndLengths: [][2]int32{{0, 5}},
			}, {
				Preview:    "start 的<-multibyte",
				LineNumber: 2,
				// 18 is rune length, not the byte length
				OffsetAndLengths: [][2]int32{{0, 18}},
			}, {
				Preview:          "line3",
				LineNumber:       3,
				OffsetAndLengths: [][2]int32{{0, 5}},
			}},
		}, {
			input: ChunkMatch{
				Content:      "line1",
				ContentStart: Location{Line: 1},
				Ranges: Ranges{{
					Start: Location{1, 1, 1},
					End:   Location{1, 1, 3},
				}},
			},
			output: []*LineMatch{{
				Preview:          "line1",
				LineNumber:       1,
				OffsetAndLengths: [][2]int32{{1, 2}},
			}},
		}, {
			input: ChunkMatch{
				Content:      "line1\nline2",
				ContentStart: Location{Line: 1},
				Ranges: Ranges{{
					Start: Location{0, 1, 0},
					End:   Location{6, 2, 0},
				}},
			},
			output: []*LineMatch{{
				Preview:          "line1",
				LineNumber:       1,
				OffsetAndLengths: [][2]int32{{0, 5}},
			}},
		}, {
			input: ChunkMatch{
				Content:      "line1\nline2\n",
				ContentStart: Location{Line: 1},
				Ranges: Ranges{{
					Start: Location{0, 1, 0},
					End:   Location{1, 1, 1},
				}},
			},
			output: []*LineMatch{{
				Preview:          "line1",
				LineNumber:       1,
				OffsetAndLengths: [][2]int32{{0, 1}},
			}},
		}}

		for _, tc := range cases {
			t.Run("", func(t *testing.T) {
				require.Equal(t, tc.output, tc.input.AsLineMatches())
			})
		}
	})

	t.Run("ChunkMatchesAsLineMatches", func(t *testing.T) {
		cases := []struct {
			input  ChunkMatches
			output []*LineMatch
		}{{
			input: ChunkMatches{{
				Content:      "line1\nline2\nline3\nline4",
				ContentStart: Location{Line: 1},
				Ranges: Ranges{{
					Start: Location{1, 1, 1},
					End:   Location{13, 3, 1},
				}, {
					Start: Location{7, 2, 1},
					End:   Location{13, 4, 1},
				}},
			}},
			output: []*LineMatch{{
				Preview:          "line1",
				LineNumber:       1,
				OffsetAndLengths: [][2]int32{{1, 4}},
			}, {
				Preview:          "line2",
				LineNumber:       2,
				OffsetAndLengths: [][2]int32{{0, 5}, {1, 4}},
			}, {
				Preview:          "line3",
				LineNumber:       3,
				OffsetAndLengths: [][2]int32{{0, 1}, {0, 5}},
			}, {
				Preview:          "line4",
				LineNumber:       4,
				OffsetAndLengths: [][2]int32{{0, 1}},
			}},
		}, {
			input: ChunkMatches{{
				Content:      "line1\nline2\nline3",
				ContentStart: Location{Line: 1},
				Ranges: Ranges{{
					Start: Location{1, 1, 1},
					End:   Location{13, 3, 1},
				}},
			}, {
				Content:      "line4\nline5\nline6",
				ContentStart: Location{Line: 4},
				Ranges: Ranges{{
					Start: Location{19, 4, 1},
					End:   Location{31, 6, 1},
				}},
			}},
			output: []*LineMatch{{
				Preview:          "line1",
				LineNumber:       1,
				OffsetAndLengths: [][2]int32{{1, 4}},
			}, {
				Preview:          "line2",
				LineNumber:       2,
				OffsetAndLengths: [][2]int32{{0, 5}},
			}, {
				Preview:          "line3",
				LineNumber:       3,
				OffsetAndLengths: [][2]int32{{0, 1}},
			}, {
				Preview:          "line4",
				LineNumber:       4,
				OffsetAndLengths: [][2]int32{{1, 4}},
			}, {
				Preview:          "line5",
				LineNumber:       5,
				OffsetAndLengths: [][2]int32{{0, 5}},
			}, {
				Preview:          "line6",
				LineNumber:       6,
				OffsetAndLengths: [][2]int32{{0, 1}},
			}},
		}, {
			input:  ChunkMatches{},
			output: []*LineMatch{},
		}}

		for _, tc := range cases {
			t.Run("", func(t *testing.T) {
				require.Equal(t, tc.output, tc.input.AsLineMatches())
			})
		}
	})
}

func TestChunkMatches_Limit(t *testing.T) {
	cases := []struct {
		rangeLens         []int
		limit             int
		expectedRangeLens []int
	}{{
		rangeLens:         []int{1, 1, 1},
		limit:             1,
		expectedRangeLens: []int{1},
	}, {
		rangeLens:         []int{1, 1, 1},
		limit:             3,
		expectedRangeLens: []int{1, 1, 1},
	}, {
		rangeLens:         []int{1, 1, 1},
		limit:             4,
		expectedRangeLens: []int{1, 1, 1},
	}, {
		rangeLens:         []int{2, 2, 2},
		limit:             4,
		expectedRangeLens: []int{2, 2},
	}, {
		rangeLens:         []int{2, 2, 2},
		limit:             3,
		expectedRangeLens: []int{2, 1},
	}, {
		rangeLens:         []int{2, 2, 2},
		limit:             1,
		expectedRangeLens: []int{1},
	}}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			var hs ChunkMatches
			for _, i := range tc.rangeLens {
				hs = append(hs, ChunkMatch{Ranges: make(Ranges, i)})
			}
			hs.Limit(tc.limit)
			var gotLens []int
			for _, h := range hs {
				gotLens = append(gotLens, len(h.Ranges))
			}
			require.Equal(t, tc.expectedRangeLens, gotLens)
		})
	}
}

func TestChunkMatches_MatchedContent(t *testing.T) {
	cases := []struct {
		input  ChunkMatch
		output []string
	}{{
		input: ChunkMatch{
			Content:      "abc",
			ContentStart: Location{0, 0, 0},
			Ranges: Ranges{{
				Start: Location{1, 0, 1},
				End:   Location{2, 0, 2},
			}},
		},
		output: []string{"b"},
	}, {
		input: ChunkMatch{
			Content:      "def",
			ContentStart: Location{4, 1, 0}, // abc\ndef
			Ranges: Ranges{{
				Start: Location{5, 1, 1},
				End:   Location{6, 1, 2},
			}},
		},
		output: []string{"e"},
	}, {
		input: ChunkMatch{
			Content:      "abc\ndef",
			ContentStart: Location{0, 0, 0},
			Ranges: Ranges{{
				Start: Location{2, 0, 2},
				End:   Location{5, 1, 1},
			}},
		},
		output: []string{"c\nd"},
	}, {
		input: ChunkMatch{
			Content:      "abc\ndef",
			ContentStart: Location{0, 0, 0},
			Ranges: Ranges{{
				Start: Location{0, 0, 0},
				End:   Location{2, 0, 2},
			}, {
				Start: Location{2, 0, 2},
				End:   Location{5, 1, 1},
			}, {
				Start: Location{5, 1, 1},
				End:   Location{7, 1, 3},
			}},
		},
		output: []string{"ab", "c\nd", "ef"},
	}, {
		input: ChunkMatch{
			Content:      "abc\ndef",
			ContentStart: Location{0, 0, 0},
			// Out of bounds ranges
			Ranges: Ranges{{
				// Starts at beginning, ends past end
				Start: Location{0, 0, 0},
				End:   Location{30, 3, 2},
			}, {
				// Starts in middle, ends past end
				Start: Location{4, 1, 0},
				End:   Location{1000, 1, 996},
			}, {
				// Starts past end, ends past end
				Start: Location{1000, 1000, 1000},
				End:   Location{2000, 2000, 2000},
			}},
		},
		output: []string{"abc\ndef", "def", ""},
	}}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			require.Equal(t, tc.output, tc.input.MatchedContent())
		})
	}
}

func TestFileMatch_Limit(t *testing.T) {
	tests := []struct {
		numHunkRanges       int
		numSymbolMatches    int
		limit               int
		expNumHunkRanges    int
		expNumSymbolMatches int
		expRemainingLimit   int
		wantLimitHit        bool
	}{
		{
			numHunkRanges:     3,
			numSymbolMatches:  0,
			limit:             1,
			expNumHunkRanges:  1,
			expRemainingLimit: 0,
			wantLimitHit:      true,
		},
		{
			numHunkRanges:       0,
			numSymbolMatches:    3,
			limit:               1,
			expNumSymbolMatches: 1,
			expRemainingLimit:   0,
			wantLimitHit:        true,
		},
		{
			numHunkRanges:     3,
			numSymbolMatches:  0,
			limit:             5,
			expNumHunkRanges:  3,
			expRemainingLimit: 2,
			wantLimitHit:      false,
		},
		{
			numHunkRanges:       0,
			numSymbolMatches:    3,
			limit:               5,
			expNumSymbolMatches: 3,
			expRemainingLimit:   2,
			wantLimitHit:        false,
		},
		{
			numHunkRanges:     3,
			numSymbolMatches:  0,
			limit:             3,
			expNumHunkRanges:  3,
			expRemainingLimit: 0,
			wantLimitHit:      false,
		},
		{
			numHunkRanges:       0,
			numSymbolMatches:    3,
			limit:               3,
			expNumSymbolMatches: 3,
			expRemainingLimit:   0,
			wantLimitHit:        false,
		},
		{
			// An empty FileMatch should still count against the limit
			numHunkRanges:       0,
			numSymbolMatches:    0,
			limit:               1,
			expNumSymbolMatches: 0,
			expNumHunkRanges:    0,
			wantLimitHit:        false,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			fileMatch := &FileMatch{
				File:         File{},
				ChunkMatches: ChunkMatches{{Ranges: make(Ranges, tt.numHunkRanges)}},
				Symbols:      make([]*SymbolMatch, tt.numSymbolMatches),
				LimitHit:     false,
			}

			got := fileMatch.Limit(tt.limit)

			require.Equal(t, tt.expNumHunkRanges, fileMatch.ChunkMatches.MatchCount())
			require.Equal(t, tt.expNumSymbolMatches, len(fileMatch.Symbols))
			require.Equal(t, tt.expRemainingLimit, got)
			require.Equal(t, tt.wantLimitHit, fileMatch.LimitHit)
		})
	}
}
