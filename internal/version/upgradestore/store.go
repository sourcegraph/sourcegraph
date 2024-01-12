package upgradestore

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Masterminds/semver"
	"github.com/derision-test/glock"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/hostname"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// store manages checking and updating the version of the instance that was running prior to an ongoing
// instance upgrade or downgrade operation.
type store struct {
	db    *basestore.Store
	clock glock.Clock
}

// New returns a new version store with the given database handle.
func New(db database.DB) *store {
	return NewWith(db.Handle())
}

// NewWith returns a new version store with the given transactable handle.
func NewWith(db basestore.TransactableHandle) *store {
	return newStore(basestore.NewWithHandle(db), glock.NewRealClock())
}

func newStore(db *basestore.Store, clock glock.Clock) *store {
	return &store{
		db:    db,
		clock: clock,
	}
}

// GetFirstServiceVersion returns the first version registered for the given Sourcegraph service. This
// method will return a false-valued flag if UpdateServiceVersion has never been called for the given
// service.
func (s *store) GetFirstServiceVersion(ctx context.Context) (string, bool, error) {
	version, ok, err := basestore.ScanFirstString(s.db.Query(ctx, sqlf.Sprintf(getFirstServiceVersionQuery, "frontend")))
	return version, ok, filterMissingRelationError(err)
}

const getFirstServiceVersionQuery = `
SELECT first_version FROM versions WHERE service = %s
`

// GetServiceVersion returns the previous version registered for the given Sourcegraph service. This
// method will return a false-valued flag if UpdateServiceVersion has never been called for the given
// service.
func (s *store) GetServiceVersion(ctx context.Context) (string, bool, error) {
	version, ok, err := basestore.ScanFirstString(s.db.Query(ctx, sqlf.Sprintf(getServiceVersionQuery, "frontend")))
	return version, ok, filterMissingRelationError(err)
}

const getServiceVersionQuery = `
SELECT version FROM versions WHERE service = %s
`

// ValidateUpgrade enforces our documented upgrade policy and will return an error (performing no side-effects)
// if the upgrade is between two unsupported versions. See https://docs.sourcegraph.com/#upgrading-sourcegraph.
func (s *store) ValidateUpgrade(ctx context.Context, service, version string) error {
	return s.updateServiceVersion(ctx, service, version, false)
}

// UpdateServiceVersion updates the latest version for the given Sourcegraph service. This method also enforces
// our documented upgrade policy and will return an error (performing no side-effects) if the upgrade is between
// two unsupported versions. See https://docs.sourcegraph.com/#upgrading-sourcegraph.
func (s *store) UpdateServiceVersion(ctx context.Context, version string) error {
	return s.updateServiceVersion(ctx, "frontend", version, true)
}

func (s *store) updateServiceVersion(ctx context.Context, service, version string, update bool) error {
	prev, _, err := basestore.ScanFirstString(s.db.Query(ctx, sqlf.Sprintf(updateServiceVersionSelectQuery, service)))
	if err != nil {
		if !update && isMissingRelation(err) {
			// If we are only validating and the relation does not exist, then we are applying the
			// instance upgrade from nothing, which should never be an error (get lost, new users!).
			// If we are also planning to _update_ the version string, return the error eagerly. If
			// we don't, we will no-op an important update from the booted frontend service, which
			// nullfies the point of doing these upgrade checks in the first place.
			return nil
		}

		return err
	}

	latest, _ := semver.NewVersion(version)
	previous, _ := semver.NewVersion(prev)

	if !IsValidUpgrade(previous, latest) {
		return &UpgradeError{Service: service, Previous: previous, Latest: latest}
	}

	if update {
		if err := s.db.Exec(ctx, sqlf.Sprintf(updateServiceVersionSelectUpsertQuery, service, version, time.Now().UTC(), prev)); err != nil {
			return err
		}
	}

	return nil
}

const updateServiceVersionSelectQuery = `
SELECT version FROM versions WHERE service = %s
`

const updateServiceVersionSelectUpsertQuery = `
INSERT INTO versions (service, version, updated_at)
VALUES (%s, %s, %s) ON CONFLICT (service) DO
UPDATE SET (version, updated_at) = (excluded.version, excluded.updated_at)
WHERE versions.version = %s
`

// SetServiceVersion updates the latest version for the given Sourcegraph service. This method also enforces
// our documented upgrade policy and will return an error (performing no side-effects) if the upgrade is between
// two unsupported versions. See https://docs.sourcegraph.com/#upgrading-sourcegraph.
func (s *store) SetServiceVersion(ctx context.Context, version string) error {
	return s.db.Exec(ctx, sqlf.Sprintf(setServiceVersionQuery, version, time.Now().UTC(), "frontend"))
}

const setServiceVersionQuery = `
UPDATE versions SET version = %s, updated_at = %s WHERE versions.service = %s
`

// filterMissingRelationError returns a nil error if the given error was caused by
// the target relation not yet existing. We will need this behavior to be acceptable
// once we begin adding instance version checks in the migrator, which occurs before
// schemas are applied.
func filterMissingRelationError(err error) error {
	if isMissingRelation(err) {
		return nil
	}

	return err
}

func isMissingRelation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == pgerrcode.UndefinedTable
}

// GetAutoUpgrade gets the current value of versions.version and versions.auto_upgrade in the frontend database.
func (s *store) GetAutoUpgrade(ctx context.Context) (version string, enabled bool, err error) {
	if err = s.db.QueryRow(ctx, sqlf.Sprintf(getAutoUpgradeQuery)).Scan(&version, &enabled); err != nil {
		if errors.HasPostgresCode(err, pgerrcode.UndefinedColumn) {
			if err = s.db.QueryRow(ctx, sqlf.Sprintf(getAutoUpgradeFallbackQuery)).Scan(&version); err != nil {
				return "", false, errors.Wrap(err, "failed to get frontend version from fallback")
			}
			return version, enabled, nil
		}
		return "", false, errors.Wrap(err, "failed to get frontend version and auto_upgrade state")
	}
	return version, enabled, nil
}

const getAutoUpgradeQuery = `
SELECT version, auto_upgrade FROM versions WHERE service = 'frontend'
`

const getAutoUpgradeFallbackQuery = `
SELECT version FROM versions WHERE service = 'frontend'
`

// SetAutoUpgrade sets the value of versions.auto_upgrade in the frontend database.
func (s *store) SetAutoUpgrade(ctx context.Context, enable bool) error {
	if err := s.db.Exec(ctx, sqlf.Sprintf(setAutoUpgradeQuery, enable)); err != nil {
		return errors.Wrap(err, "failed to set auto_upgrade")
	}
	return nil
}

const setAutoUpgradeQuery = `
UPDATE versions SET auto_upgrade = %v
`

func (s *store) EnsureUpgradeTable(ctx context.Context) (err error) {
	queries := []*sqlf.Query{
		sqlf.Sprintf(`CREATE TABLE IF NOT EXISTS upgrade_logs(id SERIAL PRIMARY KEY)`),
		sqlf.Sprintf(`ALTER TABLE upgrade_logs ADD COLUMN IF NOT EXISTS started_at timestamptz NOT NULL DEFAULT now()`),
		sqlf.Sprintf(`ALTER TABLE upgrade_logs ADD COLUMN IF NOT EXISTS finished_at timestamptz`),
		sqlf.Sprintf(`ALTER TABLE upgrade_logs ADD COLUMN IF NOT EXISTS success boolean`),
		sqlf.Sprintf(`ALTER TABLE upgrade_logs ADD COLUMN IF NOT EXISTS from_version text NOT NULL`),
		sqlf.Sprintf(`ALTER TABLE upgrade_logs ADD COLUMN IF NOT EXISTS to_version text NOT NULL`),
		sqlf.Sprintf(`ALTER TABLE upgrade_logs ADD COLUMN IF NOT EXISTS upgrader_hostname text NOT NULL`),
		sqlf.Sprintf(`ALTER TABLE upgrade_logs ADD COLUMN IF NOT EXISTS plan json NOT NULL DEFAULT '{}'::json`),
		sqlf.Sprintf(`ALTER TABLE upgrade_logs ADD COLUMN IF NOT EXISTS last_heartbeat_at timestamptz NOT NULL DEFAULT now()`),
	}

	if err := s.db.WithTransact(ctx, func(tx *basestore.Store) error {
		for _, query := range queries {
			if err := tx.Exec(ctx, query); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (s *store) ClaimAutoUpgrade(ctx context.Context, from, to string) (claimed bool, err error) {
	err = s.db.WithTransact(ctx, func(tx *basestore.Store) error {
		// Allow selects to still work (for UI purposes) but serializes claiming.
		// May impact writing logs.
		if err := tx.Exec(ctx, sqlf.Sprintf("LOCK TABLE upgrade_logs IN EXCLUSIVE MODE NOWAIT")); err != nil {
			var pgerr *pgconn.PgError
			if errors.As(err, &pgerr) && pgerr.Code == pgerrcode.LockNotAvailable {
				return nil
			}
			return err
		}

		query := sqlf.Sprintf(claimAutoUpgradeQuery, from, to, hostname.Get(), s.clock.Now(), heartbeatStaleInterval, to)
		if claimed, _, err = basestore.ScanFirstBool(tx.Query(ctx, query)); err != nil {
			return err
		}

		return nil
	})

	return claimed, err
}

const heartbeatStaleInterval = time.Second * 30

const claimAutoUpgradeQuery = `
WITH claim_attempt AS (
	-- claim the upgrade slot, marking the from and to versions, as well as hostname
	INSERT INTO upgrade_logs (from_version, to_version, upgrader_hostname)
	SELECT %s, %s, %s
	-- but only if the latest upgrade log matching these requirements doesn't exist:
	WHERE NOT EXISTS (
		SELECT 1
		FROM upgrade_logs
		-- the latest upgrade attempt
		WHERE id = (
			SELECT MAX(id)
			FROM upgrade_logs
		)
		-- that is currently running
		AND (
			(
				finished_at IS NULL
				AND (
					last_heartbeat_at >= %s::timestamptz - %s::interval
				)
			)
			-- or that succeeded to the expected version
			OR (
				success = true
				AND to_version = %s
			)
		)
	)
	RETURNING true AS claimed
)
SELECT COALESCE((
	SELECT claimed FROM claim_attempt
), false)`

type UpgradePlan struct {
	OutOfBandMigrationIDs []int
	Migrations            map[string][]int
	MigrationNames        map[string]map[int]string
}

// TODO(efritz) - probably want to pass a claim id here as well instead of just hitting the max from upgrade logs
func (s *store) SetUpgradePlan(ctx context.Context, plan UpgradePlan) error {
	serialized, err := json.Marshal(plan)
	if err != nil {
		return err
	}

	return s.db.Exec(ctx, sqlf.Sprintf(setUpgradePlanQuery, serialized))
}

const setUpgradePlanQuery = `
UPDATE upgrade_logs
SET
	plan = %s
WHERE id = (
	SELECT MAX(id) FROM upgrade_logs
)
`

// TODO(efritz) - probably want to pass a claim id here as well instead of just hitting the max from upgrade logs
func (s *store) SetUpgradeStatus(ctx context.Context, success bool) error {
	return s.db.Exec(ctx, sqlf.Sprintf(setUpgradeStatusQuery, time.Now(), success))
}

const setUpgradeStatusQuery = `
UPDATE upgrade_logs
SET
	finished_at = %s,
	success = %s
WHERE id = (
	SELECT MAX(id) FROM upgrade_logs
)
`

func (s *store) Heartbeat(ctx context.Context) error {
	return s.db.Exec(ctx, sqlf.Sprintf(heartbeatQuery, s.clock.Now()))
}

const heartbeatQuery = `
UPDATE upgrade_logs
SET last_heartbeat_at = %s::timestamptz
WHERE id = (
	SELECT MAX(id) FROM upgrade_logs
)
`
