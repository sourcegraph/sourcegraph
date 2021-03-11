package semantic

import (
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFindRanges(t *testing.T) {
	ranges := []RangeData{
		{
			StartLine:      0,
			StartCharacter: 3,
			EndLine:        0,
			EndCharacter:   5,
		},
		{
			StartLine:      1,
			StartCharacter: 3,
			EndLine:        1,
			EndCharacter:   5,
		},
		{
			StartLine:      2,
			StartCharacter: 3,
			EndLine:        2,
			EndCharacter:   5,
		},
		{
			StartLine:      3,
			StartCharacter: 3,
			EndLine:        3,
			EndCharacter:   5,
		},
		{
			StartLine:      4,
			StartCharacter: 3,
			EndLine:        4,
			EndCharacter:   5,
		},
	}

	m := map[ID]RangeData{}
	for i, r := range ranges {
		m[ID(strconv.Itoa(i))] = r
	}

	for i, r := range ranges {
		actual := FindRanges(m, i, 4)
		expected := []RangeData{r}
		if diff := cmp.Diff(expected, actual); diff != "" {
			t.Errorf("unexpected findRanges result %d (-want +got):\n%s", i, diff)
		}
	}
}

func TestFindRangesOrder(t *testing.T) {
	ranges := []RangeData{
		{
			StartLine:      0,
			StartCharacter: 3,
			EndLine:        4,
			EndCharacter:   5,
		},
		{
			StartLine:      1,
			StartCharacter: 3,
			EndLine:        3,
			EndCharacter:   5,
		},
		{
			StartLine:      2,
			StartCharacter: 3,
			EndLine:        2,
			EndCharacter:   5,
		},
		{
			StartLine:      5,
			StartCharacter: 3,
			EndLine:        5,
			EndCharacter:   5,
		},
		{
			StartLine:      6,
			StartCharacter: 3,
			EndLine:        6,
			EndCharacter:   5,
		},
	}

	m := map[ID]RangeData{}
	for i, r := range ranges {
		m[ID(strconv.Itoa(i))] = r
	}

	actual := FindRanges(m, 2, 4)
	expected := []RangeData{ranges[2], ranges[1], ranges[0]}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("unexpected findRanges result (-want +got):\n%s", diff)
	}

}

func TestComparePosition(t *testing.T) {
	left := RangeData{
		StartLine:      5,
		StartCharacter: 11,
		EndLine:        5,
		EndCharacter:   13,
	}

	testCases := []struct {
		line      int
		character int
		expected  int
	}{
		{5, 11, 0},
		{5, 12, 0},
		{5, 13, 0},
		{4, 12, +1},
		{5, 10, +1},
		{5, 14, -1},
		{6, 12, -1},
	}

	for _, testCase := range testCases {
		if cmp := ComparePosition(left, testCase.line, testCase.character); cmp != testCase.expected {
			t.Errorf("unexpected comparisonPosition result for %d:%d. want=%d have=%d", testCase.line, testCase.character, testCase.expected, cmp)
		}
	}
}

func TestRangeIntersectsSpan(t *testing.T) {
	testCases := []struct {
		startLine int
		endLine   int
		expected  bool
	}{
		{startLine: 1, endLine: 4, expected: false},
		{startLine: 7, endLine: 9, expected: false},
		{startLine: 1, endLine: 6, expected: true},
		{startLine: 6, endLine: 7, expected: true},
	}

	r := RangeData{StartLine: 5, StartCharacter: 1, EndLine: 6, EndCharacter: 10}

	for _, testCase := range testCases {
		if val := RangeIntersectsSpan(r, testCase.startLine, testCase.endLine); val != testCase.expected {
			t.Errorf("unexpected result. want=%v have=%v", testCase.expected, val)
		}
	}
}
