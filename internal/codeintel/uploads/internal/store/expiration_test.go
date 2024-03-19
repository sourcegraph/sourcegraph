package store

import (
	"context"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func TestSoftDeleteExpiredUploads(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertUploads(t, db,
		shared.Upload{ID: 50, RepositoryID: 100, State: "completed"},
		shared.Upload{ID: 51, RepositoryID: 101, State: "completed"},
		shared.Upload{ID: 52, RepositoryID: 102, State: "completed"},
		shared.Upload{ID: 53, RepositoryID: 102, State: "completed"}, // referenced by 51, 52, 54, 55, 56
		shared.Upload{ID: 54, RepositoryID: 103, State: "completed"}, // referenced by 52
		shared.Upload{ID: 55, RepositoryID: 103, State: "completed"}, // referenced by 51
		shared.Upload{ID: 56, RepositoryID: 103, State: "completed"}, // referenced by 52, 53
	)
	insertPackages(t, store, []shared.Package{
		{UploadID: 53, Scheme: "test", Name: "p1", Version: "1.2.3"},
		{UploadID: 54, Scheme: "test", Name: "p2", Version: "1.2.3"},
		{UploadID: 55, Scheme: "test", Name: "p3", Version: "1.2.3"},
		{UploadID: 56, Scheme: "test", Name: "p4", Version: "1.2.3"},
	})
	insertPackageReferences(t, store, []shared.PackageReference{
		// References removed
		{Package: shared.Package{UploadID: 51, Scheme: "test", Name: "p1", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 51, Scheme: "test", Name: "p2", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 51, Scheme: "test", Name: "p3", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 52, Scheme: "test", Name: "p1", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 52, Scheme: "test", Name: "p4", Version: "1.2.3"}},

		// Remaining references
		{Package: shared.Package{UploadID: 53, Scheme: "test", Name: "p4", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 54, Scheme: "test", Name: "p1", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 55, Scheme: "test", Name: "p1", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 56, Scheme: "test", Name: "p1", Version: "1.2.3"}},
	})

	// expire uploads 51-54
	if err := store.UpdateUploadRetention(context.Background(), []int{}, []int{51, 52, 53, 54}); err != nil {
		t.Fatalf("unexpected error marking uploads as expired: %s", err)
	}

	if _, count, err := store.SoftDeleteExpiredUploads(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error soft deleting uploads: %s", err)
	} else if count != 2 {
		t.Fatalf("unexpected number of uploads deleted: want=%d have=%d", 2, count)
	}

	// Ensure records were deleted
	expectedStates := map[int]string{
		50: "completed",
		51: "deleting",
		52: "deleting",
		53: "completed",
		54: "completed",
		55: "completed",
		56: "completed",
	}
	if states, err := getUploadStates(db, 50, 51, 52, 53, 54, 55, 56); err != nil {
		t.Fatalf("unexpected error getting states: %s", err)
	} else if diff := cmp.Diff(expectedStates, states); diff != "" {
		t.Errorf("unexpected upload states (-want +got):\n%s", diff)
	}

	// Ensure repository was marked as dirty
	dirtyRepositories, err := store.GetDirtyRepositories(context.Background())
	if err != nil {
		t.Fatalf("unexpected error listing dirty repositories: %s", err)
	}

	var keys []int
	for _, dirtyRepository := range dirtyRepositories {
		keys = append(keys, dirtyRepository.RepositoryID)
	}
	sort.Ints(keys)

	expectedKeys := []int{101, 102}
	if diff := cmp.Diff(expectedKeys, keys); diff != "" {
		t.Errorf("unexpected dirty repositories (-want +got):\n%s", diff)
	}
}

func TestSoftDeleteExpiredUploadsViaTraversal(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// The packages in this test reference each other in the following way:
	//
	//     [p1] ---> [p2] -> [p3]    [p8]
	//      ^         ^       |       ^
	//      |         |       |       |
	//      +----+----+       |       |
	//           |            v       v
	// [p6] --> [p5] <------ [p4]    [p9]
	//  ^
	//  |
	//  v
	// [p7]
	//
	// Note that all packages except for p6 are attached to an expired upload,
	// and each upload is _reachable_ from a non-expired upload.

	insertUploads(t, db,
		shared.Upload{ID: 100, RepositoryID: 50, State: "completed"}, // Referenced by 104
		shared.Upload{ID: 101, RepositoryID: 51, State: "completed"}, // Referenced by 100, 104
		shared.Upload{ID: 102, RepositoryID: 52, State: "completed"}, // Referenced by 101
		shared.Upload{ID: 103, RepositoryID: 53, State: "completed"}, // Referenced by 102
		shared.Upload{ID: 104, RepositoryID: 54, State: "completed"}, // Referenced by 103, 105
		shared.Upload{ID: 105, RepositoryID: 55, State: "completed"}, // Referenced by 106
		shared.Upload{ID: 106, RepositoryID: 56, State: "completed"}, // Referenced by 105

		// Another component
		shared.Upload{ID: 107, RepositoryID: 57, State: "completed"}, // Referenced by 108
		shared.Upload{ID: 108, RepositoryID: 58, State: "completed"}, // Referenced by 107
	)
	insertPackages(t, store, []shared.Package{
		{UploadID: 100, Scheme: "test", Name: "p1", Version: "1.2.3"},
		{UploadID: 101, Scheme: "test", Name: "p2", Version: "1.2.3"},
		{UploadID: 102, Scheme: "test", Name: "p3", Version: "1.2.3"},
		{UploadID: 103, Scheme: "test", Name: "p4", Version: "1.2.3"},
		{UploadID: 104, Scheme: "test", Name: "p5", Version: "1.2.3"},
		{UploadID: 105, Scheme: "test", Name: "p6", Version: "1.2.3"},
		{UploadID: 106, Scheme: "test", Name: "p7", Version: "1.2.3"},

		// Another component
		{UploadID: 107, Scheme: "test", Name: "p8", Version: "1.2.3"},
		{UploadID: 108, Scheme: "test", Name: "p9", Version: "1.2.3"},
	})
	insertPackageReferences(t, store, []shared.PackageReference{
		{Package: shared.Package{UploadID: 100, Scheme: "test", Name: "p2", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 101, Scheme: "test", Name: "p3", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 102, Scheme: "test", Name: "p4", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 103, Scheme: "test", Name: "p5", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 104, Scheme: "test", Name: "p1", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 104, Scheme: "test", Name: "p2", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 105, Scheme: "test", Name: "p5", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 106, Scheme: "test", Name: "p6", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 105, Scheme: "test", Name: "p7", Version: "1.2.3"}},

		// Another component
		{Package: shared.Package{UploadID: 107, Scheme: "test", Name: "p9", Version: "1.2.3"}},
		{Package: shared.Package{UploadID: 108, Scheme: "test", Name: "p8", Version: "1.2.3"}},
	})

	// We'll first confirm that none of the uploads can be deleted by either of the soft delete mechanisms;
	// once we expire the upload providing p6, the "unreferenced" method should no-op, but the traversal
	// method should soft delete all fo them.

	// expire all uploads except 105 and 109
	if err := store.UpdateUploadRetention(context.Background(), []int{}, []int{100, 101, 102, 103, 104, 106, 107}); err != nil {
		t.Fatalf("unexpected error marking uploads as expired: %s", err)
	}
	if _, count, err := store.SoftDeleteExpiredUploads(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error soft deleting uploads: %s", err)
	} else if count != 0 {
		t.Fatalf("unexpected number of uploads deleted via refcount: want=%d have=%d", 0, count)
	}
	for range 9 {
		// Initially null last_traversal_scan_at values; run once for each upload (overkill)
		if _, count, err := store.SoftDeleteExpiredUploadsViaTraversal(context.Background(), 100); err != nil {
			t.Fatalf("unexpected error soft deleting uploads: %s", err)
		} else if count != 0 {
			t.Fatalf("unexpected number of uploads deleted via traversal: want=%d have=%d", 0, count)
		}
	}
	if _, count, err := store.SoftDeleteExpiredUploadsViaTraversal(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error soft deleting uploads: %s", err)
	} else if count != 0 {
		t.Fatalf("unexpected number of uploads deleted via traversal: want=%d have=%d", 0, count)
	}

	// Expire upload 105, making the connected component soft-deletable
	if err := store.UpdateUploadRetention(context.Background(), []int{}, []int{105}); err != nil {
		t.Fatalf("unexpected error marking uploads as expired: %s", err)
	}
	// Reset timestamps so the test is deterministics
	if _, err := db.ExecContext(context.Background(), "UPDATE lsif_uploads SET last_traversal_scan_at = NULL"); err != nil {
		t.Fatalf("unexpected error clearing last_traversal_scan_at: %s", err)
	}
	if _, count, err := store.SoftDeleteExpiredUploads(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error soft deleting uploads: %s", err)
	} else if count != 0 {
		t.Fatalf("unexpected number of uploads deleted via refcount: want=%d have=%d", 0, count)
	}
	// First connected component (rooted with upload 100)
	if _, count, err := store.SoftDeleteExpiredUploadsViaTraversal(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error soft deleting uploads: %s", err)
	} else if count != 7 {
		t.Fatalf("unexpected number of uploads deleted via traversal: want=%d have=%d", 7, count)
	}
	// Second connected component (rooted with upload 107)
	if _, count, err := store.SoftDeleteExpiredUploadsViaTraversal(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error soft deleting uploads: %s", err)
	} else if count != 0 {
		t.Fatalf("unexpected number of uploads deleted via traversal: want=%d have=%d", 0, count)
	}

	// Ensure records were deleted
	expectedStates := map[int]string{
		100: "deleting",
		101: "deleting",
		102: "deleting",
		103: "deleting",
		104: "deleting",
		105: "deleting",
		106: "deleting",
		107: "completed",
		108: "completed",
	}
	if states, err := getUploadStates(db, 100, 101, 102, 103, 104, 105, 106, 107, 108); err != nil {
		t.Fatalf("unexpected error getting states: %s", err)
	} else if diff := cmp.Diff(expectedStates, states); diff != "" {
		t.Errorf("unexpected upload states (-want +got):\n%s", diff)
	}

	// Ensure repository was marked as dirty
	dirtyRepositories, err := store.GetDirtyRepositories(context.Background())
	if err != nil {
		t.Fatalf("unexpected error listing dirty repositories: %s", err)
	}

	var keys []int
	for _, dirtyRepository := range dirtyRepositories {
		keys = append(keys, dirtyRepository.RepositoryID)
	}
	sort.Ints(keys)

	expectedKeys := []int{50, 51, 52, 53, 54, 55, 56}
	if diff := cmp.Diff(expectedKeys, keys); diff != "" {
		t.Errorf("unexpected dirty repositories (-want +got):\n%s", diff)
	}

	// expire uploads 107-108, making the second connected component soft-deletable
	if err := store.UpdateUploadRetention(context.Background(), []int{}, []int{107, 108}); err != nil {
		t.Fatalf("unexpected error marking uploads as expired: %s", err)
	}
	if _, count, err := store.SoftDeleteExpiredUploads(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error soft deleting uploads: %s", err)
	} else if count != 0 {
		t.Fatalf("unexpected number of uploads deleted via refcount: want=%d have=%d", 0, count)
	}
	if _, count, err := store.SoftDeleteExpiredUploadsViaTraversal(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error soft deleting uploads: %s", err)
	} else if count != 2 {
		t.Fatalf("unexpected number of uploads deleted via traversal: want=%d have=%d", 2, count)
	}

	// Ensure new records were deleted
	expectedStates = map[int]string{
		107: "deleting",
		108: "deleting",
	}
	if states, err := getUploadStates(db, 107, 108); err != nil {
		t.Fatalf("unexpected error getting states: %s", err)
	} else if diff := cmp.Diff(expectedStates, states); diff != "" {
		t.Errorf("unexpected upload states (-want +got):\n%s", diff)
	}
}
