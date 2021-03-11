package semantic

import "sort"

// FindRanges filters the given ranges and returns those that contain the position constructed
// from line and character. The order of the output slice is "outside-in", so that earlier
// ranges properly enclose later ranges.
func FindRanges(ranges map[ID]RangeData, line, character int) []RangeData {
	var filtered []RangeData
	for _, r := range ranges {
		if ComparePosition(r, line, character) == 0 {
			filtered = append(filtered, r)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return ComparePosition(filtered[i], filtered[j].StartLine, filtered[j].StartCharacter) != 0
	})

	return filtered
}

// FindRangesInWIndow filters the given ranges and returns those that intersect with the
// given window of lines. Ranges are returned in reading order (top-down/left-right).
func FindRangesInWindow(ranges map[ID]RangeData, startLine, endLine int) []RangeData {
	var filtered []RangeData
	for _, r := range ranges {
		if RangeIntersectsSpan(r, startLine, endLine) {
			filtered = append(filtered, r)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return CompareRanges(filtered[i], filtered[j]) < 0
	})

	return filtered
}

// CompareRanges compares two ranges.
// Returns -1 if the range A starts before range B, or starts at the same place but ends earlier.
// Returns 0 if they're exactly equal. Returns 1 otherwise.
func CompareRanges(a RangeData, b RangeData) int {
	if a.StartLine < b.StartLine {
		return -1
	}

	if a.StartLine > b.StartLine {
		return 1
	}

	if a.StartCharacter < b.StartCharacter {
		return -1
	}

	if a.StartCharacter > b.StartCharacter {
		return 1
	}

	if a.EndLine < b.EndLine {
		return -1
	}

	if a.EndLine > b.EndLine {
		return 1
	}

	if a.EndCharacter < b.EndCharacter {
		return -1
	}

	if a.EndCharacter > b.EndCharacter {
		return 1
	}

	return 0
}

// ComparePosition compres the range r with the position constructed from line and character.
// Returns -1 if the position occurs before the range, +1 if it occurs after, and 0 if the
// position is inside of the range.
func ComparePosition(r RangeData, line, character int) int {
	if line < r.StartLine {
		return 1
	}

	if line > r.EndLine {
		return -1
	}

	if line == r.StartLine && character < r.StartCharacter {
		return 1
	}

	if line == r.EndLine && character > r.EndCharacter {
		return -1
	}

	return 0
}

// RangeIntersectsSpan determines if the given range falls within the window denoted by the
// given start and end lines.
func RangeIntersectsSpan(r RangeData, startLine, endLine int) bool {
	return (startLine <= r.StartLine && r.StartLine < endLine) || (startLine <= r.EndLine && r.EndLine < endLine)
}
