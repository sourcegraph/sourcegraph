package syncer

import (
	"container/heap"
	"time"
)

type scheduledSync struct {
	changesetID int64
	nextSync    time.Time
	priority    priority
}

// changesetPriorityQueue is a min heap that sorts syncs by priority
// and time of next sync. It is not safe for concurrent use.
type changesetPriorityQueue struct {
	items []scheduledSync
	index map[int64]int // changesetID -> index
}

// newChangesetPriorityQueue creates a new queue for holding changeset sync instructions in chronological order.
// items with a high priority will always appear at the front of the queue.
func newChangesetPriorityQueue() *changesetPriorityQueue {
	q := &changesetPriorityQueue{
		items: make([]scheduledSync, 0),
		index: make(map[int64]int),
	}
	heap.Init(q)
	return q
}

// The following methods implement heap.Interface based on the priority queue example:
// https://golang.org/pkg/container/heap/#example__priorityQueue

func (pq *changesetPriorityQueue) Len() int { return len(pq.items) }

func (pq *changesetPriorityQueue) Less(i, j int) bool {
	// We want items ordered by priority, then NextSync
	// Order by priority and then NextSync
	a := pq.items[i]
	b := pq.items[j]

	if a.priority != b.priority {
		// Greater than here since we want high priority items to be ranked before low priority
		return a.priority > b.priority
	}
	if !a.nextSync.Equal(b.nextSync) {
		return a.nextSync.Before(b.nextSync)
	}
	return a.changesetID < b.changesetID
}

func (pq *changesetPriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.index[pq.items[i].changesetID] = i
	pq.index[pq.items[j].changesetID] = j
}

// Push is here to implement the Heap interface, please use Upsert
func (pq *changesetPriorityQueue) Push(x interface{}) {
	n := len(pq.items)
	item := x.(scheduledSync)
	pq.index[item.changesetID] = n
	pq.items = append(pq.items, item)
}

// Pop is not to be used directly, use heap.Pop(pq)
func (pq *changesetPriorityQueue) Pop() interface{} {
	item := pq.items[len(pq.items)-1]
	delete(pq.index, item.changesetID)
	pq.items = pq.items[:len(pq.items)-1]
	return item
}

// End of heap methods

// Peek fetches the highest priority item without removing it.
func (pq *changesetPriorityQueue) Peek() (scheduledSync, bool) {
	if len(pq.items) == 0 {
		return scheduledSync{}, false
	}
	return pq.items[0], true
}

// Upsert modifies at item if it exists or adds a new item if not.
// NOTE: If an existing item is high priority, it will not be changed back
// to normal. This allows high priority items to stay that way through reschedules.
func (pq *changesetPriorityQueue) Upsert(ss ...scheduledSync) {
	for _, s := range ss {
		i, ok := pq.index[s.changesetID]
		if !ok {
			heap.Push(pq, s)
			continue
		}
		oldPriority := pq.items[i].priority
		pq.items[i] = s
		if oldPriority == priorityHigh {
			pq.items[i].priority = priorityHigh
		}
		heap.Fix(pq, i)
	}
}

// Get fetches the item with the supplied id without removing it.
func (pq *changesetPriorityQueue) Get(id int64) (scheduledSync, bool) {
	i, ok := pq.index[id]
	if !ok {
		return scheduledSync{}, false
	}
	item := pq.items[i]
	return item, true
}

func (pq *changesetPriorityQueue) Remove(id int64) {
	i, ok := pq.index[id]
	if !ok {
		return
	}
	heap.Remove(pq, i)
}

type priority int

const (
	priorityNormal priority = iota
	priorityHigh
)
