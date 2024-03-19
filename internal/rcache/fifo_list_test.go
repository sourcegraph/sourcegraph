package rcache

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_FIFOList_All_OK(t *testing.T) {
	SetupForTest(t)

	type testcase struct {
		key     string
		size    int
		inserts [][]byte
		want    [][]byte
	}

	cases := []testcase{
		{
			key:     "a",
			size:    3,
			inserts: bytes("a1", "a2", "a3"),
			want:    bytes("a3", "a2", "a1"),
		},
		{
			key:     "b",
			size:    3,
			inserts: bytes("a1", "a2", "a3", "a4", "a5", "a6"),
			want:    bytes("a6", "a5", "a4"),
		},
		{
			key:     "c",
			size:    3,
			inserts: bytes("a1", "a2"),
			want:    bytes("a2", "a1"),
		},
		{
			key:     "d",
			size:    0,
			inserts: bytes("a1", "a2", "a3"),
			want:    bytes(),
		},
		{
			key:     "f",
			size:    -1,
			inserts: bytes("a1", "a2", "a3"),
			want:    bytes(),
		},
	}

	for _, c := range cases {
		r := NewFIFOList(c.key, c.size)
		t.Run(fmt.Sprintf("size %d with %d entries", c.size, len(c.inserts)), func(t *testing.T) {
			for _, b := range c.inserts {
				if err := r.Insert(b); err != nil {
					t.Errorf("expected no error, got %q", err)
				}
			}
			got, err := r.All(context.Background())
			if err != nil {
				t.Errorf("expected no error, got %q", err)
			}
			s, err := r.Size()
			if err != nil {
				t.Errorf("expected no error, got %q", err)
			}
			if s != len(c.want) {
				t.Errorf("expected %d items, got %d instead", s, len(c.want))
			}
			if !reflect.DeepEqual(c.want, got) {
				t.Errorf("Expected %v, but got %v", str(c.want...), str(got...))
			}
		})
	}
}

func Test_FIFOList_Slice_OK(t *testing.T) {
	SetupForTest(t)

	type testcase struct {
		key     string
		size    int
		inserts [][]byte
		want    [][]byte
		from    int
		to      int
	}

	cases := []testcase{
		{
			key:     "a",
			size:    3,
			inserts: bytes("a1", "a2", "a3"),
			want:    bytes("a3", "a2", "a1"),
			from:    0,
			to:      -1,
		},
		{
			key:     "b",
			size:    3,
			inserts: bytes("a1", "a2", "a3"),
			want:    bytes("a2", "a1"),
			from:    1,
			to:      2,
		},
		{
			key:     "c",
			size:    3,
			inserts: bytes("a1", "a2", "a3", "a4", "a5", "a6"),
			want:    bytes("a5", "a4"),
			from:    1,
			to:      2,
		},
		{
			key:     "d",
			size:    0,
			inserts: bytes("a1", "a2", "a3"),
			want:    bytes(),
			from:    0,
			to:      -1,
		},
		{
			key:     "e",
			size:    3,
			inserts: bytes("a1", "a2", "a3", "a4", "a5", "a6"),
			want:    bytes("a4"),
			from:    2,
			to:      -1,
		},
		{
			key:     "f",
			size:    -1,
			inserts: bytes("a1", "a2", "a3"),
			want:    bytes(),
			from:    0,
			to:      -1,
		},
	}

	for _, c := range cases {
		r := NewFIFOList(c.key, c.size)
		t.Run(fmt.Sprintf("size %d with %d entries, [%d,%d]", c.size, len(c.inserts), c.from, c.to), func(t *testing.T) {
			for _, b := range c.inserts {
				if err := r.Insert(b); err != nil {
					t.Errorf("expected no error, got %q", err)
				}
			}
			got, err := r.Slice(context.Background(), c.from, c.to)
			if err != nil {
				t.Errorf("expected no error, got %q", err)
			}
			if !reflect.DeepEqual(c.want, got) {
				t.Errorf("Expected %v, but got %v", str(c.want...), str(got...))
			}
		})
	}
}

func Test_NewFIFOListDynamic(t *testing.T) {
	SetupForTest(t)
	maxSize := 3
	r := NewFIFOListDynamic("a", func() int { return maxSize })
	for range 10 {
		err := r.Insert([]byte("a"))
		if err != nil {
			t.Errorf("expected no error, got %q", err)
		}
	}

	got, err := r.Slice(context.Background(), 0, -1)
	if err != nil {
		t.Errorf("expected no error, got %q", err)
	}
	if want := bytes("a", "a", "a"); !reflect.DeepEqual(want, got) {
		t.Errorf("expected %v, but got %v", str(want...), str(got...))
	}

	maxSize = 2
	for range 10 {
		err := r.Insert([]byte("b"))
		if err != nil {
			t.Errorf("expected no error, got %q", err)
		}
	}

	got, err = r.Slice(context.Background(), 0, -1)
	if err != nil {
		t.Errorf("expected no error, got %q", err)
	}
	if want := bytes("b", "b"); !reflect.DeepEqual(want, got) {
		t.Errorf("expected %v, but got %v", str(want...), str(got...))
	}
}

func Test_FIFOListContextCancellation(t *testing.T) {
	SetupForTest(t)
	r := NewFIFOList("a", 3)
	err := r.Insert([]byte("a"))
	if err != nil {
		t.Errorf("expected no error, got %q", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = r.All(ctx)
	if err == nil {
		t.Fatal("expected error, got none")
	}
}

func Test_FIFOListIsEmpty(t *testing.T) {
	SetupForTest(t)
	r := NewFIFOList("a", 3)
	empty, err := r.IsEmpty()
	require.NoError(t, err)
	assert.True(t, empty)
	err = r.Insert([]byte("a"))
	require.NoError(t, err)
	empty, err = r.IsEmpty()
	require.NoError(t, err)
	assert.False(t, empty)
}

func str(bs ...[]byte) []string {
	strs := make([]string, 0, len(bs))
	for _, b := range bs {
		strs = append(strs, string(b))
	}
	return strs
}
