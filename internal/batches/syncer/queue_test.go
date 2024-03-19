package syncer

import (
	"container/heap"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestChangesetPriorityQueue(t *testing.T) {
	t.Parallel()

	assertOrder := func(t *testing.T, q *changesetPriorityQueue, expected []int64) {
		t.Helper()
		ids := make([]int64, len(q.items))
		for i := range ids {
			ids[i] = q.items[i].changesetID
		}
		if diff := cmp.Diff(expected, ids); diff != "" {
			t.Fatal(diff)
		}
	}

	now := time.Now()
	q := newChangesetPriorityQueue()

	items := []scheduledSync{
		{
			changesetID: 1,
			nextSync:    now,
			priority:    priorityNormal,
		},
		{
			changesetID: 2,
			nextSync:    now,
			priority:    priorityHigh,
		},
		{
			changesetID: 3,
			nextSync:    now.Add(-1 * time.Minute),
			priority:    priorityNormal,
		},
		{
			changesetID: 4,
			nextSync:    now.Add(-2 * time.Hour),
			priority:    priorityNormal,
		},
		{
			changesetID: 5,
			nextSync:    now.Add(1 * time.Hour),
			priority:    priorityNormal,
		},
	}

	for i := range items {
		q.Upsert(items[i])
	}

	assertOrder(t, q, []int64{2, 4, 3, 1, 5})

	// Set item to high priority
	q.Upsert(scheduledSync{
		changesetID: 4,
		nextSync:    now.Add(-2 * time.Hour),
		priority:    priorityHigh,
	})

	assertOrder(t, q, []int64{4, 2, 3, 1, 5})

	// Can't reduce priority of existing item
	q.Upsert(scheduledSync{
		changesetID: 4,
		nextSync:    now.Add(-2 * time.Hour),
		priority:    priorityNormal,
	})

	if q.Len() != len(items) {
		t.Fatalf("Expected %d, got %d", q.Len(), len(items))
	}

	assertOrder(t, q, []int64{4, 2, 3, 1, 5})

	for range len(items) {
		peeked, ok := q.Peek()
		if !ok {
			t.Fatalf("Queue should not be empty")
		}
		item := heap.Pop(q).(scheduledSync)
		if peeked.changesetID != item.changesetID {
			t.Fatalf("Peeked and Popped item should have the same id")
		}
	}

	// Len() should be zero after all items popped
	if q.Len() != 0 {
		t.Fatalf("Expected %d, got %d", q.Len(), 0)
	}
}
