package database

import (
	"context"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/keegancsmith/sqlf"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// CodeMonitorStore is an interface for interacting with the code monitor tables in the database
type CodeMonitorStore interface {
	basestore.ShareableStore
	Transact(context.Context) (CodeMonitorStore, error)
	Done(error) error
	Now() time.Time
	Clock() func() time.Time
	Exec(ctx context.Context, query *sqlf.Query) error

	CreateMonitor(ctx context.Context, args MonitorArgs) (*Monitor, error)
	UpdateMonitor(ctx context.Context, id int64, args MonitorArgs) (*Monitor, error)
	UpdateMonitorEnabled(ctx context.Context, id int64, enabled bool) (*Monitor, error)
	DeleteMonitor(ctx context.Context, id int64) error
	GetMonitor(ctx context.Context, monitorID int64) (*Monitor, error)
	ListMonitors(context.Context, ListMonitorsOpts) ([]*Monitor, error)
	CountMonitors(ctx context.Context, opts ListMonitorsOpts) (int32, error)

	CreateQueryTrigger(ctx context.Context, monitorID int64, query string) (*QueryTrigger, error)
	UpdateQueryTrigger(ctx context.Context, id int64, query string) error
	GetQueryTriggerForMonitor(ctx context.Context, monitorID int64) (*QueryTrigger, error)
	ResetQueryTriggerTimestamps(ctx context.Context, queryID int64) error
	SetQueryTriggerNextRun(ctx context.Context, triggerQueryID int64, next time.Time, latestResults time.Time) error
	GetQueryTriggerForJob(ctx context.Context, triggerJob int32) (*QueryTrigger, error)
	EnqueueQueryTriggerJobs(context.Context) ([]*TriggerJob, error)
	ListQueryTriggerJobs(context.Context, ListTriggerJobsOpts) ([]*TriggerJob, error)
	CountQueryTriggerJobs(ctx context.Context, queryID int64) (int32, error)

	UpdateTriggerJobWithResults(ctx context.Context, triggerJobID int32, queryString string, results []*result.CommitMatch) error
	DeleteOldTriggerJobs(ctx context.Context, retentionInDays int) error
	UpdateTriggerJobWithLogs(ctx context.Context, triggerJobID int32, entry TriggerJobLogs) error

	UpdateEmailAction(_ context.Context, id int64, _ *EmailActionArgs) (*EmailAction, error)
	CreateEmailAction(ctx context.Context, monitorID int64, _ *EmailActionArgs) (*EmailAction, error)
	DeleteEmailActions(ctx context.Context, actionIDs []int64, monitorID int64) error
	GetEmailAction(ctx context.Context, emailID int64) (*EmailAction, error)
	ListEmailActions(context.Context, ListActionsOpts) ([]*EmailAction, error)

	UpdateWebhookAction(_ context.Context, id int64, enabled, includeResults bool, url string) (*WebhookAction, error)
	CreateWebhookAction(ctx context.Context, monitorID int64, enabled, includeResults bool, url string) (*WebhookAction, error)
	DeleteWebhookActions(ctx context.Context, monitorID int64, ids ...int64) error
	CountWebhookActions(ctx context.Context, monitorID int64) (int, error)
	GetWebhookAction(ctx context.Context, id int64) (*WebhookAction, error)
	ListWebhookActions(context.Context, ListActionsOpts) ([]*WebhookAction, error)

	UpdateSlackWebhookAction(_ context.Context, id int64, enabled, includeResults bool, url string) (*SlackWebhookAction, error)
	CreateSlackWebhookAction(ctx context.Context, monitorID int64, enabled, includeResults bool, url string) (*SlackWebhookAction, error)
	DeleteSlackWebhookActions(ctx context.Context, monitorID int64, ids ...int64) error
	CountSlackWebhookActions(ctx context.Context, monitorID int64) (int, error)
	GetSlackWebhookAction(ctx context.Context, id int64) (*SlackWebhookAction, error)
	ListSlackWebhookActions(context.Context, ListActionsOpts) ([]*SlackWebhookAction, error)

	CreateRecipient(ctx context.Context, emailID int64, userID, orgID *int32) (*Recipient, error)
	DeleteRecipients(ctx context.Context, emailID int64) error
	ListRecipients(context.Context, ListRecipientsOpts) ([]*Recipient, error)
	CountRecipients(ctx context.Context, emailID int64) (int32, error)

	ListActionJobs(context.Context, ListActionJobsOpts) ([]*ActionJob, error)
	CountActionJobs(context.Context, ListActionJobsOpts) (int, error)
	GetActionJobMetadata(ctx context.Context, jobID int32) (*ActionJobMetadata, error)
	GetActionJob(ctx context.Context, jobID int32) (*ActionJob, error)
	EnqueueActionJobsForMonitor(ctx context.Context, monitorID int64, triggerJob int32) ([]*ActionJob, error)

	// HasAnyLastSearched returns whether there have ever been any repo-aware code monitor
	// searches executed for this code monitor. This should only be needed during the transition
	// version so that we don't detect every repo as a new repo and search their entire history
	// when a code monitor transitions from non-repo-aware to repo-aware.
	HasAnyLastSearched(ctx context.Context, monitorID int64) (bool, error)
	UpsertLastSearched(ctx context.Context, monitorID int64, repoID api.RepoID, lastSearched []string) error
	GetLastSearched(ctx context.Context, monitorID int64, repoID api.RepoID) ([]string, error)
}

// codeMonitorStore exposes methods to read and write codemonitors domain models
// from persistent storage.
type codeMonitorStore struct {
	*basestore.Store
	userStore UserStore
	now       func() time.Time
}

var _ CodeMonitorStore = (*codeMonitorStore)(nil)

// CodeMonitorsWith returns a new Store backed by the given database.
func CodeMonitorsWith(other basestore.ShareableStore) *codeMonitorStore {
	return CodeMonitorsWithClock(other, timeutil.Now)
}

// CodeMonitorsWithClock returns a new Store backed by the given database and
// clock for timestamps.
func CodeMonitorsWithClock(other basestore.ShareableStore, clock func() time.Time) *codeMonitorStore {
	handle := basestore.NewWithHandle(other.Handle())
	return &codeMonitorStore{Store: handle, userStore: UsersWith(log.Scoped("codemonitors"), handle), now: clock}
}

// Clock returns the clock of the underlying store.
func (s *codeMonitorStore) Clock() func() time.Time {
	return s.now
}

func (s *codeMonitorStore) Now() time.Time {
	return s.now()
}

// Transact creates a new transaction.
// It's required to implement this method and wrap the Transact method of the
// underlying basestore.Store.
func (s *codeMonitorStore) Transact(ctx context.Context) (CodeMonitorStore, error) {
	txBase, err := s.Store.Transact(ctx)
	if err != nil {
		return nil, err
	}
	return &codeMonitorStore{Store: txBase, now: s.now}, nil
}

type JobTable int

const (
	TriggerJobs JobTable = iota
	ActionJobs
)

type JobState int

const (
	Queued JobState = iota
	Processing
	Completed
	Errored
	Failed
)

const setStatusFmtStr = `
UPDATE %s
SET state = %s,
    started_at = %s,
    finished_at = %s
WHERE id = %s;
`

func (s *TestStore) SetJobStatus(ctx context.Context, table JobTable, state JobState, id int) error {
	st := []string{"queued", "processing", "completed", "errored", "failed"}[state]
	t := []string{"cm_trigger_jobs", "cm_action_jobs"}[table]
	return s.Exec(ctx, sqlf.Sprintf(setStatusFmtStr, quote(t), st, s.Now(), s.Now(), id))
}

type TestStore struct {
	CodeMonitorStore
}

func (s *TestStore) InsertTestMonitor(ctx context.Context, t *testing.T) (*Monitor, error) {
	t.Helper()

	actions := []*EmailActionArgs{
		{
			Enabled:        true,
			IncludeResults: false,
			Priority:       "NORMAL",
			Header:         "test header 1",
		},
		{
			Enabled:        true,
			IncludeResults: false,
			Priority:       "CRITICAL",
			Header:         "test header 2",
		},
	}

	// Create monitor.
	uid := actor.FromContext(ctx).UID
	m, err := s.CreateMonitor(ctx, MonitorArgs{
		Description:     testDescription,
		Enabled:         true,
		NamespaceUserID: &uid,
	})
	if err != nil {
		return nil, err
	}

	// Create trigger.
	_, err = s.CreateQueryTrigger(ctx, m.ID, testQuery)
	if err != nil {
		return nil, err
	}

	for _, a := range actions {
		e, err := s.CreateEmailAction(ctx, m.ID, &EmailActionArgs{
			Enabled:        a.Enabled,
			IncludeResults: a.IncludeResults,
			Priority:       a.Priority,
			Header:         a.Header,
		})
		if err != nil {
			return nil, err
		}

		_, err = s.CreateRecipient(ctx, e.ID, &uid, nil)
		if err != nil {
			return nil, err
		}
		// TODO(camdencheek): add other action types (webhooks) here
	}
	return m, nil
}

func namespaceScopeQuery(user *types.User) *sqlf.Query {
	namespaceScope := sqlf.Sprintf("cm_monitors.namespace_user_id = %s", user.ID)
	if user.SiteAdmin {
		namespaceScope = sqlf.Sprintf("TRUE")
	}
	return namespaceScope
}

func NewTestStore(t *testing.T, db DB) (context.Context, *TestStore) {
	ctx := actor.WithInternalActor(context.Background())
	now := time.Now().Truncate(time.Microsecond)
	return ctx, &TestStore{CodeMonitorsWithClock(db, func() time.Time { return now })}
}

func NewTestUser(ctx context.Context, t *testing.T, db dbutil.DB) (name string, id int32, namespace graphql.ID, userContext context.Context) {
	t.Helper()

	name = "cm-user1"
	id = insertTestUser(ctx, t, db, name, true)
	namespace = relay.MarshalID("User", id)
	ctx = actor.WithActor(ctx, actor.FromUser(id))
	return name, id, namespace, ctx
}

const (
	//nolint:unused // used in tests
	testQuery = "repo:github\\.com/sourcegraph/sourcegraph func type:diff patternType:literal"
	//nolint:unused // used in tests
	testDescription = "test description"
)

//nolint:unused // used in tests
func newTestStore(t *testing.T) (context.Context, DB, *codeMonitorStore) {
	logger := logtest.Scoped(t)
	ctx := actor.WithInternalActor(context.Background())
	db := NewDB(logger, dbtest.NewDB(t))
	now := time.Now().Truncate(time.Microsecond)
	return ctx, db, CodeMonitorsWithClock(db, func() time.Time { return now })
}

//nolint:unused // used in tests
func newTestUser(ctx context.Context, t *testing.T, db dbutil.DB) (name string, id int32, userContext context.Context) {
	t.Helper()

	name = "cm-user1"
	id = insertTestUser(ctx, t, db, name, true)
	_ = relay.MarshalID("User", id)
	ctx = actor.WithActor(ctx, actor.FromUser(id))
	return name, id, ctx
}

//nolint:unused // used in tests
func insertTestUser(ctx context.Context, t *testing.T, db dbutil.DB, name string, isAdmin bool) (userID int32) {
	t.Helper()

	q := sqlf.Sprintf("INSERT INTO users (username, site_admin) VALUES (%s, %t) RETURNING id", name, isAdmin)
	err := db.QueryRowContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...).Scan(&userID)
	require.NoError(t, err)
	return userID
}
