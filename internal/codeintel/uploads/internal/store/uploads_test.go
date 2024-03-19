package store

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/commitgraph"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestGetUploads(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)
	ctx := context.Background()

	t1 := time.Unix(1587396557, 0).UTC()
	t2 := t1.Add(-time.Minute * 1)
	t3 := t1.Add(-time.Minute * 2)
	t4 := t1.Add(-time.Minute * 3)
	t5 := t1.Add(-time.Minute * 4)
	t6 := t1.Add(-time.Minute * 5)
	t7 := t1.Add(-time.Minute * 6)
	t8 := t1.Add(-time.Minute * 7)
	t9 := t1.Add(-time.Minute * 8)
	t10 := t1.Add(-time.Minute * 9)
	t11 := t1.Add(-time.Minute * 10)
	failureMessage := "unlucky 333"

	insertUploads(t, db,
		shared.Upload{ID: 1, Commit: makeCommit(3331), UploadedAt: t1, Root: "sub1/", State: "queued"},
		shared.Upload{ID: 2, UploadedAt: t2, FinishedAt: &t1, State: "errored", FailureMessage: &failureMessage, Indexer: "scip-typescript"},
		shared.Upload{ID: 3, Commit: makeCommit(3333), UploadedAt: t3, Root: "sub2/", State: "queued"},
		shared.Upload{ID: 4, UploadedAt: t4, State: "queued", RepositoryID: 51, RepositoryName: "foo bar x"},
		shared.Upload{ID: 5, Commit: makeCommit(3333), UploadedAt: t5, Root: "sub1/", State: "processing", Indexer: "scip-typescript"},
		shared.Upload{ID: 6, UploadedAt: t6, Root: "sub2/", State: "processing", RepositoryID: 52, RepositoryName: "foo bar y"},
		shared.Upload{ID: 7, UploadedAt: t7, FinishedAt: &t4, Root: "sub1/", Indexer: "scip-typescript"},
		shared.Upload{ID: 8, UploadedAt: t8, FinishedAt: &t4, Indexer: "lsif-typescript"},
		shared.Upload{ID: 9, UploadedAt: t9, State: "queued"},
		shared.Upload{ID: 10, UploadedAt: t10, FinishedAt: &t6, Root: "sub1/", Indexer: "lsif-ocaml"},
		shared.Upload{ID: 11, UploadedAt: t11, FinishedAt: &t6, Root: "sub1/", Indexer: "scip-typescript"},

		// Deleted duplicates
		shared.Upload{ID: 12, Commit: makeCommit(3331), UploadedAt: t1, FinishedAt: &t1, Root: "sub1/", State: "deleted"},
		shared.Upload{ID: 13, UploadedAt: t2, FinishedAt: &t1, State: "deleted", FailureMessage: &failureMessage, Indexer: "scip-typescript"},
		shared.Upload{ID: 14, Commit: makeCommit(3333), UploadedAt: t3, FinishedAt: &t2, Root: "sub2/", State: "deleted"},

		// deleted repo
		shared.Upload{ID: 15, Commit: makeCommit(3334), UploadedAt: t4, State: "deleted", RepositoryID: 53, RepositoryName: "DELETED-barfoo"},

		// to-be hard deleted
		shared.Upload{ID: 16, Commit: makeCommit(3333), UploadedAt: t4, FinishedAt: &t3, State: "deleted"},
		shared.Upload{ID: 17, Commit: makeCommit(3334), UploadedAt: t4, FinishedAt: &t5, State: "deleting"},
	)
	insertVisibleAtTip(t, db, 50, 2, 5, 7, 8)

	updateUploads(t, db, shared.Upload{
		ID: 17, State: "deleted",
	})

	deleteUploads(t, db, 16)
	deleteUploads(t, db, 17)

	// upload 10 depends on uploads 7 and 8
	insertPackages(t, store, []shared.Package{
		{UploadID: 7, Scheme: "npm", Name: "foo", Version: "0.1.0"},
		{UploadID: 8, Scheme: "npm", Name: "bar", Version: "1.2.3"},
		{UploadID: 11, Scheme: "npm", Name: "foo", Version: "0.1.0"}, // duplicate package
	})
	insertPackageReferences(t, store, []shared.PackageReference{
		{Package: shared.Package{UploadID: 7, Scheme: "npm", Name: "bar", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 10, Scheme: "npm", Name: "foo", Version: "0.1.0"}},
		{Package: shared.Package{UploadID: 10, Scheme: "npm", Name: "bar", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 11, Scheme: "npm", Name: "bar", Version: "1.2.3"}},
	})

	dirtyRepositoryQuery := sqlf.Sprintf(
		`INSERT INTO lsif_dirty_repositories(repository_id, update_token, dirty_token, updated_at) VALUES (%s, 10, 20, %s)`,
		50,
		t5,
	)
	if _, err := db.ExecContext(ctx, dirtyRepositoryQuery.Query(sqlf.PostgresBindVar), dirtyRepositoryQuery.Args()...); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	type testCase struct {
		repositoryID        int
		state               string
		states              []string
		term                string
		visibleAtTip        bool
		dependencyOf        int
		dependentOf         int
		indexerNames        []string
		uploadedBefore      *time.Time
		uploadedAfter       *time.Time
		inCommitGraph       bool
		oldestFirst         bool
		allowDeletedRepo    bool
		alllowDeletedUpload bool
		expectedIDs         []int
	}
	testCases := []testCase{
		{expectedIDs: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}},
		{oldestFirst: true, expectedIDs: []int{11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}},
		{repositoryID: 50, expectedIDs: []int{1, 2, 3, 5, 7, 8, 9, 10, 11}},
		{state: "completed", expectedIDs: []int{7, 8, 10, 11}},
		{term: "sub", expectedIDs: []int{1, 3, 5, 6, 7, 10, 11}}, // searches root
		{term: "003", expectedIDs: []int{1, 3, 5}},               // searches commits
		{term: "333", expectedIDs: []int{1, 2, 3, 5}},            // searches commits and failure message
		{term: "typescript", expectedIDs: []int{2, 5, 7, 8, 11}}, // searches indexer
		{term: "QuEuEd", expectedIDs: []int{1, 3, 4, 9}},         // searches text status
		{term: "bAr", expectedIDs: []int{4, 6}},                  // search repo names
		{state: "failed", expectedIDs: []int{2}},                 // treats errored/failed states equivalently
		{visibleAtTip: true, expectedIDs: []int{2, 5, 7, 8}},
		{uploadedBefore: &t5, expectedIDs: []int{6, 7, 8, 9, 10, 11}},
		{uploadedAfter: &t4, expectedIDs: []int{1, 2, 3}},
		{inCommitGraph: true, expectedIDs: []int{10, 11}},
		{dependencyOf: 7, expectedIDs: []int{8}},
		{dependentOf: 7, expectedIDs: []int{10}},
		{dependencyOf: 8, expectedIDs: []int{}},
		{dependentOf: 8, expectedIDs: []int{7, 10, 11}},
		{dependencyOf: 10, expectedIDs: []int{7, 8}},
		{dependentOf: 10, expectedIDs: []int{}},
		{dependencyOf: 11, expectedIDs: []int{8}},
		{dependentOf: 11, expectedIDs: []int{}},
		{indexerNames: []string{"typescript", "ocaml"}, expectedIDs: []int{2, 5, 7, 8, 10, 11}}, // search indexer names (only)
		{allowDeletedRepo: true, state: "deleted", expectedIDs: []int{12, 13, 14, 15}},
		{allowDeletedRepo: true, state: "deleted", alllowDeletedUpload: true, expectedIDs: []int{12, 13, 14, 15, 16, 17}},
		{states: []string{"completed", "failed"}, expectedIDs: []int{2, 7, 8, 10, 11}},
	}

	runTest := func(testCase testCase, lo, hi int) (errors int) {
		name := fmt.Sprintf(
			"repositoryID=%d|state='%s'|states='%s',term='%s'|visibleAtTip=%v|dependencyOf=%d|dependentOf=%d|indexersNames=%v|offset=%d",
			testCase.repositoryID,
			testCase.state,
			strings.Join(testCase.states, ","),
			testCase.term,
			testCase.visibleAtTip,
			testCase.dependencyOf,
			testCase.dependentOf,
			testCase.indexerNames,
			lo,
		)

		t.Run(name, func(t *testing.T) {
			uploads, totalCount, err := store.GetUploads(ctx, shared.GetUploadsOptions{
				RepositoryID:       testCase.repositoryID,
				State:              testCase.state,
				States:             testCase.states,
				Term:               testCase.term,
				VisibleAtTip:       testCase.visibleAtTip,
				DependencyOf:       testCase.dependencyOf,
				DependentOf:        testCase.dependentOf,
				IndexerNames:       testCase.indexerNames,
				UploadedBefore:     testCase.uploadedBefore,
				UploadedAfter:      testCase.uploadedAfter,
				InCommitGraph:      testCase.inCommitGraph,
				OldestFirst:        testCase.oldestFirst,
				AllowDeletedRepo:   testCase.allowDeletedRepo,
				AllowDeletedUpload: testCase.alllowDeletedUpload,
				Limit:              3,
				Offset:             lo,
			})
			if err != nil {
				t.Fatalf("unexpected error getting uploads for repo: %s", err)
			}
			if totalCount != len(testCase.expectedIDs) {
				t.Errorf("unexpected total count. want=%d have=%d", len(testCase.expectedIDs), totalCount)
				errors++
			}

			if totalCount != 0 {
				var ids []int
				for _, upload := range uploads {
					ids = append(ids, upload.ID)
				}
				if diff := cmp.Diff(testCase.expectedIDs[lo:hi], ids); diff != "" {
					t.Errorf("unexpected upload ids at offset %d-%d (-want +got):\n%s", lo, hi, diff)
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

	t.Run("enforce repository permissions", func(t *testing.T) {
		// Enable permissions user mapping forces checking repository permissions
		// against permissions tables in the database, which should effectively block
		// all access because permissions tables are empty.
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				PermissionsUserMapping: &schema.PermissionsUserMapping{
					Enabled: true,
					BindID:  "email",
				},
			},
		})
		t.Cleanup(func() { conf.Mock(nil) })

		uploads, totalCount, err := store.GetUploads(ctx,
			shared.GetUploadsOptions{
				Limit: 1,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(uploads) > 0 || totalCount > 0 {
			t.Fatalf("Want no upload but got %d uploads with totalCount %d", len(uploads), totalCount)
		}
	})
}

func TestGetUploadByID(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// Upload does not exist initially
	if _, exists, err := store.GetUploadByID(ctx, 1); err != nil {
		t.Fatalf("unexpected error getting upload: %s", err)
	} else if exists {
		t.Fatal("unexpected record")
	}

	uploadedAt := time.Unix(1587396557, 0).UTC()
	startedAt := uploadedAt.Add(time.Minute)
	expected := shared.Upload{
		ID:             1,
		Commit:         makeCommit(1),
		Root:           "sub/",
		VisibleAtTip:   true,
		UploadedAt:     uploadedAt,
		State:          "processing",
		FailureMessage: nil,
		StartedAt:      &startedAt,
		FinishedAt:     nil,
		RepositoryID:   123,
		RepositoryName: "n-123",
		Indexer:        "lsif-go",
		IndexerVersion: "1.2.3",
		NumParts:       1,
		UploadedParts:  []int{},
		Rank:           nil,
	}

	insertUploads(t, db, expected)
	insertVisibleAtTip(t, db, 123, 1)

	if upload, exists, err := store.GetUploadByID(ctx, 1); err != nil {
		t.Fatalf("unexpected error getting upload: %s", err)
	} else if !exists {
		t.Fatal("expected record to exist")
	} else if diff := cmp.Diff(expected, upload); diff != "" {
		t.Errorf("unexpected upload (-want +got):\n%s", diff)
	}

	t.Run("enforce repository permissions", func(t *testing.T) {
		// Enable permissions user mapping forces checking repository permissions
		// against permissions tables in the database, which should effectively block
		// all access because permissions tables are empty.
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				PermissionsUserMapping: &schema.PermissionsUserMapping{
					Enabled: true,
					BindID:  "email",
				},
			},
		})
		t.Cleanup(func() { conf.Mock(nil) })

		_, exists, err := store.GetUploadByID(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Fatalf("exists: want false but got %v", exists)
		}
	})
}

func TestGetUploadByIDDeleted(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// Upload does not exist initially
	if _, exists, err := store.GetUploadByID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error getting upload: %s", err)
	} else if exists {
		t.Fatal("unexpected record")
	}

	uploadedAt := time.Unix(1587396557, 0).UTC()
	startedAt := uploadedAt.Add(time.Minute)
	expected := shared.Upload{
		ID:             1,
		Commit:         makeCommit(1),
		Root:           "sub/",
		VisibleAtTip:   true,
		UploadedAt:     uploadedAt,
		State:          "deleted",
		FailureMessage: nil,
		StartedAt:      &startedAt,
		FinishedAt:     nil,
		RepositoryID:   123,
		RepositoryName: "n-123",
		Indexer:        "lsif-go",
		NumParts:       1,
		UploadedParts:  []int{},
		Rank:           nil,
	}

	insertUploads(t, db, expected)

	// Should still not be queryable
	if _, exists, err := store.GetUploadByID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error getting upload: %s", err)
	} else if exists {
		t.Fatal("unexpected record")
	}
}

func TestGetCompletedUploadsByIDs(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// Dumps do not exist initially
	if uploads, err := store.GetCompletedUploadsByIDs(context.Background(), []int{1, 2}); err != nil {
		t.Fatalf("unexpected error getting dump: %s", err)
	} else if len(uploads) > 0 {
		t.Fatal("unexpected record")
	}

	uploadedAt := time.Unix(1587396557, 0).UTC()
	startedAt := uploadedAt.Add(time.Minute)
	finishedAt := uploadedAt.Add(time.Minute * 2)
	expectedAssociatedIndexID := 42
	expected1 := shared.CompletedUpload{
		ID:                1,
		Commit:            makeCommit(1),
		Root:              "sub/",
		VisibleAtTip:      true,
		UploadedAt:        uploadedAt,
		State:             "completed",
		FailureMessage:    nil,
		StartedAt:         &startedAt,
		FinishedAt:        &finishedAt,
		RepositoryID:      50,
		RepositoryName:    "n-50",
		Indexer:           "lsif-go",
		IndexerVersion:    "latest",
		AssociatedIndexID: &expectedAssociatedIndexID,
	}
	expected2 := shared.CompletedUpload{
		ID:                2,
		Commit:            makeCommit(2),
		Root:              "other/",
		VisibleAtTip:      false,
		UploadedAt:        uploadedAt,
		State:             "completed",
		FailureMessage:    nil,
		StartedAt:         &startedAt,
		FinishedAt:        &finishedAt,
		RepositoryID:      50,
		RepositoryName:    "n-50",
		Indexer:           "scip-typescript",
		IndexerVersion:    "1.2.3",
		AssociatedIndexID: nil,
	}

	insertUploads(t, db, expected1.ConvertToUpload(), expected2.ConvertToUpload())
	insertVisibleAtTip(t, db, 50, 1)

	if uploads, err := store.GetCompletedUploadsByIDs(context.Background(), []int{1}); err != nil {
		t.Fatalf("unexpected error getting dump: %s", err)
	} else if len(uploads) != 1 {
		t.Fatal("expected one record")
	} else if diff := cmp.Diff(expected1, uploads[0]); diff != "" {
		t.Errorf("unexpected dump (-want +got):\n%s", diff)
	}

	if uploads, err := store.GetCompletedUploadsByIDs(context.Background(), []int{1, 2}); err != nil {
		t.Fatalf("unexpected error getting dump: %s", err)
	} else if len(uploads) != 2 {
		t.Fatal("expected two records")
	} else if diff := cmp.Diff(expected1, uploads[0]); diff != "" {
		t.Errorf("unexpected dump (-want +got):\n%s", diff)
	} else if diff := cmp.Diff(expected2, uploads[1]); diff != "" {
		t.Errorf("unexpected dump (-want +got):\n%s", diff)
	}
}

func TestGetUploadsByIDs(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertUploads(t, db,
		shared.Upload{ID: 1},
		shared.Upload{ID: 2},
		shared.Upload{ID: 3},
		shared.Upload{ID: 4},
		shared.Upload{ID: 5},
		shared.Upload{ID: 6},
		shared.Upload{ID: 7},
		shared.Upload{ID: 8},
		shared.Upload{ID: 9},
		shared.Upload{ID: 10},
	)

	t.Run("fetch", func(t *testing.T) {
		indexes, err := store.GetUploadsByIDs(ctx, 2, 4, 6, 8, 12)
		if err != nil {
			t.Fatalf("unexpected error getting indexes for repo: %s", err)
		}

		var ids []int
		for _, index := range indexes {
			ids = append(ids, index.ID)
		}
		sort.Ints(ids)

		if diff := cmp.Diff([]int{2, 4, 6, 8}, ids); diff != "" {
			t.Errorf("unexpected index ids (-want +got):\n%s", diff)
		}
	})

	t.Run("enforce repository permissions", func(t *testing.T) {
		// Enable permissions user mapping forces checking repository permissions
		// against permissions tables in the database, which should effectively block
		// all access because permissions tables are empty.
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				PermissionsUserMapping: &schema.PermissionsUserMapping{
					Enabled: true,
					BindID:  "email",
				},
			},
		})
		t.Cleanup(func() { conf.Mock(nil) })

		indexes, err := store.GetUploadsByIDs(ctx, 1, 2, 3, 4)
		if err != nil {
			t.Fatal(err)
		}
		if len(indexes) > 0 {
			t.Fatalf("Want no index but got %d indexes", len(indexes))
		}
	})
}

func TestGetVisibleUploadsMatchingMonikers(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertUploads(t, db,
		shared.Upload{ID: 1, Commit: makeCommit(2), Root: "sub1/"},
		shared.Upload{ID: 2, Commit: makeCommit(3), Root: "sub2/"},
		shared.Upload{ID: 3, Commit: makeCommit(4), Root: "sub3/"},
		shared.Upload{ID: 4, Commit: makeCommit(3), Root: "sub4/"},
		shared.Upload{ID: 5, Commit: makeCommit(2), Root: "sub5/"},
	)

	insertNearestUploads(t, db, 50, map[string][]commitgraph.UploadMeta{
		makeCommit(1): {
			{UploadID: 1, Distance: 1},
			{UploadID: 2, Distance: 2},
			{UploadID: 3, Distance: 3},
			{UploadID: 4, Distance: 2},
			{UploadID: 5, Distance: 1},
		},
		makeCommit(2): {
			{UploadID: 1, Distance: 0},
			{UploadID: 2, Distance: 1},
			{UploadID: 3, Distance: 2},
			{UploadID: 4, Distance: 1},
			{UploadID: 5, Distance: 0},
		},
		makeCommit(3): {
			{UploadID: 1, Distance: 1},
			{UploadID: 2, Distance: 0},
			{UploadID: 3, Distance: 1},
			{UploadID: 4, Distance: 0},
			{UploadID: 5, Distance: 1},
		},
		makeCommit(4): {
			{UploadID: 1, Distance: 2},
			{UploadID: 2, Distance: 1},
			{UploadID: 3, Distance: 0},
			{UploadID: 4, Distance: 1},
			{UploadID: 5, Distance: 2},
		},
	})

	insertPackageReferences(t, store, []shared.PackageReference{
		{Package: shared.Package{UploadID: 1, Scheme: "gomod", Name: "leftpad", Version: "0.1.0"}},
		{Package: shared.Package{UploadID: 2, Scheme: "gomod", Name: "leftpad", Version: "0.1.0"}},
		{Package: shared.Package{UploadID: 3, Scheme: "gomod", Name: "leftpad", Version: "0.1.0"}},
		{Package: shared.Package{UploadID: 4, Scheme: "gomod", Name: "leftpad", Version: "0.1.0"}},
		{Package: shared.Package{UploadID: 5, Scheme: "gomod", Name: "leftpad", Version: "0.1.0"}},
	})

	moniker := precise.QualifiedMonikerData{
		MonikerData: precise.MonikerData{
			Scheme: "gomod",
		},
		PackageInformationData: precise.PackageInformationData{
			Name:    "leftpad",
			Version: "0.1.0",
		},
	}

	refs := []shared.PackageReference{
		{Package: shared.Package{UploadID: 1, Scheme: "gomod", Name: "leftpad", Version: "0.1.0"}},
		{Package: shared.Package{UploadID: 2, Scheme: "gomod", Name: "leftpad", Version: "0.1.0"}},
		{Package: shared.Package{UploadID: 3, Scheme: "gomod", Name: "leftpad", Version: "0.1.0"}},
		{Package: shared.Package{UploadID: 4, Scheme: "gomod", Name: "leftpad", Version: "0.1.0"}},
		{Package: shared.Package{UploadID: 5, Scheme: "gomod", Name: "leftpad", Version: "0.1.0"}},
	}

	testCases := []struct {
		limit    int
		offset   int
		expected []shared.PackageReference
	}{
		{5, 0, refs},
		{5, 2, refs[2:]},
		{2, 1, refs[1:3]},
		{5, 5, nil},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("i=%d", i), func(t *testing.T) {
			scanner, totalCount, err := store.GetVisibleUploadsMatchingMonikers(context.Background(), 50, makeCommit(1), []precise.QualifiedMonikerData{moniker}, testCase.limit, testCase.offset)
			if err != nil {
				t.Fatalf("unexpected error getting scanner: %s", err)
			}

			if totalCount != 5 {
				t.Errorf("unexpected count. want=%d have=%d", 5, totalCount)
			}

			filters, err := consumeScanner(scanner)
			if err != nil {
				t.Fatalf("unexpected error from scanner: %s", err)
			}

			if diff := cmp.Diff(testCase.expected, filters); diff != "" {
				t.Errorf("unexpected filters (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("enforce repository permissions", func(t *testing.T) {
		// Enable permissions user mapping forces checking repository permissions
		// against permissions tables in the database, which should effectively block
		// all access because permissions tables are empty.
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				PermissionsUserMapping: &schema.PermissionsUserMapping{
					Enabled: true,
					BindID:  "email",
				},
			},
		})
		t.Cleanup(func() { conf.Mock(nil) })

		_, totalCount, err := store.GetVisibleUploadsMatchingMonikers(context.Background(), 50, makeCommit(1), []precise.QualifiedMonikerData{moniker}, 50, 0)
		if err != nil {
			t.Fatalf("unexpected error getting filters: %s", err)
		}
		if totalCount != 0 {
			t.Errorf("unexpected count. want=%d have=%d", 0, totalCount)
		}
	})
}

func TestDefinitionDumps(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	moniker1 := precise.QualifiedMonikerData{
		MonikerData: precise.MonikerData{
			Scheme: "gomod",
		},
		PackageInformationData: precise.PackageInformationData{
			Name:    "leftpad",
			Version: "0.1.0",
		},
	}

	moniker2 := precise.QualifiedMonikerData{
		MonikerData: precise.MonikerData{
			Scheme: "npm",
		},
		PackageInformationData: precise.PackageInformationData{
			Name:    "rightpad",
			Version: "0.2.0",
		},
	}

	// Package does not exist initially
	if uploads, err := store.GetCompletedUploadsWithDefinitionsForMonikers(context.Background(), []precise.QualifiedMonikerData{moniker1}); err != nil {
		t.Fatalf("unexpected error getting package: %s", err)
	} else if len(uploads) != 0 {
		t.Fatal("unexpected record")
	}

	uploadedAt := time.Unix(1587396557, 0).UTC()
	startedAt := uploadedAt.Add(time.Minute)
	finishedAt := uploadedAt.Add(time.Minute * 2)
	expected1 := shared.CompletedUpload{
		ID:             1,
		Commit:         makeCommit(1),
		Root:           "sub/",
		VisibleAtTip:   true,
		UploadedAt:     uploadedAt,
		State:          "completed",
		FailureMessage: nil,
		StartedAt:      &startedAt,
		FinishedAt:     &finishedAt,
		RepositoryID:   50,
		RepositoryName: "n-50",
		Indexer:        "lsif-go",
		IndexerVersion: "latest",
	}
	expected2 := shared.CompletedUpload{
		ID:                2,
		Commit:            makeCommit(2),
		Root:              "other/",
		VisibleAtTip:      false,
		UploadedAt:        uploadedAt,
		State:             "completed",
		FailureMessage:    nil,
		StartedAt:         &startedAt,
		FinishedAt:        &finishedAt,
		RepositoryID:      50,
		RepositoryName:    "n-50",
		Indexer:           "scip-typescript",
		IndexerVersion:    "1.2.3",
		AssociatedIndexID: nil,
	}
	expected3 := shared.CompletedUpload{
		ID:             3,
		Commit:         makeCommit(3),
		Root:           "sub/",
		VisibleAtTip:   true,
		UploadedAt:     uploadedAt,
		State:          "completed",
		FailureMessage: nil,
		StartedAt:      &startedAt,
		FinishedAt:     &finishedAt,
		RepositoryID:   50,
		RepositoryName: "n-50",
		Indexer:        "lsif-go",
		IndexerVersion: "latest",
	}

	insertUploads(t, db, expected1.ConvertToUpload(), expected2.ConvertToUpload(), expected3.ConvertToUpload())
	insertVisibleAtTip(t, db, 50, 1)

	if err := store.UpdatePackages(context.Background(), 1, []precise.Package{
		{Scheme: "gomod", Name: "leftpad", Version: "0.1.0"},
	}); err != nil {
		t.Fatalf("unexpected error updating packages: %s", err)
	}

	if err := store.UpdatePackages(context.Background(), 2, []precise.Package{
		{Scheme: "npm", Name: "rightpad", Version: "0.2.0"},
	}); err != nil {
		t.Fatalf("unexpected error updating packages: %s", err)
	}

	// Duplicate package
	if err := store.UpdatePackages(context.Background(), 3, []precise.Package{
		{Scheme: "gomod", Name: "leftpad", Version: "0.1.0"},
	}); err != nil {
		t.Fatalf("unexpected error updating packages: %s", err)
	}

	if uploads, err := store.GetCompletedUploadsWithDefinitionsForMonikers(context.Background(), []precise.QualifiedMonikerData{moniker1}); err != nil {
		t.Fatalf("unexpected error getting package: %s", err)
	} else if len(uploads) != 1 {
		t.Fatal("expected one record")
	} else if diff := cmp.Diff(expected1, uploads[0]); diff != "" {
		t.Errorf("unexpected dump (-want +got):\n%s", diff)
	}

	if uploads, err := store.GetCompletedUploadsWithDefinitionsForMonikers(context.Background(), []precise.QualifiedMonikerData{moniker1, moniker2}); err != nil {
		t.Fatalf("unexpected error getting package: %s", err)
	} else if len(uploads) != 2 {
		t.Fatal("expected two records")
	} else if diff := cmp.Diff(expected1, uploads[0]); diff != "" {
		t.Errorf("unexpected dump (-want +got):\n%s", diff)
	} else if diff := cmp.Diff(expected2, uploads[1]); diff != "" {
		t.Errorf("unexpected dump (-want +got):\n%s", diff)
	}

	t.Run("enforce repository permissions", func(t *testing.T) {
		// Turning on explicit permissions forces checking repository permissions
		// against permissions tables in the database, which should effectively block
		// all access because permissions tables are empty and repo that dumps belong
		// to are private.
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				PermissionsUserMapping: &schema.PermissionsUserMapping{
					Enabled: true,
					BindID:  "email",
				},
			},
		})
		t.Cleanup(func() { conf.Mock(nil) })

		if uploads, err := store.GetCompletedUploadsWithDefinitionsForMonikers(context.Background(), []precise.QualifiedMonikerData{moniker1, moniker2}); err != nil {
			t.Fatalf("unexpected error getting package: %s", err)
		} else if len(uploads) != 0 {
			t.Errorf("unexpected count. want=%d have=%d", 0, len(uploads))
		}
	})
}

func TestUploadAuditLogs(t *testing.T) {
	logger := logtest.Scoped(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(logger, sqlDB)
	store := New(&observation.TestContext, db)

	insertUploads(t, db, shared.Upload{ID: 1})
	updateUploads(t, db, shared.Upload{ID: 1, State: "deleting"})

	logs, err := store.GetAuditLogsForUpload(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error fetching audit logs: %s", err)
	}
	if len(logs) != 2 {
		t.Fatalf("unexpected number of logs. want=%v have=%v", 2, len(logs))
	}

	stateTransition := transitionForColumn(t, "state", logs[1].TransitionColumns)
	if *stateTransition["new"] != "deleting" {
		t.Fatalf("unexpected state column transition values. want=%v got=%v", "deleting", *stateTransition["new"])
	}
}

func transitionForColumn(t *testing.T, key string, transitions []map[string]*string) map[string]*string {
	for _, transition := range transitions {
		if val := transition["column"]; val != nil && *val == key {
			return transition
		}
	}

	t.Fatalf("no transition for key found. key=%s, transitions=%v", key, transitions)
	return nil
}

func TestDeleteUploads(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	t1 := time.Unix(1587396557, 0).UTC()
	t2 := t1.Add(time.Minute * 1)
	t3 := t1.Add(time.Minute * 2)
	t4 := t1.Add(time.Minute * 3)
	t5 := t1.Add(time.Minute * 4)

	insertUploads(t, db,
		shared.Upload{ID: 1, Commit: makeCommit(1111), UploadedAt: t1, State: "queued"},    // will not be deleted
		shared.Upload{ID: 2, Commit: makeCommit(1112), UploadedAt: t2, State: "uploading"}, // will be deleted
		shared.Upload{ID: 3, Commit: makeCommit(1113), UploadedAt: t3, State: "uploading"}, // will be deleted
		shared.Upload{ID: 4, Commit: makeCommit(1114), UploadedAt: t4, State: "completed"}, // will not be deleted
		shared.Upload{ID: 5, Commit: makeCommit(1115), UploadedAt: t5, State: "uploading"}, // will be deleted
	)

	err := store.DeleteUploads(context.Background(), shared.DeleteUploadsOptions{
		States:       []string{"uploading"},
		Term:         "",
		VisibleAtTip: false,
	})
	if err != nil {
		t.Fatalf("unexpected error deleting uploads: %s", err)
	}

	uploads, totalCount, err := store.GetUploads(context.Background(), shared.GetUploadsOptions{Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error getting uploads: %s", err)
	}

	var ids []int
	for _, upload := range uploads {
		ids = append(ids, upload.ID)
	}
	sort.Ints(ids)

	expectedIDs := []int{1, 4}

	if totalCount != len(expectedIDs) {
		t.Errorf("unexpected total count. want=%d have=%d", len(expectedIDs), totalCount)
	}
	if diff := cmp.Diff(expectedIDs, ids); diff != "" {
		t.Errorf("unexpected upload ids (-want +got):\n%s", diff)
	}
}

func TestDeleteUploadsWithIndexerKey(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// note: queued so we delete, not go to deleting state first (makes assertion simpler)
	insertUploads(t, db, shared.Upload{ID: 1, State: "queued", Indexer: "sourcegraph/scip-go@sha256:123456"})
	insertUploads(t, db, shared.Upload{ID: 2, State: "queued", Indexer: "sourcegraph/scip-go"})
	insertUploads(t, db, shared.Upload{ID: 3, State: "queued", Indexer: "sourcegraph/scip-typescript"})
	insertUploads(t, db, shared.Upload{ID: 4, State: "queued", Indexer: "sourcegraph/scip-typescript"})

	err := store.DeleteUploads(context.Background(), shared.DeleteUploadsOptions{
		IndexerNames: []string{"scip-go"},
		Term:         "",
		VisibleAtTip: false,
	})
	if err != nil {
		t.Fatalf("unexpected error deleting uploads: %s", err)
	}

	uploads, totalCount, err := store.GetUploads(context.Background(), shared.GetUploadsOptions{Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error getting uploads: %s", err)
	}

	var ids []int
	for _, upload := range uploads {
		ids = append(ids, upload.ID)
	}
	sort.Ints(ids)

	expectedIDs := []int{3, 4}

	if totalCount != len(expectedIDs) {
		t.Errorf("unexpected total count. want=%d have=%d", len(expectedIDs), totalCount)
	}
	if diff := cmp.Diff(expectedIDs, ids); diff != "" {
		t.Errorf("unexpected upload ids (-want +got):\n%s", diff)
	}
}

func TestDeleteUploadByID(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertUploads(t, db,
		shared.Upload{ID: 1, RepositoryID: 50},
	)

	if found, err := store.DeleteUploadByID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error deleting upload: %s", err)
	} else if !found {
		t.Fatalf("expected record to exist")
	}

	// Ensure record was deleted
	if states, err := getUploadStates(db, 1); err != nil {
		t.Fatalf("unexpected error getting states: %s", err)
	} else if diff := cmp.Diff(map[int]string{1: "deleting"}, states); diff != "" {
		t.Errorf("unexpected dump (-want +got):\n%s", diff)
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

	if len(keys) != 1 || keys[0] != 50 {
		t.Errorf("expected repository to be marked dirty")
	}
}

func TestDeleteUploadByIDMissingRow(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	if found, err := store.DeleteUploadByID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error deleting upload: %s", err)
	} else if found {
		t.Fatalf("unexpected record")
	}
}

func TestDeleteUploadByIDNotCompleted(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertUploads(t, db,
		shared.Upload{ID: 1, RepositoryID: 50, State: "uploading"},
	)

	if found, err := store.DeleteUploadByID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error deleting upload: %s", err)
	} else if !found {
		t.Fatalf("expected record to exist")
	}

	// Ensure record was deleted
	if states, err := getUploadStates(db, 1); err != nil {
		t.Fatalf("unexpected error getting states: %s", err)
	} else if diff := cmp.Diff(map[int]string{1: "deleted"}, states); diff != "" {
		t.Errorf("unexpected dump (-want +got):\n%s", diff)
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

	if len(keys) != 1 || keys[0] != 50 {
		t.Errorf("expected repository to be marked dirty")
	}
}

func TestReindexUploads(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertUploads(t, db, shared.Upload{ID: 1, State: "completed"})
	insertUploads(t, db, shared.Upload{ID: 2, State: "errored"})

	if err := store.ReindexUploads(context.Background(), shared.ReindexUploadsOptions{
		States:       []string{"errored"},
		Term:         "",
		RepositoryID: 0,
	}); err != nil {
		t.Fatalf("unexpected error reindexing uploads: %s", err)
	}

	// Upload has been marked for reindexing
	if upload, exists, err := store.GetUploadByID(context.Background(), 2); err != nil {
		t.Fatalf("unexpected error getting upload: %s", err)
	} else if !exists {
		t.Fatal("upload missing")
	} else if !upload.ShouldReindex {
		t.Fatal("upload not marked for reindexing")
	}
}

func TestReindexUploadsWithIndexerKey(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertUploads(t, db, shared.Upload{ID: 1, Indexer: "sourcegraph/scip-go@sha256:123456"})
	insertUploads(t, db, shared.Upload{ID: 2, Indexer: "sourcegraph/scip-go"})
	insertUploads(t, db, shared.Upload{ID: 3, Indexer: "sourcegraph/scip-typescript"})
	insertUploads(t, db, shared.Upload{ID: 4, Indexer: "sourcegraph/scip-typescript"})

	if err := store.ReindexUploads(context.Background(), shared.ReindexUploadsOptions{
		IndexerNames: []string{"scip-go"},
		Term:         "",
		RepositoryID: 0,
	}); err != nil {
		t.Fatalf("unexpected error reindexing uploads: %s", err)
	}

	// Expected uploads marked for re-indexing
	for id, expected := range map[int]bool{
		1: true, 2: true,
		3: false, 4: false,
	} {
		if upload, exists, err := store.GetUploadByID(context.Background(), id); err != nil {
			t.Fatalf("unexpected error getting upload: %s", err)
		} else if !exists {
			t.Fatal("upload missing")
		} else if upload.ShouldReindex != expected {
			t.Fatalf("unexpected mark. want=%v have=%v", expected, upload.ShouldReindex)
		}
	}
}

func TestReindexUploadByID(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertUploads(t, db, shared.Upload{ID: 1, State: "completed"})
	insertUploads(t, db, shared.Upload{ID: 2, State: "errored"})

	if err := store.ReindexUploadByID(context.Background(), 2); err != nil {
		t.Fatalf("unexpected error reindexing uploads: %s", err)
	}

	// Upload has been marked for reindexing
	if upload, exists, err := store.GetUploadByID(context.Background(), 2); err != nil {
		t.Fatalf("unexpected error getting upload: %s", err)
	} else if !exists {
		t.Fatal("upload missing")
	} else if !upload.ShouldReindex {
		t.Fatal("upload not marked for reindexing")
	}
}

func updateUploads(t testing.TB, db database.DB, uploads ...shared.Upload) {
	for _, upload := range uploads {
		query := sqlf.Sprintf(`
			UPDATE lsif_uploads
			SET
				commit = COALESCE(NULLIF(%s, ''), commit),
				root = COALESCE(NULLIF(%s, ''), root),
				uploaded_at = COALESCE(NULLIF(%s, '0001-01-01 00:00:00+00'::timestamptz), uploaded_at),
				state = COALESCE(NULLIF(%s, ''), state),
				failure_message  = COALESCE(%s, failure_message),
				started_at = COALESCE(%s, started_at),
				finished_at = COALESCE(%s, finished_at),
				process_after = COALESCE(%s, process_after),
				num_resets = COALESCE(NULLIF(%s, 0), num_resets),
				num_failures = COALESCE(NULLIF(%s, 0), num_failures),
				repository_id = COALESCE(NULLIF(%s, 0), repository_id),
				indexer = COALESCE(NULLIF(%s, ''), indexer),
				indexer_version = COALESCE(NULLIF(%s, ''), indexer_version),
				num_parts = COALESCE(NULLIF(%s, 0), num_parts),
				uploaded_parts = COALESCE(NULLIF(%s, '{}'::integer[]), uploaded_parts),
				upload_size = COALESCE(%s, upload_size),
				associated_index_id = COALESCE(%s, associated_index_id),
				content_type = COALESCE(NULLIF(%s, ''), content_type),
				should_reindex = COALESCE(NULLIF(%s, false), should_reindex)
			WHERE id = %s
		`,
			upload.Commit,
			upload.Root,
			upload.UploadedAt,
			upload.State,
			upload.FailureMessage,
			upload.StartedAt,
			upload.FinishedAt,
			upload.ProcessAfter,
			upload.NumResets,
			upload.NumFailures,
			upload.RepositoryID,
			upload.Indexer,
			upload.IndexerVersion,
			upload.NumParts,
			pq.Array(upload.UploadedParts),
			upload.UploadSize,
			upload.AssociatedIndexID,
			upload.ContentType,
			upload.ShouldReindex,
			upload.ID,
		)

		if _, err := db.ExecContext(context.Background(), query.Query(sqlf.PostgresBindVar), query.Args()...); err != nil {
			t.Fatalf("unexpected error while updating upload: %s", err)
		}
	}
}

func deleteUploads(t testing.TB, db database.DB, uploads ...int) {
	for _, upload := range uploads {
		query := sqlf.Sprintf(`DELETE FROM lsif_uploads WHERE id = %s`, upload)
		if _, err := db.ExecContext(context.Background(), query.Query(sqlf.PostgresBindVar), query.Args()...); err != nil {
			t.Fatalf("unexpected error while deleting upload: %s", err)
		}
	}
}

// insertVisibleAtTip populates rows of the lsif_uploads_visible_at_tip table for the given repository
// with the given identifiers. Each upload is assumed to refer to the tip of the default branch. To mark
// an upload as protected (visible to _some_ branch) butn ot visible from the default branch, use the
// insertVisibleAtTipNonDefaultBranch method instead.
func insertVisibleAtTip(t testing.TB, db database.DB, repositoryID int, uploadIDs ...int) {
	insertVisibleAtTipInternal(t, db, repositoryID, true, uploadIDs...)
}

func insertVisibleAtTipInternal(t testing.TB, db database.DB, repositoryID int, isDefaultBranch bool, uploadIDs ...int) {
	var rows []*sqlf.Query
	for _, uploadID := range uploadIDs {
		rows = append(rows, sqlf.Sprintf("(%s, %s, %s)", repositoryID, uploadID, isDefaultBranch))
	}

	query := sqlf.Sprintf(
		`INSERT INTO lsif_uploads_visible_at_tip (repository_id, upload_id, is_default_branch) VALUES %s`,
		sqlf.Join(rows, ","),
	)
	if _, err := db.ExecContext(context.Background(), query.Query(sqlf.PostgresBindVar), query.Args()...); err != nil {
		t.Fatalf("unexpected error while updating uploads visible at tip: %s", err)
	}
}
