package datastructures

import (
	"fmt"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestIDSetAdd(t *testing.T) {
	for _, max := range []int{SmallSetThreshold / 2, SmallSetThreshold, SmallSetThreshold * 16} {
		name := fmt.Sprintf("max=%d", max)

		t.Run(name, func(t *testing.T) {
			ids := NewIDSet()
			for i := 1; i <= max; i++ {
				ids.Add(i)
			}

			if ids.Len() != max {
				t.Errorf("unexpected length. want=%d have=%d", max, ids.Len())
			}

			for i := 1; i <= max; i++ {
				if !ids.Contains(i) {
					t.Errorf("unexpected contains. want=%v have=%v", true, ids.Contains(i))
				}
			}
		})
	}
}

func TestIDSetUnion(t *testing.T) {
	for _, max := range []int{16, 10000} {
		name := fmt.Sprintf("max=%d", max)

		t.Run(name, func(t *testing.T) {
			ids1 := NewIDSet()
			ids2 := NewIDSet()
			for i := 1; i <= max; i++ {
				if i%2 == 0 {
					ids1.Add(i)
				}
				if i%3 == 0 {
					ids2.Add(i)
				}
			}

			ids1.Union(nil)
			ids1.Union(ids2)

			if ids1.Len() != (max/2)+(max/3)-(max/6) {
				t.Errorf("unexpected length. want=%d have=%d", (max/2)+(max/3)-(max/6), ids1.Len())
			}

			for i := 1; i <= max/2; i++ {
				expected := (i%2 == 0) || (i%3 == 0)

				if ids1.Contains(i) != expected {
					t.Errorf("unexpected contains. want=%v have=%v", expected, ids1.Contains(i))
				}
			}
		})
	}
}

func TestIDSetMin(t *testing.T) {
	testCases := []struct {
		add int
		min int
	}{
		{5, 5},
		{6, 5},
		{4, 4},
	}

	for _, numUpperValues := range []int{0, 1000} {
		ids := NewIDSet()

		for i := 1; i <= numUpperValues; i++ {
			ids.Add(1000 + i)
		}

		for _, testCase := range testCases {
			ids.Add(testCase.add)
			if val, ok := ids.Min(); !ok {
				t.Errorf("unexpected not ok")
			} else if val != testCase.min {
				t.Errorf("unexpected min. want=%d have=%d", testCase.min, val)
			}
		}
	}
}

func TestIDSetMinEmpty(t *testing.T) {
	ids := NewIDSet()
	if _, ok := ids.Min(); ok {
		t.Errorf("unexpected ok")
	}
}

func TestIDSetPop(t *testing.T) {
	small := []int{1, 2, 3, 4, 5}

	large := make([]int, 0, 10000)
	for i := 1; i <= 10000; i++ {
		large = append(large, i)
	}

	for _, values := range [][]int{small, large} {
		set := IDSetWith(values...)

		popped := []int{}
		for i := 1; i <= len(values); i++ {
			var v int
			if !set.Pop(&v) {
				t.Fatalf("failed to pop")
			}

			if set.Contains(v) {
				t.Errorf("set contains popped element")
			}

			popped = append(popped, v)
		}
		sort.Ints(popped)

		if diff := cmp.Diff(values, popped); diff != "" {
			t.Errorf("unexpected values (-want +got):\n%s", diff)
		}
	}
}

func TestIDSetPopEmpty(t *testing.T) {
	set := NewIDSet()

	var v int
	if set.Pop(&v) {
		t.Fatalf("unexpected pop")
	}
}
