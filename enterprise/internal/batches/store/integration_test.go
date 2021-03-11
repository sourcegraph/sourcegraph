package store

import (
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
)

func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	db := dbtesting.GetDB(t)

	t.Run("Store", func(t *testing.T) {
		t.Run("BatchChanges", storeTest(db, testStoreBatchChanges))
		t.Run("Changesets", storeTest(db, testStoreChangesets))
		t.Run("ChangesetEvents", storeTest(db, testStoreChangesetEvents))
		t.Run("ListChangesetSyncData", storeTest(db, testStoreListChangesetSyncData))
		t.Run("ListChangesetsTextSearch", storeTest(db, testStoreListChangesetsTextSearch))
		t.Run("BatchSpecs", storeTest(db, testStoreBatchSpecs))
		t.Run("ChangesetSpecs", storeTest(db, testStoreChangesetSpecs))
		t.Run("ChangesetSpecsCurrentState", storeTest(db, testStoreChangesetSpecsCurrentState))
		t.Run("ChangesetSpecsCurrentStateAndTextSearch", storeTest(db, testStoreChangesetSpecsCurrentStateAndTextSearch))
		t.Run("ChangesetSpecsTextSearch", storeTest(db, testStoreChangesetSpecsTextSearch))
		t.Run("CodeHosts", storeTest(db, testStoreCodeHost))
		t.Run("UserDeleteCascades", storeTest(db, testUserDeleteCascades))
	})
}
