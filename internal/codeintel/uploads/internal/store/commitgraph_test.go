package store

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/commitgraph"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func TestSetRepositoryAsDirty(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	for _, id := range []int{50, 51, 52} {
		insertRepo(t, db, id, "", false)
	}

	for _, repositoryID := range []int{50, 51, 52, 51, 52} {
		if err := store.SetRepositoryAsDirty(context.Background(), repositoryID); err != nil {
			t.Errorf("unexpected error marking repository as dirty: %s", err)
		}
	}

	dirtyRepositories, err := store.GetDirtyRepositories(context.Background())
	if err != nil {
		t.Fatalf("unexpected error listing dirty repositories: %s", err)
	}

	var keys []int
	for _, dirtyRepository := range dirtyRepositories {
		keys = append(keys, dirtyRepository.RepositoryID)
	}
	sort.Ints(keys)

	if diff := cmp.Diff([]int{50, 51, 52}, keys); diff != "" {
		t.Errorf("unexpected repository ids (-want +got):\n%s", diff)
	}
}

func TestSkipsDeletedRepositories(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertRepo(t, db, 50, "should not be dirty", false)
	deleteRepo(t, db, 50, time.Now())
	insertRepo(t, db, 51, "should be dirty", false)

	// NOTE: We did not insert 52, so it should not show up as dirty, even though we mark it below.

	for _, repositoryID := range []int{50, 51, 52} {
		if err := store.SetRepositoryAsDirty(context.Background(), repositoryID); err != nil {
			t.Fatalf("unexpected error marking repository as dirty: %s", err)
		}
	}

	dirtyRepositories, err := store.GetDirtyRepositories(context.Background())
	if err != nil {
		t.Fatalf("unexpected error listing dirty repositories: %s", err)
	}

	var keys []int
	for _, dirtyRepository := range dirtyRepositories {
		keys = append(keys, dirtyRepository.RepositoryID)
	}
	sort.Ints(keys)

	if diff := cmp.Diff([]int{51}, keys); diff != "" {
		t.Errorf("unexpected repository ids (-want +got):\n%s", diff)
	}
}

func TestCalculateVisibleUploadsResetsDirtyFlagTransactionTimestamp(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(1)},
		{ID: 2, Commit: makeCommit(2)},
		{ID: 3, Commit: makeCommit(3)},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(3), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	refDescriptions := map[string][]gitdomain.RefDescription{
		makeCommit(3): {{IsDefaultBranch: true}},
	}

	for range 3 {
		// Set dirty token to 3
		if err := store.SetRepositoryAsDirty(context.Background(), 50); err != nil {
			t.Fatalf("unexpected error marking repository as dirty: %s", err)
		}
	}

	// This test is mainly a syntax check against `transaction_timestamp()`
	if err := store.UpdateUploadsVisibleToCommits(context.Background(), 50, graph, refDescriptions, time.Hour, time.Hour, 3, time.Now()); err != nil {
		t.Fatalf("unexpected error while calculating visible uploads: %s", err)
	}
}

func TestCalculateVisibleUploadsNonDefaultBranches(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	//                +-- [08] ----- {09} --+
	//                |                     |
	// [01] -- {02} --+-- [03] --+-- {04} --+-- {05} -- [06] -- {07}
	//                           |
	//                           +--- 10 ------ [11] -- {12}
	//
	// 02: tag v1
	// 04: tag v2
	// 05: tag v3
	// 07: tip of main branch
	// 09: tip of branch feat1
	// 12: tip of branch feat2

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(1)},
		{ID: 2, Commit: makeCommit(3)},
		{ID: 3, Commit: makeCommit(6)},
		{ID: 4, Commit: makeCommit(8)},
		{ID: 5, Commit: makeCommit(11)},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(12), makeCommit(11)}, " "),
		strings.Join([]string{makeCommit(11), makeCommit(10)}, " "),
		strings.Join([]string{makeCommit(10), makeCommit(3)}, " "),
		strings.Join([]string{makeCommit(7), makeCommit(6)}, " "),
		strings.Join([]string{makeCommit(6), makeCommit(5)}, " "),
		strings.Join([]string{makeCommit(5), makeCommit(4), makeCommit(9)}, " "),
		strings.Join([]string{makeCommit(9), makeCommit(8)}, " "),
		strings.Join([]string{makeCommit(8), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(4), makeCommit(3)}, " "),
		strings.Join([]string{makeCommit(3), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	t1 := time.Now().Add(-time.Minute * 90) // > 1 hr
	t2 := time.Now().Add(-time.Minute * 30) // < 1 hr

	refDescriptions := map[string][]gitdomain.RefDescription{
		// stale
		makeCommit(2): {{Name: "v1", Type: gitdomain.RefTypeTag, CreatedDate: &t1}},
		makeCommit(9): {{Name: "feat1", Type: gitdomain.RefTypeBranch, CreatedDate: &t1}},

		// fresh
		makeCommit(4):  {{Name: "v2", Type: gitdomain.RefTypeTag, CreatedDate: &t2}},
		makeCommit(5):  {{Name: "v3", Type: gitdomain.RefTypeTag, CreatedDate: &t2}},
		makeCommit(7):  {{Name: "main", Type: gitdomain.RefTypeBranch, IsDefaultBranch: true, CreatedDate: &t2}},
		makeCommit(12): {{Name: "feat2", Type: gitdomain.RefTypeBranch, CreatedDate: &t2}},
	}

	if err := store.UpdateUploadsVisibleToCommits(context.Background(), 50, graph, refDescriptions, time.Hour, time.Hour, 0, time.Now()); err != nil {
		t.Fatalf("unexpected error while calculating visible uploads: %s", err)
	}

	expectedVisibleUploads := map[string][]int{
		makeCommit(1):  {1},
		makeCommit(2):  {1},
		makeCommit(3):  {2},
		makeCommit(4):  {2},
		makeCommit(5):  {2},
		makeCommit(6):  {3},
		makeCommit(7):  {3},
		makeCommit(8):  {4},
		makeCommit(9):  {4},
		makeCommit(10): {2},
		makeCommit(11): {5},
		makeCommit(12): {5},
	}
	if diff := cmp.Diff(expectedVisibleUploads, getVisibleUploads(t, db, 50, keysOf(expectedVisibleUploads))); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	// Ensure data can be queried in reverse direction as well
	assertCommitsVisibleFromUploads(t, store, uploads, expectedVisibleUploads)

	if diff := cmp.Diff([]int{3}, getUploadsVisibleAtTip(t, db, 50)); diff != "" {
		t.Errorf("unexpected uploads visible at tip (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]int{2, 3, 5}, getProtectedUploads(t, db, 50)); diff != "" {
		t.Errorf("unexpected protected uploads (-want +got):\n%s", diff)
	}
}

func TestCalculateVisibleUploadsNonDefaultBranchesWithCustomRetentionConfiguration(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	//                +-- [08] ----- {09} --+
	//                |                     |
	// [01] -- {02} --+-- [03] --+-- {04} --+-- {05} -- [06] -- {07}
	//                           |
	//                           +--- 10 ------ [11] -- {12}
	//
	// 02: tag v1
	// 04: tag v2
	// 05: tag v3
	// 07: tip of main branch
	// 09: tip of branch feat1
	// 12: tip of branch feat2

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(1)},
		{ID: 2, Commit: makeCommit(3)},
		{ID: 3, Commit: makeCommit(6)},
		{ID: 4, Commit: makeCommit(8)},
		{ID: 5, Commit: makeCommit(11)},
	}
	insertUploads(t, db, uploads...)

	retentionConfigurationQuery := `
		INSERT INTO lsif_retention_configuration (
			id,
			repository_id,
			max_age_for_non_stale_branches_seconds,
			max_age_for_non_stale_tags_seconds
		) VALUES (
			1,
			50,
			3600,
			3600
		)
	`
	if _, err := db.ExecContext(context.Background(), retentionConfigurationQuery); err != nil {
		t.Fatalf("unexpected error inserting retention configuration: %s", err)
	}

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(12), makeCommit(11)}, " "),
		strings.Join([]string{makeCommit(11), makeCommit(10)}, " "),
		strings.Join([]string{makeCommit(10), makeCommit(3)}, " "),
		strings.Join([]string{makeCommit(7), makeCommit(6)}, " "),
		strings.Join([]string{makeCommit(6), makeCommit(5)}, " "),
		strings.Join([]string{makeCommit(5), makeCommit(4), makeCommit(9)}, " "),
		strings.Join([]string{makeCommit(9), makeCommit(8)}, " "),
		strings.Join([]string{makeCommit(8), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(4), makeCommit(3)}, " "),
		strings.Join([]string{makeCommit(3), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	t1 := time.Now().Add(-time.Minute * 90) // > 1 hr
	t2 := time.Now().Add(-time.Minute * 30) // < 1 hr

	refDescriptions := map[string][]gitdomain.RefDescription{
		// stale
		makeCommit(2): {{Name: "v1", Type: gitdomain.RefTypeTag, CreatedDate: &t1}},
		makeCommit(9): {{Name: "feat1", Type: gitdomain.RefTypeBranch, CreatedDate: &t1}},

		// fresh
		makeCommit(4):  {{Name: "v2", Type: gitdomain.RefTypeTag, CreatedDate: &t2}},
		makeCommit(5):  {{Name: "v3", Type: gitdomain.RefTypeTag, CreatedDate: &t2}},
		makeCommit(7):  {{Name: "main", Type: gitdomain.RefTypeBranch, IsDefaultBranch: true, CreatedDate: &t2}},
		makeCommit(12): {{Name: "feat2", Type: gitdomain.RefTypeBranch, CreatedDate: &t2}},
	}

	if err := store.UpdateUploadsVisibleToCommits(context.Background(), 50, graph, refDescriptions, time.Second, time.Second, 0, time.Now()); err != nil {
		t.Fatalf("unexpected error while calculating visible uploads: %s", err)
	}

	expectedVisibleUploads := map[string][]int{
		makeCommit(1):  {1},
		makeCommit(2):  {1},
		makeCommit(3):  {2},
		makeCommit(4):  {2},
		makeCommit(5):  {2},
		makeCommit(6):  {3},
		makeCommit(7):  {3},
		makeCommit(8):  {4},
		makeCommit(9):  {4},
		makeCommit(10): {2},
		makeCommit(11): {5},
		makeCommit(12): {5},
	}
	if diff := cmp.Diff(expectedVisibleUploads, getVisibleUploads(t, db, 50, keysOf(expectedVisibleUploads))); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	// Ensure data can be queried in reverse direction as well
	assertCommitsVisibleFromUploads(t, store, uploads, expectedVisibleUploads)

	if diff := cmp.Diff([]int{3}, getUploadsVisibleAtTip(t, db, 50)); diff != "" {
		t.Errorf("unexpected uploads visible at tip (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]int{2, 3, 5}, getProtectedUploads(t, db, 50)); diff != "" {
		t.Errorf("unexpected protected uploads (-want +got):\n%s", diff)
	}
}

func TestUpdateUploadsVisibleToCommits(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	// [1] --+--- 2 --------+--5 -- 6 --+-- [7]
	//       |              |           |
	//       +-- [3] -- 4 --+           +--- 8

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(1)},
		{ID: 2, Commit: makeCommit(3)},
		{ID: 3, Commit: makeCommit(7)},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(8), makeCommit(6)}, " "),
		strings.Join([]string{makeCommit(7), makeCommit(6)}, " "),
		strings.Join([]string{makeCommit(6), makeCommit(5)}, " "),
		strings.Join([]string{makeCommit(5), makeCommit(2), makeCommit(4)}, " "),
		strings.Join([]string{makeCommit(4), makeCommit(3)}, " "),
		strings.Join([]string{makeCommit(3), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	refDescriptions := map[string][]gitdomain.RefDescription{
		makeCommit(8): {{IsDefaultBranch: true}},
	}

	if err := store.UpdateUploadsVisibleToCommits(context.Background(), 50, graph, refDescriptions, time.Hour, time.Hour, 0, time.Now()); err != nil {
		t.Fatalf("unexpected error while calculating visible uploads: %s", err)
	}

	expectedVisibleUploads := map[string][]int{
		makeCommit(1): {1},
		makeCommit(2): {1},
		makeCommit(3): {2},
		makeCommit(4): {2},
		makeCommit(5): {1},
		makeCommit(6): {1},
		makeCommit(7): {3},
		makeCommit(8): {1},
	}
	if diff := cmp.Diff(expectedVisibleUploads, getVisibleUploads(t, db, 50, keysOf(expectedVisibleUploads))); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	// Ensure data can be queried in reverse direction as well
	assertCommitsVisibleFromUploads(t, store, uploads, expectedVisibleUploads)

	if diff := cmp.Diff([]int{1}, getUploadsVisibleAtTip(t, db, 50)); diff != "" {
		t.Errorf("unexpected uploads visible at tip (-want +got):\n%s", diff)
	}
}

func TestUpdateUploadsVisibleToCommitsAlternateCommitGraph(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	// 1 --+-- [2] ---- 3
	//     |
	//     +--- 4 --+-- 5 -- 6
	//              |
	//              +-- 7 -- 8

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(2)},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(8), makeCommit(7)}, " "),
		strings.Join([]string{makeCommit(7), makeCommit(4)}, " "),
		strings.Join([]string{makeCommit(6), makeCommit(5)}, " "),
		strings.Join([]string{makeCommit(5), makeCommit(4)}, " "),
		strings.Join([]string{makeCommit(4), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(3), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	refDescriptions := map[string][]gitdomain.RefDescription{
		makeCommit(3): {{IsDefaultBranch: true}},
	}

	if err := store.UpdateUploadsVisibleToCommits(context.Background(), 50, graph, refDescriptions, time.Hour, time.Hour, 0, time.Now()); err != nil {
		t.Fatalf("unexpected error while calculating visible uploads: %s", err)
	}

	expectedVisibleUploads := map[string][]int{
		makeCommit(2): {1},
		makeCommit(3): {1},
	}
	if diff := cmp.Diff(expectedVisibleUploads, getVisibleUploads(t, db, 50, keysOf(expectedVisibleUploads))); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	// Ensure data can be queried in reverse direction as well
	assertCommitsVisibleFromUploads(t, store, uploads, expectedVisibleUploads)

	if diff := cmp.Diff([]int{1}, getUploadsVisibleAtTip(t, db, 50)); diff != "" {
		t.Errorf("unexpected uploads visible at tip (-want +got):\n%s", diff)
	}
}

func TestUpdateUploadsVisibleToCommitsDistinctRoots(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	// 1 -- [2]

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(2), Root: "root1/"},
		{ID: 2, Commit: makeCommit(2), Root: "root2/"},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	refDescriptions := map[string][]gitdomain.RefDescription{
		makeCommit(2): {{IsDefaultBranch: true}},
	}

	if err := store.UpdateUploadsVisibleToCommits(context.Background(), 50, graph, refDescriptions, time.Hour, time.Hour, 0, time.Now()); err != nil {
		t.Fatalf("unexpected error while calculating visible uploads: %s", err)
	}

	expectedVisibleUploads := map[string][]int{
		makeCommit(2): {1, 2},
	}
	if diff := cmp.Diff(expectedVisibleUploads, getVisibleUploads(t, db, 50, keysOf(expectedVisibleUploads))); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	// Ensure data can be queried in reverse direction as well
	assertCommitsVisibleFromUploads(t, store, uploads, expectedVisibleUploads)

	if diff := cmp.Diff([]int{1, 2}, getUploadsVisibleAtTip(t, db, 50)); diff != "" {
		t.Errorf("unexpected uploads visible at tip (-want +got):\n%s", diff)
	}
}

func TestUpdateUploadsVisibleToCommitsOverlappingRoots(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	// 1 -- 2 --+-- 3 --+-- 5 -- 6
	//          |       |
	//          +-- 4 --+
	//
	// With the following LSIF uploads:
	//
	// | UploadID | Commit | Root    | Indexer |
	// | -------- + ------ + ------- + ------- |
	// | 1        | 1      | root3/  | lsif-go |
	// | 2        | 1      | root4/  | scip-python |
	// | 3        | 2      | root1/  | lsif-go |
	// | 4        | 2      | root2/  | lsif-go |
	// | 5        | 2      |         | scip-python | (overwrites root4/ at commit 1)
	// | 6        | 3      | root1/  | lsif-go | (overwrites root1/ at commit 2)
	// | 7        | 4      |         | scip-python | (overwrites (root) at commit 2)
	// | 8        | 5      | root2/  | lsif-go | (overwrites root2/ at commit 2)
	// | 9        | 6      | root1/  | lsif-go | (overwrites root1/ at commit 2)

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(1), Indexer: "lsif-go", Root: "root3/"},
		{ID: 2, Commit: makeCommit(1), Indexer: "scip-python", Root: "root4/"},
		{ID: 3, Commit: makeCommit(2), Indexer: "lsif-go", Root: "root1/"},
		{ID: 4, Commit: makeCommit(2), Indexer: "lsif-go", Root: "root2/"},
		{ID: 5, Commit: makeCommit(2), Indexer: "scip-python", Root: ""},
		{ID: 6, Commit: makeCommit(3), Indexer: "lsif-go", Root: "root1/"},
		{ID: 7, Commit: makeCommit(4), Indexer: "scip-python", Root: ""},
		{ID: 8, Commit: makeCommit(5), Indexer: "lsif-go", Root: "root2/"},
		{ID: 9, Commit: makeCommit(6), Indexer: "lsif-go", Root: "root1/"},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(6), makeCommit(5)}, " "),
		strings.Join([]string{makeCommit(5), makeCommit(3), makeCommit(4)}, " "),
		strings.Join([]string{makeCommit(4), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(3), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	refDescriptions := map[string][]gitdomain.RefDescription{
		makeCommit(6): {{IsDefaultBranch: true}},
	}

	if err := store.UpdateUploadsVisibleToCommits(context.Background(), 50, graph, refDescriptions, time.Hour, time.Hour, 0, time.Now()); err != nil {
		t.Fatalf("unexpected error while calculating visible uploads: %s", err)
	}

	expectedVisibleUploads := map[string][]int{
		makeCommit(1): {1, 2},
		makeCommit(2): {1, 2, 3, 4, 5},
		makeCommit(3): {1, 2, 4, 5, 6},
		makeCommit(4): {1, 2, 3, 4, 7},
		makeCommit(5): {1, 2, 6, 7, 8},
		makeCommit(6): {1, 2, 7, 8, 9},
	}
	if diff := cmp.Diff(expectedVisibleUploads, getVisibleUploads(t, db, 50, keysOf(expectedVisibleUploads))); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	// Ensure data can be queried in reverse direction as well
	assertCommitsVisibleFromUploads(t, store, uploads, expectedVisibleUploads)

	if diff := cmp.Diff([]int{1, 2, 7, 8, 9}, getUploadsVisibleAtTip(t, db, 50)); diff != "" {
		t.Errorf("unexpected uploads visible at tip (-want +got):\n%s", diff)
	}
}

func TestUpdateUploadsVisibleToCommitsIndexerName(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	// [1] -- [2] -- [3] -- [4] -- 5

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(1), Root: "root1/", Indexer: "idx1"},
		{ID: 2, Commit: makeCommit(2), Root: "root2/", Indexer: "idx1"},
		{ID: 3, Commit: makeCommit(3), Root: "root3/", Indexer: "idx1"},
		{ID: 4, Commit: makeCommit(4), Root: "root4/", Indexer: "idx1"},
		{ID: 5, Commit: makeCommit(1), Root: "root1/", Indexer: "idx2"},
		{ID: 6, Commit: makeCommit(2), Root: "root2/", Indexer: "idx2"},
		{ID: 7, Commit: makeCommit(3), Root: "root3/", Indexer: "idx2"},
		{ID: 8, Commit: makeCommit(4), Root: "root4/", Indexer: "idx2"},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(5), makeCommit(4)}, " "),
		strings.Join([]string{makeCommit(4), makeCommit(3)}, " "),
		strings.Join([]string{makeCommit(3), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	refDescriptions := map[string][]gitdomain.RefDescription{
		makeCommit(5): {{IsDefaultBranch: true}},
	}

	if err := store.UpdateUploadsVisibleToCommits(context.Background(), 50, graph, refDescriptions, time.Hour, time.Hour, 0, time.Now()); err != nil {
		t.Fatalf("unexpected error while calculating visible uploads: %s", err)
	}

	expectedVisibleUploads := map[string][]int{
		makeCommit(1): {1, 5},
		makeCommit(2): {1, 2, 5, 6},
		makeCommit(3): {1, 2, 3, 5, 6, 7},
		makeCommit(4): {1, 2, 3, 4, 5, 6, 7, 8},
		makeCommit(5): {1, 2, 3, 4, 5, 6, 7, 8},
	}
	if diff := cmp.Diff(expectedVisibleUploads, getVisibleUploads(t, db, 50, keysOf(expectedVisibleUploads))); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	// Ensure data can be queried in reverse direction as well
	assertCommitsVisibleFromUploads(t, store, uploads, expectedVisibleUploads)

	if diff := cmp.Diff([]int{1, 2, 3, 4, 5, 6, 7, 8}, getUploadsVisibleAtTip(t, db, 50)); diff != "" {
		t.Errorf("unexpected uploads visible at tip (-want +got):\n%s", diff)
	}
}

func TestUpdateUploadsVisibleToCommitsResetsDirtyFlag(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(1)},
		{ID: 2, Commit: makeCommit(2)},
		{ID: 3, Commit: makeCommit(3)},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(3), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	refDescriptions := map[string][]gitdomain.RefDescription{
		makeCommit(3): {{IsDefaultBranch: true}},
	}

	for range 3 {
		// Set dirty token to 3
		if err := store.SetRepositoryAsDirty(context.Background(), 50); err != nil {
			t.Fatalf("unexpected error marking repository as dirty: %s", err)
		}
	}

	now := time.Unix(1587396557, 0).UTC()

	// Non-latest dirty token - should not clear flag
	if err := store.UpdateUploadsVisibleToCommits(context.Background(), 50, graph, refDescriptions, time.Hour, time.Hour, 2, now); err != nil {
		t.Fatalf("unexpected error while calculating visible uploads: %s", err)
	}
	dirtyRepositories, err := store.GetDirtyRepositories(context.Background())
	if err != nil {
		t.Fatalf("unexpected error listing dirty repositories: %s", err)
	}
	if len(dirtyRepositories) == 0 {
		t.Errorf("did not expect repository to be unmarked")
	}

	// Latest dirty token - should clear flag
	if err := store.UpdateUploadsVisibleToCommits(context.Background(), 50, graph, refDescriptions, time.Hour, time.Hour, 3, now); err != nil {
		t.Fatalf("unexpected error while calculating visible uploads: %s", err)
	}
	dirtyRepositories, err = store.GetDirtyRepositories(context.Background())
	if err != nil {
		t.Fatalf("unexpected error listing dirty repositories: %s", err)
	}
	if len(dirtyRepositories) != 0 {
		t.Errorf("expected repository to be unmarked")
	}

	stale, updatedAt, err := store.GetCommitGraphMetadata(context.Background(), 50)
	if err != nil {
		t.Fatalf("unexpected error getting commit graph metadata: %s", err)
	}
	if stale {
		t.Errorf("unexpected value for stale. want=%v have=%v", false, stale)
	}
	if diff := cmp.Diff(&now, updatedAt); diff != "" {
		t.Errorf("unexpected value for uploadedAt (-want +got):\n%s", diff)
	}
}

func TestFindClosestCompletedUploads(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	// [1] --+--- 2 --------+--5 -- 6 --+-- [7]
	//       |              |           |
	//       +-- [3] -- 4 --+           +--- 8

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(1)},
		{ID: 2, Commit: makeCommit(3)},
		{ID: 3, Commit: makeCommit(7)},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(8), makeCommit(6)}, " "),
		strings.Join([]string{makeCommit(7), makeCommit(6)}, " "),
		strings.Join([]string{makeCommit(6), makeCommit(5)}, " "),
		strings.Join([]string{makeCommit(5), makeCommit(2), makeCommit(4)}, " "),
		strings.Join([]string{makeCommit(4), makeCommit(3)}, " "),
		strings.Join([]string{makeCommit(3), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	visibleUploads, links := commitgraph.NewGraph(graph, toCommitGraphView(uploads)).Gather()

	expectedVisibleUploads := map[string][]commitgraph.UploadMeta{
		makeCommit(1): {{UploadID: 1, Distance: 0}},
		makeCommit(2): {{UploadID: 1, Distance: 1}},
		makeCommit(3): {{UploadID: 2, Distance: 0}},
		makeCommit(4): {{UploadID: 2, Distance: 1}},
		makeCommit(5): {{UploadID: 1, Distance: 2}},
		makeCommit(6): {{UploadID: 1, Distance: 3}},
		makeCommit(7): {{UploadID: 3, Distance: 0}},
		makeCommit(8): {{UploadID: 1, Distance: 4}},
	}
	if diff := cmp.Diff(expectedVisibleUploads, normalizeVisibleUploads(visibleUploads)); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}
	expectedLinks := map[string]commitgraph.LinkRelationship{}
	if diff := cmp.Diff(expectedLinks, links); diff != "" {
		t.Errorf("unexpected visible links (-want +got):\n%s", diff)
	}

	// Prep
	insertNearestUploads(t, db, 50, visibleUploads)
	insertLinks(t, db, 50, links)

	// Test
	testFindClosestCompletedUploads(t, store, []FindClosestCompletedUploadsTestCase{
		{commit: makeCommit(1), file: "file.ts", rootMustEnclosePath: true, graph: graph, anyOfIDs: []int{1}},
		{commit: makeCommit(2), file: "file.ts", rootMustEnclosePath: true, graph: graph, anyOfIDs: []int{1}},
		{commit: makeCommit(3), file: "file.ts", rootMustEnclosePath: true, graph: graph, anyOfIDs: []int{2}},
		{commit: makeCommit(4), file: "file.ts", rootMustEnclosePath: true, graph: graph, anyOfIDs: []int{2}},
		{commit: makeCommit(6), file: "file.ts", rootMustEnclosePath: true, graph: graph, anyOfIDs: []int{1}},
		{commit: makeCommit(7), file: "file.ts", rootMustEnclosePath: true, graph: graph, anyOfIDs: []int{3}},
		{commit: makeCommit(5), file: "file.ts", rootMustEnclosePath: true, graph: graph, anyOfIDs: []int{1, 2, 3}},
		{commit: makeCommit(8), file: "file.ts", rootMustEnclosePath: true, graph: graph, anyOfIDs: []int{1, 2}},
	})
}

func TestFindClosestCompletedUploadsAlternateCommitGraph(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	// 1 --+-- [2] ---- 3
	//     |
	//     +--- 4 --+-- 5 -- 6
	//              |
	//              +-- 7 -- 8

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(2)},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(8), makeCommit(7)}, " "),
		strings.Join([]string{makeCommit(7), makeCommit(4)}, " "),
		strings.Join([]string{makeCommit(6), makeCommit(5)}, " "),
		strings.Join([]string{makeCommit(5), makeCommit(4)}, " "),
		strings.Join([]string{makeCommit(4), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(3), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	visibleUploads, links := commitgraph.NewGraph(graph, toCommitGraphView(uploads)).Gather()

	expectedVisibleUploads := map[string][]commitgraph.UploadMeta{
		makeCommit(2): {{UploadID: 1, Distance: 0}},
		makeCommit(3): {{UploadID: 1, Distance: 1}},
	}
	if diff := cmp.Diff(expectedVisibleUploads, normalizeVisibleUploads(visibleUploads)); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	expectedLinks := map[string]commitgraph.LinkRelationship{}
	if diff := cmp.Diff(expectedLinks, links); diff != "" {
		t.Errorf("unexpected visible links (-want +got):\n%s", diff)
	}

	// Prep
	insertNearestUploads(t, db, 50, visibleUploads)
	insertLinks(t, db, 50, links)

	// Test
	testFindClosestCompletedUploads(t, store, []FindClosestCompletedUploadsTestCase{
		{commit: makeCommit(2), graph: graph, allOfIDs: []int{1}},
		{commit: makeCommit(3), graph: graph, allOfIDs: []int{1}},
		{commit: makeCommit(4), graph: graph},
		{commit: makeCommit(6), graph: graph},
		{commit: makeCommit(7), graph: graph},
		{commit: makeCommit(5), graph: graph},
		{commit: makeCommit(8), graph: graph},
	})
}

func TestFindClosestCompletedUploadsAlternateCommitGraphWithOverwrittenVisibleUploads(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	// 1 -- [2] -- 3 -- 4 -- [5]

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(2)},
		{ID: 2, Commit: makeCommit(5)},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(5), makeCommit(4)}, " "),
		strings.Join([]string{makeCommit(4), makeCommit(3)}, " "),
		strings.Join([]string{makeCommit(3), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	visibleUploads, links := commitgraph.NewGraph(graph, toCommitGraphView(uploads)).Gather()

	expectedVisibleUploads := map[string][]commitgraph.UploadMeta{
		makeCommit(2): {{UploadID: 1, Distance: 0}},
		makeCommit(3): {{UploadID: 1, Distance: 1}},
		makeCommit(4): {{UploadID: 1, Distance: 2}},
		makeCommit(5): {{UploadID: 2, Distance: 0}},
	}
	if diff := cmp.Diff(expectedVisibleUploads, normalizeVisibleUploads(visibleUploads)); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	expectedLinks := map[string]commitgraph.LinkRelationship{}
	if diff := cmp.Diff(expectedLinks, links); diff != "" {
		t.Errorf("unexpected visible links (-want +got):\n%s", diff)
	}

	// Prep
	insertNearestUploads(t, db, 50, visibleUploads)
	insertLinks(t, db, 50, links)

	// Test
	testFindClosestCompletedUploads(t, store, []FindClosestCompletedUploadsTestCase{
		{commit: makeCommit(2), graph: graph, allOfIDs: []int{1}},
		{commit: makeCommit(3), graph: graph, allOfIDs: []int{1}},
		{commit: makeCommit(4), graph: graph, allOfIDs: []int{1}},
		{commit: makeCommit(5), graph: graph, allOfIDs: []int{2}},
	})
}

func TestFindClosestCompletedUploadsDistinctRoots(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	// [1] -- 2

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(1), Root: "root1/"},
		{ID: 2, Commit: makeCommit(1), Root: "root2/"},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	visibleUploads, links := commitgraph.NewGraph(graph, toCommitGraphView(uploads)).Gather()

	expectedVisibleUploads := map[string][]commitgraph.UploadMeta{
		makeCommit(1): {{UploadID: 1, Distance: 0}, {UploadID: 2, Distance: 0}},
	}
	if diff := cmp.Diff(expectedVisibleUploads, normalizeVisibleUploads(visibleUploads)); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	expectedLinks := map[string]commitgraph.LinkRelationship{
		makeCommit(2): {Commit: makeCommit(2), AncestorCommit: makeCommit(1), Distance: 1},
	}
	if diff := cmp.Diff(expectedLinks, links); diff != "" {
		t.Errorf("unexpected visible links (-want +got):\n%s", diff)
	}

	// Prep
	insertNearestUploads(t, db, 50, visibleUploads)
	insertLinks(t, db, 50, links)

	// Test
	testFindClosestCompletedUploads(t, store, []FindClosestCompletedUploadsTestCase{
		{commit: makeCommit(1), file: "blah", rootMustEnclosePath: true, graph: graph},
		{commit: makeCommit(2), file: "root1/file.ts", rootMustEnclosePath: true, graph: graph, allOfIDs: []int{1}},
		{commit: makeCommit(1), file: "root2/file.ts", rootMustEnclosePath: true, graph: graph, allOfIDs: []int{2}},
		{commit: makeCommit(2), file: "root2/file.ts", rootMustEnclosePath: true, graph: graph, allOfIDs: []int{2}},
		{commit: makeCommit(1), file: "root3/file.ts", rootMustEnclosePath: true, graph: graph},
	})
}

func TestFindClosestCompletedUploadsOverlappingRoots(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	// 1 -- 2 --+-- 3 --+-- 5 -- 6
	//          |       |
	//          +-- 4 --+
	//
	// With the following LSIF uploads:
	//
	// | UploadID | Commit | Root    | Indexer |
	// | -------- + ------ + ------- + ------- |
	// | 1        | 1      | root3/  | lsif-go |
	// | 2        | 1      | root4/  | scip-python |
	// | 3        | 2      | root1/  | lsif-go |
	// | 4        | 2      | root2/  | lsif-go |
	// | 5        | 2      |         | scip-python | (overwrites root4/ at commit 1)
	// | 6        | 3      | root1/  | lsif-go | (overwrites root1/ at commit 2)
	// | 7        | 4      |         | scip-python | (overwrites (root) at commit 2)
	// | 8        | 5      | root2/  | lsif-go | (overwrites root2/ at commit 2)
	// | 9        | 6      | root1/  | lsif-go | (overwrites root1/ at commit 2)

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(1), Indexer: "lsif-go", Root: "root3/"},
		{ID: 2, Commit: makeCommit(1), Indexer: "scip-python", Root: "root4/"},
		{ID: 3, Commit: makeCommit(2), Indexer: "lsif-go", Root: "root1/"},
		{ID: 4, Commit: makeCommit(2), Indexer: "lsif-go", Root: "root2/"},
		{ID: 5, Commit: makeCommit(2), Indexer: "scip-python", Root: ""},
		{ID: 6, Commit: makeCommit(3), Indexer: "lsif-go", Root: "root1/"},
		{ID: 7, Commit: makeCommit(4), Indexer: "scip-python", Root: ""},
		{ID: 8, Commit: makeCommit(5), Indexer: "lsif-go", Root: "root2/"},
		{ID: 9, Commit: makeCommit(6), Indexer: "lsif-go", Root: "root1/"},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(6), makeCommit(5)}, " "),
		strings.Join([]string{makeCommit(5), makeCommit(3), makeCommit(4)}, " "),
		strings.Join([]string{makeCommit(4), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(3), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	visibleUploads, links := commitgraph.NewGraph(graph, toCommitGraphView(uploads)).Gather()

	expectedVisibleUploads := map[string][]commitgraph.UploadMeta{
		makeCommit(1): {{UploadID: 1, Distance: 0}, {UploadID: 2, Distance: 0}},
		makeCommit(2): {{UploadID: 1, Distance: 1}, {UploadID: 2, Distance: 1}, {UploadID: 3, Distance: 0}, {UploadID: 4, Distance: 0}, {UploadID: 5, Distance: 0}},
		makeCommit(3): {{UploadID: 1, Distance: 2}, {UploadID: 2, Distance: 2}, {UploadID: 4, Distance: 1}, {UploadID: 5, Distance: 1}, {UploadID: 6, Distance: 0}},
		makeCommit(4): {{UploadID: 1, Distance: 2}, {UploadID: 2, Distance: 2}, {UploadID: 3, Distance: 1}, {UploadID: 4, Distance: 1}, {UploadID: 7, Distance: 0}},
		makeCommit(5): {{UploadID: 1, Distance: 3}, {UploadID: 2, Distance: 3}, {UploadID: 6, Distance: 1}, {UploadID: 7, Distance: 1}, {UploadID: 8, Distance: 0}},
		makeCommit(6): {{UploadID: 1, Distance: 4}, {UploadID: 2, Distance: 4}, {UploadID: 7, Distance: 2}, {UploadID: 8, Distance: 1}, {UploadID: 9, Distance: 0}},
	}
	if diff := cmp.Diff(expectedVisibleUploads, normalizeVisibleUploads(visibleUploads)); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	expectedLinks := map[string]commitgraph.LinkRelationship{}
	if diff := cmp.Diff(expectedLinks, links); diff != "" {
		t.Errorf("unexpected visible links (-want +got):\n%s", diff)
	}

	// Prep
	insertNearestUploads(t, db, 50, visibleUploads)
	insertLinks(t, db, 50, links)

	// Test
	testFindClosestCompletedUploads(t, store, []FindClosestCompletedUploadsTestCase{
		{commit: makeCommit(4), file: "root1/file.ts", rootMustEnclosePath: true, graph: graph, allOfIDs: []int{7, 3}},
		{commit: makeCommit(5), file: "root2/file.ts", rootMustEnclosePath: true, graph: graph, allOfIDs: []int{8, 7}},
		{commit: makeCommit(3), file: "root3/file.ts", rootMustEnclosePath: true, graph: graph, allOfIDs: []int{5, 1}},
		{commit: makeCommit(1), file: "root4/file.ts", rootMustEnclosePath: true, graph: graph, allOfIDs: []int{2}},
		{commit: makeCommit(2), file: "root4/file.ts", rootMustEnclosePath: true, graph: graph, allOfIDs: []int{2, 5}},
	})
}

func TestFindClosestCompletedUploadsIndexerName(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	// [1] --+-- [2] --+-- [3] --+-- [4] --+-- 5

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(1), Root: "root1/", Indexer: "idx1"},
		{ID: 2, Commit: makeCommit(2), Root: "root2/", Indexer: "idx1"},
		{ID: 3, Commit: makeCommit(3), Root: "root3/", Indexer: "idx1"},
		{ID: 4, Commit: makeCommit(4), Root: "root4/", Indexer: "idx1"},
		{ID: 5, Commit: makeCommit(1), Root: "root1/", Indexer: "idx2"},
		{ID: 6, Commit: makeCommit(2), Root: "root2/", Indexer: "idx2"},
		{ID: 7, Commit: makeCommit(3), Root: "root3/", Indexer: "idx2"},
		{ID: 8, Commit: makeCommit(4), Root: "root4/", Indexer: "idx2"},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(5), makeCommit(4)}, " "),
		strings.Join([]string{makeCommit(4), makeCommit(3)}, " "),
		strings.Join([]string{makeCommit(3), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	visibleUploads, links := commitgraph.NewGraph(graph, toCommitGraphView(uploads)).Gather()

	expectedVisibleUploads := map[string][]commitgraph.UploadMeta{
		makeCommit(1): {
			{UploadID: 1, Distance: 0},
			{UploadID: 5, Distance: 0},
		},
		makeCommit(2): {
			{UploadID: 1, Distance: 1},
			{UploadID: 2, Distance: 0},
			{UploadID: 5, Distance: 1},
			{UploadID: 6, Distance: 0},
		},
		makeCommit(3): {
			{UploadID: 1, Distance: 2},
			{UploadID: 2, Distance: 1},
			{UploadID: 3, Distance: 0},
			{UploadID: 5, Distance: 2},
			{UploadID: 6, Distance: 1},
			{UploadID: 7, Distance: 0},
		},
		makeCommit(4): {
			{UploadID: 1, Distance: 3},
			{UploadID: 2, Distance: 2},
			{UploadID: 3, Distance: 1},
			{UploadID: 4, Distance: 0},
			{UploadID: 5, Distance: 3},
			{UploadID: 6, Distance: 2},
			{UploadID: 7, Distance: 1},
			{UploadID: 8, Distance: 0},
		},
	}
	if diff := cmp.Diff(expectedVisibleUploads, normalizeVisibleUploads(visibleUploads)); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	expectedLinks := map[string]commitgraph.LinkRelationship{
		makeCommit(5): {Commit: makeCommit(5), AncestorCommit: makeCommit(4), Distance: 1},
	}
	if diff := cmp.Diff(expectedLinks, links); diff != "" {
		t.Errorf("unexpected visible links (-want +got):\n%s", diff)
	}

	// Prep
	insertNearestUploads(t, db, 50, visibleUploads)
	insertLinks(t, db, 50, links)

	// Test
	testFindClosestCompletedUploads(t, store, []FindClosestCompletedUploadsTestCase{
		{commit: makeCommit(5), file: "root1/file.ts", indexer: "idx1", graph: graph, allOfIDs: []int{1}},
		{commit: makeCommit(5), file: "root2/file.ts", indexer: "idx1", graph: graph, allOfIDs: []int{2}},
		{commit: makeCommit(5), file: "root3/file.ts", indexer: "idx1", graph: graph, allOfIDs: []int{3}},
		{commit: makeCommit(5), file: "root4/file.ts", indexer: "idx1", graph: graph, allOfIDs: []int{4}},
		{commit: makeCommit(5), file: "root1/file.ts", indexer: "idx2", graph: graph, allOfIDs: []int{5}},
		{commit: makeCommit(5), file: "root2/file.ts", indexer: "idx2", graph: graph, allOfIDs: []int{6}},
		{commit: makeCommit(5), file: "root3/file.ts", indexer: "idx2", graph: graph, allOfIDs: []int{7}},
		{commit: makeCommit(5), file: "root4/file.ts", indexer: "idx2", graph: graph, allOfIDs: []int{8}},
	})
}

func TestFindClosestCompletedUploadsIntersectingPath(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	// [1]

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(1), Root: "web/src/", Indexer: "lsif-eslint"},
	}
	insertUploads(t, db, uploads...)

	graph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(1)}, " "),
	})

	visibleUploads, links := commitgraph.NewGraph(graph, toCommitGraphView(uploads)).Gather()

	expectedVisibleUploads := map[string][]commitgraph.UploadMeta{
		makeCommit(1): {{UploadID: 1}},
	}
	if diff := cmp.Diff(expectedVisibleUploads, normalizeVisibleUploads(visibleUploads)); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	expectedLinks := map[string]commitgraph.LinkRelationship{}
	if diff := cmp.Diff(expectedLinks, links); diff != "" {
		t.Errorf("unexpected visible links (-want +got):\n%s", diff)
	}

	// Prep
	insertNearestUploads(t, db, 50, visibleUploads)
	insertLinks(t, db, 50, links)

	// Test
	testFindClosestCompletedUploads(t, store, []FindClosestCompletedUploadsTestCase{
		{commit: makeCommit(1), file: "", rootMustEnclosePath: false, graph: graph, allOfIDs: []int{1}},
		{commit: makeCommit(1), file: "web/", rootMustEnclosePath: false, graph: graph, allOfIDs: []int{1}},
		{commit: makeCommit(1), file: "web/src/file.ts", rootMustEnclosePath: false, graph: graph, allOfIDs: []int{1}},
	})
}

func TestFindClosestCompletedUploadsFromGraphFragment(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// This database has the following commit graph:
	//
	//       <- known commits || new commits ->
	//                        ||
	// [1] --+--- 2 --- 3 --  || -- 4 --+-- 7
	//       |                ||       /
	//       +-- [5] -- 6 --- || -----+

	uploads := []shared.Upload{
		{ID: 1, Commit: makeCommit(1)},
		{ID: 2, Commit: makeCommit(5)},
	}
	insertUploads(t, db, uploads...)

	currentGraph := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(6), makeCommit(5)}, " "),
		strings.Join([]string{makeCommit(5), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(3), makeCommit(2)}, " "),
		strings.Join([]string{makeCommit(2), makeCommit(1)}, " "),
		strings.Join([]string{makeCommit(1)}, " "),
	})

	visibleUploads, links := commitgraph.NewGraph(currentGraph, toCommitGraphView(uploads)).Gather()

	expectedVisibleUploads := map[string][]commitgraph.UploadMeta{
		makeCommit(1): {{UploadID: 1, Distance: 0}},
		makeCommit(2): {{UploadID: 1, Distance: 1}},
		makeCommit(3): {{UploadID: 1, Distance: 2}},
		makeCommit(5): {{UploadID: 2, Distance: 0}},
		makeCommit(6): {{UploadID: 2, Distance: 1}},
	}
	if diff := cmp.Diff(expectedVisibleUploads, normalizeVisibleUploads(visibleUploads)); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	expectedLinks := map[string]commitgraph.LinkRelationship{}
	if diff := cmp.Diff(expectedLinks, links); diff != "" {
		t.Errorf("unexpected visible links (-want +got):\n%s", diff)
	}

	// Prep
	insertNearestUploads(t, db, 50, visibleUploads)
	insertLinks(t, db, 50, links)

	// Test
	graphFragment := gitdomain.ParseCommitGraph([]string{
		strings.Join([]string{makeCommit(7), makeCommit(4), makeCommit(6)}, " "),
		strings.Join([]string{makeCommit(4), makeCommit(3)}, " "),
		strings.Join([]string{makeCommit(6)}, " "),
		strings.Join([]string{makeCommit(3)}, " "),
	})

	testFindClosestCompletedUploads(t, store, []FindClosestCompletedUploadsTestCase{
		// Note: Can't query anything outside of the graph fragment
		{commit: makeCommit(3), file: "file.ts", rootMustEnclosePath: true, graph: graphFragment, anyOfIDs: []int{1}},
		{commit: makeCommit(6), file: "file.ts", rootMustEnclosePath: true, graph: graphFragment, anyOfIDs: []int{2}},
		{commit: makeCommit(4), file: "file.ts", rootMustEnclosePath: true, graph: graphFragment, graphFragmentOnly: true, anyOfIDs: []int{1}},
		{commit: makeCommit(7), file: "file.ts", rootMustEnclosePath: true, graph: graphFragment, graphFragmentOnly: true, anyOfIDs: []int{2}},
	})
}

func TestGetRepositoriesMaxStaleAge(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	for _, id := range []int{50, 51, 52} {
		insertRepo(t, db, id, "", false)
	}

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO lsif_dirty_repositories (
			repository_id,
			update_token,
			dirty_token,
			set_dirty_at
		)
		VALUES
			(50, 10, 10, NOW() - '45 minutes'::interval), -- not dirty
			(51, 20, 25, NOW() - '30 minutes'::interval), -- dirty
			(52, 30, 35, NOW() - '20 minutes'::interval), -- dirty
			(53, 40, 45, NOW() - '30 minutes'::interval); -- no associated repo
	`); err != nil {
		t.Fatalf("unexpected error marking repostiory as dirty: %s", err)
	}

	age, err := store.GetRepositoriesMaxStaleAge(context.Background())
	if err != nil {
		t.Fatalf("unexpected error listing dirty repositories: %s", err)
	}
	if age.Round(time.Second) != 30*time.Minute {
		t.Fatalf("unexpected max age. want=%s have=%s", 30*time.Minute, age)
	}
}

func TestCommitGraphMetadata(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	if err := store.SetRepositoryAsDirty(context.Background(), 50); err != nil {
		t.Errorf("unexpected error marking repository as dirty: %s", err)
	}

	updatedAt := time.Unix(1587396557, 0).UTC()
	query := sqlf.Sprintf("INSERT INTO lsif_dirty_repositories VALUES (%s, %s, %s, %s)", 51, 10, 10, updatedAt)
	if _, err := db.ExecContext(context.Background(), query.Query(sqlf.PostgresBindVar), query.Args()...); err != nil {
		t.Fatalf("unexpected error inserting commit graph metadata: %s", err)
	}

	testCases := []struct {
		RepositoryID int
		Stale        bool
		UpdatedAt    *time.Time
	}{
		{50, true, nil},
		{51, false, &updatedAt},
		{52, false, nil},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("repositoryID=%d", testCase.RepositoryID), func(t *testing.T) {
			stale, updatedAt, err := store.GetCommitGraphMetadata(context.Background(), testCase.RepositoryID)
			if err != nil {
				t.Fatalf("unexpected error getting commit graph metadata: %s", err)
			}

			if stale != testCase.Stale {
				t.Errorf("unexpected value for stale. want=%v have=%v", testCase.Stale, stale)
			}

			if diff := cmp.Diff(testCase.UpdatedAt, updatedAt); diff != "" {
				t.Errorf("unexpected value for uploadedAt (-want +got):\n%s", diff)
			}
		})
	}
}

//
//
//

type FindClosestCompletedUploadsTestCase struct {
	commit              string
	file                string
	rootMustEnclosePath bool
	indexer             string
	graph               *gitdomain.CommitGraph
	graphFragmentOnly   bool
	anyOfIDs            []int
	allOfIDs            []int
}

func testFindClosestCompletedUploads(t *testing.T, store Store, testCases []FindClosestCompletedUploadsTestCase) {
	for _, testCase := range testCases {
		name := fmt.Sprintf(
			"commit=%s file=%s rootMustEnclosePath=%v indexer=%s",
			testCase.commit,
			testCase.file,
			testCase.rootMustEnclosePath,
			testCase.indexer,
		)

		assertUploadIDs := func(t *testing.T, uploads []shared.CompletedUpload) {
			if len(testCase.anyOfIDs) > 0 {
				testAnyOf(t, uploads, testCase.anyOfIDs)
				return
			}

			if len(testCase.allOfIDs) > 0 {
				testAllOf(t, uploads, testCase.allOfIDs)
				return
			}

			if len(uploads) != 0 {
				t.Errorf("unexpected nearest upload length. want=%d have=%d", 0, len(uploads))
				return
			}
		}

		if !testCase.graphFragmentOnly {
			t.Run(name, func(t *testing.T) {
				uploads, err := store.FindClosestCompletedUploads(context.Background(), 50, testCase.commit, testCase.file, testCase.rootMustEnclosePath, testCase.indexer)
				if err != nil {
					t.Fatalf("unexpected error finding closest uploads: %s", err)
				}

				assertUploadIDs(t, uploads)
			})
		}

		if testCase.graph != nil {
			t.Run(name+" [graph-fragment]", func(t *testing.T) {
				uploads, err := store.FindClosestCompletedUploadsFromGraphFragment(context.Background(), 50, testCase.commit, testCase.file, testCase.rootMustEnclosePath, testCase.indexer, testCase.graph)
				if err != nil {
					t.Fatalf("unexpected error finding closest uploads: %s", err)
				}

				assertUploadIDs(t, uploads)
			})
		}
	}
}

func testAnyOf(t *testing.T, uploads []shared.CompletedUpload, expectedIDs []int) {
	if len(uploads) != 1 {
		t.Errorf("unexpected nearest upload length. want=%d have=%d", 1, len(uploads))
		return
	}

	if !testPresence(uploads[0].ID, expectedIDs) {
		t.Errorf("unexpected nearest dump ids. want one of %v have=%v", expectedIDs, uploads[0].ID)
	}
}

func testPresence(needle int, haystack []int) bool {
	for _, candidate := range haystack {
		if needle == candidate {
			return true
		}
	}

	return false
}

func testAllOf(t *testing.T, uploads []shared.CompletedUpload, expectedIDs []int) {
	if len(uploads) != len(expectedIDs) {
		t.Errorf("unexpected nearest upload length. want=%d have=%d", 1, len(uploads))
	}

	var uploadIDs []int
	for _, upload := range uploads {
		uploadIDs = append(uploadIDs, upload.ID)
	}

	for _, expectedID := range expectedIDs {
		if !testPresence(expectedID, uploadIDs) {
			t.Errorf("unexpected nearest dump ids. want all of %v have=%v", expectedIDs, uploadIDs)
			return
		}
	}
}

//
//
//

// Marks a repo as deleted
func deleteRepo(t testing.TB, db database.DB, id int, deleted_at time.Time) {
	query := sqlf.Sprintf(
		`UPDATE repo SET deleted_at = %s WHERE id = %s`,
		deleted_at,
		id,
	)
	if _, err := db.ExecContext(context.Background(), query.Query(sqlf.PostgresBindVar), query.Args()...); err != nil {
		t.Fatalf("unexpected error while deleting repository: %s", err)
	}
}

func toCommitGraphView(uploads []shared.Upload) *commitgraph.CommitGraphView {
	commitGraphView := commitgraph.NewCommitGraphView()
	for _, upload := range uploads {
		commitGraphView.Add(commitgraph.UploadMeta{UploadID: upload.ID}, upload.Commit, fmt.Sprintf("%s:%s", upload.Root, upload.Indexer))
	}

	return commitGraphView
}

func normalizeVisibleUploads(uploadMetas map[string][]commitgraph.UploadMeta) map[string][]commitgraph.UploadMeta {
	for _, uploads := range uploadMetas {
		sort.Slice(uploads, func(i, j int) bool {
			return uploads[i].UploadID-uploads[j].UploadID < 0
		})
	}

	return uploadMetas
}

func insertLinks(t testing.TB, db database.DB, repositoryID int, links map[string]commitgraph.LinkRelationship) {
	if len(links) == 0 {
		return
	}

	var rows []*sqlf.Query
	for commit, link := range links {
		rows = append(rows, sqlf.Sprintf(
			"(%s, %s, %s, %s)",
			repositoryID,
			dbutil.CommitBytea(commit),
			dbutil.CommitBytea(link.AncestorCommit),
			link.Distance,
		))
	}

	query := sqlf.Sprintf(
		`INSERT INTO lsif_nearest_uploads_links (repository_id, commit_bytea, ancestor_commit_bytea, distance) VALUES %s`,
		sqlf.Join(rows, ","),
	)
	if _, err := db.ExecContext(context.Background(), query.Query(sqlf.PostgresBindVar), query.Args()...); err != nil {
		t.Fatalf("unexpected error while updating links: %s %s", err, query.Query(sqlf.PostgresBindVar))
	}
}

func getProtectedUploads(t testing.TB, db database.DB, repositoryID int) []int {
	query := sqlf.Sprintf(
		`SELECT DISTINCT upload_id FROM lsif_uploads_visible_at_tip WHERE repository_id = %s ORDER BY upload_id`,
		repositoryID,
	)

	ids, err := basestore.ScanInts(db.QueryContext(context.Background(), query.Query(sqlf.PostgresBindVar), query.Args()...))
	if err != nil {
		t.Fatalf("unexpected error getting protected uploads: %s", err)
	}

	return ids
}

func getVisibleUploads(t testing.TB, db database.DB, repositoryID int, commits []string) map[string][]int {
	idsByCommit := map[string][]int{}
	for _, commit := range commits {
		query := makeVisibleUploadsQuery(repositoryID, commit)

		uploadIDs, err := basestore.ScanInts(db.QueryContext(
			context.Background(),
			query.Query(sqlf.PostgresBindVar),
			query.Args()...,
		))
		if err != nil {
			t.Fatalf("unexpected error getting visible upload IDs: %s", err)
		}
		sort.Ints(uploadIDs)

		idsByCommit[commit] = uploadIDs
	}

	return idsByCommit
}

func getUploadsVisibleAtTip(t testing.TB, db database.DB, repositoryID int) []int {
	query := sqlf.Sprintf(
		`SELECT upload_id FROM lsif_uploads_visible_at_tip WHERE repository_id = %s AND is_default_branch ORDER BY upload_id`,
		repositoryID,
	)

	ids, err := basestore.ScanInts(db.QueryContext(context.Background(), query.Query(sqlf.PostgresBindVar), query.Args()...))
	if err != nil {
		t.Fatalf("unexpected error getting uploads visible at tip: %s", err)
	}

	return ids
}

func assertCommitsVisibleFromUploads(t *testing.T, store Store, uploads []shared.Upload, expectedVisibleUploads map[string][]int) {
	expectedVisibleCommits := map[int][]string{}
	for commit, uploadIDs := range expectedVisibleUploads {
		for _, uploadID := range uploadIDs {
			expectedVisibleCommits[uploadID] = append(expectedVisibleCommits[uploadID], commit)
		}
	}
	for _, commits := range expectedVisibleCommits {
		sort.Strings(commits)
	}

	// Test pagination by requesting only a couple of
	// results at a time in this assertion helper.
	testPageSize := 2

	for _, upload := range uploads {
		var token *string
		var allCommits []string

		for {
			commits, nextToken, err := store.GetCommitsVisibleToUpload(context.Background(), upload.ID, testPageSize, token)
			if err != nil {
				t.Fatalf("unexpected error getting commits visible to upload %d: %s", upload.ID, err)
			}
			if nextToken == nil {
				break
			}

			allCommits = append(allCommits, commits...)
			token = nextToken
		}

		if diff := cmp.Diff(expectedVisibleCommits[upload.ID], allCommits); diff != "" {
			t.Errorf("unexpected commits visible to upload %d (-want +got):\n%s", upload.ID, diff)
		}
	}
}

func keysOf(m map[string][]int) (keys []string) {
	for k := range m {
		keys = append(keys, k)
	}

	return keys
}

//
// Benchmarks
//

func BenchmarkCalculateVisibleUploads(b *testing.B) {
	logger := logtest.Scoped(b)
	db := database.NewDB(logger, dbtest.NewDB(b))
	store := New(&observation.TestContext, db)

	graph, err := readBenchmarkCommitGraph()
	if err != nil {
		b.Fatalf("unexpected error reading benchmark commit graph: %s", err)
	}

	refDescriptions := map[string][]gitdomain.RefDescription{
		makeCommit(3): {{IsDefaultBranch: true}},
	}

	uploads, err := readBenchmarkCommitGraphView()
	if err != nil {
		b.Fatalf("unexpected error reading benchmark uploads: %s", err)
	}
	insertUploads(b, db, uploads...)

	b.ResetTimer()
	b.ReportAllocs()

	if err := store.UpdateUploadsVisibleToCommits(context.Background(), 50, graph, refDescriptions, time.Hour, time.Hour, 0, time.Now()); err != nil {
		b.Fatalf("unexpected error while calculating visible uploads: %s", err)
	}
}

func readBenchmarkCommitGraph() (*gitdomain.CommitGraph, error) {
	contents, err := readBenchmarkFile("../../../commitgraph/testdata/customer1/commits.txt.gz")
	if err != nil {
		return nil, err
	}

	return gitdomain.ParseCommitGraph(strings.Split(string(contents), "\n")), nil
}

func readBenchmarkCommitGraphView() ([]shared.Upload, error) {
	contents, err := readBenchmarkFile("../../../../codeintel/commitgraph/testdata/customer1/uploads.csv.gz")
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(bytes.NewReader(contents))

	var uploads []shared.Upload
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		id, err := strconv.Atoi(record[0])
		if err != nil {
			return nil, err
		}

		uploads = append(uploads, shared.Upload{
			ID:           id,
			RepositoryID: 50,
			Commit:       record[1],
			Root:         record[2],
		})
	}

	return uploads, nil
}

func readBenchmarkFile(path string) ([]byte, error) {
	uploadsFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer uploadsFile.Close()

	r, err := gzip.NewReader(uploadsFile)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	contents, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return contents, nil
}
