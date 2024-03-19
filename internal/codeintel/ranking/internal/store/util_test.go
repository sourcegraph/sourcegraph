package store

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestBatchChannel(t *testing.T) {
	ch := make(chan int, 10)
	for i := range 10 {
		ch <- i
	}
	close(ch)

	batches := [][]int{}
	for batch := range batchChannel(ch, 3) {
		batches = append(batches, batch)
	}

	if diff := cmp.Diff([][]int{{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, {9}}, batches); diff != "" {
		t.Errorf("unexpected batches (-want +got):\n%s", diff)
	}
}
