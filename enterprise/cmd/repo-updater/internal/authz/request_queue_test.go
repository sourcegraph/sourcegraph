package authz

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// The options to allow cmp to compare unexported fields.
var cmpOpts = cmp.AllowUnexported(syncRequest{}, requestMeta{}, requestQueueKey{})

func Test_requestQueue_enqueue(t *testing.T) {
	lowRepo1 := &requestMeta{Priority: priorityLow, Type: requestTypeRepo, ID: 1}
	highRepo1 := &requestMeta{Priority: priorityHigh, Type: requestTypeRepo, ID: 1}
	lowRepo2 := &requestMeta{Priority: priorityLow, Type: requestTypeRepo, ID: 2}
	highRepo2 := &requestMeta{Priority: priorityHigh, Type: requestTypeRepo, ID: 2}
	lowRepo3 := &requestMeta{Priority: priorityLow, Type: requestTypeRepo, ID: 3}
	highRepo3 := &requestMeta{Priority: priorityHigh, Type: requestTypeRepo, ID: 3}
	lowRepo4 := &requestMeta{Priority: priorityLow, Type: requestTypeRepo, ID: 3, NextSyncAt: time.Now()}

	lowUser1 := &requestMeta{Priority: priorityLow, Type: requestTypeUser, ID: 1}

	tests := []struct {
		name             string
		metas            []*requestMeta
		acquires         int // To acquire n requests before assertions
		expHeap          []*syncRequest
		expUpdated       []requestQueueKey
		expNotifications int // The number of notifications expect to receive
	}{
		{
			name: "enqueue a low priority repo 1",
			metas: []*requestMeta{
				lowRepo1,
			},
			expHeap: []*syncRequest{
				{requestMeta: lowRepo1},
			},
			expNotifications: 1,
		},
		{
			name: "enqueue a high priority repo 2",
			metas: []*requestMeta{
				highRepo2,
			},
			expHeap: []*syncRequest{
				{requestMeta: highRepo2},
			},
			expNotifications: 1,
		},
		{
			name: "enqueue a low repo 1 then a high repo 2",
			metas: []*requestMeta{
				lowRepo1,
				highRepo2,
			},
			expHeap: []*syncRequest{
				{requestMeta: highRepo2, index: 0},
				{requestMeta: lowRepo1, index: 1},
			},
			expNotifications: 2,
		},
		{
			name: "enqueue a high repo 2 then a low repo 1",
			metas: []*requestMeta{
				highRepo2,
				lowRepo1,
			},
			expHeap: []*syncRequest{
				{requestMeta: highRepo2, index: 0},
				{requestMeta: lowRepo1, index: 1},
			},
			expNotifications: 2,
		},
		{
			name: "enqueue low repo 1 twice",
			metas: []*requestMeta{
				lowRepo1,
				lowRepo1,
			},
			expHeap: []*syncRequest{
				{requestMeta: lowRepo1, index: 0},
			},
			expNotifications: 1,
		},
		{
			name: "enqueue low repo 1, acquired, then enqueue low repo 1 again, do nothing",
			metas: []*requestMeta{
				lowRepo1,
				lowRepo1,
			},
			acquires: 1,
			expHeap: []*syncRequest{
				{requestMeta: lowRepo1, acquired: true, index: 0},
			},
			expNotifications: 1,
		},
		{
			name: "enqueue low repo 1 then high repo 1, updated",
			metas: []*requestMeta{
				lowRepo1,
				highRepo1,
			},
			expHeap: []*syncRequest{
				{requestMeta: highRepo1, index: 0},
			},
			expUpdated: []requestQueueKey{
				{typ: requestTypeRepo, id: 1},
			},
			expNotifications: 2,
		},
		{
			name: "enqueue high repo 1 then low repo 1",
			metas: []*requestMeta{
				highRepo1,
				lowRepo1,
			},
			expHeap: []*syncRequest{
				{requestMeta: highRepo1, index: 0},
			},
			expNotifications: 1,
		},
		{
			name: "heap is fixed when priority is bumped",
			metas: []*requestMeta{
				lowRepo3,
				lowRepo2,
				lowRepo1,
				highRepo1,
				highRepo2,
				highRepo3,
			},
			expHeap: []*syncRequest{
				{requestMeta: highRepo1, index: 0},
				{requestMeta: highRepo2, index: 1},
				{requestMeta: highRepo3, index: 2},
			},
			expUpdated: []requestQueueKey{
				{typ: requestTypeRepo, id: 1},
				{typ: requestTypeRepo, id: 2},
				{typ: requestTypeRepo, id: 3},
			},
			expNotifications: 6,
		},
		{
			name: "acquired sorted to last",
			metas: []*requestMeta{
				lowRepo1,
				lowRepo2,
			},
			acquires: 1,
			expHeap: []*syncRequest{
				{requestMeta: lowRepo2, acquired: false, index: 0},
				{requestMeta: lowRepo1, acquired: true, index: 1},
			},
			expNotifications: 2,
		},
		{
			name: "earlier nextSyncAt sorted to first",
			metas: []*requestMeta{
				lowRepo4,
				lowRepo1,
			},
			expHeap: []*syncRequest{
				{requestMeta: lowRepo1, index: 0},
				{requestMeta: lowRepo4, index: 1},
			},
			expNotifications: 2,
		},
		{
			name: "user request sorted to first",
			metas: []*requestMeta{
				lowRepo1,
				lowUser1,
			},
			expHeap: []*syncRequest{
				{requestMeta: lowUser1, index: 0},
				{requestMeta: lowRepo1, index: 1},
			},
			expNotifications: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := newRequestQueue()

			// Make the channel buffer large enough to get all notifications,
			// plus one in case we receive one more unexpected. It is not a
			// accurate difference but enough to fail the test and rise attention.
			q.notifyEnqueue = make(chan struct{}, test.expNotifications+1)

			var updated []requestQueueKey
			for _, meta := range test.metas {
				if q.enqueue(meta) {
					updated = append(updated, requestQueueKey{
						typ: meta.Type,
						id:  meta.ID,
					})
				}
			}

			for test.acquires > 0 {
				q.acquireNext()
				test.acquires--
			}

			numNotifications := 0
		loop:
			for {
				select {
				case <-q.notifyEnqueue:
					numNotifications++
				default:
					break loop
				}
			}

			if diff := cmp.Diff(test.expHeap, q.heap, cmpOpts); diff != "" {
				t.Fatalf("heap: %v", diff)
			}

			if diff := cmp.Diff(test.expUpdated, updated, cmpOpts); diff != "" {
				t.Fatalf("updated: %v", diff)
			}

			if numNotifications != test.expNotifications {
				t.Fatalf("numNotifications: want %d but got %d", test.expNotifications, numNotifications)
			}
		})
	}
}

func Test_requestQueue_remove(t *testing.T) {
	repo1 := &requestMeta{Type: requestTypeRepo, ID: 1}
	repo1Key := requestQueueKey{typ: requestTypeRepo, id: 1}
	repo2 := &requestMeta{Type: requestTypeRepo, ID: 2}
	repo2Key := requestQueueKey{typ: requestTypeRepo, id: 2}
	repo3 := &requestMeta{Type: requestTypeRepo, ID: 3}
	repo3Key := requestQueueKey{typ: requestTypeRepo, id: 3}

	type remove struct {
		requestQueueKey
		acquired bool
	}

	tests := []struct {
		name    string
		metas   []*requestMeta
		removes []*remove
		expHeap []*syncRequest
	}{
		{
			name: "remove the only one",
			metas: []*requestMeta{
				repo1,
			},
			removes: []*remove{
				{requestQueueKey: repo1Key, acquired: false},
			},
			expHeap: []*syncRequest{},
		},
		{
			name: "remove the front",
			metas: []*requestMeta{
				repo1,
				repo2,
			},
			removes: []*remove{
				{requestQueueKey: repo1Key, acquired: false},
			},
			expHeap: []*syncRequest{
				{requestMeta: repo2, index: 0},
			},
		},
		{
			name: "remove the back",
			metas: []*requestMeta{
				repo1,
				repo2,
			},
			removes: []*remove{
				{requestQueueKey: repo2Key, acquired: false},
			},
			expHeap: []*syncRequest{
				{requestMeta: repo1, index: 0},
			},
		},
		{
			name: "remove the middle",
			metas: []*requestMeta{
				repo1,
				repo2,
				repo3,
			},
			removes: []*remove{
				{requestQueueKey: repo2Key, acquired: false},
			},
			expHeap: []*syncRequest{
				{requestMeta: repo1, index: 0},
				{requestMeta: repo3, index: 1},
			},
		},
		{
			name: "remove not present",
			metas: []*requestMeta{
				repo1,
				repo2,
			},
			removes: []*remove{
				{requestQueueKey: repo3Key, acquired: false},
			},
			expHeap: []*syncRequest{
				{requestMeta: repo1, index: 0},
				{requestMeta: repo2, index: 1},
			},
		},
		{
			name:  "remove from empty queue",
			metas: []*requestMeta{},
			removes: []*remove{
				{requestQueueKey: repo1Key, acquired: false},
			},
		},
		{
			name: "remove all",
			metas: []*requestMeta{
				repo1,
				repo2,
				repo3,
			},
			removes: []*remove{
				{requestQueueKey: repo1Key, acquired: false},
				{requestQueueKey: repo2Key, acquired: false},
				{requestQueueKey: repo3Key, acquired: false},
			},
			expHeap: []*syncRequest{},
		},
		{
			name: "remove all reverse",
			metas: []*requestMeta{
				repo1,
				repo2,
				repo3,
			},
			removes: []*remove{
				{requestQueueKey: repo3Key, acquired: false},
				{requestQueueKey: repo2Key, acquired: false},
				{requestQueueKey: repo1Key, acquired: false},
			},
			expHeap: []*syncRequest{},
		},
		{
			name: "don't remove when acquired mismatch",
			metas: []*requestMeta{
				repo1,
			},
			removes: []*remove{
				{requestQueueKey: repo1Key, acquired: true},
			},
			expHeap: []*syncRequest{
				{requestMeta: repo1, index: 0},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := newRequestQueue()

			for _, meta := range test.metas {
				q.enqueue(meta)
			}

			for _, remove := range test.removes {
				q.remove(remove.typ, remove.id, remove.acquired)
			}

			if diff := cmp.Diff(test.expHeap, q.heap, cmpOpts); diff != "" {
				t.Fatalf("heap: %v", diff)
			}
		})
	}
}

func Test_requestQueue_acquireNext(t *testing.T) {
	repo1 := &requestMeta{Type: requestTypeRepo, ID: 1}
	repo2 := &requestMeta{Type: requestTypeRepo, ID: 2}

	tests := []struct {
		name        string
		metas       []*requestMeta
		acquires    int // To acquire n requests before assertions
		expAcquires []*syncRequest
		expHeap     []*syncRequest
	}{
		{
			name:     "acquire from empty queue returns nothing",
			acquires: 1,
		},
		{
			name: "acquire sets acquired to true",
			metas: []*requestMeta{
				repo1,
			},
			acquires: 1,
			expAcquires: []*syncRequest{
				{requestMeta: repo1, acquired: true, index: 0},
			},
			expHeap: []*syncRequest{
				{requestMeta: repo1, acquired: true, index: 0},
			},
		},
		{
			name: "acquire moves request to back of queue",
			metas: []*requestMeta{
				repo1,
				repo2,
			},
			acquires: 1,
			expAcquires: []*syncRequest{
				{requestMeta: repo1, acquired: true, index: 1},
			},
			expHeap: []*syncRequest{
				{requestMeta: repo2, acquired: false, index: 0},
				{requestMeta: repo1, acquired: true, index: 1},
			},
		},
		{
			name: "acquire returns nil when already acquired",
			metas: []*requestMeta{
				repo1,
			},
			acquires: 2,
			expAcquires: []*syncRequest{
				{requestMeta: repo1, acquired: true, index: 0},
			},
			expHeap: []*syncRequest{
				{requestMeta: repo1, acquired: true, index: 0},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := newRequestQueue()

			for _, meta := range test.metas {
				q.enqueue(meta)
			}

			// Acquire requests until reached desired times or hits nil
			var requests []*syncRequest
			for test.acquires > 0 {
				request := q.acquireNext()
				if request == nil {
					break
				}

				requests = append(requests, request)
				test.acquires--
			}

			if diff := cmp.Diff(test.expAcquires, requests, cmpOpts); diff != "" {
				t.Fatalf("requests: %v", diff)
			}

			if diff := cmp.Diff(test.expHeap, q.heap, cmpOpts); diff != "" {
				t.Fatalf("heap: %v", diff)
			}
		})
	}
}

func Test_requestQueue_release(t *testing.T) {
	user1 := &requestMeta{Type: requestTypeUser, ID: 1}
	repo2 := &requestMeta{Type: requestTypeRepo, ID: 2}

	q := newRequestQueue()
	q.enqueue(user1)
	q.enqueue(repo2)

	expHeap := []*syncRequest{
		{requestMeta: user1, acquired: false, index: 0},
		{requestMeta: repo2, acquired: false, index: 1},
	}
	if diff := cmp.Diff(expHeap, q.heap, cmpOpts); diff != "" {
		t.Fatalf("heap: %v", diff)
	}

	// Acquire the next request
	r := q.acquireNext()

	expHeap = []*syncRequest{
		{requestMeta: repo2, acquired: false, index: 0},
		{requestMeta: user1, acquired: true, index: 1},
	}
	if diff := cmp.Diff(expHeap, q.heap, cmpOpts); diff != "" {
		t.Fatalf("heap: %v", diff)
	}

	// Release the request
	q.release(r.Type, r.ID)
	expHeap = []*syncRequest{
		{requestMeta: user1, acquired: false, index: 0},
		{requestMeta: repo2, acquired: false, index: 1},
	}
	if diff := cmp.Diff(expHeap, q.heap, cmpOpts); diff != "" {
		t.Fatalf("heap: %v", diff)
	}
}

func Test_requestQueue_Less(t *testing.T) {
	q := newRequestQueue()

	tests := []struct {
		name   string
		heap   []*syncRequest
		expVal bool
	}{
		{
			name: "i is acquired",
			heap: []*syncRequest{
				{acquired: true},
				{acquired: false},
			},
			expVal: false,
		},
		{
			name: "j is acquired",
			heap: []*syncRequest{
				{acquired: false},
				{acquired: true},
			},
			expVal: true,
		},
		{
			name: "i has high priority",
			heap: []*syncRequest{
				{requestMeta: &requestMeta{Priority: priorityHigh}},
				{requestMeta: &requestMeta{Priority: priorityLow}},
			},
			expVal: true,
		},
		{
			name: "j has high priority",
			heap: []*syncRequest{
				{requestMeta: &requestMeta{Priority: priorityLow}},
				{requestMeta: &requestMeta{Priority: priorityHigh}},
			},
			expVal: false,
		},
		{
			name: "i is a user request",
			heap: []*syncRequest{
				{requestMeta: &requestMeta{Type: requestTypeUser}},
				{requestMeta: &requestMeta{Type: requestTypeRepo}},
			},
			expVal: true,
		},
		{
			name: "j is a user request",
			heap: []*syncRequest{
				{requestMeta: &requestMeta{Type: requestTypeRepo}},
				{requestMeta: &requestMeta{Type: requestTypeUser}},
			},
			expVal: false,
		},
		{
			name: "i has older nextSyncAt",
			heap: []*syncRequest{
				{requestMeta: &requestMeta{}},
				{requestMeta: &requestMeta{NextSyncAt: time.Now()}},
			},
			expVal: true,
		},
		{
			name: "j has older nextSyncAt",
			heap: []*syncRequest{
				{requestMeta: &requestMeta{NextSyncAt: time.Now()}},
				{requestMeta: &requestMeta{}},
			},
			expVal: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q.heap = test.heap
			got := q.Less(0, 1)
			if test.expVal != got {
				t.Fatalf("want %v but got %v", test.expVal, got)
			}
		})
	}
}
