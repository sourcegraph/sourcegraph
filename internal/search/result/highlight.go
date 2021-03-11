package result

type HighlightedRange struct {
	Line      int32
	Character int32
	Length    int32
}

type HighlightedString struct {
	Value      string
	Highlights []HighlightedRange
}
