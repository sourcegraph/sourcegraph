package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type GitserverRepoStore interface {
	basestore.ShareableStore
	With(other basestore.ShareableStore) GitserverRepoStore

	// Update updates the given rows with the GitServer status of a repo.
	Update(ctx context.Context, repos ...*types.GitserverRepo) error
	// IterateRepoGitserverStatus iterates over the status of all repos by joining
	// our repo and gitserver_repos table. It is impossible for us not to have a
	// corresponding row in gitserver_repos because of the trigger on repos table.
	// Use cursors and limit batch size to paginate through the full set.
	IterateRepoGitserverStatus(ctx context.Context, options IterateRepoGitserverStatusOptions) (rs []types.RepoGitserverStatus, nextCursor int, err error)
	GetByID(ctx context.Context, id api.RepoID) (*types.GitserverRepo, error)
	GetByName(ctx context.Context, name api.RepoName) (*types.GitserverRepo, error)
	GetByNames(ctx context.Context, names ...api.RepoName) (map[api.RepoName]*types.GitserverRepo, error)
	// LogCorruption sets the corrupted at value and logs the corruption reason. Reason will be truncated if it exceeds
	// MaxReasonSizeInMB
	LogCorruption(ctx context.Context, name api.RepoName, reason string, shardID string) error
	// SetCloning will attempt to bring the gitserver repo record into a pristine
	// state, and mark it as cloning.
	SetCloning(ctx context.Context, name api.RepoName, shardID string) error
	// SetNotCloned will attempt to bring the gitserver repo record into a pristine
	// state, and mark it as not cloned.
	SetNotCloned(ctx context.Context, name api.RepoName) error
	// SetCloned will attempt to bring the gitserver repo record into a pristine
	// state, and mark it as cloned.
	SetCloned(ctx context.Context, name api.RepoName, shardID string) error
	// SetLastError will attempt to update ONLY the last error of a GitServerRepo. If
	// a matching row does not yet exist a new one will be created.
	// If the error value hasn't changed, the row will not be updated.
	SetLastError(ctx context.Context, name api.RepoName, error, shardID string) error
	// SetLastOutput will attempt to create/update the output of the last repository clone/fetch.
	// If a matching row does not exist, a new one will be created.
	// Only one record will be maintained, so this records only the most recent output.
	SetLastOutput(ctx context.Context, name api.RepoName, output string) error
	// SetLastFetched will attempt to update ONLY the last fetched data (last_fetched, last_changed, shard_id) of a GitServerRepo and ensures it is marked as cloned.
	SetLastFetched(ctx context.Context, name api.RepoName, data GitserverFetchData) error
	// SetRepoSize will attempt to update ONLY the repo size of a GitServerRepo. If
	// a matching row does not yet exist a new one will be created.
	// If the size value hasn't changed, the row will not be updated.
	SetRepoSize(ctx context.Context, name api.RepoName, size int64, shardID string) error
	// ListReposWithLastError iterates over repos w/ non-empty last_error field and calls the repoFn for these repos.
	// note that this currently filters out any repos which do not have an associated external service where cloud_default = true.
	ListReposWithLastError(ctx context.Context) ([]api.RepoName, error)
	// ListPurgeableRepos returns all purgeable repos. These are repos that
	// are cloned on disk but have been deleted or blocked.
	ListPurgeableRepos(ctx context.Context, options ListPurgableReposOptions) ([]api.RepoName, error)
	// TotalErroredCloudDefaultRepos returns the total number of repos which have a non-empty last_error field. Note that this is only
	// counting repos with an associated cloud_default external service.
	TotalErroredCloudDefaultRepos(ctx context.Context) (int, error)
	// UpdateRepoSizes sets repo sizes according to input map. Key is repoID, value is repo_size_bytes.
	UpdateRepoSizes(ctx context.Context, logger log.Logger, shardID string, repos map[api.RepoName]int64) (int, error)
	// GetLastSyncOutput returns the last stored output from a repo sync (clone or fetch), or ok: false if
	// no log is found.
	GetLastSyncOutput(ctx context.Context, name api.RepoName) (output string, ok bool, err error)
	// GetGitserverGitDirSize returns the total size of all git directories of cloned
	// repos across all gitservers.
	GetGitserverGitDirSize(ctx context.Context) (sizeBytes int64, err error)
}

var _ GitserverRepoStore = (*gitserverRepoStore)(nil)

// Max reason size megabyte - 1 MB
const MaxReasonSizeInMB = 1 << 20

// gitserverRepoStore is responsible for data stored in the gitserver_repos table.
type gitserverRepoStore struct {
	*basestore.Store
}

// GitserverReposWith instantiates and returns a new gitserverRepoStore using
// the other store handle.
func GitserverReposWith(other basestore.ShareableStore) GitserverRepoStore {
	return &gitserverRepoStore{Store: basestore.NewWithHandle(other.Handle())}
}

func (s *gitserverRepoStore) With(other basestore.ShareableStore) GitserverRepoStore {
	return &gitserverRepoStore{Store: s.Store.With(other)}
}

func (s *gitserverRepoStore) Transact(ctx context.Context) (GitserverRepoStore, error) {
	txBase, err := s.Store.Transact(ctx)
	return &gitserverRepoStore{Store: txBase}, err
}

func (s *gitserverRepoStore) Update(ctx context.Context, repos ...*types.GitserverRepo) error {
	values := make([]*sqlf.Query, 0, len(repos))
	for _, gr := range repos {
		values = append(values, sqlf.Sprintf("(%s::integer, %s::text, %s::text, %s::text, %s::timestamp with time zone, %s::timestamp with time zone, %s::timestamp with time zone, %s::bigint, NOW())",
			gr.RepoID,
			gr.CloneStatus,
			gr.ShardID,
			dbutil.NewNullString(sanitizeToUTF8(gr.LastError)),
			gr.LastFetched,
			gr.LastChanged,
			dbutil.NullTimeColumn(gr.CorruptedAt),
			&dbutil.NullInt64{N: &gr.RepoSizeBytes},
		))
	}

	err := s.Exec(ctx, sqlf.Sprintf(updateGitserverReposQueryFmtstr, sqlf.Join(values, ",")))

	return errors.Wrap(err, "updating GitserverRepo")
}

const updateGitserverReposQueryFmtstr = `
WITH update_data AS (
	SELECT * FROM (
		VALUES
		-- (<repo_id>, <clone_status>, <shard_id>, <last_error>, <last_fetched>, <last_changed>, <corrupted_at>, <repo_size_bytes>),
			%s
	) AS tmp(repo_id, clone_status, shard_id, last_error, last_fetched, last_changed, corrupted_at, repo_size_bytes)
),
locked_data AS (
	SELECT update_data.*
	FROM update_data
	JOIN gitserver_repos gr ON gr.repo_id = update_data.repo_id
	ORDER BY update_data.repo_id ASC
	FOR UPDATE OF gr
)
UPDATE gitserver_repos AS gr
SET
	clone_status = locked_data.clone_status,
	shard_id = locked_data.shard_id,
	last_error = locked_data.last_error,
	last_fetched = locked_data.last_fetched,
	last_changed = locked_data.last_changed,
	corrupted_at = locked_data.corrupted_at,
	repo_size_bytes = locked_data.repo_size_bytes,
	updated_at = NOW()
FROM locked_data
WHERE
	locked_data.repo_id = gr.repo_id
`

func (s *gitserverRepoStore) TotalErroredCloudDefaultRepos(ctx context.Context) (int, error) {
	count, _, err := basestore.ScanFirstInt(s.Query(ctx, sqlf.Sprintf(totalErroredCloudDefaultReposQuery)))
	return count, err
}

const totalErroredCloudDefaultReposQuery = `
SELECT
	COUNT(*)
FROM gitserver_repos gr
JOIN repo r ON r.id = gr.repo_id
JOIN external_service_repos esr ON gr.repo_id = esr.repo_id
JOIN external_services es on esr.external_service_id = es.id
WHERE
	gr.last_error != ''
	AND r.blocked IS NULL
	AND r.deleted_at IS NULL
	AND es.cloud_default IS TRUE
`

func (s *gitserverRepoStore) ListReposWithLastError(ctx context.Context) ([]api.RepoName, error) {
	rows, err := s.Query(ctx, sqlf.Sprintf(nonemptyLastErrorQuery))
	return scanLastErroredRepos(rows, err)
}

const nonemptyLastErrorQuery = `
SELECT
	repo.name
FROM repo
JOIN gitserver_repos gr ON repo.id = gr.repo_id
JOIN external_service_repos esr ON repo.id = esr.repo_id
JOIN external_services es on esr.external_service_id = es.id
WHERE
	gr.last_error != ''
	AND repo.blocked IS NULL
	AND repo.deleted_at IS NULL
	AND es.cloud_default IS TRUE
`

func scanLastErroredRepoRow(scanner dbutil.Scanner) (name api.RepoName, err error) {
	err = scanner.Scan(&name)
	return name, err
}

var scanLastErroredRepos = basestore.NewSliceScanner(scanLastErroredRepoRow)

type ListPurgableReposOptions struct {
	// DeletedBefore will filter the deleted repos to only those that were deleted
	// before the given time. The zero value will not apply filtering.
	DeletedBefore time.Time
	// Limit optionally limits the repos iterated over. The zero value means no
	// limits are applied. Repos are ordered by their deleted at date, oldest first.
	Limit int
	// Limiter is an optional rate limiter that limits the rate at which we iterate
	// through the repos.
	Limiter *ratelimit.InstrumentedLimiter
}

var scanRepoNames = basestore.NewSliceScanner(basestore.ScanAny[api.RepoName])

func (s *gitserverRepoStore) ListPurgeableRepos(ctx context.Context, options ListPurgableReposOptions) (repos []api.RepoName, err error) {
	deletedAtClause := sqlf.Sprintf("deleted_at IS NOT NULL")
	if !options.DeletedBefore.IsZero() {
		deletedAtClause = sqlf.Sprintf("(deleted_at IS NOT NULL AND deleted_at < %s)", options.DeletedBefore)
	}
	query := purgableReposQuery
	if options.Limit > 0 {
		query = query + fmt.Sprintf(" LIMIT %d", options.Limit)
	}
	return scanRepoNames(s.Query(ctx, sqlf.Sprintf(query, deletedAtClause, types.CloneStatusCloned)))
}

const purgableReposQuery = `
SELECT
	repo.name
FROM repo
JOIN gitserver_repos gr ON repo.id = gr.repo_id
WHERE
	(%s OR repo.blocked IS NOT NULL)
	AND gr.clone_status = %s
ORDER BY deleted_at ASC
`

type IterateRepoGitserverStatusOptions struct {
	// If set, will only iterate over repos that have not been assigned to a shard
	OnlyWithoutShard bool
	// If true, also include deleted repos. Note that their repo name will start with
	// 'DELETED-'
	IncludeDeleted bool
	BatchSize      int
	NextCursor     int
}

func (s *gitserverRepoStore) IterateRepoGitserverStatus(ctx context.Context, options IterateRepoGitserverStatusOptions) (rs []types.RepoGitserverStatus, nextCursor int, err error) {
	preds := []*sqlf.Query{}

	if !options.IncludeDeleted {
		preds = append(preds, sqlf.Sprintf("repo.deleted_at IS NULL"))
	}

	if options.OnlyWithoutShard {
		preds = append(preds, sqlf.Sprintf("gr.shard_id = ''"))
	}

	if options.NextCursor > 0 {
		preds = append(preds, sqlf.Sprintf("gr.repo_id > %s", options.NextCursor))
		// Performance improvement: Postgres picks a more optimal strategy when we also constrain
		// set of potential joins.
		preds = append(preds, sqlf.Sprintf("repo.id > %s", options.NextCursor))
	}

	if len(preds) == 0 {
		preds = append(preds, sqlf.Sprintf("TRUE"))
	}

	var limitOffset *LimitOffset
	if options.BatchSize > 0 {
		limitOffset = &LimitOffset{Limit: options.BatchSize}
	}

	q := sqlf.Sprintf(iterateRepoGitserverQuery, sqlf.Join(preds, "AND"), limitOffset.SQL())

	rows, err := s.Query(ctx, q)
	if err != nil {
		return rs, nextCursor, errors.Wrap(err, "fetching gitserver status")
	}
	defer func() {
		err = basestore.CloseRows(rows, err)
	}()

	rs = make([]types.RepoGitserverStatus, 0, options.BatchSize)

	for rows.Next() {
		gr, name, err := scanGitserverRepo(rows)
		if err != nil {
			return rs, nextCursor, errors.Wrap(err, "scanning row")
		}

		nextCursor = int(gr.RepoID)

		rgs := types.RepoGitserverStatus{
			ID:            gr.RepoID,
			Name:          name,
			GitserverRepo: gr,
		}
		rs = append(rs, rgs)
	}

	return rs, nextCursor, nil
}

const iterateRepoGitserverQuery = `
SELECT
	gr.repo_id,
	repo.name,
	gr.clone_status,
	gr.shard_id,
	gr.last_error,
	gr.last_fetched,
	gr.last_changed,
	gr.repo_size_bytes,
	gr.updated_at,
	gr.corrupted_at,
	gr.corruption_logs
FROM gitserver_repos gr
JOIN repo ON gr.repo_id = repo.id
WHERE %s
ORDER BY gr.repo_id ASC
%s
`

func (s *gitserverRepoStore) GetByID(ctx context.Context, id api.RepoID) (*types.GitserverRepo, error) {
	repo, _, err := scanGitserverRepo(s.QueryRow(ctx, sqlf.Sprintf(getGitserverRepoByIDQueryFmtstr, id)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ErrGitserverRepoNotFound{}
		}
		return nil, err
	}
	return repo, nil
}

const getGitserverRepoByIDQueryFmtstr = `
SELECT
	gr.repo_id,
	-- We don't need this here, but the scanner needs it.
	'' as name,
	gr.clone_status,
	gr.shard_id,
	gr.last_error,
	gr.last_fetched,
	gr.last_changed,
	gr.repo_size_bytes,
	gr.updated_at,
	gr.corrupted_at,
	gr.corruption_logs
FROM gitserver_repos gr
WHERE gr.repo_id = %s
`

func (s *gitserverRepoStore) GetByName(ctx context.Context, name api.RepoName) (*types.GitserverRepo, error) {
	repo, _, err := scanGitserverRepo(s.QueryRow(ctx, sqlf.Sprintf(getGitserverRepoByNameQueryFmtstr, name)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ErrGitserverRepoNotFound{}
		}
		return nil, err
	}
	return repo, nil
}

const getGitserverRepoByNameQueryFmtstr = `
SELECT
	gr.repo_id,
	-- We don't need this here, but the scanner needs it.
	'' as name,
	gr.clone_status,
	gr.shard_id,
	gr.last_error,
	gr.last_fetched,
	gr.last_changed,
	gr.repo_size_bytes,
	gr.updated_at,
	gr.corrupted_at,
	gr.corruption_logs
FROM gitserver_repos gr
JOIN repo r ON r.id = gr.repo_id
WHERE r.name = %s
`

type ErrGitserverRepoNotFound struct{}

func (err *ErrGitserverRepoNotFound) Error() string { return "gitserver repo not found" }
func (ErrGitserverRepoNotFound) NotFound() bool     { return true }

const getByNamesQueryTemplate = `
SELECT
	gr.repo_id,
	r.name,
	gr.clone_status,
	gr.shard_id,
	gr.last_error,
	gr.last_fetched,
	gr.last_changed,
	gr.repo_size_bytes,
	gr.updated_at,
	gr.corrupted_at,
	gr.corruption_logs
FROM gitserver_repos gr
JOIN repo r on r.id = gr.repo_id
WHERE r.name = ANY (%s)
`

func (s *gitserverRepoStore) GetByNames(ctx context.Context, names ...api.RepoName) (map[api.RepoName]*types.GitserverRepo, error) {
	repos := make(map[api.RepoName]*types.GitserverRepo, len(names))

	rows, err := s.Query(ctx, sqlf.Sprintf(getByNamesQueryTemplate, pq.Array(names)))
	if err != nil {
		return nil, err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	for rows.Next() {
		repo, repoName, err := scanGitserverRepo(rows)
		if err != nil {
			return nil, err
		}
		repos[repoName] = repo
	}

	return repos, nil
}

func scanGitserverRepo(scanner dbutil.Scanner) (*types.GitserverRepo, api.RepoName, error) {
	var gr types.GitserverRepo
	var rawLogs []byte
	var cloneStatus string
	var repoName api.RepoName
	err := scanner.Scan(
		&gr.RepoID,
		&repoName,
		&cloneStatus,
		&gr.ShardID,
		&dbutil.NullString{S: &gr.LastError},
		&gr.LastFetched,
		&gr.LastChanged,
		&dbutil.NullInt64{N: &gr.RepoSizeBytes},
		&gr.UpdatedAt,
		&dbutil.NullTime{Time: &gr.CorruptedAt},
		&rawLogs,
	)
	if err != nil {
		return nil, "", errors.Wrap(err, "scanning GitserverRepo")
	}

	gr.CloneStatus = types.ParseCloneStatus(cloneStatus)

	err = json.Unmarshal(rawLogs, &gr.CorruptionLogs)
	if err != nil {
		return nil, repoName, errors.Wrap(err, "unmarshal of corruption_logs failed")
	}
	return &gr, repoName, nil
}

func (s *gitserverRepoStore) SetCloneStatus(ctx context.Context, name api.RepoName, status types.CloneStatus, shardID string) error {
	err := s.Exec(ctx, sqlf.Sprintf(`
UPDATE gitserver_repos
SET
	corrupted_at = NULL,
	clone_status = %s,
	shard_id = %s,
	updated_at = NOW()
WHERE
	repo_id = (SELECT id FROM repo WHERE name = %s)
	AND
	clone_status IS DISTINCT FROM %s
`, status, shardID, name, status))
	if err != nil {
		return errors.Wrap(err, "setting clone status")
	}

	return nil
}

func (s *gitserverRepoStore) SetNotCloned(ctx context.Context, name api.RepoName) error {
	q := sqlf.Sprintf(gitserverReposSetNotClonedQueryFmtstr, types.CloneStatusNotCloned, name)

	return errors.Wrap(s.Exec(ctx, q), "setting gitserver repo to not cloned")
}

const gitserverReposSetNotClonedQueryFmtstr = `
UPDATE gitserver_repos
SET
	corrupted_at = NULL,
	clone_status = %s,
	shard_id = '',
	last_error = NULL,
	updated_at = NOW(),
	repo_size_bytes = NULL,
	corrupted_at = NULL
WHERE
	repo_id = (SELECT id FROM repo WHERE name = %s)
`

func (s *gitserverRepoStore) SetCloned(ctx context.Context, name api.RepoName, shardID string) error {
	q := sqlf.Sprintf(gitserverReposSetClonedQueryFmtstr, types.CloneStatusCloned, shardID, name)

	return errors.Wrap(s.Exec(ctx, q), "setting gitserver repo to cloned")
}

const gitserverReposSetClonedQueryFmtstr = `
UPDATE gitserver_repos
SET
	corrupted_at = NULL,
	clone_status = %s,
	shard_id = %s,
	last_error = NULL,
	updated_at = NOW(),
	corrupted_at = NULL
WHERE
	repo_id = (SELECT id FROM repo WHERE name = %s)
`

func (s *gitserverRepoStore) SetCloning(ctx context.Context, name api.RepoName, shardID string) error {
	q := sqlf.Sprintf(gitserverReposSetCloningQueryFmtstr, types.CloneStatusCloning, shardID, name)

	return errors.Wrap(s.Exec(ctx, q), "setting gitserver repo to cloning")
}

const gitserverReposSetCloningQueryFmtstr = `
UPDATE gitserver_repos
SET
	corrupted_at = NULL,
	clone_status = %s,
	shard_id = %s,
	last_error = NULL,
	updated_at = NOW(),
	repo_size_bytes = NULL,
	corrupted_at = NULL
WHERE
	repo_id = (SELECT id FROM repo WHERE name = %s)
`

func (s *gitserverRepoStore) SetLastError(ctx context.Context, name api.RepoName, error, shardID string) error {
	ns := dbutil.NewNullString(sanitizeToUTF8(error))

	err := s.Exec(ctx, sqlf.Sprintf(`
UPDATE gitserver_repos
SET
	last_error = %s,
	shard_id = %s,
	updated_at = NOW()
WHERE
	repo_id = (SELECT id FROM repo WHERE name = %s)
	AND
	last_error IS DISTINCT FROM %s
`, ns, shardID, name, ns))
	if err != nil {
		return errors.Wrap(err, "setting last error")
	}

	return nil
}

func (s *gitserverRepoStore) SetLastOutput(ctx context.Context, name api.RepoName, output string) error {
	ns := sanitizeToUTF8(output)

	err := s.Exec(ctx, sqlf.Sprintf(`
INSERT INTO gitserver_repos_sync_output(repo_id, last_output)
SELECT id, %s FROM repo WHERE name = %s
ON CONFLICT(repo_id)
DO UPDATE SET last_output = EXCLUDED.last_output, updated_at = NOW()
`, ns, name))
	if err != nil {
		return errors.Wrap(err, "setting last output")
	}

	return nil
}

func (s *gitserverRepoStore) GetLastSyncOutput(ctx context.Context, name api.RepoName) (output string, ok bool, err error) {
	q := sqlf.Sprintf(getLastSyncOutputQueryFmtstr, name)
	output, ok, err = basestore.ScanFirstString(s.Query(ctx, q))
	// We don't store NULLs in the db, so we need to map empty string to not ok as well.s
	if output == "" {
		ok = false
	}
	return output, ok, err
}

const getLastSyncOutputQueryFmtstr = `
SELECT
	last_output
FROM
	gitserver_repos_sync_output
WHERE
	repo_id = (SELECT id FROM repo WHERE name = %s)
`

func (s *gitserverRepoStore) GetGitserverGitDirSize(ctx context.Context) (sizeBytes int64, err error) {
	conds := []*sqlf.Query{
		sqlf.Sprintf("gitserver_repos.clone_status = %s", types.CloneStatusCloned),
	}
	q := sqlf.Sprintf(getGitserverGitDirSizeQueryFmtstr, sqlf.Join(conds, "AND"))
	sizeBytes, _, err = basestore.ScanFirstNullInt64(s.Query(ctx, q))
	return sizeBytes, err
}

const getGitserverGitDirSizeQueryFmtstr = `
SELECT
	SUM(gitserver_repos.repo_size_bytes)
FROM
	gitserver_repos
WHERE
	%s
`

func (s *gitserverRepoStore) SetRepoSize(ctx context.Context, name api.RepoName, size int64, shardID string) error {
	err := s.Exec(ctx, sqlf.Sprintf(`
UPDATE gitserver_repos
SET
	repo_size_bytes = %s,
	shard_id = %s,
	updated_at = NOW()
WHERE
	repo_id = (SELECT id FROM repo WHERE name = %s)
	AND
	repo_size_bytes IS DISTINCT FROM %s
	`, size, shardID, name, size))
	if err != nil {
		return errors.Wrap(err, "setting repo size")
	}

	return nil
}

func (s *gitserverRepoStore) LogCorruption(ctx context.Context, name api.RepoName, reason string, shardID string) error {
	// trim reason to 1 MB so that we don't store huge reasons and run into trouble when it gets too large
	if len(reason) > MaxReasonSizeInMB {
		reason = reason[:MaxReasonSizeInMB]
	}

	log := types.RepoCorruptionLog{
		Timestamp: time.Now(),
		Reason:    reason,
	}
	var rawLog []byte
	if data, err := json.Marshal(log); err != nil {
		return errors.Wrap(err, "could not marshal corruption_logs")
	} else {
		rawLog = data
	}

	res, err := s.ExecResult(ctx, sqlf.Sprintf(`
UPDATE gitserver_repos as gtr
SET
	corrupted_at = NOW(),
	-- prepend the json and then ensure we only keep 10 items in the resulting json array
	corruption_logs = (SELECT jsonb_path_query_array(%s||gtr.corruption_logs, '$[0 to 9]')),
	updated_at = NOW()
WHERE
	repo_id = (SELECT id FROM repo WHERE name = %s)
AND
	(shard_id = %s OR shard_id = '')
AND
	corrupted_at IS NULL`, rawLog, name, shardID))
	if err != nil {
		return errors.Wrapf(err, "logging repo corruption")
	}

	if nrows, err := res.RowsAffected(); err != nil {
		return errors.Wrapf(err, "getting rows affected")
	} else if nrows != 1 {
		return errors.New("repo not found or already corrupt")
	}
	return nil
}

// GitserverFetchData is the metadata associated with a fetch operation on
// gitserver.
type GitserverFetchData struct {
	// LastFetched was the time the fetch operation completed (gitserver_repos.last_fetched).
	LastFetched time.Time
	// LastChanged was the last time a fetch changed the contents of the repo (gitserver_repos.last_changed).
	LastChanged time.Time
	// ShardID is the name of the gitserver the fetch ran on (gitserver.shard_id).
	ShardID string
}

func (s *gitserverRepoStore) SetLastFetched(ctx context.Context, name api.RepoName, data GitserverFetchData) error {
	res, err := s.ExecResult(ctx, sqlf.Sprintf(`
UPDATE gitserver_repos
SET
	corrupted_at = NULL,
	last_fetched = %s,
	last_changed = %s,
	shard_id = %s,
	clone_status = %s,
	updated_at = NOW()
WHERE repo_id = (SELECT id FROM repo WHERE name = %s)
`, data.LastFetched, data.LastChanged, data.ShardID, types.CloneStatusCloned, name))
	if err != nil {
		return errors.Wrap(err, "setting last fetched")
	}

	if nrows, err := res.RowsAffected(); err != nil {
		return errors.Wrap(err, "getting rows affected")
	} else if nrows != 1 {
		return errors.New("repo not found")
	}

	return nil
}

func (s *gitserverRepoStore) UpdateRepoSizes(ctx context.Context, logger log.Logger, shardID string, repos map[api.RepoName]int64) (updated int, err error) {
	logger = logger.Scoped("gitserverRepoStore.UpdateRepoSizes")
	// batchSize is 1000 because we started with really large batch sizes (32k)
	// and noticed that it was really slow in production.
	//
	// As of today (14 Dec 2023) updating 1000 rows (worst case: all of them
	// need to be updated) would take ~2 seconds. But normal case should be
	// much faster.
	const batchSize = 1000
	return s.updateRepoSizesWithBatchSize(ctx, logger, repos, batchSize)
}

func (s *gitserverRepoStore) updateRepoSizesWithBatchSize(ctx context.Context, logger log.Logger, repos map[api.RepoName]int64, batchSize int) (updated int, err error) {
	queries := make([]*sqlf.Query, batchSize)

	logger.Info("Updating repository sizes", log.Int("count", len(repos)), log.Int("batchSize", batchSize))

	left := len(repos)
	currentCount := 0
	updatedRows := 0
	for repo, size := range repos {
		start := time.Now()

		queries[currentCount] = sqlf.Sprintf("(%s::citext, %s::bigint)", repo, size)

		currentCount += 1

		if currentCount == batchSize || currentCount == left {
			logger.Info("Updating batch of repository sizes", log.Int("left", left))

			// IMPORTANT: we only take the elements of batch up to currentCount
			q := sqlf.Sprintf(updateRepoSizesQueryFmtstr, sqlf.Join(queries[:currentCount], ","))
			res, err := s.ExecResult(ctx, q)
			if err != nil {
				logger.Info("Failed to update batch", log.Error(err))
				return updatedRows, err
			}

			rowsAffected, err := res.RowsAffected()
			if err != nil {
				logger.Info("Failed to read updated rows", log.Error(err))
				return updatedRows, err
			}
			updatedRows += int(rowsAffected)

			duration := time.Since(start)
			logger.Info("Batch updated", log.Duration("duration", duration), log.Int("rowsAffected", int(rowsAffected)))

			left -= currentCount
			currentCount = 0
		}
	}

	return updatedRows, nil
}

const updateRepoSizesQueryFmtstr = `
WITH update_data AS (
	SELECT * FROM (VALUES
		-- (<repo_name>, <repo_size_bytes>),
		%s
	) AS tmp(repo_name, repo_size_bytes)
)
UPDATE gitserver_repos AS gr
SET
    repo_size_bytes = update_data.repo_size_bytes,
	updated_at = NOW()
FROM repo r
INNER JOIN update_data on update_data.repo_name = r.name
WHERE
	r.id = gr.repo_id
AND
	update_data.repo_size_bytes IS DISTINCT FROM gr.repo_size_bytes
`

// sanitizeToUTF8 will remove any null character terminated string. The null character can be
// represented in one of the following ways in Go:
//
// Hex: \x00
// Unicode: \u0000
// Octal digits: \000
//
// Using any of them to replace the null character has the same effect. See this playground
// example: https://play.golang.org/p/8SKPmalJRRW
//
// See this for a detailed answer:
// https://stackoverflow.com/a/38008565/1773961
func sanitizeToUTF8(s string) string {
	// Replace any null characters in the string. We would have expected strings.ToValidUTF8 to take
	// care of replacing any null characters, but it seems like this character is treated as valid a
	// UTF-8 character. See
	// https://stackoverflow.com/questions/6907297/can-utf-8-contain-zero-byte/6907327#6907327.

	// And it only appears that Postgres has a different idea of UTF-8 (only slightly). Without
	// using this function call, inserts for this string in Postgres return the following error:
	//
	// ERROR: invalid byte sequence for encoding "UTF8": 0x00 (SQLSTATE 22021)
	t := strings.ReplaceAll(s, "\x00", "")

	// Sanitize to a valid UTF-8 string and return it.
	return strings.ToValidUTF8(t, "")
}
