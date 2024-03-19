package database

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/cockroachdb/errors/errbase"
	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgconn"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestExecutorsList(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	store := db.Executors().(*executorStore)
	ctx := context.Background()

	executors := []types.Executor{
		{Hostname: "h1", QueueName: "q1", OS: "win", Architecture: "amd", DockerVersion: "d1", ExecutorVersion: "e1", GitVersion: "g1", IgniteVersion: "i1", SrcCliVersion: "s1"}, // id=1
		{Hostname: "h2", QueueName: "q2", OS: "win", Architecture: "x86", DockerVersion: "d2", ExecutorVersion: "e2", GitVersion: "g2", IgniteVersion: "i2", SrcCliVersion: "s2"}, // id=2
		{Hostname: "h3", QueueName: "q3", OS: "win", Architecture: "amd", DockerVersion: "d3", ExecutorVersion: "e3", GitVersion: "g3", IgniteVersion: "i3", SrcCliVersion: "s3"}, // id=3
		{Hostname: "h4", QueueName: "q4", OS: "win", Architecture: "x86", DockerVersion: "d1", ExecutorVersion: "e4", GitVersion: "g4", IgniteVersion: "i4", SrcCliVersion: "s4"}, // id=4
		{Hostname: "h5", QueueName: "q5", OS: "win", Architecture: "amd", DockerVersion: "d2", ExecutorVersion: "e1", GitVersion: "g5", IgniteVersion: "i5", SrcCliVersion: "s5"}, // id=5
		{Hostname: "h6", QueueName: "q6", OS: "mac", Architecture: "x86", DockerVersion: "d3", ExecutorVersion: "e2", GitVersion: "g1", IgniteVersion: "i6", SrcCliVersion: "s6"}, // id=6
		{Hostname: "h7", QueueName: "q7", OS: "mac", Architecture: "amd", DockerVersion: "d1", ExecutorVersion: "e3", GitVersion: "g2", IgniteVersion: "i1", SrcCliVersion: "s7"}, // id=7
		{Hostname: "h8", QueueName: "q8", OS: "mac", Architecture: "x86", DockerVersion: "d2", ExecutorVersion: "e4", GitVersion: "g3", IgniteVersion: "i2", SrcCliVersion: "s1"}, // id=8
		{Hostname: "h9", QueueName: "q9", OS: "mac", Architecture: "amd", DockerVersion: "d3", ExecutorVersion: "e1", GitVersion: "g4", IgniteVersion: "i3", SrcCliVersion: "s2"}, // id=9
		{Hostname: "h0", QueueName: "q0", OS: "mac", Architecture: "x86", DockerVersion: "d1", ExecutorVersion: "e2", GitVersion: "g5", IgniteVersion: "i4", SrcCliVersion: "s3"}, // id=10
	}

	for _, executor := range executors {
		db.Executors().UpsertHeartbeat(ctx, executor)
	}

	now := time.Unix(1587396557, 0).UTC()
	t1 := now.Add(-time.Minute * 10) // active
	t2 := now.Add(-time.Minute * 45) // inactive

	lastSeenAtByID := map[int]time.Time{
		1:  t1,
		2:  t1,
		3:  t1,
		4:  t1,
		5:  t1,
		6:  t2,
		7:  t2,
		8:  t2,
		9:  t2,
		10: t2,
	}
	for id, lastSeenAt := range lastSeenAtByID {
		q := sqlf.Sprintf(`UPDATE executor_heartbeats SET last_seen_at = %s WHERE id = %s`, lastSeenAt, id)
		if _, err := db.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
			t.Fatalf("failed to set up executors for test: %s", err)
		}
	}

	type testCase struct {
		query       string
		active      bool
		expectedIDs []int
	}
	testCases := []testCase{
		{expectedIDs: []int{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}},
		{query: "win", expectedIDs: []int{5, 4, 3, 2, 1}},  // test search by OS
		{query: "x86", expectedIDs: []int{10, 8, 6, 4, 2}}, // test search by architecture
		{query: "d2", expectedIDs: []int{8, 5, 2}},         // test search by docker version
		{query: "e2", expectedIDs: []int{10, 6, 2}},        // test search by executor version
		{query: "g2", expectedIDs: []int{7, 2}},            // test search by git version
		{query: "i2", expectedIDs: []int{8, 2}},            // test search by ignite version
		{query: "s2", expectedIDs: []int{9, 2}},            // test search by src-cli version
		{active: true, expectedIDs: []int{5, 4, 3, 2, 1}},
	}

	runTest := func(testCase testCase, lo, hi int) (errors int) {
		name := fmt.Sprintf(
			"query=%q active=%v offset=%d",
			testCase.query,
			testCase.active,
			lo,
		)

		t.Run(name, func(t *testing.T) {
			opts := ExecutorStoreListOptions{
				Query:  testCase.query,
				Active: testCase.active,
				Limit:  3,
				Offset: lo,
			}
			executors, err := store.list(ctx, opts, now)
			if err != nil {
				t.Fatalf("unexpected error getting executors: %s", err)
			}
			totalCount, err := store.count(ctx, opts, now)
			if err != nil {
				t.Fatalf("unexpected error counting executors: %s", err)
			}
			if totalCount != len(testCase.expectedIDs) {
				t.Errorf("unexpected total count. want=%d have=%d", len(testCase.expectedIDs), totalCount)
				errors++
			}

			if totalCount != 0 {
				var ids []int
				for _, executor := range executors {
					ids = append(ids, executor.ID)
				}
				if diff := cmp.Diff(testCase.expectedIDs[lo:hi], ids); diff != "" {
					t.Errorf("unexpected executor ids at offset %d (-want +got):\n%s", lo, diff)
					errors++
				}
			}
		})

		return errors
	}

	for _, testCase := range testCases {
		if n := len(testCase.expectedIDs); n == 0 {
			runTest(testCase, 0, 0)
		} else {
			for lo := range n {
				if numErrors := runTest(testCase, lo, int(math.Min(float64(lo)+3, float64(n)))); numErrors > 0 {
					break
				}
			}
		}
	}
}

func TestExecutorsGetByID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	store := db.Executors().(*executorStore)
	ctx := context.Background()

	// Executor does not exist initially
	if _, exists, err := db.Executors().GetByID(ctx, 1); err != nil {
		t.Fatalf("unexpected error getting executor: %s", err)
	} else if exists {
		t.Fatal("unexpected record")
	}

	now := time.Unix(1587396557, 0).UTC()
	t1 := now.Add(-time.Minute * 15)
	t2 := now.Add(-time.Minute * 45)

	expected := types.Executor{
		ID:              1,
		Hostname:        "test-hostname",
		QueueName:       "test-queue-name",
		OS:              "test-os",
		Architecture:    "test-architecture",
		DockerVersion:   "test-docker-version",
		ExecutorVersion: "test-executor-version",
		GitVersion:      "test-git-version",
		IgniteVersion:   "test-ignite-version",
		SrcCliVersion:   "test-src-cli-version",
		FirstSeenAt:     t1,
		LastSeenAt:      t2,
	}

	// update first seen at
	if err := store.upsertHeartbeat(ctx, expected, t1); err != nil {
		t.Fatalf("unexpected error inserting heartbeat: %s", err)
	}

	expected.QueueName += "-changed"
	expected.OS += "-changed"
	expected.Architecture += "-changed"
	expected.DockerVersion += "-changed"
	expected.ExecutorVersion += "-changed"
	expected.GitVersion += "-changed"
	expected.IgniteVersion += "-changed"
	expected.SrcCliVersion += "-changed"

	// update values as well as last seen at
	if err := store.upsertHeartbeat(ctx, expected, t2); err != nil {
		t.Fatalf("unexpected error inserting heartbeat: %s", err)
	}

	if executor, exists, err := store.GetByID(ctx, 1); err != nil {
		t.Fatalf("unexpected error getting executor: %s", err)
	} else if !exists {
		t.Fatal("expected record to exist")
	} else if diff := cmp.Diff(expected, executor); diff != "" {
		t.Errorf("unexpected executor (-want +got):\n%s", diff)
	}
}

func TestExecutorsGetByHostname(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	store := db.Executors().(*executorStore)
	ctx := context.Background()

	hostname := "megahost-somuchfast"

	// Executor does not exist initially
	if _, exists, err := db.Executors().GetByHostname(ctx, hostname); err != nil {
		t.Fatalf("unexpected error getting executor: %s", err)
	} else if exists {
		t.Fatal("unexpected record")
	}

	now := time.Unix(1587396557, 0).UTC()
	t1 := now.Add(-time.Minute * 15)
	t2 := now.Add(-time.Minute * 45)

	expected := types.Executor{
		ID:              1,
		Hostname:        hostname,
		QueueName:       "test-queue-name",
		OS:              "test-os",
		Architecture:    "test-architecture",
		DockerVersion:   "test-docker-version",
		ExecutorVersion: "test-executor-version",
		GitVersion:      "test-git-version",
		IgniteVersion:   "test-ignite-version",
		SrcCliVersion:   "test-src-cli-version",
		FirstSeenAt:     t1,
		LastSeenAt:      t2,
	}

	// update first seen at
	if err := store.upsertHeartbeat(ctx, expected, t1); err != nil {
		t.Fatalf("unexpected error inserting heartbeat: %s", err)
	}

	expected.QueueName += "-changed"
	expected.OS += "-changed"
	expected.Architecture += "-changed"
	expected.DockerVersion += "-changed"
	expected.ExecutorVersion += "-changed"
	expected.GitVersion += "-changed"
	expected.IgniteVersion += "-changed"
	expected.SrcCliVersion += "-changed"

	// update values as well as last seen at
	if err := store.upsertHeartbeat(ctx, expected, t2); err != nil {
		t.Fatalf("unexpected error inserting heartbeat: %s", err)
	}

	if executor, exists, err := db.Executors().GetByHostname(ctx, hostname); err != nil {
		t.Fatalf("unexpected error getting executor: %s", err)
	} else if !exists {
		t.Fatal("expected record to exist")
	} else if diff := cmp.Diff(expected, executor); diff != "" {
		t.Errorf("unexpected executor (-want +got):\n%s", diff)
	}
}

func TestExecutorsDeleteInactiveHeartbeats(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	store := db.Executors().(*executorStore)
	ctx := context.Background()

	for i := range 10 {
		db.Executors().UpsertHeartbeat(ctx, types.Executor{Hostname: fmt.Sprintf("h%02d", i+1), QueueName: "q1"})
	}

	now := time.Unix(1587396557, 0).UTC()
	t1 := now.Add(-time.Minute * 10) // active
	t2 := now.Add(-time.Minute * 45) // inactive

	lastSeenAtByID := map[int]time.Time{
		1:  t1,
		2:  t1,
		3:  t1,
		4:  t1,
		5:  t1,
		6:  t2,
		7:  t2,
		8:  t2,
		9:  t2,
		10: t2,
	}
	for id, lastSeenAt := range lastSeenAtByID {
		q := sqlf.Sprintf(`UPDATE executor_heartbeats SET last_seen_at = %s WHERE id = %s`, lastSeenAt, id)
		if _, err := db.ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
			t.Fatalf("failed to set up executors for test: %s", err)
		}
	}

	if err := store.deleteInactiveHeartbeats(ctx, time.Minute*30, now); err != nil {
		t.Fatalf("unexpected error deleting inactive heartbeats: %s", err)
	}

	if totalCount, err := db.Executors().Count(ctx, ExecutorStoreListOptions{}); err != nil {
		t.Fatalf("unexpected error counting executors: %s", err)
	} else if totalCount != 5 {
		t.Fatalf("unexpected total count. want=%d have=%d", 5, totalCount)
	}
}

func TestExecutorsUpsertHeartbeat(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	tests := []struct {
		name                 string
		executor             types.Executor
		expectedErrorMessage string
	}{
		{
			name:     "Single queue defined",
			executor: types.Executor{Hostname: "happy_single_queue", QueueName: "single", OS: "win", Architecture: "amd", DockerVersion: "d1", ExecutorVersion: "e1", GitVersion: "g1", IgniteVersion: "i1", SrcCliVersion: "s1"},
		},
		{
			name:     "Multiple queues defined",
			executor: types.Executor{Hostname: "happy_multi_queue", QueueNames: []string{"multi1", "multi2"}, OS: "win", Architecture: "amd", DockerVersion: "d1", ExecutorVersion: "e1", GitVersion: "g1", IgniteVersion: "i1", SrcCliVersion: "s1"},
		},
		{
			name:                 "Both single queue and multiple queues defined",
			executor:             types.Executor{Hostname: "sad_both_defined", QueueName: "single", QueueNames: []string{"multi1", "multi2"}, OS: "win", Architecture: "amd", DockerVersion: "d1", ExecutorVersion: "e1", GitVersion: "g1", IgniteVersion: "i1", SrcCliVersion: "s1"},
			expectedErrorMessage: `new row for relation "executor_heartbeats" violates check constraint "one_of_queue_name_queue_names"`,
		},
		{
			name:                 "No queues defined",
			executor:             types.Executor{Hostname: "sad_none_defined", OS: "win", Architecture: "amd", DockerVersion: "d1", ExecutorVersion: "e1", GitVersion: "g1", IgniteVersion: "i1", SrcCliVersion: "s1"},
			expectedErrorMessage: `new row for relation "executor_heartbeats" violates check constraint "one_of_queue_name_queue_names"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := db.Executors().UpsertHeartbeat(ctx, test.executor)
			if err != nil {
				err = errbase.UnwrapAll(err)
				pgErr, ok := err.(*pgconn.PgError)
				if !ok {
					t.Fatalf("unexpected error while upserting heartbeat: %s", err)
				}
				if pgErr.Message != test.expectedErrorMessage {
					t.Errorf("Unexpected error while upserting heartbeat. expected=%s actual=%s", test.expectedErrorMessage, pgErr.Message)
				}
			}
		})
	}
}
