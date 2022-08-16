package insights

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func NewMigrator(insightsDB, postgresDB database.DB) oobmigration.Migrator {
	return &migrator{
		frontendStore: basestore.NewWithHandle(postgresDB.Handle()),
		insightsStore: basestore.NewWithHandle(insightsDB.Handle()),
	}
}

func (m *migrator) Progress(ctx context.Context) (float64, error) {
	progress, _, err := basestore.ScanFirstFloat(m.frontendStore.Query(ctx, sqlf.Sprintf(`
		SELECT CASE c2.count
				   WHEN 0 THEN 1
				   ELSE
					   CAST(c1.count AS FLOAT) / CAST(c2.count AS FLOAT) END
		FROM (SELECT COUNT(*) AS count FROM insights_settings_migration_jobs WHERE completed_at IS NOT NULL) c1,
			 (SELECT COUNT(*) AS count FROM insights_settings_migration_jobs) c2;
	`)))
	return progress, err
}

func (m *migrator) Up(ctx context.Context) (err error) {
	globalMigrationComplete, err := m.performBatchMigration(ctx, GlobalJob)
	if err != nil {
		return err
	}
	if !globalMigrationComplete {
		return nil
	}

	orgMigrationComplete, err := m.performBatchMigration(ctx, OrgJob)
	if err != nil {
		return err
	}
	if !orgMigrationComplete {
		return nil
	}

	userMigrationComplete, err := m.performBatchMigration(ctx, UserJob)
	if err != nil {
		return err
	}
	if !userMigrationComplete {
		return nil
	}

	return nil
}

func (m *migrator) Down(ctx context.Context) (err error) {
	return nil
}

var scanJobs = basestore.NewSliceScanner(func(s dbutil.Scanner) (j settingsMigrationJob, _ error) {
	err := s.Scan(&j.UserId, &j.OrgId, &j.Global, &j.MigratedInsights, &j.MigratedDashboards, &j.Runs)
	return j, err
})

var scanOrgs = basestore.NewSliceScanner(func(s dbutil.Scanner) (org organization, _ error) {
	err := s.Scan(&org.ID, &org.Name, &org.DisplayName, &org.CreatedAt, &org.UpdatedAt)
	return org, err
})

var scanUsers = basestore.NewSliceScanner(func(s dbutil.Scanner) (u user, _ error) {
	var displayName, avatarURL sql.NullString
	err := s.Scan(&u.ID, &u.Username, &displayName, &avatarURL, &u.CreatedAt, &u.UpdatedAt, &u.SiteAdmin, &u.BuiltinAuth, pq.Array(&u.Tags), &u.InvalidatedSessionsAt, &u.TosAccepted, &u.Searchable)
	u.DisplayName = displayName.String
	u.AvatarURL = avatarURL.String
	return u, err
})

var scanSettings = basestore.NewSliceScanner(func(scanner dbutil.Scanner) (s settings, _ error) {
	err := scanner.Scan(&s.ID, &s.Subject.Org, &s.Subject.User, &s.AuthorUserID, &s.Contents, &s.CreatedAt)
	if s.Subject.Org == nil && s.Subject.User == nil {
		s.Subject.Site = true
	}
	return s, err
})

var scanSeries = basestore.NewSliceScanner(func(scanner dbutil.Scanner) (s insightSeries, _ error) {
	err := scanner.Scan(
		&s.ID,
		&s.SeriesID,
		&s.Query,
		&s.CreatedAt,
		&s.OldestHistoricalAt,
		&s.LastRecordedAt,
		&s.NextRecordingAfter,
		&s.LastSnapshotAt,
		&s.NextSnapshotAfter,
		&s.Enabled,
		&s.SampleIntervalUnit,
		&s.SampleIntervalValue,
		&s.GeneratedFromCaptureGroups,
		&s.JustInTime,
		&s.GenerationMethod,
		pq.Array(&s.Repositories),
		&s.GroupBy,
		&s.BackfillAttempts,
	)
	return s, err
})

func (m *migrator) performBatchMigration(ctx context.Context, jobType settingsMigrationJobType) (bool, error) {
	// This transaction will allow us to lock the jobs rows while working on them.
	tx, err := m.frontendStore.Transact(ctx)
	if err != nil {
		return false, err
	}
	defer func() {
		err = tx.Done(err)
	}()

	var cond *sqlf.Query
	switch jobType {
	case "USER":
		cond = sqlf.Sprintf("user_id IS NOT NULL")
	case "ORG":
		cond = sqlf.Sprintf("org_id IS NOT NULL")
	default:
		cond = sqlf.Sprintf("global IS TRUE")
	}
	count, _, err := basestore.ScanFirstInt(tx.Query(ctx, sqlf.Sprintf(`
		SELECT COUNT(*) FROM insights_settings_migration_jobs WHERE %s AND completed_at IS NULL
	`, cond)))
	if err != nil {
		return false, err
	}
	if count == 0 {
		return true, nil
	}

	jobs, err := scanJobs(tx.Query(ctx, sqlf.Sprintf(`
	SELECT
		user_id,
		org_id,
		(CASE WHEN global IS NULL THEN FALSE ELSE TRUE END) AS global,
		migrated_insights,
		migrated_dashboards,
		runs
	FROM insights_settings_migration_jobs
	WHERE
		%s AND
		completed_at IS NULL
	LIMIT 100
	FOR UPDATE SKIP LOCKED
	`, cond)))
	if err != nil {
		return false, err
	}
	if len(jobs) == 0 {
		return false, nil
	}

	var errs error
	for _, job := range jobs {
		err := m.performMigrationForRow(ctx, tx, job)
		if err != nil {
			errs = errors.Append(errs, err)
		}
	}

	// We'll rely on the next thread to return true right away, if everything has completed.
	return false, errs
}

func (m *migrator) performMigrationForRow(ctx context.Context, tx *basestore.Store, job settingsMigrationJob) error {
	var subject settingsSubject
	var migrationContext migrationContext
	var subjectName string

	defer func() {
		var cond *sqlf.Query
		if job.UserId != nil {
			cond = sqlf.Sprintf("user_id = %s", *job.UserId)
		} else if job.OrgId != nil {
			cond = sqlf.Sprintf("org_id = %s", *job.OrgId)
		} else {
			cond = sqlf.Sprintf("global IS TRUE")
		}
		tx.Exec(ctx, sqlf.Sprintf(`UPDATE insights_settings_migration_jobs SET runs = %s WHERE %s`, job.Runs+1, cond))
	}()

	if job.UserId != nil {
		userId := int32(*job.UserId)
		subject = settingsSubject{User: &userId}

		// when this is a user setting we need to load all of the organizations the user is a member of so that we can
		// resolve insight ID collisions as if it were in a setting cascade
		orgs, err := scanOrgs(tx.Query(ctx, sqlf.Sprintf(`
			SELECT
				orgs.id,
				orgs.name,
				orgs.display_name,
				orgs.created_at,
				orgs.updated_at
			FROM org_members
			LEFT OUTER JOIN orgs ON org_members.org_id = orgs.id
			WHERE
				user_id = %s AND
				orgs.deleted_at IS NULL
		`,
			userId,
		)))
		if err != nil {
			return err
		}
		orgIds := make([]int, 0, len(orgs))
		for _, org := range orgs {
			orgIds = append(orgIds, int(org.ID))
		}
		migrationContext.userId = int(userId)
		migrationContext.orgIds = orgIds

		users, err := scanUsers(tx.Query(ctx, sqlf.Sprintf(`
			SELECT
				u.id,
				u.username,
				u.display_name,
				u.avatar_url,
				u.created_at,
				u.updated_at,
				u.site_admin,
				u.passwd IS NOT NULL,
				u.tags,
				u.invalidated_sessions_at,
				u.tos_accepted,
				u.searchable
			FROM users u
			WHERE
				id = %s AND
				deleted_at IS NULL
			LIMIT 1
		`,
			userId,
		)))
		if err != nil {
			return errors.Wrap(err, "UserStoreGetByID")
		}
		if len(users) == 0 {
			// If the user doesn't exist, just mark the job complete.
			err = tx.Exec(ctx, sqlf.Sprintf(`UPDATE insights_settings_migration_jobs SET completed_at = NOW() WHERE user_id = %s`, userId))
			if err != nil {
				return errors.Wrap(err, "MarkCompleted")
			}
			return nil
		}
		user := users[0]
		subjectName = replaceIfEmpty(&user.DisplayName, user.Username)
	} else if job.OrgId != nil {
		orgId := int32(*job.OrgId)
		subject = settingsSubject{Org: &orgId}
		migrationContext.orgIds = []int{*job.OrgId}
		orgs, err := scanOrgs(tx.Query(ctx, sqlf.Sprintf(`
			SELECT
				id,
				name,
				display_name,
				created_at,
				updated_at
			FROM orgs
			WHERE
				deleted_at IS NULL AND
				id = %s
			LIMIT 1
		`, orgId,
		)))
		if err != nil {
			return errors.Wrap(err, "OrgStoreGetByID")
		}
		if len(orgs) == 0 {
			// If the org doesn't exist, just mark the job complete.
			err = tx.Exec(ctx, sqlf.Sprintf(`UPDATE insights_settings_migration_jobs SET completed_at = NOW() WHERE org_id = %s`, orgId))
			if err != nil {
				return errors.Wrap(err, "MarkCompleted")
			}
			return nil
		}
		org := orgs[0]
		subjectName = replaceIfEmpty(org.DisplayName, org.Name)
	} else {
		subject = settingsSubject{Site: true}
		// nothing to set for migration context, it will infer global based on the lack of user / orgs
		subjectName = "Global"
	}

	var cond *sqlf.Query
	switch {
	case subject.Org != nil:
		cond = sqlf.Sprintf("org_id = %d", *subject.Org)
	case subject.User != nil:
		cond = sqlf.Sprintf("user_id = %d AND EXISTS (SELECT NULL FROM users WHERE id=%d AND deleted_at IS NULL)", *subject.User, *subject.User)
	default:
		// No org and no user represents global site settings.
		cond = sqlf.Sprintf("user_id IS NULL AND org_id IS NULL")
	}
	settings, err := scanSettings(tx.Query(ctx, sqlf.Sprintf(`
		SELECT
			s.id,
			s.org_id,
			s.user_id,
			CASE WHEN users.deleted_at IS NULL THEN s.author_user_id ELSE NULL END,
			s.contents,
			s.created_at
		FROM settings s
		LEFT JOIN users ON users.id = s.author_user_id
		WHERE %s
		ORDER BY id DESC LIMIT 1
		`,
		cond,
	)))
	if err != nil {
		return err
	}
	if len(settings) == 0 {
		// If this settings object no longer exists, skip it.
		return nil
	}

	langStatsInsights := getLangStatsInsights(settings[0])
	frontendInsights := getFrontendInsights(settings[0])
	backendInsights := getBackendInsights(settings[0])

	// here we are constructing a total set of all of the insights defined in this specific settings block. This will help guide us
	// to understand which insights are created here, versus which are referenced from elsewhere. This will be useful for example
	// to reconstruct the special case user / org / global dashboard
	allDefinedInsightIds := make([]string, 0, len(langStatsInsights)+len(frontendInsights)+len(backendInsights))
	for _, insight := range langStatsInsights {
		allDefinedInsightIds = append(allDefinedInsightIds, insight.ID)
	}
	for _, insight := range frontendInsights {
		allDefinedInsightIds = append(allDefinedInsightIds, insight.ID)
	}
	for _, insight := range backendInsights {
		allDefinedInsightIds = append(allDefinedInsightIds, insight.ID)
	}
	logDuplicates(allDefinedInsightIds)

	if job.UserId != nil {
		cond = sqlf.Sprintf("user_id = %s", *job.UserId)
	} else if job.OrgId != nil {
		cond = sqlf.Sprintf("org_id = %s", *job.OrgId)
	} else {
		cond = sqlf.Sprintf("global IS TRUE")
	}

	totalInsights := len(langStatsInsights) + len(frontendInsights) + len(backendInsights)
	var migratedInsightsCount int
	var insightMigrationErrors error
	if totalInsights != job.MigratedInsights {
		err = tx.Exec(ctx, sqlf.Sprintf(`UPDATE insights_settings_migration_jobs SET total_insights = %s WHERE %s`, totalInsights, cond))
		if err != nil {
			return err
		}

		count, err := m.migrateLangStatsInsights(ctx, langStatsInsights)
		insightMigrationErrors = errors.Append(insightMigrationErrors, err)
		migratedInsightsCount += count

		count, err = m.migrateInsights(ctx, frontendInsights, frontend)
		insightMigrationErrors = errors.Append(insightMigrationErrors, err)
		migratedInsightsCount += count

		count, err = m.migrateInsights(ctx, backendInsights, backend)
		insightMigrationErrors = errors.Append(insightMigrationErrors, err)
		migratedInsightsCount += count

		err = tx.Exec(ctx, sqlf.Sprintf(`UPDATE insights_settings_migration_jobs SET migrated_insights = %s WHERE %s`, migratedInsightsCount, cond))
		if err != nil {
			return errors.Append(insightMigrationErrors, err)
		}
		if totalInsights != migratedInsightsCount {
			return insightMigrationErrors
		}
	}

	dashboards := getDashboards(settings[0])
	totalDashboards := len(dashboards)
	if totalDashboards != job.MigratedDashboards {
		err = tx.Exec(ctx, sqlf.Sprintf(`UPDATE insights_settings_migration_jobs SET total_dashboards = %s WHERE %s`, totalDashboards, cond))
		if err != nil {
			return err
		}
		migratedDashboardsCount, dashboardMigrationErrors := m.migrateDashboards(ctx, dashboards, migrationContext)
		err = tx.Exec(ctx, sqlf.Sprintf(`UPDATE insights_settings_migration_jobs SET migrated_dashboards = %s WHERE %s`, migratedDashboardsCount, cond))
		if err != nil {
			return err
		}
		if totalDashboards != migratedDashboardsCount {
			return dashboardMigrationErrors
		}
	}
	err = m.createSpecialCaseDashboard(ctx, subjectName, allDefinedInsightIds, migrationContext)
	if err != nil {
		return err
	}

	if job.UserId != nil {
		cond = sqlf.Sprintf("user_id = %s", *job.UserId)
	} else if job.OrgId != nil {
		cond = sqlf.Sprintf("org_id = %s", *job.OrgId)
	} else {
		cond = sqlf.Sprintf("global IS TRUE")
	}
	err = tx.Exec(ctx, sqlf.Sprintf(`UPDATE insights_settings_migration_jobs SET completed_at = NOW() WHERE %s`, cond))
	if err != nil {
		return errors.Wrap(err, "MarkCompleted")
	}

	return nil
}

func (m *migrator) createSpecialCaseDashboard(ctx context.Context, subjectName string, insightReferences []string, migration migrationContext) error {
	tx, err := m.insightsStore.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	err = m.createDashboard(ctx, tx, specialCaseDashboardTitle(subjectName), insightReferences, migration)
	if err != nil {
		return errors.Wrap(err, "CreateSpecialCaseDashboard")
	}
	return nil
}

func (m *migrator) createDashboard(ctx context.Context, tx *basestore.Store, title string, insightReferences []string, migration migrationContext) (err error) {
	var mapped []string

	for _, reference := range insightReferences {
		id, _, err := m.lookupUniqueId(ctx, migration, reference)
		if err != nil {
			return err
		}
		mapped = append(mapped, id)
	}

	var grants []dashboardGrant
	if migration.userId != 0 {
		grants = append(grants, userDashboardGrant(migration.userId))
	} else if len(migration.orgIds) == 1 {
		grants = append(grants, orgDashboardGrant(migration.orgIds[0]))
	} else {
		grants = append(grants, globalDashboardGrant())
	}

	dashboardId, _, err := basestore.ScanFirstInt(tx.Query(ctx, sqlf.Sprintf(`
		INSERT INTO dashboard (title, save, type)
		VALUES (%s, %s, %s)
		RETURNING id
	`,
		title,
		true,
		standard,
	)))
	if len(mapped) > 0 {
		// Create rows for an inline table which is used to preserve the ordering of the viewIds.
		orderings := make([]*sqlf.Query, 0, 1)
		for i, viewId := range mapped {
			orderings = append(orderings, sqlf.Sprintf("(%s, %s)", viewId, fmt.Sprintf("%d", i)))
		}

		err = tx.Exec(ctx, sqlf.Sprintf(`
			INSERT INTO dashboard_insight_view (dashboard_id, insight_view_id)
			SELECT %s AS dashboard_id, insight_view.id AS insight_view_id
			FROM insight_view
			JOIN (VALUES %s) as ids (id, ordering) ON ids.id = insight_view.unique_id
			WHERE unique_id = ANY(%s)
			ORDER BY ids.ordering
			ON CONFLICT DO NOTHING;
		`,
			dashboardId,
			sqlf.Join(orderings, ","),
			pq.Array(mapped),
		))
		if err != nil {
			return errors.Wrap(err, "AddViewsToDashboard")
		}
	}

	values := make([]*sqlf.Query, 0, len(grants))
	for _, grant := range grants {
		values = append(values, sqlf.Sprintf("(%s, %s, %s, %s)", dashboardId, grant.UserID, grant.OrgID, grant.Global))
	}
	err = tx.Exec(ctx, sqlf.Sprintf(`INSERT INTO dashboard_grants (dashboard_id, user_id, org_id, global) VALUES %s`, sqlf.Join(values, ", ")))
	if err != nil {
		return errors.Wrap(err, "AddDashboardGrants")
	}

	return nil
}

// migrationContext represents a context for which we are currently migrating. If we are migrating a user setting we would populate this with their
// user ID, as well as any orgs they belong to. If we are migrating an org, we would populate this with just that orgID.
type migrationContext struct {
	userId int
	orgIds []int
}

func (m *migrator) lookupUniqueId(ctx context.Context, migration migrationContext, insightId string) (string, bool, error) {
	return basestore.ScanFirstString(m.insightsStore.Query(ctx, migration.ToInsightUniqueIdQuery(insightId)))
}

func (c migrationContext) ToInsightUniqueIdQuery(insightId string) *sqlf.Query {
	similarClause := sqlf.Sprintf("unique_id similar to %s", c.buildUniqueIdCondition(insightId))
	globalClause := sqlf.Sprintf("unique_id = %s", insightId)

	q := sqlf.Sprintf("select unique_id from insight_view where %s limit 1", sqlf.Join([]*sqlf.Query{similarClause, globalClause}, "OR"))

	// log.Println(q.Query(sqlf.PostgresBindVar), q.Args())
	return q
}

func (c migrationContext) buildUniqueIdCondition(insightId string) string {
	var conds []string
	for _, orgId := range c.orgIds {
		conds = append(conds, fmt.Sprintf("org-%d", orgId))
	}
	if c.userId != 0 {
		conds = append(conds, fmt.Sprintf("user-%d", c.userId))
	}
	return fmt.Sprintf("%s-%%(%s)%%", insightId, strings.Join(conds, "|"))
}

func (m *migrator) migrateDashboard(ctx context.Context, from settingDashboard, migrationContext migrationContext) (err error) {
	tx, err := m.insightsStore.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	var grantsQuery *sqlf.Query
	if from.UserID != nil {
		grantsQuery = sqlf.Sprintf("dg.user_id = %s", *from.UserID)
	} else if from.OrgID != nil {
		grantsQuery = sqlf.Sprintf("dg.org_id = %s", *from.OrgID)
	} else {
		grantsQuery = sqlf.Sprintf("dg.global IS TRUE")
	}

	count, _, err := basestore.ScanFirstInt(tx.Query(ctx, sqlf.Sprintf(`
		SELECT COUNT(*) from dashboard
		JOIN dashboard_grants dg ON dashboard.id = dg.dashboard_id
		WHERE
			dashboard.title = %s AND
			%s
	`,
		from.Title,
		grantsQuery,
	)))
	if err != nil {
		return err
	}
	if count != 0 {
		return nil
	}

	err = m.createDashboard(ctx, tx, from.Title, from.InsightIds, migrationContext)
	if err != nil {
		return err
	}

	return nil
}

func updateTimeSeriesReferences(handle *basestore.Store, ctx context.Context, oldId, newId string) (int, error) {
	q := sqlf.Sprintf(`
		WITH updated AS (
			UPDATE series_points sp
			SET series_id = %s
			WHERE series_id = %s
			RETURNING sp.series_id
		)
		SELECT count(*) FROM updated;
	`, newId, oldId)
	tempStore := basestore.NewWithHandle(handle.Handle())
	count, _, err := basestore.ScanFirstInt(tempStore.Query(ctx, q))
	if err != nil {
		return 0, errors.Wrap(err, "updateTimeSeriesReferences")
	}
	return count, nil
}

func updateTimeSeriesJobReferences(workerStore *basestore.Store, ctx context.Context, oldId, newId string) error {
	q := sqlf.Sprintf("update insights_query_runner_jobs set series_id = %s where series_id = %s", newId, oldId)
	err := workerStore.Exec(ctx, q)
	if err != nil {
		return errors.Wrap(err, "updateTimeSeriesJobReferences")
	}
	return nil
}
