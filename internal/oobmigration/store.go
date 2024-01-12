package oobmigration

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"

	"github.com/sourcegraph/sourcegraph/internal/collections"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Migration stores metadata and tracks progress of an out-of-band migration routine.
// These fields mirror the out_of_band_migrations table in the database. For docs see
// the [schema](https://github.com/sourcegraph/sourcegraph/blob/main/internal/database/schema.md#table-publicout_of_band_migrations).
type Migration struct {
	ID             int
	Team           string
	Component      string
	Description    string
	Introduced     Version
	Deprecated     *Version
	Progress       float64
	Created        time.Time
	LastUpdated    *time.Time
	NonDestructive bool
	IsEnterprise   bool
	ApplyReverse   bool
	Errors         []MigrationError
	// Metadata can be used to store custom JSON data
	Metadata json.RawMessage
}

// Complete returns true if the migration has 0 un-migrated record in whichever
// direction is indicated by the ApplyReverse flag.
func (m Migration) Complete() bool {
	if m.Progress == 1 && !m.ApplyReverse {
		return true
	}

	if m.Progress == 0 && m.ApplyReverse {
		return true
	}

	return false
}

// MigrationError pairs an error message and the time the error occurred.
type MigrationError struct {
	Message string
	Created time.Time
}

// scanMigrations scans a slice of migrations from the return value of `*Store.query`.
func scanMigrations(rows *sql.Rows, queryErr error) (_ []Migration, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	var values []Migration
	for rows.Next() {
		var message string
		var created *time.Time
		var deprecatedMajor, deprecatedMinor *int
		value := Migration{Errors: []MigrationError{}}

		if err := rows.Scan(
			&value.ID,
			&value.Team,
			&value.Component,
			&value.Description,
			&value.Introduced.Major,
			&value.Introduced.Minor,
			&deprecatedMajor,
			&deprecatedMinor,
			&value.Progress,
			&value.Created,
			&value.LastUpdated,
			&value.NonDestructive,
			&value.IsEnterprise,
			&value.ApplyReverse,
			&value.Metadata,
			&dbutil.NullString{S: &message},
			&created,
		); err != nil {
			return nil, err
		}

		if message != "" {
			value.Errors = append(value.Errors, MigrationError{
				Message: message,
				Created: *created,
			})
		}

		if deprecatedMajor != nil && deprecatedMinor != nil {
			value.Deprecated = &Version{
				Major: *deprecatedMajor,
				Minor: *deprecatedMinor,
			}
		}

		if n := len(values); n > 0 && values[n-1].ID == value.ID {
			values[n-1].Errors = append(values[n-1].Errors, value.Errors...)
		} else {
			values = append(values, value)
		}
	}

	return values, nil
}

// Store is the interface over the out-of-band migrations tables.
type Store struct {
	*basestore.Store
}

// NewStoreWithDB creates a new Store with the given database connection.
func NewStoreWithDB(db database.DB) *Store {
	return &Store{Store: basestore.NewWithHandle(db.Handle())}
}

var _ basestore.ShareableStore = &Store{}

// With creates a new store with the underlying database handle from the given store.
// This method should be used when two distinct store instances need to perform an
// operation within the same shared transaction.
//
// This method wraps the basestore.With method.
func (s *Store) With(other basestore.ShareableStore) *Store {
	return &Store{Store: s.Store.With(other)}
}

// Transact returns a new store whose methods operate within the context of a new transaction
// or a new savepoint. This method will return an error if the underlying connection cannot be
// interface upgraded to a TxBeginner.
//
// This method wraps the basestore.Transact method.
func (s *Store) Transact(ctx context.Context) (*Store, error) {
	txBase, err := s.Store.Transact(ctx)
	return &Store{Store: txBase}, err
}

type yamlMigration struct {
	ID                     int    `yaml:"id"`
	Team                   string `yaml:"team"`
	Component              string `yaml:"component"`
	Description            string `yaml:"description"`
	NonDestructive         bool   `yaml:"non_destructive"`
	IsEnterprise           bool   `yaml:"is_enterprise"`
	IntroducedVersionMajor int    `yaml:"introduced_version_major"`
	IntroducedVersionMinor int    `yaml:"introduced_version_minor"`
	DeprecatedVersionMajor *int   `yaml:"deprecated_version_major"`
	DeprecatedVersionMinor *int   `yaml:"deprecated_version_minor"`
}

//go:embed oobmigrations.yaml
var migrations embed.FS

var yamlMigrations = func() []yamlMigration {
	contents, err := migrations.ReadFile("oobmigrations.yaml")
	if err != nil {
		panic(fmt.Sprintf("malformed oobmigration definitions: %s", err.Error()))
	}

	var parsedMigrations []yamlMigration
	if err := yaml.Unmarshal(contents, &parsedMigrations); err != nil {
		panic(fmt.Sprintf("malformed oobmigration definitions: %s", err.Error()))
	}

	sort.Slice(parsedMigrations, func(i, j int) bool {
		return parsedMigrations[i].ID < parsedMigrations[j].ID
	})

	return parsedMigrations
}()

var yamlMigrationIDs = func() []int {
	ids := make([]int, 0, len(yamlMigrations))
	for _, migration := range yamlMigrations {
		ids = append(ids, migration.ID)
	}

	return ids
}()

// SynchronizeMetadata upserts the metadata defined in the sibling file oobmigrations.yaml.
// Existing out-of-band migration metadata that does not match one of the identifiers in the
// referenced file are not removed, as they have likely been registered by a later version of
// the instance prior to a downgrade.
//
// This method will use a fallback query to support an older version of the table (prior to 3.29)
// so that upgrades of historic instances work with the migrator. This is true of select methods
// in this store, but not all methods.
func (s *Store) SynchronizeMetadata(ctx context.Context) (err error) {
	var fallback bool

	tx, err := s.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if !fallback {
			err = tx.Done(err)
		}
	}()

	for _, migration := range yamlMigrations {
		if err := tx.Exec(ctx, sqlf.Sprintf(
			synchronizeMetadataUpsertQuery,
			migration.ID,
			migration.Team,
			migration.Component,
			migration.Description,
			migration.NonDestructive,
			migration.IsEnterprise,
			migration.IntroducedVersionMajor,
			migration.IntroducedVersionMinor,
			migration.DeprecatedVersionMajor,
			migration.DeprecatedVersionMinor,
			migration.Team,
			migration.Component,
			migration.Description,
			migration.NonDestructive,
			migration.IsEnterprise,
			migration.IntroducedVersionMajor,
			migration.IntroducedVersionMinor,
			migration.DeprecatedVersionMajor,
			migration.DeprecatedVersionMinor,
		)); err != nil {
			if !shouldFallback(err) {
				return err
			}

			fallback = true
			_ = tx.Done(err)
			return s.synchronizeMetadataFallback(ctx)
		}
	}

	return nil
}

const synchronizeMetadataUpsertQuery = `
INSERT INTO out_of_band_migrations
(
	id,
	team,
	component,
	description,
	created,
	non_destructive,
	is_enterprise,
	introduced_version_major,
	introduced_version_minor,
	deprecated_version_major,
	deprecated_version_minor
)
VALUES (%s, %s, %s, %s, NOW(), %s, %s, %s, %s, %s, %s)
ON CONFLICT (id) DO UPDATE SET
	team = %s,
	component = %s,
	description = %s,
	non_destructive = %s,
	is_enterprise = %s,
	introduced_version_major = %s,
	introduced_version_minor = %s,
	deprecated_version_major = %s,
	deprecated_version_minor = %s
`

func (s *Store) synchronizeMetadataFallback(ctx context.Context) (err error) {
	tx, err := s.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	for _, migration := range yamlMigrations {
		introduced := versionString(migration.IntroducedVersionMajor, migration.IntroducedVersionMinor)
		var deprecated *string
		if migration.DeprecatedVersionMajor != nil {
			s := versionString(*migration.DeprecatedVersionMajor, *migration.DeprecatedVersionMinor)
			deprecated = &s
		}

		if err := tx.Exec(ctx, sqlf.Sprintf(
			synchronizeMetadataFallbackUpsertQuery,
			migration.ID,
			migration.Team,
			migration.Component,
			migration.Description,
			migration.NonDestructive,
			introduced,
			deprecated,
			migration.Team,
			migration.Component,
			migration.Description,
			migration.NonDestructive,
			introduced,
			deprecated,
		)); err != nil {
			return err
		}
	}

	return nil
}

const synchronizeMetadataFallbackUpsertQuery = `
INSERT INTO out_of_band_migrations
(
	id,
	team,
	component,
	description,
	created,
	non_destructive,
	introduced,
	deprecated
)
VALUES (%s, %s, %s, %s, NOW(), %s, %s, %s)
ON CONFLICT (id) DO UPDATE SET
	team = %s,
	component = %s,
	description = %s,
	non_destructive = %s,
	introduced = %s,
	deprecated = %s
`

// GetByID retrieves a migration by its identifier. If the migration does not exist, a false
// valued flag is returned.
func (s *Store) GetByID(ctx context.Context, id int) (_ Migration, _ bool, err error) {
	migrations, err := scanMigrations(s.Store.Query(ctx, sqlf.Sprintf(getByIDQuery, id)))
	if err != nil {
		return Migration{}, false, err
	}

	if len(migrations) == 0 {
		return Migration{}, false, nil
	}

	return migrations[0], true, nil
}

const getByIDQuery = `
SELECT
	m.id,
	m.team,
	m.component,
	m.description,
	m.introduced_version_major,
	m.introduced_version_minor,
	m.deprecated_version_major,
	m.deprecated_version_minor,
	m.progress,
	m.created,
	m.last_updated,
	m.non_destructive,
	m.is_enterprise,
	m.apply_reverse,
	m.metadata,
	e.message,
	e.created
FROM out_of_band_migrations m
LEFT JOIN out_of_band_migrations_errors e ON e.migration_id = m.id
WHERE m.id = %s
ORDER BY e.created desc
`

func (s *Store) GetByIDs(ctx context.Context, ids []int) (_ []Migration, err error) {
	migrations, err := scanMigrations(s.Store.Query(ctx, sqlf.Sprintf(getByIDsQuery, pq.Array(ids))))
	if err != nil {
		return nil, err
	}

	wanted := collections.NewSet(ids...)
	received := collections.NewSet[int]()
	for _, migration := range migrations {
		received.Add(migration.ID)
	}
	difference := wanted.Difference(received).Values()
	if len(difference) > 0 {
		slices.Sort(difference)
		return nil, errors.Newf("unknown migration id(s) %v", difference)
	}

	return migrations, nil
}

const getByIDsQuery = `
SELECT
	m.id,
	m.team,
	m.component,
	m.description,
	m.introduced_version_major,
	m.introduced_version_minor,
	m.deprecated_version_major,
	m.deprecated_version_minor,
	m.progress,
	m.created,
	m.last_updated,
	m.non_destructive,
	m.is_enterprise,
	m.apply_reverse,
	m.metadata,
	e.message,
	e.created
FROM out_of_band_migrations m
LEFT JOIN out_of_band_migrations_errors e ON e.migration_id = m.id
WHERE m.id = ANY(%s)
ORDER BY m.id ASC, e.created DESC
`

// List returns the complete list of out-of-band migrations.
//
// This method will use a fallback query to support an older version of the table (prior to 3.29)
// so that upgrades of historic instances work with the migrator. This is true of select methods
// in this store, but not all methods.
func (s *Store) List(ctx context.Context) (_ []Migration, err error) {
	conds := []*sqlf.Query{
		// Syncing metadata does not remove unknown migration fields. If we've removed them,
		// we want to block them from returning from old instances. We also want to ignore
		// any database content that we don't have metadata for. Similar checks should not
		// be necessary on the other access methods, as they use ids returned by this method.
		sqlf.Sprintf("m.id = ANY(%s)", pq.Array(yamlMigrationIDs)),
	}

	migrations, err := scanMigrations(s.Store.Query(ctx, sqlf.Sprintf(listQuery, sqlf.Join(conds, "AND"))))
	if err != nil {
		if !shouldFallback(err) {
			return nil, err
		}

		return scanMigrations(s.Store.Query(ctx, sqlf.Sprintf(listFallbackQuery, sqlf.Join(conds, "AND"))))
	}

	return migrations, nil
}

const listQuery = `
SELECT
	m.id,
	m.team,
	m.component,
	m.description,
	m.introduced_version_major,
	m.introduced_version_minor,
	m.deprecated_version_major,
	m.deprecated_version_minor,
	m.progress,
	m.created,
	m.last_updated,
	m.non_destructive,
	m.is_enterprise,
	m.apply_reverse,
	m.metadata,
	e.message,
	e.created
FROM out_of_band_migrations m
LEFT JOIN out_of_band_migrations_errors e ON e.migration_id = m.id
WHERE %s
ORDER BY m.id desc, e.created DESC
`

const listFallbackQuery = `
WITH split_migrations AS (
	SELECT
		m.*,
		regexp_matches(m.introduced, E'^(\\d+)\.(\\d+)') AS introduced_parts,
		regexp_matches(m.deprecated, E'^(\\d+)\.(\\d+)') AS deprecated_parts
	FROM out_of_band_migrations m
)
SELECT
	m.id,
	m.team,
	m.component,
	m.description,
	introduced_parts[1] AS introduced_version_major,
	introduced_parts[2] AS introduced_version_minor,
	CASE WHEN m.deprecated = '' THEN NULL ELSE deprecated_parts[1] END AS deprecated_version_major,
	CASE WHEN m.deprecated = '' THEN NULL ELSE deprecated_parts[2] END AS deprecated_version_minor,
	m.progress,
	m.created,
	m.last_updated,
	m.non_destructive,
	-- Note that we use true here as a default as we only expect to require this fallback
	-- query when using a newer migrator version against an old instance, and multi-version
	-- upgrades are an enterprise feature.
	true AS is_enterprise,
	m.apply_reverse,
	m.metadata,
	e.message,
	e.created
FROM split_migrations m
LEFT JOIN out_of_band_migrations_errors e ON e.migration_id = m.id
WHERE %s
ORDER BY m.id desc, e.created DESC
`

// UpdateDirection updates the direction for the given migration.
func (s *Store) UpdateDirection(ctx context.Context, id int, applyReverse bool) error {
	return s.Store.Exec(ctx, sqlf.Sprintf(updateDirectionQuery, applyReverse, id))
}

const updateDirectionQuery = `
UPDATE out_of_band_migrations SET apply_reverse = %s WHERE id = %s
`

// UpdateProgress updates the progress for the given migration.
func (s *Store) UpdateProgress(ctx context.Context, id int, progress float64) error {
	return s.updateProgress(ctx, id, progress, time.Now())
}

func (s *Store) updateProgress(ctx context.Context, id int, progress float64, now time.Time) error {
	return s.Store.Exec(ctx, sqlf.Sprintf(updateProgressQuery, progress, now, id, progress))
}

const updateProgressQuery = `
UPDATE out_of_band_migrations SET progress = %s, last_updated = %s WHERE id = %s AND progress != %s
`

// UpdateMetadata updates the metadata for the given migration.
func (s *Store) UpdateMetadata(ctx context.Context, id int, meta json.RawMessage) error {
	return s.updateMetadata(ctx, id, meta, time.Now())
}

func (s *Store) updateMetadata(ctx context.Context, id int, meta json.RawMessage, now time.Time) error {
	return s.Store.Exec(ctx, sqlf.Sprintf(updateMetadataQuery, meta, now, id, meta))
}

const updateMetadataQuery = `
UPDATE out_of_band_migrations SET metadata = %s, last_updated = %s WHERE id = %s AND metadata != %s
`

// MaxMigrationErrors is the maximum number of errors we'll track for a single migration before
// pruning older entries.
const MaxMigrationErrors = 100

// AddError associates the given error message with the given migration. While there are more
// than MaxMigrationErrors errors for this, the oldest error entries will be pruned to keep the
// error list relevant and short.
func (s *Store) AddError(ctx context.Context, id int, message string) (err error) {
	return s.addError(ctx, id, message, time.Now())
}

func (s *Store) addError(ctx context.Context, id int, message string, now time.Time) (err error) {
	tx, err := s.Store.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	if err := tx.Exec(ctx, sqlf.Sprintf(addErrorQuery, id, message, now)); err != nil {
		return err
	}

	if err := tx.Exec(ctx, sqlf.Sprintf(addErrorUpdateTimeQuery, now, id)); err != nil {
		return err
	}

	if err := tx.Exec(ctx, sqlf.Sprintf(addErrorPruneQuery, id, MaxMigrationErrors)); err != nil {
		return err
	}

	return nil
}

const addErrorQuery = `
INSERT INTO out_of_band_migrations_errors (migration_id, message, created) VALUES (%s, %s, %s)
`

const addErrorUpdateTimeQuery = `
UPDATE out_of_band_migrations SET last_updated = %s where id = %s
`

const addErrorPruneQuery = `
DELETE FROM out_of_band_migrations_errors WHERE id IN (
	SELECT id FROM out_of_band_migrations_errors WHERE migration_id = %s ORDER BY created DESC OFFSET %s
)
`

var columnsSupporingFallback = []string{
	"is_enterprise",
	"introduced_version_major",
	"introduced_version_minor",
	"deprecated_version_major",
	"deprecated_version_minor",
}

func shouldFallback(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42703" {
		for _, column := range columnsSupporingFallback {
			if strings.Contains(pgErr.Message, column) {
				return true
			}
		}
	}

	return false
}

func versionString(major, minor int) string {
	return fmt.Sprintf("%d.%d.0", major, minor)
}
