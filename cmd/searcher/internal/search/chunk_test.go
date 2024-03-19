package search

import (
	"bytes"
	"testing"
	"testing/quick"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/searcher/protocol"
)

func Test_chunkRanges(t *testing.T) {
	cases := []struct {
		ranges         []protocol.Range
		mergeThreshold int32
		output         []rangeChunk
	}{{
		// Single range
		ranges: []protocol.Range{{
			Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
			End:   protocol.Location{Offset: 20, Line: 1, Column: 10},
		}},
		mergeThreshold: 0,
		output: []rangeChunk{{
			cover: protocol.Range{
				Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
				End:   protocol.Location{Offset: 20, Line: 1, Column: 10},
			},
			ranges: []protocol.Range{{
				Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
				End:   protocol.Location{Offset: 20, Line: 1, Column: 10},
			}},
		}},
	}, {
		// Overlapping ranges
		ranges: []protocol.Range{{
			Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
			End:   protocol.Location{Offset: 20, Line: 1, Column: 10},
		}, {
			Start: protocol.Location{Offset: 5, Line: 0, Column: 5},
			End:   protocol.Location{Offset: 25, Line: 1, Column: 15},
		}},
		mergeThreshold: 0,
		output: []rangeChunk{{
			cover: protocol.Range{
				Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
				End:   protocol.Location{Offset: 25, Line: 1, Column: 15},
			},
			ranges: []protocol.Range{{
				Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
				End:   protocol.Location{Offset: 20, Line: 1, Column: 10},
			}, {
				Start: protocol.Location{Offset: 5, Line: 0, Column: 5},
				End:   protocol.Location{Offset: 25, Line: 1, Column: 15},
			}},
		}},
	}, {
		// Non-overlapping ranges, but share a line
		ranges: []protocol.Range{{
			Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
			End:   protocol.Location{Offset: 20, Line: 1, Column: 10},
		}, {
			Start: protocol.Location{Offset: 25, Line: 1, Column: 15},
			End:   protocol.Location{Offset: 35, Line: 2, Column: 5},
		}},
		mergeThreshold: 0,
		output: []rangeChunk{{
			cover: protocol.Range{
				Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
				End:   protocol.Location{Offset: 35, Line: 2, Column: 5},
			},
			ranges: []protocol.Range{{
				Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
				End:   protocol.Location{Offset: 20, Line: 1, Column: 10},
			}, {
				Start: protocol.Location{Offset: 25, Line: 1, Column: 15},
				End:   protocol.Location{Offset: 35, Line: 2, Column: 5},
			}},
		}},
	}, {
		// Ranges on adjacent lines, but not merged because of low merge threshold
		ranges: []protocol.Range{{
			Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
			End:   protocol.Location{Offset: 10, Line: 0, Column: 10},
		}, {
			Start: protocol.Location{Offset: 11, Line: 1, Column: 0},
			End:   protocol.Location{Offset: 20, Line: 1, Column: 9},
		}},
		mergeThreshold: 0,
		output: []rangeChunk{{
			cover: protocol.Range{
				Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
				End:   protocol.Location{Offset: 10, Line: 0, Column: 10},
			},
			ranges: []protocol.Range{{
				Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
				End:   protocol.Location{Offset: 10, Line: 0, Column: 10},
			}},
		}, {
			cover: protocol.Range{
				Start: protocol.Location{Offset: 11, Line: 1, Column: 0},
				End:   protocol.Location{Offset: 20, Line: 1, Column: 9},
			},
			ranges: []protocol.Range{{
				Start: protocol.Location{Offset: 11, Line: 1, Column: 0},
				End:   protocol.Location{Offset: 20, Line: 1, Column: 9},
			}},
		}},
	}, {
		// Ranges on adjacent lines, merged because of high merge threshold
		ranges: []protocol.Range{{
			Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
			End:   protocol.Location{Offset: 10, Line: 0, Column: 10},
		}, {
			Start: protocol.Location{Offset: 11, Line: 1, Column: 0},
			End:   protocol.Location{Offset: 20, Line: 1, Column: 9},
		}},
		mergeThreshold: 1,
		output: []rangeChunk{{
			cover: protocol.Range{
				Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
				End:   protocol.Location{Offset: 20, Line: 1, Column: 9},
			},
			ranges: []protocol.Range{{
				Start: protocol.Location{Offset: 0, Line: 0, Column: 0},
				End:   protocol.Location{Offset: 10, Line: 0, Column: 10},
			}, {
				Start: protocol.Location{Offset: 11, Line: 1, Column: 0},
				End:   protocol.Location{Offset: 20, Line: 1, Column: 9},
			}},
		}},
	}}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			got := chunkRanges(tc.ranges, tc.mergeThreshold)
			require.Equal(t, tc.output, got)
		})
	}
}

func Test_addContext(t *testing.T) {
	l := func(offset, line, column int32) protocol.Location {
		return protocol.Location{Offset: offset, Line: line, Column: column}
	}

	r := func(start, end protocol.Location) protocol.Range {
		return protocol.Range{Start: start, End: end}
	}

	testCases := []struct {
		file         string
		contextLines int32
		inputRange   protocol.Range
		expected     string
	}{
		{
			"",
			0,
			r(l(0, 0, 0), l(0, 0, 0)),
			"",
		},
		{
			"",
			1,
			r(l(0, 0, 0), l(0, 0, 0)),
			"",
		},
		{
			"\n",
			0,
			r(l(0, 0, 0), l(0, 0, 0)),
			"",
		},
		{
			"\n",
			1,
			r(l(0, 0, 0), l(0, 0, 0)),
			"",
		},
		{
			"\n\n\n",
			0,
			r(l(1, 1, 0), l(1, 1, 0)),
			"",
		},
		{
			"\n\n\n\n",
			1,
			r(l(1, 1, 0), l(1, 1, 0)),
			"\n\n",
		},
		{
			"\n\n\n\n",
			2,
			r(l(1, 1, 0), l(1, 1, 0)),
			"\n\n\n",
		},
		{
			"abc\ndef\nghi\n",
			0,
			r(l(1, 0, 1), l(1, 0, 1)),
			"abc",
		},
		{
			"abc\ndef\nghi\n",
			1,
			r(l(1, 0, 1), l(1, 0, 1)),
			"abc\ndef",
		},
		{
			"abc\ndef\nghi\n",
			2,
			r(l(1, 0, 1), l(1, 0, 1)),
			"abc\ndef\nghi",
		},
		{
			"abc\ndef\nghi",
			0,
			r(l(1, 0, 1), l(1, 0, 1)),
			"abc",
		},
		{
			"abc\ndef\nghi",
			1,
			r(l(1, 0, 1), l(1, 0, 1)),
			"abc\ndef",
		},
		{
			"abc\ndef\nghi",
			2,
			r(l(1, 0, 1), l(1, 0, 1)),
			"abc\ndef\nghi",
		},
		{
			"abc\ndef\nghi",
			2,
			r(l(5, 1, 1), l(6, 1, 2)),
			"abc\ndef\nghi",
		},
		{
			"abc",
			0,
			r(l(1, 0, 1), l(2, 0, 2)),
			"abc",
		},
		{
			"abc",
			1,
			r(l(1, 0, 1), l(2, 0, 2)),
			"abc",
		},
		{
			"abc\r\ndef\r\nghi\r\n",
			1,
			r(l(1, 0, 1), l(2, 0, 2)),
			"abc\r\ndef",
		},
		{
			"abc\r\ndef\r\nghi",
			3,
			r(l(1, 0, 1), l(2, 0, 2)),
			"abc\r\ndef\r\nghi",
		},
		{
			"\r\n",
			0,
			r(l(0, 0, 0), l(0, 0, 0)),
			"",
		},
		{
			"\r\n",
			1,
			r(l(0, 0, 0), l(0, 0, 0)),
			"",
		},
		{
			"abc\nd\xE2\x9D\x89f\nghi",
			0,
			r(l(4, 1, 0), l(5, 1, 1)),
			"d\xE2\x9D\x89f",
		},
		{
			"abc\nd\xE2\x9D\x89f\nghi",
			1,
			r(l(4, 1, 0), l(5, 1, 1)),
			"abc\nd\xE2\x9D\x89f\nghi",
		},
	}

	for _, testCase := range testCases {
		t.Run("", func(t *testing.T) {
			buf := []byte(testCase.file)
			extendedRange := extendRangeToLines(testCase.inputRange, buf)
			contextedRange := addContextLines(extendedRange, buf, testCase.contextLines)
			require.Equal(t, testCase.expected, string(buf[contextedRange.Start.Offset:contextedRange.End.Offset]))
		})
	}
}

func TestColumnHelper(t *testing.T) {
	f := func(line0, line1 string) bool {
		data := []byte(line0 + line1)
		lineOffset := len(line0)

		columnHelper := columnHelper{data: data}

		// We check every second rune returns the correct answer
		offset := lineOffset
		column := 0
		for offset < len(data) {
			if column%2 == 0 {
				got := columnHelper.get(lineOffset, offset)
				if got != column {
					return false
				}
			}
			_, size := utf8.DecodeRune(data[offset:])
			offset += size
			column++
		}

		return true
	}

	if err := quick.Check(f, nil); err != nil {
		t.Fatal(err)
	}

	// Corner cases

	// empty data, shouldn't happen but just in case it slips through
	ch := columnHelper{data: nil}
	if got := ch.get(0, 0); got != 0 {
		t.Fatal("empty data didn't return 1", got)
	}

	// Repeating a call to get should return the same value
	// empty data, shouldn't happen but just in case it slips through
	ch = columnHelper{data: []byte("hello\nworld")}
	if got := ch.get(6, 8); got != 2 {
		t.Fatal("unexpected value for third column on second line", got)
	}
	if got := ch.get(6, 8); got != 2 {
		t.Fatal("unexpected value for repeated call for third column on second line", got)
	}

	// Now make sure if we go backwards we do not incorrectly use the cache
	if got := ch.get(6, 6); got != 0 {
		t.Fatal("unexpected value for backwards call for first column on second line", got)
	}
}

func BenchmarkColumnHelper(b *testing.B) {
	// We simulate looking up columns of evenly spaced matches
	const matches = 10_000
	const match = "match"
	const space = "         "
	const dist = len(match) + len(space)
	data := bytes.Repeat([]byte(match+space), matches)

	b.ResetTimer()

	for range b.N {
		columnHelper := columnHelper{data: data}

		lineOffset := 0
		offset := 0
		for offset < len(data) {
			col := columnHelper.get(lineOffset, offset)
			if col != offset+1 {
				b.Fatal("column is not offset even though data is ASCII")
			}
			offset += dist
		}
	}
}
