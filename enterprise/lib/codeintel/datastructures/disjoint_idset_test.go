package datastructures

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDisjointIDSetExtract(t *testing.T) {
	s := NewDisjointIDSet()
	s.Link(1, 2)
	s.Link(3, 4)
	s.Link(1, 3)
	s.Link(5, 6)

	setA := []int{1, 2, 3, 4}
	setB := []int{5, 6}

	for _, i := range setA {
		if diff := cmp.Diff(IDSetWith(setA...), s.ExtractSet(i), Comparers...); diff != "" {
			t.Errorf("unexpected set (-want +got):\n%s", diff)
		}
	}

	for _, i := range setB {
		if diff := cmp.Diff(IDSetWith(setB...), s.ExtractSet(i), Comparers...); diff != "" {
			t.Errorf("unexpected set (-want +got):\n%s", diff)
		}
	}
}

func TestDisjointIDSetExtractEmptyReturnsValue(t *testing.T) {
	s := NewDisjointIDSet()
	s.Link(1, 2)
	s.Link(2, 3)
	s.Link(3, 4)

	if diff := cmp.Diff(IDSetWith(5), s.ExtractSet(5), Comparers...); diff != "" {
		t.Errorf("unexpected set (-want +got):\n%s", diff)
	}
}
