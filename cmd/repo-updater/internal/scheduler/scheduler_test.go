package scheduler

import (
	"container/heap"
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	gitserverprotocol "github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/limiter"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

var defaultTime = time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC)

func init() {
	timeNow = nil
	notify = nil
	timeAfterFunc = nil
}

func mockTime(t time.Time) {
	timeNow = func() time.Time {
		return t
	}
}

type recording struct {
	notifications       []chan struct{}
	timeAfterFuncDelays []time.Duration
}

func startRecording() (*recording, func()) {
	var r recording

	mockTime(defaultTime)

	notify = func(ch chan struct{}) {
		r.notifications = append(r.notifications, ch)
	}

	timeAfterFunc = func(delay time.Duration, f func()) *time.Timer {
		r.timeAfterFuncDelays = append(r.timeAfterFuncDelays, delay)
		f()
		return nil
	}

	return &r, func() {
		timeNow = nil
		notify = nil
		timeAfterFunc = nil
	}
}

func TestUpdateQueue_enqueue(t *testing.T) {
	a := configuredRepo{ID: 1, Name: "a"}
	a2 := configuredRepo{ID: 1, Name: "a2"}
	b := configuredRepo{ID: 2, Name: "b"}
	c := configuredRepo{ID: 3, Name: "c"}
	d := configuredRepo{ID: 4, Name: "d"}
	e := configuredRepo{ID: 5, Name: "e"}
	db := dbmocks.NewMockDB()

	type enqueueCall struct {
		repo     configuredRepo
		priority priority
	}
	tests := []struct {
		name                  string
		calls                 []*enqueueCall
		acquire               int // acquire n updates before assertions
		expectedUpdates       []*repoUpdate
		expectedNotifications int
	}{
		{
			name: "enqueue low priority",
			calls: []*enqueueCall{
				{repo: a, priority: priorityLow},
			},
			expectedUpdates: []*repoUpdate{
				{
					Repo:     a,
					Priority: priorityLow,
					Seq:      1,
				},
			},
			expectedNotifications: 1,
		},
		{
			name: "enqueue high priority",
			calls: []*enqueueCall{
				{repo: a, priority: priorityHigh},
			},
			expectedUpdates: []*repoUpdate{
				{
					Repo:     a,
					Priority: priorityHigh,
					Seq:      1,
				},
			},
			expectedNotifications: 1,
		},
		{
			name: "enqueue low b then high a",
			calls: []*enqueueCall{
				{repo: b, priority: priorityLow},
				{repo: a, priority: priorityHigh},
			},
			expectedUpdates: []*repoUpdate{
				{
					Repo:     a,
					Priority: priorityHigh,
					Seq:      2,
				},
				{
					Repo:     b,
					Priority: priorityLow,
					Seq:      1,
				},
			},
			expectedNotifications: 2,
		},
		{
			name: "enqueue high a then low b",
			calls: []*enqueueCall{
				{repo: a, priority: priorityHigh},
				{repo: b, priority: priorityLow},
			},
			expectedUpdates: []*repoUpdate{
				{
					Repo:     a,
					Priority: priorityHigh,
					Seq:      1,
				},
				{
					Repo:     b,
					Priority: priorityLow,
					Seq:      2,
				},
			},
			expectedNotifications: 2,
		},
		{
			name: "enqueue low a then low a",
			calls: []*enqueueCall{
				{repo: a, priority: priorityLow},
				{repo: a, priority: priorityLow},
			},
			expectedUpdates: []*repoUpdate{
				{
					Repo:     a,
					Priority: priorityLow,
					Seq:      1,
				},
			},
			expectedNotifications: 1,
		},
		{
			name: "enqueue high a then low a",
			calls: []*enqueueCall{
				{repo: a, priority: priorityHigh},
				{repo: a, priority: priorityLow},
			},
			expectedUpdates: []*repoUpdate{
				{
					Repo:     a,
					Priority: priorityHigh,
					Seq:      1,
				},
			},
			expectedNotifications: 1,
		},
		{
			name: "enqueue low a then high a",
			calls: []*enqueueCall{
				{repo: a, priority: priorityLow},
				{repo: a, priority: priorityHigh},
			},
			expectedUpdates: []*repoUpdate{
				{
					Repo:     a,
					Priority: priorityHigh,
					Seq:      2,
				},
			},
			expectedNotifications: 2,
		},
		{
			name: "repo is updated if not already updating",
			calls: []*enqueueCall{
				{repo: a, priority: priorityHigh},
				{repo: a2, priority: priorityLow},
			},
			expectedUpdates: []*repoUpdate{
				{
					Repo:     a2,
					Priority: priorityHigh,
					Seq:      1, // Priority remains high
				},
			},
			expectedNotifications: 1,
		},
		{
			name: "repo is NOT updated if already updating",
			calls: []*enqueueCall{
				{repo: a, priority: priorityHigh},
				{repo: a2, priority: priorityLow},
			},
			acquire: 1,
			expectedUpdates: []*repoUpdate{
				{
					Repo:     a,
					Priority: priorityHigh,
					Updating: true,
					Seq:      1,
				},
			},
			expectedNotifications: 1,
		},
		{
			name: "heap is fixed when priority is bumped",
			calls: []*enqueueCall{
				{repo: c, priority: priorityLow},
				{repo: d, priority: priorityLow},
				{repo: a, priority: priorityLow},
				{repo: e, priority: priorityLow},
				{repo: b, priority: priorityLow},

				{repo: a, priority: priorityHigh},
				{repo: b, priority: priorityHigh},
				{repo: c, priority: priorityHigh},
				{repo: d, priority: priorityHigh},
				{repo: e, priority: priorityHigh},
			},
			expectedUpdates: []*repoUpdate{
				{
					Repo:     a,
					Priority: priorityHigh,
					Seq:      6,
				},
				{
					Repo:     b,
					Priority: priorityHigh,
					Seq:      7,
				},
				{
					Repo:     c,
					Priority: priorityHigh,
					Seq:      8,
				},
				{
					Repo:     d,
					Priority: priorityHigh,
					Seq:      9,
				},
				{
					Repo:     e,
					Priority: priorityHigh,
					Seq:      10,
				},
			},
			expectedNotifications: 10,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, stop := startRecording()
			defer stop()

			s := NewUpdateScheduler(logtest.Scoped(t), db, gitserver.NewMockClient())

			for _, call := range test.calls {
				s.updateQueue.enqueue(call.repo, call.priority)
				if test.acquire > 0 {
					s.updateQueue.acquireNext()
					test.acquire--
				}
			}

			verifyQueue(t, s, test.expectedUpdates)

			// Verify notifications.
			expectedRecording := &recording{}
			for range test.expectedNotifications {
				expectedRecording.notifications = append(expectedRecording.notifications, s.updateQueue.notifyEnqueue)
			}
			if !reflect.DeepEqual(expectedRecording, r) {
				t.Log(cmp.Diff(expectedRecording, r))
				t.Fatalf("\nexpected\n%s\ngot\n%s", spew.Sdump(expectedRecording), spew.Sdump(r))
			}
		})
	}
}

func TestUpdateQueue_remove(t *testing.T) {
	a := configuredRepo{ID: 1, Name: "a"}
	b := configuredRepo{ID: 2, Name: "b"}
	c := configuredRepo{ID: 3, Name: "c"}

	type removeCall struct {
		repo     configuredRepo
		updating bool
	}

	tests := []struct {
		name         string
		initialQueue []*repoUpdate
		removeCalls  []*removeCall
		finalQueue   []*repoUpdate
	}{
		{
			name: "remove only",
			initialQueue: []*repoUpdate{
				{Repo: a, Seq: 1},
			},
			removeCalls: []*removeCall{
				{repo: a},
			},
		},
		{
			name: "remove front",
			initialQueue: []*repoUpdate{
				{Repo: a, Seq: 1},
				{Repo: b, Seq: 2},
			},
			removeCalls: []*removeCall{
				{repo: a},
			},
			finalQueue: []*repoUpdate{
				{Repo: b, Seq: 2},
			},
		},
		{
			name: "remove back",
			initialQueue: []*repoUpdate{
				{Repo: a, Seq: 1},
				{Repo: b, Seq: 2},
			},
			removeCalls: []*removeCall{
				{repo: b},
			},
			finalQueue: []*repoUpdate{
				{Repo: a, Seq: 1},
			},
		},
		{
			name: "remove middle",
			initialQueue: []*repoUpdate{
				{Repo: a, Seq: 1},
				{Repo: b, Seq: 2},
				{Repo: c, Seq: 3},
			},
			removeCalls: []*removeCall{
				{repo: c},
			},
			finalQueue: []*repoUpdate{
				{Repo: a, Seq: 1},
				{Repo: b, Seq: 2},
			},
		},
		{
			name: "remove not present",
			initialQueue: []*repoUpdate{
				{Repo: a, Seq: 1},
			},
			removeCalls: []*removeCall{
				{repo: b},
			},
			finalQueue: []*repoUpdate{
				{Repo: a, Seq: 1},
			},
		},
		{
			name: "remove from empty queue",
			removeCalls: []*removeCall{
				{repo: a},
			},
		},
		{
			name: "remove all",
			initialQueue: []*repoUpdate{
				{Repo: a, Seq: 1},
				{Repo: b, Seq: 2},
				{Repo: c, Seq: 3},
			},
			removeCalls: []*removeCall{
				{repo: a},
				{repo: b},
				{repo: c},
			},
		},
		{
			name: "remove all reverse",
			initialQueue: []*repoUpdate{
				{Repo: a, Seq: 1},
				{Repo: b, Seq: 2},
				{Repo: c, Seq: 3},
			},
			removeCalls: []*removeCall{
				{repo: c},
				{repo: b},
				{repo: a},
			},
		},
		{
			name: "don't remove updating",
			initialQueue: []*repoUpdate{
				{Repo: a, Seq: 1, Updating: true},
			},
			removeCalls: []*removeCall{
				{repo: a, updating: false},
			},
			finalQueue: []*repoUpdate{
				{Repo: a, Seq: 1, Updating: true},
			},
		},
		{
			name: "don't remove not updating",
			initialQueue: []*repoUpdate{
				{Repo: a, Seq: 1, Updating: false},
			},
			removeCalls: []*removeCall{
				{repo: a, updating: true},
			},
			finalQueue: []*repoUpdate{
				{Repo: a, Seq: 1, Updating: false},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, stop := startRecording()
			defer stop()

			s := NewUpdateScheduler(logtest.Scoped(t), dbmocks.NewMockDB(), gitserver.NewMockClient())
			setupInitialQueue(s, test.initialQueue)

			// Perform the removals.
			for _, call := range test.removeCalls {
				s.updateQueue.remove(call.repo, call.updating)
			}

			verifyQueue(t, s, test.finalQueue)

			// Verify no notifications.
			expectedRecording := &recording{}
			if !reflect.DeepEqual(expectedRecording, r) {
				t.Fatalf("\nexpected\n%s\ngot\n%s", spew.Sdump(expectedRecording), spew.Sdump(r))
			}
		})
	}
}

func TestUpdateQueue_acquireNext(t *testing.T) {
	a := configuredRepo{ID: 1, Name: "a"}
	b := configuredRepo{ID: 2, Name: "b"}

	tests := []struct {
		name           string
		initialQueue   []*repoUpdate
		acquireResults []*configuredRepo
		finalQueue     []*repoUpdate
	}{
		{
			name:           "acquire from empty queue returns nil",
			acquireResults: []*configuredRepo{nil},
		},
		{
			name: "acquire sets updating to true",
			initialQueue: []*repoUpdate{
				{Repo: a, Updating: false, Seq: 1},
			},
			acquireResults: []*configuredRepo{&a},
			finalQueue: []*repoUpdate{
				{Repo: a, Updating: true, Seq: 1},
			},
		},
		{
			name: "acquire sends update to back of queue",
			initialQueue: []*repoUpdate{
				{Repo: a, Updating: false, Seq: 1},
				{Repo: b, Updating: false, Seq: 2},
			},
			acquireResults: []*configuredRepo{&a},
			finalQueue: []*repoUpdate{
				{Repo: b, Updating: false, Seq: 2},
				{Repo: a, Updating: true, Seq: 1},
			},
		},
		{
			name: "acquire does not return repos that are already updating",
			initialQueue: []*repoUpdate{
				{Repo: a, Updating: true, Seq: 1},
			},
			acquireResults: []*configuredRepo{nil},
			finalQueue: []*repoUpdate{
				{Repo: a, Updating: true, Seq: 1},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, stop := startRecording()
			defer stop()

			s := NewUpdateScheduler(logtest.Scoped(t), dbmocks.NewMockDB(), gitserver.NewMockClient())
			setupInitialQueue(s, test.initialQueue)

			// Test aquireNext.
			for i, expected := range test.acquireResults {
				actual, ok := s.updateQueue.acquireNext()
				got := &actual
				if !ok {
					got = nil
				}
				if !reflect.DeepEqual(expected, got) {
					t.Fatalf("\nacquireNext expected %d\n%s\ngot\n%s", i, spew.Sdump(expected), spew.Sdump(got))
				}
			}

			verifyQueue(t, s, test.finalQueue)

			// Verify no notifications.
			expectedRecording := &recording{}
			if !reflect.DeepEqual(expectedRecording, r) {
				t.Fatalf("\nexpected\n%s\ngot\n%s", spew.Sdump(expectedRecording), spew.Sdump(r))
			}
		})
	}
}

func setupInitialQueue(s *UpdateScheduler, initialQueue []*repoUpdate) {
	for _, update := range initialQueue {
		heap.Push(s.updateQueue, update)
	}
}

func verifyQueue(t *testing.T, s *UpdateScheduler, expected []*repoUpdate) {
	t.Helper()

	var actualQueue []*repoUpdate
	for len(s.updateQueue.heap) > 0 {
		update := heap.Pop(s.updateQueue).(*repoUpdate)
		update.Index = 0 // this will always be -1, but easier to set it to 0 to avoid boilerplate in test cases
		actualQueue = append(actualQueue, update)
	}

	if !reflect.DeepEqual(expected, actualQueue) {
		t.Fatalf("\nexpected final queue\n%s\ngot\n%s", spew.Sdump(expected), spew.Sdump(actualQueue))
	}
}

func Test_updateScheduler_UpdateFromDiff(t *testing.T) {
	a := configuredRepo{ID: 1, Name: "a"}
	b := configuredRepo{ID: 2, Name: "b"}

	tests := []struct {
		name            string
		initialSchedule []*scheduledRepoUpdate
		initialQueue    []*repoUpdate
		diff            types.RepoSyncDiff
		finalSchedule   []*scheduledRepoUpdate
		finalQueue      []*repoUpdate
	}{
		{
			name: "diff with deleted repos",
			initialSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: minDelay, Due: defaultTime.Add(minDelay)},
			},
			initialQueue: []*repoUpdate{
				{Repo: a, Seq: 1, Updating: false},
			},
			diff: types.RepoSyncDiff{
				Deleted: []*types.Repo{
					{ID: a.ID, Name: a.Name},
				},
			},
		},
		{
			name: "diff with add and modified repos",
			diff: types.RepoSyncDiff{
				Added: []*types.Repo{
					{
						ID:   a.ID,
						Name: a.Name,
					},
				},
				Modified: types.ReposModified{
					{
						Repo: &types.Repo{
							ID:   b.ID,
							Name: b.Name,
						},
					},
				},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: minDelay, Due: defaultTime.Add(minDelay)},
				{Repo: b, Interval: minDelay, Due: defaultTime.Add(minDelay)},
			},
			finalQueue: []*repoUpdate{
				{Repo: a, Seq: 1, Updating: false},
				{Repo: b, Seq: 2, Updating: false},
			},
		},
		{
			name: "diff with unmodified but partially deleted repos",
			initialSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: minDelay, Due: defaultTime.Add(minDelay)},
			},
			initialQueue: []*repoUpdate{
				{Repo: a, Seq: 1, Updating: false},
			},
			diff: types.RepoSyncDiff{
				Unmodified: []*types.Repo{
					{
						ID:        a.ID,
						Name:      a.Name,
						DeletedAt: defaultTime,
					},
					{
						ID:   b.ID,
						Name: b.Name,
					},
				},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{Repo: b, Interval: minDelay, Due: defaultTime.Add(minDelay)},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// The recording is not important for testing this method, but we want to mock and clean up timers.
			_, stop := startRecording()
			defer stop()

			s := NewUpdateScheduler(logtest.Scoped(t), dbmocks.NewMockDB(), gitserver.NewMockClient())
			setupInitialSchedule(s, test.initialSchedule)
			setupInitialQueue(s, test.initialQueue)

			s.UpdateFromDiff(test.diff)

			verifySchedule(t, s, test.finalSchedule)
			verifyQueue(t, s, test.finalQueue)
		})
	}
}

func TestSchedule_upsert(t *testing.T) {
	a := configuredRepo{ID: 1, Name: "a"}
	a2 := configuredRepo{ID: 1, Name: "a2"}
	b := configuredRepo{ID: 2, Name: "b"}

	type upsertCall struct {
		time time.Time
		repo configuredRepo
	}

	tests := []struct {
		name                string
		initialSchedule     []*scheduledRepoUpdate
		upsertCalls         []*upsertCall
		finalSchedule       []*scheduledRepoUpdate
		timeAfterFuncDelays []time.Duration
		wakeupNotifications int
	}{
		{
			name: "upsert empty schedule",
			upsertCalls: []*upsertCall{
				{repo: a, time: defaultTime},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{
					Interval: minDelay,
					Due:      defaultTime.Add(minDelay),
					Repo:     a,
				},
			},
			timeAfterFuncDelays: []time.Duration{minDelay},
			wakeupNotifications: 1,
		},
		{
			name: "upsert duplicate is no-op",
			initialSchedule: []*scheduledRepoUpdate{
				{
					Interval: minDelay,
					Due:      defaultTime,
					Repo:     a,
				},
			},
			upsertCalls: []*upsertCall{
				{repo: a, time: defaultTime.Add(time.Minute)},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{
					Interval: minDelay,
					Due:      defaultTime,
					Repo:     a,
				},
			},
		},
		{
			name: "existing update repo is updated",
			initialSchedule: []*scheduledRepoUpdate{
				{
					Interval: minDelay,
					Due:      defaultTime,
					Repo:     a,
				},
			},
			upsertCalls: []*upsertCall{
				{repo: a2, time: defaultTime.Add(time.Minute)},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{
					Interval: minDelay,
					Due:      defaultTime,
					Repo:     a2,
				},
			},
		},
		{
			name: "upsert later",
			initialSchedule: []*scheduledRepoUpdate{
				{
					Interval: minDelay,
					Due:      defaultTime.Add(30 * time.Second),
					Repo:     a,
				},
			},
			upsertCalls: []*upsertCall{
				{repo: b, time: defaultTime.Add(time.Second)},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{
					Interval: minDelay,
					Due:      defaultTime.Add(30 * time.Second),
					Repo:     a,
				},
				{
					Interval: minDelay,
					Due:      defaultTime.Add(time.Second + minDelay),
					Repo:     b,
				},
			},
			timeAfterFuncDelays: []time.Duration{29 * time.Second},
			wakeupNotifications: 1,
		},
		{
			name: "upsert before",
			initialSchedule: []*scheduledRepoUpdate{
				{
					Interval: minDelay,
					Due:      defaultTime.Add(time.Minute),
					Repo:     a,
				},
			},
			upsertCalls: []*upsertCall{
				{repo: b, time: defaultTime.Add(time.Second)},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{
					Interval: minDelay,
					Due:      defaultTime.Add(time.Second + minDelay),
					Repo:     b,
				},
				{
					Interval: minDelay,
					Due:      defaultTime.Add(time.Minute),
					Repo:     a,
				},
			},
			timeAfterFuncDelays: []time.Duration{minDelay},
			wakeupNotifications: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, stop := startRecording()
			defer stop()

			s := NewUpdateScheduler(logtest.Scoped(t), dbmocks.NewMockDB(), gitserver.NewMockClient())
			setupInitialSchedule(s, test.initialSchedule)

			for _, call := range test.upsertCalls {
				mockTime(call.time)
				s.schedule.upsert(call.repo)
			}

			verifySchedule(t, s, test.finalSchedule)
			verifyScheduleRecording(t, s, test.timeAfterFuncDelays, test.wakeupNotifications, r)
		})
	}
}

func TestUpdateQueue_PrioritiseUncloned(t *testing.T) {
	cloned1 := configuredRepo{ID: 1, Name: "cloned1"}
	cloned2 := configuredRepo{ID: 2, Name: "CLONED2"}
	notcloned := configuredRepo{ID: 3, Name: "notcloned"}

	_, stop := startRecording()
	defer stop()

	s := NewUpdateScheduler(logtest.Scoped(t), dbmocks.NewMockDB(), gitserver.NewMockClient())

	assertFront := func(name api.RepoName) {
		t.Helper()
		front := s.schedule.heap[0].Repo.Name
		if front != name {
			t.Fatalf("front of schedule is %q, want %q", front, name)
		}
	}

	// add everything to the scheduler for the distant future.
	mockTime(defaultTime.Add(time.Hour))
	for _, repo := range []configuredRepo{cloned1, cloned2, notcloned} {
		s.schedule.upsert(repo)
	}

	assertFront(cloned1.Name)

	// Reset the time to now and do prioritiseUncloned. We then verify that notcloned
	// is now at the front of the queue.
	mockTime(defaultTime)
	s.schedule.prioritiseUncloned([]types.MinimalRepo{
		{
			ID:   3,
			Name: "notcloned",
		},
	})

	assertFront(notcloned.Name)
}

func TestScheduleInsertNew(t *testing.T) {
	repo1 := types.MinimalRepo{ID: 1, Name: "repo1"}
	repo2 := types.MinimalRepo{ID: 2, Name: "repo2"}

	_, stop := startRecording()
	defer stop()

	s := NewUpdateScheduler(logtest.Scoped(t), dbmocks.NewMockDB(), gitserver.NewMockClient())

	assertFront := func(name api.RepoName) {
		t.Helper()
		front := s.schedule.heap[0].Repo.Name
		if front != name {
			t.Fatalf("front of schedule is %q, want %q", front, name)
		}
	}

	// add everything to the scheduler for the distant future.
	mockTime(defaultTime.Add(time.Hour))
	s.schedule.insertNew([]types.MinimalRepo{repo1})
	assertFront(repo1.Name)

	// Add including old
	mockTime(defaultTime)
	s.schedule.insertNew([]types.MinimalRepo{repo1, repo2})
	assertFront(repo2.Name)
}

type mockRandomGenerator struct{}

func (m *mockRandomGenerator) Int63n(n int64) int64 {
	return n / 2
}

func TestSchedule_updateInterval(t *testing.T) {
	a := configuredRepo{ID: 1, Name: "a"}
	b := configuredRepo{ID: 2, Name: "b"}
	c := configuredRepo{ID: 3, Name: "c"}
	d := configuredRepo{ID: 4, Name: "d"}
	e := configuredRepo{ID: 5, Name: "e"}

	type updateCall struct {
		time     time.Time
		repo     configuredRepo
		interval time.Duration
	}

	tests := []struct {
		name                string
		initialSchedule     []*scheduledRepoUpdate
		updateCalls         []*updateCall
		finalSchedule       []*scheduledRepoUpdate
		timeAfterFuncDelays []time.Duration
		wakeupNotifications int
	}{
		{
			name: "update has no effect if repo isn't in schedule",
			updateCalls: []*updateCall{
				{repo: a, time: defaultTime},
			},
		},
		{
			name: "update earlier",
			initialSchedule: []*scheduledRepoUpdate{
				{
					Repo:     a,
					Interval: minDelay,
					Due:      defaultTime.Add(time.Hour),
				},
			},
			updateCalls: []*updateCall{
				{
					repo:     a,
					time:     defaultTime.Add(time.Second),
					interval: 123 * time.Second,
				},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{
					Repo:     a,
					Interval: 123 * time.Second,
					Due:      defaultTime.Add(124 * time.Second),
				},
			},
			timeAfterFuncDelays: []time.Duration{123 * time.Second},
			wakeupNotifications: 1,
		},
		{
			name: "minimum interval",
			initialSchedule: []*scheduledRepoUpdate{
				{
					Repo:     a,
					Interval: maxDelay,
					Due:      defaultTime.Add(maxDelay),
				},
			},
			updateCalls: []*updateCall{
				{
					repo:     a,
					time:     defaultTime,
					interval: time.Second,
				},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{
					Repo:     a,
					Interval: minDelay,
					Due:      defaultTime.Add(minDelay),
				},
			},
			timeAfterFuncDelays: []time.Duration{minDelay},
			wakeupNotifications: 1,
		},
		{
			name: "maximum interval",
			initialSchedule: []*scheduledRepoUpdate{
				{
					Repo:     a,
					Interval: minDelay,
					Due:      defaultTime.Add(minDelay),
				},
			},
			updateCalls: []*updateCall{
				{
					repo:     a,
					time:     defaultTime,
					interval: 365 * 25 * time.Hour,
				},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{
					Repo:     a,
					Interval: maxDelay,
					Due:      defaultTime.Add(maxDelay),
				},
			},
			timeAfterFuncDelays: []time.Duration{maxDelay},
			wakeupNotifications: 1,
		},
		{
			name: "update later",
			initialSchedule: []*scheduledRepoUpdate{
				{
					Repo:     a,
					Interval: minDelay,
					Due:      defaultTime.Add(time.Hour),
				},
			},
			updateCalls: []*updateCall{
				{
					repo:     a,
					time:     defaultTime.Add(time.Second),
					interval: 123 * time.Minute,
				},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{
					Repo:     a,
					Interval: 123 * time.Minute,
					Due:      defaultTime.Add(time.Second + 123*time.Minute),
				},
			},
			timeAfterFuncDelays: []time.Duration{123 * time.Minute},
			wakeupNotifications: 1,
		},
		{
			name: "heap reorders correctly",
			initialSchedule: []*scheduledRepoUpdate{
				{Repo: c, Interval: minDelay, Due: defaultTime.Add(1 * time.Minute)},
				{Repo: d, Interval: minDelay, Due: defaultTime.Add(2 * time.Minute)},
				{Repo: a, Interval: minDelay, Due: defaultTime.Add(3 * time.Minute)},
				{Repo: e, Interval: minDelay, Due: defaultTime.Add(4 * time.Minute)},
				{Repo: b, Interval: minDelay, Due: defaultTime.Add(5 * time.Minute)},
			},
			updateCalls: []*updateCall{
				{repo: a, time: defaultTime, interval: 1 * time.Minute},
				{repo: b, time: defaultTime, interval: 2 * time.Minute},
				{repo: c, time: defaultTime, interval: 3 * time.Minute},
				{repo: d, time: defaultTime, interval: 4 * time.Minute},
				{repo: e, time: defaultTime, interval: 5 * time.Minute},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: 1 * time.Minute, Due: defaultTime.Add(1 * time.Minute)},
				{Repo: b, Interval: 2 * time.Minute, Due: defaultTime.Add(2 * time.Minute)},
				{Repo: c, Interval: 3 * time.Minute, Due: defaultTime.Add(3 * time.Minute)},
				{Repo: d, Interval: 4 * time.Minute, Due: defaultTime.Add(4 * time.Minute)},
				{Repo: e, Interval: 5 * time.Minute, Due: defaultTime.Add(5 * time.Minute)},
			},
			timeAfterFuncDelays: []time.Duration{time.Minute, time.Minute, time.Minute, time.Minute, time.Minute},
			wakeupNotifications: 5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, stop := startRecording()
			defer stop()

			s := NewUpdateScheduler(logtest.Scoped(t), dbmocks.NewMockDB(), gitserver.NewMockClient())
			setupInitialSchedule(s, test.initialSchedule)
			s.schedule.randGenerator = &mockRandomGenerator{}

			for _, call := range test.updateCalls {
				mockTime(call.time)
				s.schedule.updateInterval(call.repo, call.interval)
			}

			verifySchedule(t, s, test.finalSchedule)
			verifyScheduleRecording(t, s, test.timeAfterFuncDelays, test.wakeupNotifications, r)
		})
	}
}

func TestSchedule_remove(t *testing.T) {
	a := configuredRepo{ID: 1, Name: "a"}
	b := configuredRepo{ID: 2, Name: "b"}
	c := configuredRepo{ID: 3, Name: "c"}

	type removeCall struct {
		time time.Time
		repo configuredRepo
	}

	tests := []struct {
		name                string
		initialSchedule     []*scheduledRepoUpdate
		removeCalls         []*removeCall
		finalSchedule       []*scheduledRepoUpdate
		timeAfterFuncDelays []time.Duration
		wakeupNotifications int
	}{
		{
			name: "remove on empty schedule",
			removeCalls: []*removeCall{
				{repo: a, time: defaultTime},
			},
		},
		{
			name: "remove has no effect if repo isn't in schedule",
			initialSchedule: []*scheduledRepoUpdate{
				{Repo: a},
			},
			removeCalls: []*removeCall{
				{repo: b},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{Repo: a},
			},
		},
		{
			name: "remove last scheduled doesn't reschedule timer",
			initialSchedule: []*scheduledRepoUpdate{
				{Repo: a},
			},
			removeCalls: []*removeCall{
				{repo: a},
			},
		},
		{
			name: "remove next reschedules timer",
			initialSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: minDelay, Due: defaultTime},
				{Repo: b, Interval: minDelay, Due: defaultTime.Add(minDelay)},
				{Repo: c, Interval: maxDelay, Due: defaultTime.Add(maxDelay)},
			},
			removeCalls: []*removeCall{
				{repo: a, time: defaultTime},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{Repo: b, Interval: minDelay, Due: defaultTime.Add(minDelay)},
				{Repo: c, Interval: maxDelay, Due: defaultTime.Add(maxDelay)},
			},
			timeAfterFuncDelays: []time.Duration{minDelay},
			wakeupNotifications: 1,
		},
		{
			name: "remove not-next doesn't reschedule timer",
			initialSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: minDelay, Due: defaultTime},
				{Repo: b, Interval: minDelay, Due: defaultTime.Add(minDelay)},
				{Repo: c, Interval: maxDelay, Due: defaultTime.Add(maxDelay)},
			},
			removeCalls: []*removeCall{
				{repo: b, time: defaultTime},
				{repo: c, time: defaultTime},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: minDelay, Due: defaultTime},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, stop := startRecording()
			defer stop()

			s := NewUpdateScheduler(logtest.Scoped(t), dbmocks.NewMockDB(), gitserver.NewMockClient())
			setupInitialSchedule(s, test.initialSchedule)

			for _, call := range test.removeCalls {
				mockTime(call.time)
				s.schedule.remove(call.repo)
			}

			verifySchedule(t, s, test.finalSchedule)
			verifyScheduleRecording(t, s, test.timeAfterFuncDelays, test.wakeupNotifications, r)
		})
	}
}

func setupInitialSchedule(s *UpdateScheduler, initialSchedule []*scheduledRepoUpdate) {
	for _, update := range initialSchedule {
		heap.Push(s.schedule, update)
	}
}

func verifySchedule(t *testing.T, s *UpdateScheduler, expected []*scheduledRepoUpdate) {
	t.Helper()

	var actualSchedule []*scheduledRepoUpdate
	for len(s.schedule.heap) > 0 {
		update := heap.Pop(s.schedule).(*scheduledRepoUpdate)
		update.Index = 0 // this will always be -1, but easier to set it to 0 to avoid boilerplate in test cases
		actualSchedule = append(actualSchedule, update)
	}

	if !reflect.DeepEqual(expected, actualSchedule) {
		t.Fatalf("\nexpected final schedule\n%s\ngot\n%s", spew.Sdump(expected), spew.Sdump(actualSchedule))
	}
}

func verifyScheduleRecording(t *testing.T, s *UpdateScheduler, timeAfterFuncDelays []time.Duration, wakeupNotifications int, r *recording) {
	t.Helper()

	if !reflect.DeepEqual(timeAfterFuncDelays, r.timeAfterFuncDelays) {
		t.Fatalf("\nexpected timeAfterFuncDelays\n%s\ngot\n%s", spew.Sdump(timeAfterFuncDelays), spew.Sdump(r.timeAfterFuncDelays))
	}

	if l := len(r.notifications); l != wakeupNotifications {
		t.Fatalf("expected %d notifications; got %d", wakeupNotifications, l)
	}

	for _, n := range r.notifications {
		if n != s.schedule.wakeup {
			t.Fatalf("received notification on wrong channel")
		}
	}
}

func TestUpdateScheduler_runSchedule(t *testing.T) {
	a := configuredRepo{ID: 1, Name: "a"}
	b := configuredRepo{ID: 2, Name: "b"}
	c := configuredRepo{ID: 3, Name: "c"}
	d := configuredRepo{ID: 4, Name: "d"}
	e := configuredRepo{ID: 5, Name: "e"}

	tests := []struct {
		name                  string
		initialSchedule       []*scheduledRepoUpdate
		finalSchedule         []*scheduledRepoUpdate
		finalQueue            []*repoUpdate
		timeAfterFuncDelays   []time.Duration
		expectedNotifications func(s *UpdateScheduler) []chan struct{}
	}{
		{
			name: "empty schedule",
		},
		{
			name: "no updates due",
			initialSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: 11 * time.Second, Due: defaultTime.Add(time.Minute)},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: 11 * time.Second, Due: defaultTime.Add(time.Minute)},
			},
			timeAfterFuncDelays: []time.Duration{time.Minute},
			expectedNotifications: func(s *UpdateScheduler) []chan struct{} {
				return []chan struct{}{s.schedule.wakeup}
			},
		},
		{
			name: "one update due, rescheduled to front",
			initialSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: 11 * time.Second, Due: defaultTime.Add(1 * time.Microsecond)},
				{Repo: b, Interval: 22 * time.Second, Due: defaultTime.Add(time.Minute)},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: 11 * time.Second, Due: defaultTime.Add(11 * time.Second)},
				{Repo: b, Interval: 22 * time.Second, Due: defaultTime.Add(time.Minute)},
			},
			finalQueue: []*repoUpdate{
				{Repo: a, Priority: priorityLow, Seq: 1},
			},
			timeAfterFuncDelays: []time.Duration{11 * time.Second},
			expectedNotifications: func(s *UpdateScheduler) []chan struct{} {
				return []chan struct{}{s.updateQueue.notifyEnqueue, s.schedule.wakeup}
			},
		},
		{
			name: "one update due, rescheduled to back",
			initialSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: 11 * time.Minute, Due: defaultTime},
				{Repo: b, Interval: 22 * time.Second, Due: defaultTime.Add(time.Minute)},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{Repo: b, Interval: 22 * time.Second, Due: defaultTime.Add(time.Minute)},
				{Repo: a, Interval: 11 * time.Minute, Due: defaultTime.Add(11 * time.Minute)},
			},
			finalQueue: []*repoUpdate{
				{Repo: a, Priority: priorityLow, Seq: 1},
			},
			timeAfterFuncDelays: []time.Duration{time.Minute},
			expectedNotifications: func(s *UpdateScheduler) []chan struct{} {
				return []chan struct{}{s.updateQueue.notifyEnqueue, s.schedule.wakeup}
			},
		},
		{
			name: "all updates due",
			initialSchedule: []*scheduledRepoUpdate{
				{Repo: c, Interval: 3 * time.Minute, Due: defaultTime.Add(-5 * time.Minute)},
				{Repo: d, Interval: 4 * time.Minute, Due: defaultTime.Add(-4 * time.Minute)},
				{Repo: a, Interval: 1 * time.Minute, Due: defaultTime.Add(-3 * time.Minute)},
				{Repo: e, Interval: 5 * time.Minute, Due: defaultTime.Add(-2 * time.Minute)},
				{Repo: b, Interval: 2 * time.Minute, Due: defaultTime.Add(-1 * time.Minute)},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: 1 * time.Minute, Due: defaultTime.Add(1 * time.Minute)},
				{Repo: b, Interval: 2 * time.Minute, Due: defaultTime.Add(2 * time.Minute)},
				{Repo: c, Interval: 3 * time.Minute, Due: defaultTime.Add(3 * time.Minute)},
				{Repo: d, Interval: 4 * time.Minute, Due: defaultTime.Add(4 * time.Minute)},
				{Repo: e, Interval: 5 * time.Minute, Due: defaultTime.Add(5 * time.Minute)},
			},
			finalQueue: []*repoUpdate{
				{Repo: c, Priority: priorityLow, Seq: 1},
				{Repo: d, Priority: priorityLow, Seq: 2},
				{Repo: a, Priority: priorityLow, Seq: 3},
				{Repo: e, Priority: priorityLow, Seq: 4},
				{Repo: b, Priority: priorityLow, Seq: 5},
			},
			timeAfterFuncDelays: []time.Duration{1 * time.Minute},
			expectedNotifications: func(s *UpdateScheduler) []chan struct{} {
				return []chan struct{}{
					s.updateQueue.notifyEnqueue,
					s.updateQueue.notifyEnqueue,
					s.updateQueue.notifyEnqueue,
					s.updateQueue.notifyEnqueue,
					s.updateQueue.notifyEnqueue,
					s.schedule.wakeup,
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, stop := startRecording()
			defer stop()

			s := NewUpdateScheduler(logtest.Scoped(t), dbmocks.NewMockDB(), gitserver.NewMockClient())

			setupInitialSchedule(s, test.initialSchedule)

			s.runSchedule()

			verifySchedule(t, s, test.finalSchedule)
			verifyQueue(t, s, test.finalQueue)
			verifyRecording(t, s, test.timeAfterFuncDelays, test.expectedNotifications, r)
		})
	}
}

func TestUpdateScheduler_runUpdateLoop(t *testing.T) {
	a := configuredRepo{ID: 1, Name: "a"}
	b := configuredRepo{ID: 2, Name: "b"}
	c := configuredRepo{ID: 3, Name: "c"}

	type mockRequestRepoUpdate struct {
		repo configuredRepo
		resp *gitserverprotocol.RepoUpdateResponse
		err  error
	}

	tests := []struct {
		name                   string
		gitMaxConcurrentClones int
		initialSchedule        []*scheduledRepoUpdate
		initialQueue           []*repoUpdate
		mockRequestRepoUpdates []*mockRequestRepoUpdate
		finalSchedule          []*scheduledRepoUpdate
		finalQueue             []*repoUpdate
		timeAfterFuncDelays    []time.Duration
		expectedNotifications  func(s *UpdateScheduler) []chan struct{}
	}{
		{
			name: "empty queue",
		},
		{
			name: "non-empty queue at clone limit",
			initialQueue: []*repoUpdate{
				{Repo: a, Seq: 1},
			},
			finalQueue: []*repoUpdate{
				{Repo: a, Seq: 1},
			},
		},
		{
			name:                   "queue drains",
			gitMaxConcurrentClones: 1,
			initialQueue: []*repoUpdate{
				{Repo: a, Seq: 1},
				{Repo: b, Seq: 2},
				{Repo: c, Seq: 3},
			},
			mockRequestRepoUpdates: []*mockRequestRepoUpdate{
				{repo: a},
				{repo: b},
				{repo: c},
			},
		},
		{
			name:                   "schedule updated",
			gitMaxConcurrentClones: 1,
			initialSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: time.Hour, Due: defaultTime.Add(time.Hour)},
			},
			initialQueue: []*repoUpdate{
				{Repo: a, Seq: 1},
				{Repo: b, Seq: 1},
			},
			mockRequestRepoUpdates: []*mockRequestRepoUpdate{
				{
					repo: a,
					resp: &gitserverprotocol.RepoUpdateResponse{
						LastFetched: pointers.Ptr(defaultTime.Add(2 * time.Minute)),
						LastChanged: pointers.Ptr(defaultTime),
					},
				},
				{
					repo: b,
					resp: &gitserverprotocol.RepoUpdateResponse{
						LastFetched: pointers.Ptr(defaultTime.Add(2 * time.Minute)),
						LastChanged: pointers.Ptr(defaultTime),
					},
				},
			},
			finalSchedule: []*scheduledRepoUpdate{
				{Repo: a, Interval: time.Minute, Due: defaultTime.Add(time.Minute)},
			},
			timeAfterFuncDelays: []time.Duration{time.Minute},
			expectedNotifications: func(s *UpdateScheduler) []chan struct{} {
				return []chan struct{}{s.schedule.wakeup}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, stop := startRecording()
			defer stop()

			configuredLimiter = func() *limiter.MutableLimiter {
				return limiter.NewMutable(test.gitMaxConcurrentClones)
			}
			defer func() {
				configuredLimiter = nil
			}()

			expectedRequestCount := len(test.mockRequestRepoUpdates)
			mockRequestRepoUpdates := make(chan *mockRequestRepoUpdate, expectedRequestCount)
			for _, m := range test.mockRequestRepoUpdates {
				mockRequestRepoUpdates <- m
			}
			// intentionally don't close the channel so any further receives just block

			contexts := make(chan context.Context, expectedRequestCount)
			db := dbmocks.NewMockDB()
			gs := gitserver.NewMockClient()
			gs.RequestRepoUpdateFunc.SetDefaultHook(func(ctx context.Context, repo api.RepoName) (*gitserverprotocol.RepoUpdateResponse, error) {
				select {
				case mock := <-mockRequestRepoUpdates:
					if !reflect.DeepEqual(mock.repo.Name, repo) {
						t.Errorf("\nexpected requestRepoUpdate\n%s\ngot\n%s", spew.Sdump(mock.repo), spew.Sdump(repo))
					}
					contexts <- ctx // Intercept all contexts so we can wait for spawned goroutines to finish.
					return mock.resp, mock.err
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			})

			s := NewUpdateScheduler(logtest.Scoped(t), db, gs)
			s.schedule.randGenerator = &mockRandomGenerator{}

			// unbuffer the channel
			s.updateQueue.notifyEnqueue = make(chan struct{})

			setupInitialSchedule(s, test.initialSchedule)
			setupInitialQueue(s, test.initialQueue)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			done := make(chan struct{})
			go func() {
				s.runUpdateLoop(ctx)
				close(done)
			}()

			// Let the goroutine do a single loop.
			s.updateQueue.notifyEnqueue <- struct{}{}

			// Wait for all goroutines that have a mock request to finish.
			// There may be additional goroutines which don't have a mock request
			// and will block until the context is canceled.
			for range expectedRequestCount {
				ctx := <-contexts
				<-ctx.Done()
			}

			verifySchedule(t, s, test.finalSchedule)
			verifyQueue(t, s, test.finalQueue)
			verifyRecording(t, s, test.timeAfterFuncDelays, test.expectedNotifications, r)

			// Cancel the context.
			cancel()

			// Wait for the goroutine to exit.
			<-done
		})
	}
}

func verifyRecording(t *testing.T, s *UpdateScheduler, timeAfterFuncDelays []time.Duration, expectedNotifications func(s *UpdateScheduler) []chan struct{}, r *recording) {
	if !reflect.DeepEqual(timeAfterFuncDelays, r.timeAfterFuncDelays) {
		t.Fatalf("\nexpected timeAfterFuncDelays\n%s\ngot\n%s", spew.Sdump(timeAfterFuncDelays), spew.Sdump(r.timeAfterFuncDelays))
	}

	if expectedNotifications == nil {
		expectedNotifications = func(s *UpdateScheduler) []chan struct{} {
			return nil
		}
	}

	if expected := expectedNotifications(s); !reflect.DeepEqual(expected, r.notifications) {
		t.Fatalf("\nexpected notifications\n%s\ngot\n%s", spew.Sdump(expected), spew.Sdump(r.notifications))
	}
}

func Test_updateQueue_Less(t *testing.T) {
	q := &updateQueue{}
	tests := []struct {
		name   string
		heap   []*repoUpdate
		expVal bool
	}{
		{
			name: "updating",
			heap: []*repoUpdate{
				{Updating: false},
				{Updating: true},
			},
			expVal: true,
		},
		{
			name: "priority",
			heap: []*repoUpdate{
				{Priority: priorityHigh},
				{Priority: priorityLow},
			},
			expVal: true,
		},
		{
			name: "seq",
			heap: []*repoUpdate{
				{Seq: 1},
				{Seq: 2},
			},
			expVal: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q.heap = test.heap
			got := q.Less(0, 1)
			if test.expVal != got {
				t.Fatalf("want %v but got: %v", test.expVal, got)
			}
		})
	}
}

func TestGetCustomInterval(t *testing.T) {
	for _, tc := range []struct {
		name     string
		c        *conf.Unified
		repoName string
		want     time.Duration
	}{
		{
			name:     "Nil config",
			c:        nil,
			repoName: "github.com/sourcegraph/sourcegraph",
			want:     0,
		},
		{
			name: "Single match",
			c: &conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					GitUpdateInterval: []*schema.UpdateIntervalRule{
						{
							Pattern:  "github.com",
							Interval: 1,
						},
					},
				},
			},
			repoName: "github.com/sourcegraph/sourcegraph",
			want:     1 * time.Minute,
		},
		{
			name: "No match",
			c: &conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					GitUpdateInterval: []*schema.UpdateIntervalRule{
						{
							Pattern:  "gitlab.com",
							Interval: 1,
						},
					},
				},
			},
			repoName: "github.com/sourcegraph/sourcegraph",
			want:     0 * time.Minute,
		},
		{
			name: "Second match",
			c: &conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					GitUpdateInterval: []*schema.UpdateIntervalRule{
						{
							Pattern:  "gitlab.com",
							Interval: 1,
						},
						{
							Pattern:  "github.com",
							Interval: 2,
						},
					},
				},
			},
			repoName: "github.com/sourcegraph/sourcegraph",
			want:     2 * time.Minute,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			interval := getCustomInterval(logtest.Scoped(t), tc.c, tc.repoName)
			if tc.want != interval {
				t.Fatalf("Want %v, got %v", tc.want, interval)
			}
		})
	}
}
