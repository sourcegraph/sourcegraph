package oobmigration

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestSynchronizeMetadata(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := NewStoreWithDB(db)

	compareMigrations := func() {
		migrations, err := store.List(ctx)
		if err != nil {
			t.Fatalf("unexpected error getting migrations: %s", err)
		}

		var yamlizedMigrations []yamlMigration
		for _, migration := range migrations {
			yamlMigration := yamlMigration{
				ID:                     migration.ID,
				Team:                   migration.Team,
				Component:              migration.Component,
				Description:            migration.Description,
				NonDestructive:         migration.NonDestructive,
				IsEnterprise:           migration.IsEnterprise,
				IntroducedVersionMajor: migration.Introduced.Major,
				IntroducedVersionMinor: migration.Introduced.Minor,
			}

			if migration.Deprecated != nil {
				yamlMigration.DeprecatedVersionMajor = &migration.Deprecated.Major
				yamlMigration.DeprecatedVersionMinor = &migration.Deprecated.Minor
			}

			yamlizedMigrations = append(yamlizedMigrations, yamlMigration)
		}

		sort.Slice(yamlizedMigrations, func(i, j int) bool {
			return yamlizedMigrations[i].ID < yamlizedMigrations[j].ID
		})

		if diff := cmp.Diff(yamlMigrations, yamlizedMigrations); diff != "" {
			t.Errorf("unexpected migrations (-want +got):\n%s", diff)
		}
	}

	if err := store.SynchronizeMetadata(ctx); err != nil {
		t.Fatalf("unexpected error synchronizing metadata: %s", err)
	}

	compareMigrations()

	if err := store.Exec(ctx, sqlf.Sprintf(`
		UPDATE out_of_band_migrations SET
			team = 'overwritten',
			component = 'overwritten',
			description = 'overwritten',
			non_destructive = false,
			is_enterprise = false,
			introduced_version_major = -1,
			introduced_version_minor = -1,
			deprecated_version_major = -1,
			deprecated_version_minor = -1
	`)); err != nil {
		t.Fatalf("unexpected error updating migrations: %s", err)
	}

	if err := store.SynchronizeMetadata(ctx); err != nil {
		t.Fatalf("unexpected error synchronizing metadata: %s", err)
	}

	compareMigrations()
}

func TestSynchronizeMetadataFallback(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := NewStoreWithDB(db)

	if err := store.Exec(ctx, sqlf.Sprintf(`
		ALTER TABLE out_of_band_migrations
			DROP COLUMN is_enterprise,
			DROP COLUMN introduced_version_major,
			DROP COLUMN introduced_version_minor,
			DROP COLUMN deprecated_version_major,
			DROP COLUMN deprecated_version_minor,
			ADD COLUMN introduced text NOT NULL,
			ADD COLUMN deprecated text
	`)); err != nil {
		t.Fatalf("failed to alter table: %s", err)
	}

	compareMigrations := func() {
		migrations, err := store.List(ctx)
		if err != nil {
			t.Fatalf("unexpected error getting migrations: %s", err)
		}

		var yamlizedMigrations []yamlMigration
		for _, migration := range migrations {
			yamlMigration := yamlMigration{
				ID:                     migration.ID,
				Team:                   migration.Team,
				Component:              migration.Component,
				Description:            migration.Description,
				NonDestructive:         migration.NonDestructive,
				IsEnterprise:           migration.IsEnterprise,
				IntroducedVersionMajor: migration.Introduced.Major,
				IntroducedVersionMinor: migration.Introduced.Minor,
			}

			if migration.Deprecated != nil {
				yamlMigration.DeprecatedVersionMajor = &migration.Deprecated.Major
				yamlMigration.DeprecatedVersionMinor = &migration.Deprecated.Minor
			}

			yamlizedMigrations = append(yamlizedMigrations, yamlMigration)
		}

		sort.Slice(yamlizedMigrations, func(i, j int) bool {
			return yamlizedMigrations[i].ID < yamlizedMigrations[j].ID
		})

		expectedMigrations := make([]yamlMigration, len(yamlMigrations))
		copy(expectedMigrations, yamlMigrations)
		for i := range expectedMigrations {
			expectedMigrations[i].IsEnterprise = true
		}

		if diff := cmp.Diff(expectedMigrations, yamlizedMigrations); diff != "" {
			t.Errorf("unexpected migrations (-want +got):\n%s", diff)
		}
	}

	if err := store.SynchronizeMetadata(ctx); err != nil {
		t.Fatalf("unexpected error synchronizing metadata: %s", err)
	}

	compareMigrations()
}

func TestList(t *testing.T) {
	// Note: package globals block test parallelism
	withMigrationIDs(t, []int{1, 2, 3, 4, 5})

	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := testStore(t, db)

	migrations, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error getting migrations: %s", err)
	}

	expectedMigrations := make([]Migration, len(testMigrations))
	copy(expectedMigrations, testMigrations)
	sort.Slice(expectedMigrations, func(i, j int) bool {
		return expectedMigrations[i].ID > expectedMigrations[j].ID
	})

	if diff := cmp.Diff(expectedMigrations, migrations); diff != "" {
		t.Errorf("unexpected migrations (-want +got):\n%s", diff)
	}
}

func TestGetMultiple(t *testing.T) {
	t.Parallel()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := testStore(t, db)

	migrations, err := store.GetByIDs(context.Background(), []int{1, 2, 3, 4, 5})
	if err != nil {
		t.Fatalf("unexpected error getting multiple migrations: %s", err)
	}

	for i, expectedMigration := range testMigrations {
		if diff := cmp.Diff(expectedMigration, migrations[i]); diff != "" {
			t.Errorf("unexpected migration (-want +got):\n%s", diff)
		}
	}

	_, err = store.GetByIDs(context.Background(), []int{0, 1, 2, 3, 4, 5, 6})
	if err == nil {
		t.Fatalf("unexpected nil error getting multiple migrations")
	}

	if err.Error() != "unknown migration id(s) [0 6]" {
		t.Fatalf("unexpected error, got=%q", err.Error())
	}
}

func TestUpdateDirection(t *testing.T) {
	t.Parallel()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := testStore(t, db)

	if err := store.UpdateDirection(context.Background(), 3, true); err != nil {
		t.Fatalf("unexpected error updating direction: %s", err)
	}

	migration, exists, err := store.GetByID(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error getting migrations: %s", err)
	}
	if !exists {
		t.Fatalf("expected record to exist")
	}

	expectedMigration := testMigrations[2] // ID = 3
	expectedMigration.ApplyReverse = true

	if diff := cmp.Diff(expectedMigration, migration); diff != "" {
		t.Errorf("unexpected migration (-want +got):\n%s", diff)
	}
}

func TestUpdateProgress(t *testing.T) {
	t.Parallel()
	now := testTime.Add(time.Hour * 7)
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := testStore(t, db)

	if err := store.updateProgress(context.Background(), 3, 0.7, now); err != nil {
		t.Fatalf("unexpected error updating migration: %s", err)
	}

	migration, exists, err := store.GetByID(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error getting migrations: %s", err)
	}
	if !exists {
		t.Fatalf("expected record to exist")
	}

	expectedMigration := testMigrations[2] // ID = 3
	expectedMigration.Progress = 0.7
	expectedMigration.LastUpdated = pointers.Ptr(now)

	if diff := cmp.Diff(expectedMigration, migration); diff != "" {
		t.Errorf("unexpected migration (-want +got):\n%s", diff)
	}
}

func TestUpdateMetadata(t *testing.T) {
	t.Parallel()
	now := testTime.Add(time.Hour * 7)
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := testStore(t, db)

	type sampleMeta = struct {
		Message string
	}
	exampleMeta := sampleMeta{Message: "Hello"}
	marshalled, err := json.Marshal(exampleMeta)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.updateMetadata(context.Background(), 3, marshalled, now); err != nil {
		t.Fatalf("unexpected error updating migration: %s", err)
	}

	migration, exists, err := store.GetByID(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error getting migrations: %s", err)
	}
	if !exists {
		t.Fatalf("expected record to exist")
	}

	expectedMigration := testMigrations[2] // ID = 3
	// Formatting can change so we just use the value returned and confirm
	// unmarshalled value is the same lower down
	expectedMigration.Metadata = migration.Metadata
	expectedMigration.LastUpdated = pointers.Ptr(now)

	if diff := cmp.Diff(expectedMigration, migration); diff != "" {
		t.Errorf("unexpected migration (-want +got):\n%s", diff)
	}

	var metaFromDB sampleMeta
	err = json.Unmarshal(migration.Metadata, &metaFromDB)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(exampleMeta, metaFromDB); diff != "" {
		t.Errorf("unexpected metadata (-want +got):\n%s", diff)
	}
}

func TestAddError(t *testing.T) {
	t.Parallel()
	now := testTime.Add(time.Hour * 8)
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := testStore(t, db)

	if err := store.addError(context.Background(), 2, "oops", now); err != nil {
		t.Fatalf("unexpected error updating migration: %s", err)
	}

	migration, exists, err := store.GetByID(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error getting migrations: %s", err)
	}
	if !exists {
		t.Fatalf("expected record to exist")
	}

	expectedMigration := testMigrations[1] // ID = 2
	expectedMigration.LastUpdated = pointers.Ptr(now)
	expectedMigration.Errors = []MigrationError{
		{Message: "oops", Created: now},
		{Message: "uh-oh 1", Created: testTime.Add(time.Hour*5 + time.Second*2)},
		{Message: "uh-oh 2", Created: testTime.Add(time.Hour*5 + time.Second*1)},
	}

	if diff := cmp.Diff(expectedMigration, migration); diff != "" {
		t.Errorf("unexpected migration (-want +got):\n%s", diff)
	}
}

func TestAddErrorBounded(t *testing.T) {
	t.Parallel()

	now := testTime.Add(time.Hour * 9)
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := testStore(t, db)

	var expectedErrors []MigrationError
	for i := range int(MaxMigrationErrors * 1.5) {
		now = now.Add(time.Second)

		if err := store.addError(context.Background(), 2, fmt.Sprintf("oops %d", i), now); err != nil {
			t.Fatalf("unexpected error updating migration: %s", err)
		}

		expectedErrors = append(expectedErrors, MigrationError{
			Message: fmt.Sprintf("oops %d", i),
			Created: now,
		})
	}

	migration, exists, err := store.GetByID(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error getting migration: %s", err)
	}
	if !exists {
		t.Fatalf("expected record to exist")
	}

	n := len(expectedErrors) - 1
	for i := range len(expectedErrors) / 2 {
		expectedErrors[i], expectedErrors[n-i] = expectedErrors[n-i], expectedErrors[i]
	}

	expectedMigration := testMigrations[1] // ID = 2
	expectedMigration.LastUpdated = pointers.Ptr(now)
	expectedMigration.Errors = expectedErrors[:MaxMigrationErrors]

	if diff := cmp.Diff(expectedMigration, migration); diff != "" {
		t.Errorf("unexpected migrations (-want +got):\n%s", diff)
	}
}

//
//

var testTime = time.Unix(1613414740, 0)

var testMigrations = []Migration{
	{
		ID:             1,
		Team:           "search",
		Component:      "zoekt-index",
		Description:    "rot13 all the indexes for security",
		Introduced:     NewVersion(3, 25),
		Deprecated:     nil,
		Progress:       0,
		Created:        testTime,
		LastUpdated:    nil,
		NonDestructive: false,
		IsEnterprise:   false,
		ApplyReverse:   false,
		Metadata:       json.RawMessage(`{}`),
		Errors:         []MigrationError{},
	},
	{
		ID:             2,
		Team:           "codeintel",
		Component:      "lsif_data_documents",
		Description:    "denormalize counts",
		Introduced:     NewVersion(3, 26),
		Deprecated:     newVersionPtr(3, 28),
		Progress:       0.5,
		Created:        testTime.Add(time.Hour * 1),
		LastUpdated:    pointers.Ptr(testTime.Add(time.Hour * 2)),
		NonDestructive: true,
		IsEnterprise:   false,
		ApplyReverse:   false,
		Metadata:       json.RawMessage(`{}`),
		Errors: []MigrationError{
			{Message: "uh-oh 1", Created: testTime.Add(time.Hour*5 + time.Second*2)},
			{Message: "uh-oh 2", Created: testTime.Add(time.Hour*5 + time.Second*1)},
		},
	},
	{
		ID:             3,
		Team:           "platform",
		Component:      "lsif_data_documents",
		Description:    "gzip payloads",
		Introduced:     NewVersion(3, 24),
		Deprecated:     nil,
		Progress:       0.4,
		Created:        testTime.Add(time.Hour * 3),
		LastUpdated:    pointers.Ptr(testTime.Add(time.Hour * 4)),
		NonDestructive: false,
		IsEnterprise:   false,
		ApplyReverse:   true,
		Metadata:       json.RawMessage(`{}`),
		Errors: []MigrationError{
			{Message: "uh-oh 3", Created: testTime.Add(time.Hour*5 + time.Second*4)},
			{Message: "uh-oh 4", Created: testTime.Add(time.Hour*5 + time.Second*3)},
		},
	},
	{
		ID:             4,
		Team:           "search",
		Component:      "zoekt-index",
		Description:    "rot13 all the indexes for security (but with more enterprise)",
		Introduced:     NewVersion(3, 25),
		Deprecated:     nil,
		Progress:       0,
		Created:        testTime,
		LastUpdated:    nil,
		NonDestructive: false,
		ApplyReverse:   false,
		Metadata:       json.RawMessage(`{}`),
		Errors:         []MigrationError{},
	},
	{
		ID:             5,
		Team:           "codeintel",
		Component:      "lsif_data_documents",
		Description:    "denormalize counts (but with more enterprise)",
		Introduced:     NewVersion(3, 26),
		Deprecated:     newVersionPtr(3, 28),
		Progress:       0.5,
		Created:        testTime.Add(time.Hour * 1),
		LastUpdated:    pointers.Ptr(testTime.Add(time.Hour * 2)),
		NonDestructive: true,
		ApplyReverse:   false,
		Metadata:       json.RawMessage(`{}`),
		Errors:         []MigrationError{},
	},
}

func newVersionPtr(major, minor int) *Version {
	return pointers.Ptr(NewVersion(major, minor))
}

func withMigrationIDs(t *testing.T, ids []int) {
	old := yamlMigrationIDs
	yamlMigrationIDs = ids
	t.Cleanup(func() { yamlMigrationIDs = old })
}

func testStore(t *testing.T, db database.DB) *Store {
	store := NewStoreWithDB(db)

	if _, err := db.ExecContext(context.Background(), "DELETE FROM out_of_band_migrations CASCADE"); err != nil {
		t.Fatalf("unexpected error truncating migration: %s", err)
	}

	for i := range testMigrations {
		if err := insertMigration(store, testMigrations[i]); err != nil {
			t.Fatalf("unexpected error inserting migration: %s", err)
		}
	}

	return store
}

func insertMigration(store *Store, migration Migration) error {
	var deprecatedMajor, deprecatedMinor *int
	if migration.Deprecated != nil {
		deprecatedMajor = &migration.Deprecated.Major
		deprecatedMinor = &migration.Deprecated.Minor
	}

	query := sqlf.Sprintf(`
		INSERT INTO out_of_band_migrations (
			id,
			team,
			component,
			description,
			introduced_version_major,
			introduced_version_minor,
			deprecated_version_major,
			deprecated_version_minor,
			progress,
			created,
			last_updated,
			non_destructive,
			apply_reverse
		) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
	`,
		migration.ID,
		migration.Team,
		migration.Component,
		migration.Description,
		migration.Introduced.Major,
		migration.Introduced.Minor,
		deprecatedMajor,
		deprecatedMinor,
		migration.Progress,
		migration.Created,
		migration.LastUpdated,
		migration.NonDestructive,
		migration.ApplyReverse,
	)

	if err := store.Store.Exec(context.Background(), query); err != nil {
		return err
	}

	for _, err := range migration.Errors {
		query := sqlf.Sprintf(`
			INSERT INTO out_of_band_migrations_errors (
				migration_id,
				message,
				created
			) VALUES (%s, %s, %s)
		`,
			migration.ID,
			err.Message,
			err.Created,
		)

		if err := store.Store.Exec(context.Background(), query); err != nil {
			return err
		}
	}

	return nil
}
