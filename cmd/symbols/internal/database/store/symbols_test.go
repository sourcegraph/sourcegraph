package store

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestChunksOf(t *testing.T) {
	if chunksOf1000(nil) != nil {
		t.Fatalf("got %v, want nil", chunksOf1000(nil))
	}

	if diff := cmp.Diff([][]string{}, chunksOf1000([]string{})); diff != "" {
		t.Fatalf("unexpected chunks (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([][]string{{"foo"}}, chunksOf1000([]string{"foo"})); diff != "" {
		t.Fatalf("unexpected chunks (-want +got):\n%s", diff)
	}

	strings := []string{}
	for range 1001 {
		strings = append(strings, "foo")
	}
	chunks := chunksOf1000(strings)
	if len(chunks) != 2 {
		t.Fatalf("got %v, want 2", len(chunks))
	}
	if len(chunks[0]) != 1000 {
		t.Fatalf("got %v, want 1000", len(chunks[0]))
	}
	if len(chunks[1]) != 1 {
		t.Fatalf("got %v, want 1", len(chunks[1]))
	}
}
