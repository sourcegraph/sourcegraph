package dbstore

import (
	"context"
	"testing"

	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
)

func TestUpdatePackageReferences(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dbtesting.SetupGlobalTestDB(t)
	store := testStore()

	// for foreign key relation
	insertUploads(t, dbconn.Global, Upload{ID: 42})

	if err := store.UpdatePackageReferences(context.Background(), 42, []semantic.PackageReference{
		{Scheme: "s0", Name: "n0", Version: "v0"},
		{Scheme: "s1", Name: "n1", Version: "v1"},
		{Scheme: "s2", Name: "n2", Version: "v2"},
		{Scheme: "s3", Name: "n3", Version: "v3"},
		{Scheme: "s4", Name: "n4", Version: "v4"},
		{Scheme: "s5", Name: "n5", Version: "v5"},
		{Scheme: "s6", Name: "n6", Version: "v6"},
		{Scheme: "s7", Name: "n7", Version: "v7"},
		{Scheme: "s8", Name: "n8", Version: "v8"},
		{Scheme: "s9", Name: "n9", Version: "v9"},
	}); err != nil {
		t.Fatalf("unexpected error updating references: %s", err)
	}

	count, _, err := basestore.ScanFirstInt(dbconn.Global.Query("SELECT COUNT(*) FROM lsif_references"))
	if err != nil {
		t.Fatalf("unexpected error checking reference count: %s", err)
	}
	if count != 10 {
		t.Errorf("unexpected reference count. want=%d have=%d", 10, count)
	}
}

func TestUpdatePackageReferencesEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dbtesting.SetupGlobalTestDB(t)
	store := testStore()

	if err := store.UpdatePackageReferences(context.Background(), 0, nil); err != nil {
		t.Fatalf("unexpected error updating references: %s", err)
	}

	count, _, err := basestore.ScanFirstInt(dbconn.Global.Query("SELECT COUNT(*) FROM lsif_references"))
	if err != nil {
		t.Fatalf("unexpected error checking reference count: %s", err)
	}
	if count != 0 {
		t.Errorf("unexpected reference count. want=%d have=%d", 0, count)
	}
}

func TestUpdatePackageReferencesWithDuplicates(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dbtesting.SetupGlobalTestDB(t)
	store := testStore()

	// for foreign key relation
	insertUploads(t, dbconn.Global, Upload{ID: 42})

	if err := store.UpdatePackageReferences(context.Background(), 42, []semantic.PackageReference{
		{Scheme: "s0", Name: "n0", Version: "v0"},
		{Scheme: "s1", Name: "n1", Version: "v1"},
		{Scheme: "s2", Name: "n2", Version: "v2"},
		{Scheme: "s3", Name: "n3", Version: "v3"},
	}); err != nil {
		t.Fatalf("unexpected error updating references: %s", err)
	}

	if err := store.UpdatePackageReferences(context.Background(), 42, []semantic.PackageReference{
		{Scheme: "s0", Name: "n0", Version: "v0"}, // two copies
		{Scheme: "s2", Name: "n2", Version: "v2"}, // two copies
		{Scheme: "s4", Name: "n4", Version: "v4"},
		{Scheme: "s5", Name: "n5", Version: "v5"},
		{Scheme: "s6", Name: "n6", Version: "v6"},
		{Scheme: "s7", Name: "n7", Version: "v7"},
		{Scheme: "s8", Name: "n8", Version: "v8"},
		{Scheme: "s9", Name: "n9", Version: "v9"},
	}); err != nil {
		t.Fatalf("unexpected error updating references: %s", err)
	}

	count, _, err := basestore.ScanFirstInt(dbconn.Global.Query("SELECT COUNT(*) FROM lsif_references"))
	if err != nil {
		t.Fatalf("unexpected error checking reference count: %s", err)
	}
	if count != 12 {
		t.Errorf("unexpected reference count. want=%d have=%d", 12, count)
	}
}
