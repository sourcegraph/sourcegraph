package maps

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

type testCase[K comparable, V any] struct {
	name string
	src  map[K]V
	dest map[K]V
	want map[K]V
}

func runMergeTest[K comparable, V any](t *testing.T, tt testCase[K, V]) {
	t.Helper()

	t.Run(tt.name, func(t *testing.T) {
		t.Parallel()
		got := Merge(tt.dest, tt.src)
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("Merge() mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestMerge(t *testing.T) {
	t.Parallel()

	strIntTests := []testCase[string, int]{
		{
			name: "merge with overlapping keys",
			dest: map[string]int{"a": 1, "b": 2},
			src:  map[string]int{"b": 3, "c": 4},
			want: map[string]int{"a": 1, "b": 3, "c": 4},
		},
		{
			name: "src is nil",
			dest: map[string]int{"a": 1},
			src:  nil,
			want: map[string]int{"a": 1},
		},
		{
			name: "dest is nil",
			dest: nil,
			src:  map[string]int{"a": 1},
			want: map[string]int{"a": 1},
		},
		{
			name: "both are nil",
			dest: nil,
			src:  nil,
			want: map[string]int{},
		},
	}

	strStrTests := []testCase[string, string]{
		{
			name: "merge with overlapping keys",
			dest: map[string]string{"a": "1", "b": "2"},
			src:  map[string]string{"b": "3", "c": "4"},
			want: map[string]string{"a": "1", "b": "3", "c": "4"},
		},
		{
			name: "rc is nil",
			dest: map[string]string{"a": "1"},
			src:  nil,
			want: map[string]string{"a": "1"},
		},
		{
			name: "dest is nil",
			dest: nil,
			src:  map[string]string{"a": "1"},
			want: map[string]string{"a": "1"},
		},
		{
			name: "both are nil",
			dest: nil,
			src:  nil,
			want: map[string]string{},
		},
	}

	for _, tt := range strIntTests {
		tt := tt
		runMergeTest(t, tt)
	}

	for _, tt := range strStrTests {
		tt := tt
		runMergeTest(t, tt)
	}
}

func runMergePreservingExistingKeysTest[K comparable, V any](t *testing.T, tt testCase[K, V]) {
	t.Helper()

	t.Run(tt.name, func(t *testing.T) {
		t.Parallel()
		got := MergePreservingExistingKeys(tt.dest, tt.src)
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("MergePreservingExistingKeys() mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestMergePreservingExistingKeys(t *testing.T) {
	t.Parallel()

	strIntTests := []testCase[string, int]{
		{
			name: "merge with overlapping keys",
			dest: map[string]int{"a": 1, "b": 2},
			src:  map[string]int{"b": 3, "c": 4},
			want: map[string]int{"a": 1, "b": 2, "c": 4},
		},
		{
			name: "src is nil",
			dest: map[string]int{"a": 1},
			src:  nil,
			want: map[string]int{"a": 1},
		},
		{
			name: "dest is nil",
			dest: nil,
			src:  map[string]int{"a": 1},
			want: map[string]int{"a": 1},
		},
		{
			name: "both are nil",
			dest: nil,
			src:  nil,
			want: map[string]int{},
		},
	}

	strStrTests := []testCase[string, string]{
		{
			name: "merge with overlapping keys",
			dest: map[string]string{"a": "1", "b": "2"},
			src:  map[string]string{"b": "3", "c": "4"},
			want: map[string]string{"a": "1", "b": "2", "c": "4"},
		},
		{
			name: "src is nil",
			dest: map[string]string{"a": "1"},
			src:  nil,
			want: map[string]string{"a": "1"},
		},
		{
			name: "dest is nil",
			dest: nil,
			src:  map[string]string{"a": "1"},
			want: map[string]string{"a": "1"},
		},
		{
			name: "both are nil",
			dest: nil,
			src:  nil,
			want: map[string]string{},
		},
	}

	for _, tt := range strIntTests {
		tt := tt
		runMergePreservingExistingKeysTest(t, tt)
	}

	for _, tt := range strStrTests {
		tt := tt
		runMergePreservingExistingKeysTest(t, tt)
	}
}
