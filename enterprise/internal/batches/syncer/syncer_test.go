package syncer

import (
	"container/heap"
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
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

	for i := 0; i < len(items); i++ {
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

func TestSyncerRun(t *testing.T) {
	t.Run("Sync due", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		now := time.Now()
		store := MockSyncStore{
			listChangesetSyncData: func(ctx context.Context, opts store.ListChangesetSyncDataOpts) ([]*batches.ChangesetSyncData, error) {
				return []*batches.ChangesetSyncData{
					{
						ChangesetID:       1,
						UpdatedAt:         now.Add(-2 * maxSyncDelay),
						LatestEvent:       now.Add(-2 * maxSyncDelay),
						ExternalUpdatedAt: now.Add(-2 * maxSyncDelay),
					},
				}, nil
			},
		}
		syncFunc := func(ctx context.Context, ids int64) error {
			cancel()
			return nil
		}
		syncer := &changesetSyncer{
			syncStore:        store,
			scheduleInterval: 10 * time.Minute,
			syncFunc:         syncFunc,
		}
		go syncer.Run(ctx)
		select {
		case <-ctx.Done():
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Sync should have been triggered")
		}
	})

	t.Run("Sync due but reenqueued for reconciler", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		now := time.Now()
		updateCalled := false
		store := MockSyncStore{
			getChangeset: func(context.Context, store.GetChangesetOpts) (*batches.Changeset, error) {
				// Return ErrNoResults, which is the result you get when the changeset preconditions aren't met anymore.
				// The sync data checks for the reconciler state and if it changed since the sync data was loaded,
				// we don't get back the changeset here and skip it.
				//
				// If we don't return ErrNoResults, the rest of the test will fail, because not all
				// methods of sync store are mocked.
				return nil, store.ErrNoResults
			},
			updateChangeset: func(context.Context, *batches.Changeset) error {
				updateCalled = true
				return nil
			},
			listChangesetSyncData: func(ctx context.Context, opts store.ListChangesetSyncDataOpts) ([]*batches.ChangesetSyncData, error) {
				return []*batches.ChangesetSyncData{
					{
						ChangesetID:       1,
						UpdatedAt:         now.Add(-2 * maxSyncDelay),
						LatestEvent:       now.Add(-2 * maxSyncDelay),
						ExternalUpdatedAt: now.Add(-2 * maxSyncDelay),
					},
				}, nil
			},
		}
		syncer := &changesetSyncer{
			syncStore:        store,
			scheduleInterval: 10 * time.Minute,
		}
		syncer.Run(ctx)
		if updateCalled {
			t.Fatal("Called UpdateChangeset, but shouldn't have")
		}
	})

	t.Run("Sync not due", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		now := time.Now()
		store := MockSyncStore{
			listChangesetSyncData: func(ctx context.Context, opts store.ListChangesetSyncDataOpts) ([]*batches.ChangesetSyncData, error) {
				return []*batches.ChangesetSyncData{
					{
						ChangesetID:       1,
						UpdatedAt:         now,
						LatestEvent:       now,
						ExternalUpdatedAt: now,
					},
				}, nil
			},
		}
		var syncCalled bool
		syncFunc := func(ctx context.Context, ids int64) error {
			syncCalled = true
			return nil
		}
		syncer := &changesetSyncer{
			syncStore:        store,
			scheduleInterval: 10 * time.Minute,
			syncFunc:         syncFunc,
		}
		syncer.Run(ctx)
		if syncCalled {
			t.Fatal("Sync should not have been triggered")
		}
	})

	t.Run("Priority added", func(t *testing.T) {
		// Empty schedule but then we add an item
		ctx, cancel := context.WithCancel(context.Background())
		store := MockSyncStore{
			listChangesetSyncData: func(ctx context.Context, opts store.ListChangesetSyncDataOpts) ([]*batches.ChangesetSyncData, error) {
				return []*batches.ChangesetSyncData{}, nil
			},
		}
		syncFunc := func(ctx context.Context, ids int64) error {
			cancel()
			return nil
		}
		syncer := &changesetSyncer{
			syncStore:        store,
			scheduleInterval: 10 * time.Minute,
			syncFunc:         syncFunc,
			priorityNotify:   make(chan []int64, 1),
		}
		syncer.priorityNotify <- []int64{1}
		go syncer.Run(ctx)
		select {
		case <-ctx.Done():
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Sync not called")
		}
	})
}

func TestSyncRegistry(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now := time.Now()

	externalServiceID := "https://example.com/"

	codeHosts := []*batches.CodeHost{{ExternalServiceID: externalServiceID, ExternalServiceType: extsvc.TypeGitHub}}

	syncStore := MockSyncStore{
		listChangesetSyncData: func(ctx context.Context, opts store.ListChangesetSyncDataOpts) (data []*batches.ChangesetSyncData, err error) {
			return []*batches.ChangesetSyncData{
				{
					ChangesetID:           1,
					UpdatedAt:             now,
					RepoExternalServiceID: externalServiceID,
				},
			}, nil
		},
		listCodeHosts: func(c context.Context, lcho store.ListCodeHostsOpts) ([]*batches.CodeHost, error) {
			return codeHosts, nil
		},
	}

	r := NewSyncRegistry(ctx, syncStore, nil, nil, nil)

	assertSyncerCount := func(want int) {
		r.mu.Lock()
		if len(r.syncers) != want {
			t.Fatalf("Expected %d syncer, got %d", want, len(r.syncers))
		}
		r.mu.Unlock()
	}

	assertSyncerCount(1)

	// Adding it again should have no effect
	r.Add(&batches.CodeHost{ExternalServiceID: "https://example.com/", ExternalServiceType: extsvc.TypeGitHub})
	assertSyncerCount(1)

	// Simulate a service being removed
	oldCodeHosts := codeHosts
	codeHosts = []*batches.CodeHost{}
	r.HandleExternalServiceSync(api.ExternalService{
		ID:        1,
		Kind:      extsvc.KindGitHub,
		Config:    `{"url": "https://example.com/"}`,
		DeletedAt: now,
	})
	assertSyncerCount(0)
	codeHosts = oldCodeHosts

	// And added again
	r.HandleExternalServiceSync(api.ExternalService{
		ID:   1,
		Kind: extsvc.KindGitHub,
	})
	assertSyncerCount(1)

	syncChan := make(chan int64, 1)

	// In order to test that priority items are delivered we'll inject our own syncer
	// with a custom sync func
	syncer := &changesetSyncer{
		syncStore:   syncStore,
		reposStore:  nil,
		codeHostURL: "https://example.com/",
		syncFunc: func(ctx context.Context, id int64) error {
			syncChan <- id
			return nil
		},
		priorityNotify: make(chan []int64, 1),
	}
	go syncer.Run(ctx)

	// Set the syncer
	r.mu.Lock()
	r.syncers["https://example.com/"] = syncer
	r.mu.Unlock()

	// Send priority items
	err := r.EnqueueChangesetSyncs(ctx, []int64{1, 2})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case id := <-syncChan:
		if id != 1 {
			t.Fatalf("Expected 1, got %d", id)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for sync")
	}
}

type MockSyncStore struct {
	listCodeHosts         func(context.Context, store.ListCodeHostsOpts) ([]*batches.CodeHost, error)
	listChangesetSyncData func(context.Context, store.ListChangesetSyncDataOpts) ([]*batches.ChangesetSyncData, error)
	getChangeset          func(context.Context, store.GetChangesetOpts) (*batches.Changeset, error)
	updateChangeset       func(context.Context, *batches.Changeset) error
	upsertChangesetEvents func(context.Context, ...*batches.ChangesetEvent) error
	transact              func(context.Context) (*store.Store, error)
}

func (m MockSyncStore) ListChangesetSyncData(ctx context.Context, opts store.ListChangesetSyncDataOpts) ([]*batches.ChangesetSyncData, error) {
	return m.listChangesetSyncData(ctx, opts)
}

func (m MockSyncStore) GetChangeset(ctx context.Context, opts store.GetChangesetOpts) (*batches.Changeset, error) {
	return m.getChangeset(ctx, opts)
}

func (m MockSyncStore) UpdateChangeset(ctx context.Context, c *batches.Changeset) error {
	return m.updateChangeset(ctx, c)
}

func (m MockSyncStore) UpsertChangesetEvents(ctx context.Context, cs ...*batches.ChangesetEvent) error {
	return m.upsertChangesetEvents(ctx, cs...)
}

func (m MockSyncStore) Transact(ctx context.Context) (*store.Store, error) {
	return m.transact(ctx)
}

func (m MockSyncStore) Repos() *database.RepoStore {
	return database.GlobalRepos
}

func (m MockSyncStore) Clock() func() time.Time {
	return func() time.Time {
		return time.Now()
	}
}

func (m MockSyncStore) ListCodeHosts(ctx context.Context, opts store.ListCodeHostsOpts) ([]*batches.CodeHost, error) {
	return m.listCodeHosts(ctx, opts)
}

type MockRepoStore struct {
	get func(ctx context.Context, id api.RepoID) (*types.Repo, error)
}

func (m MockRepoStore) Get(ctx context.Context, id api.RepoID) (*types.Repo, error) {
	return m.get(ctx, id)
}

type MockExternalServiceStore struct {
	list func(context.Context, database.ExternalServicesListOptions) ([]*types.ExternalService, error)
}

func (m MockExternalServiceStore) List(ctx context.Context, args database.ExternalServicesListOptions) ([]*types.ExternalService, error) {
	return m.list(ctx, args)
}
