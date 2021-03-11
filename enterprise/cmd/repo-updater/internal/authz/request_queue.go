package authz

import (
	"container/heap"
	"sync"
	"time"
)

// priority defines how urgent the permissions syncing request is.
// Generally, if the request is driven from a user action (e.g. sign up, log in)
// then it should be priorityHigh. All other cases should be priorityLow.
type priority int

const (
	priorityLow priority = iota
	priorityHigh
)

// requestType is the type of the permissions syncing request. It defines the
// permissions syncing is either repository-centric or user-centric.
type requestType int

// A list of request types, the larger the value, the higher the priority.
// requestTypeUser had the highest because it is often triggered by a user
// action (e.g. sign up, log in).
const (
	requestTypeRepo requestType = iota + 1
	requestTypeUser
)

// higherPriorityThan returns true if the current request type has higher priority
// than the other one.
func (t1 requestType) higherPriorityThan(t2 requestType) bool {
	return t1 > t2
}

// requestMeta contains metadata of a permissions syncing request.
type requestMeta struct {
	Priority   priority
	Type       requestType
	ID         int32
	NextSyncAt time.Time
	NoPerms    bool
}

// syncRequest is a permissions syncing request with its current status in the queue.
type syncRequest struct {
	*requestMeta

	acquired bool // Whether the request has been acquired
	index    int  // The index in the heap
}

// requestQueueKey is the key type for index in a requestQueue.
type requestQueueKey struct {
	typ requestType
	id  int32
}

// requestQueue is a priority queue of permissions syncing requests.
// Requests with same requestType and id are guaranteed to only have
// one instance in the queue.
type requestQueue struct {
	mu    sync.RWMutex
	heap  []*syncRequest
	index map[requestQueueKey]*syncRequest

	// The queue performs a non-blocking send on this channel
	// when a new value is enqueued so that the update loop
	// can wake up if it is idle.
	notifyEnqueue chan struct{}
}

func newRequestQueue() *requestQueue {
	return &requestQueue{
		index:         make(map[requestQueueKey]*syncRequest),
		notifyEnqueue: make(chan struct{}, 1),
	}
}

// notify performs a non-blocking send to the channel, so the channel
// must be buffered. When the channel is blocked (i.e. buffer is full),
// it skips the notify thus will not send anything to the channel.
func notify(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

// enqueue adds a sync request to the queue with the given metadata.
//
// If the sync request is already in the queue and it isn't yet acquired,
// the request is updated.
//
// If the given priority is higher than the one in the queue,
// the sync request's position in the queue is updated accordingly.
func (q *requestQueue) enqueue(meta *requestMeta) (updated bool) {
	if meta == nil {
		return false
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	key := requestQueueKey{
		typ: meta.Type,
		id:  meta.ID,
	}
	request := q.index[key]
	if request == nil {
		heap.Push(q, &syncRequest{
			requestMeta: meta,
		})
		notify(q.notifyEnqueue)
		return false
	}

	if request.acquired || request.Priority >= meta.Priority {
		// Request is acquired and in processing, or is already in the queue with at least as good priority.
		return false
	}

	request.requestMeta = meta
	heap.Fix(q, request.index)
	notify(q.notifyEnqueue)
	return true
}

// remove removes the sync request from the queue if the request.acquired matches the
// acquired argument.
func (q *requestQueue) remove(typ requestType, id int32, acquired bool) (removed bool) {
	if id == 0 {
		return false
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	key := requestQueueKey{
		typ: typ,
		id:  id,
	}
	request := q.index[key]
	if request != nil && request.acquired == acquired {
		heap.Remove(q, request.index)
		return true
	}

	return false
}

// acquireNext acquires the next sync request. The acquired request must be removed from
// the queue when the request finishes (independent of success or failure). This is to
// prevent enqueuing a new request while an earlier and identical one is being processed.
func (q *requestQueue) acquireNext() *syncRequest {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.Len() == 0 {
		return nil
	}

	request := q.heap[0]
	if request.acquired {
		// Everything in the queue is already acquired and updating.
		return nil
	}

	request.acquired = true
	heap.Fix(q, request.index)
	return request
}

// release releases the acquired sync request from the queue (i.e. sets the acquired
// state back to false).
func (q *requestQueue) release(typ requestType, id int32) {
	q.mu.Lock()
	defer q.mu.Unlock()

	key := requestQueueKey{
		typ: typ,
		id:  id,
	}
	request := q.index[key]
	if request == nil {
		return
	}

	request.acquired = false
	heap.Fix(q, request.index)
}

// The following methods implement heap.Interface based on the priority queue example:
// https://golang.org/pkg/container/heap/#example__priorityQueue
// These methods are not safe for concurrent use. Therefore, it is the caller's
// responsibility to ensure they're being guarded by a mutex during any heap operation,
// i.e. heap.Fix, heap.Remove, heap.Push, heap.Pop.

func (q *requestQueue) Len() int { return len(q.heap) }

func (q *requestQueue) Less(i, j int) bool {
	qi := q.heap[i]
	qj := q.heap[j]

	if qi.acquired != qj.acquired {
		// Requests that are already acquired are sorted last.
		return qj.acquired
	}

	if qi.Priority != qj.Priority {
		// We want Pop to give us the highest, not lowest, priority so we use greater than here.
		return qi.Priority > qj.Priority
	}

	if qi.Type != qj.Type {
		return qi.Type.higherPriorityThan(qj.Type)
	}

	// Earlier scheduled next sync has higher priority.
	return qi.NextSyncAt.Before(qj.NextSyncAt)
}

func (q *requestQueue) Swap(i, j int) {
	q.heap[i], q.heap[j] = q.heap[j], q.heap[i]
	q.heap[i].index = i
	q.heap[j].index = j
}

func (q *requestQueue) Push(x interface{}) {
	n := len(q.heap)
	request := x.(*syncRequest)
	request.index = n
	q.heap = append(q.heap, request)

	key := requestQueueKey{
		typ: request.Type,
		id:  request.ID,
	}
	q.index[key] = request
}

func (q *requestQueue) Pop() interface{} {
	n := len(q.heap)
	request := q.heap[n-1]
	request.index = -1 // for safety
	q.heap = q.heap[0 : n-1]

	key := requestQueueKey{
		typ: request.Type,
		id:  request.ID,
	}
	delete(q.index, key)
	return request
}
