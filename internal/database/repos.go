package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	regexpsyntax "regexp/syntax"
	"strings"
	"sync"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/database/query"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/awscodecommit"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketcloud"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitolite"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

type RepoNotFoundErr struct {
	ID   api.RepoID
	Name api.RepoName
}

func IsRepoNotFoundErr(err error) bool {
	_, ok := err.(*RepoNotFoundErr)
	return ok
}

func (e *RepoNotFoundErr) Error() string {
	if e.Name != "" {
		return fmt.Sprintf("repo not found: name=%q", e.Name)
	}
	if e.ID != 0 {
		return fmt.Sprintf("repo not found: id=%d", e.ID)
	}
	return "repo not found"
}

func (e *RepoNotFoundErr) NotFound() bool {
	return true
}

// RepoStore handles access to the repo table
type RepoStore struct {
	*basestore.Store

	once sync.Once
}

// Repos instantiates and returns a new RepoStore with prepared statements.
func Repos(db dbutil.DB) *RepoStore {
	return &RepoStore{Store: basestore.NewWithDB(db, sql.TxOptions{})}
}

// NewRepoStoreWithDB instantiates and returns a new RepoStore using the other
// store handle.
func ReposWith(other basestore.ShareableStore) *RepoStore {
	return &RepoStore{Store: basestore.NewWithHandle(other.Handle())}
}

func (s *RepoStore) With(other basestore.ShareableStore) *RepoStore {
	return &RepoStore{Store: s.Store.With(other)}
}

func (s *RepoStore) Transact(ctx context.Context) (*RepoStore, error) {
	txBase, err := s.Store.Transact(ctx)
	return &RepoStore{Store: txBase}, err
}

// ensureStore instantiates a basestore.Store if necessary, using the dbconn.Global handle.
// This function ensures access to dbconn happens after the rest of the code or tests have
// initialized it.
func (s *RepoStore) ensureStore() {
	s.once.Do(func() {
		if s.Store == nil {
			s.Store = basestore.NewWithDB(dbconn.Global, sql.TxOptions{})
		}
	})
}

// Get returns metadata for the request repository ID. It fetches data
// only from the database and NOT from any external sources. If the
// caller is concerned the copy of the data in the database might be
// stale, the caller is responsible for fetching data from any
// external services.
func (s *RepoStore) Get(ctx context.Context, id api.RepoID) (*types.Repo, error) {
	if Mocks.Repos.Get != nil {
		return Mocks.Repos.Get(ctx, id)
	}
	s.ensureStore()

	repos, err := s.getBySQL(ctx, sqlf.Sprintf("id=%d", id), sqlf.Sprintf("LIMIT 1"))
	if err != nil {
		return nil, err
	}

	if len(repos) == 0 {
		return nil, &RepoNotFoundErr{ID: id}
	}
	return repos[0], nil
}

// GetByName returns the repository with the given nameOrUri from the
// database, or an error. If we have a match on name and uri, we prefer the
// match on name.
//
// Name is the name for this repository (e.g., "github.com/user/repo"). It is
// the same as URI, unless the user configures a non-default
// repositoryPathPattern.
func (s *RepoStore) GetByName(ctx context.Context, nameOrURI api.RepoName) (*types.Repo, error) {
	if Mocks.Repos.GetByName != nil {
		return Mocks.Repos.GetByName(ctx, nameOrURI)
	}
	s.ensureStore()

	repos, err := s.getBySQL(ctx, sqlf.Sprintf("name=%s", nameOrURI), sqlf.Sprintf("LIMIT 1"))
	if err != nil {
		return nil, err
	}

	if len(repos) == 1 {
		return repos[0], nil
	}

	// We don't fetch in the same SQL query since uri is not unique and could
	// conflict with a name. We prefer returning the matching name if it
	// exists.
	repos, err = s.getBySQL(ctx, sqlf.Sprintf("uri=%s", nameOrURI), sqlf.Sprintf("LIMIT 1"))
	if err != nil {
		return nil, err
	}

	if len(repos) == 0 {
		return nil, &RepoNotFoundErr{Name: nameOrURI}
	}

	return repos[0], nil
}

// GetByIDs returns a list of repositories by given IDs. The number of results list could be less
// than the candidate list due to no repository is associated with some IDs.
func (s *RepoStore) GetByIDs(ctx context.Context, ids ...api.RepoID) ([]*types.Repo, error) {
	if Mocks.Repos.GetByIDs != nil {
		return Mocks.Repos.GetByIDs(ctx, ids...)
	}
	s.ensureStore()

	if len(ids) == 0 {
		return []*types.Repo{}, nil
	}

	items := make([]*sqlf.Query, len(ids))
	for i := range ids {
		items[i] = sqlf.Sprintf("%d", ids[i])
	}
	q := sqlf.Sprintf("id IN (%s)", sqlf.Join(items, ","))
	var repos []*types.Repo
	err := s.getReposBySQL(ctx, false, nil, q, nil, func(rows *sql.Rows) error {
		var repo types.Repo

		if err := scanRepo(rows, &repo); err != nil {
			return err
		}

		repos = append(repos, &repo)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return repos, nil
}

// GetReposSetByIDs returns a map of repositories with the given IDs, indexed by their IDs. The number of results
// entries could be less than the candidate list due to no repository is associated with some IDs.
func (s *RepoStore) GetReposSetByIDs(ctx context.Context, ids ...api.RepoID) (map[api.RepoID]*types.Repo, error) {
	repos, err := s.GetByIDs(ctx, ids...)
	if err != nil {
		return nil, err
	}

	repoMap := make(map[api.RepoID]*types.Repo, len(repos))
	for _, r := range repos {
		repoMap[r.ID] = r
	}

	return repoMap, nil
}

func (s *RepoStore) Count(ctx context.Context, opt ReposListOptions) (ct int, err error) {
	if Mocks.Repos.Count != nil {
		return Mocks.Repos.Count(ctx, opt)
	}
	s.ensureStore()

	tr, ctx := trace.New(ctx, "repos.Count", "")
	defer func() {
		if err != nil {
			tr.SetError(err)
		}
		tr.Finish()
	}()

	conds, err := s.listSQL(opt)
	if err != nil {
		return 0, err
	}

	joins := []*sqlf.Query{}

	if len(opt.ExternalServiceIDs) != 0 {
		serviceIDQuery := []*sqlf.Query{}
		for _, id := range opt.ExternalServiceIDs {
			serviceIDQuery = append(serviceIDQuery, sqlf.Sprintf("%s", id))
		}
		joins = append(joins, sqlf.Sprintf("JOIN external_service_repos e ON (repo.id = e.repo_id AND e.external_service_id IN (%s))", sqlf.Join(serviceIDQuery, ",")))
	} else if opt.UserID != 0 {
		joins = append(joins, sqlf.Sprintf(`JOIN external_service_repos e ON (repo.id = e.repo_id)
												   JOIN external_services es on es.id = e.external_service_id AND es.namespace_user_id = %s`, opt.UserID))
	}

	predQ := sqlf.Sprintf("TRUE")
	if len(conds) > 0 {
		predQ = sqlf.Sprintf("(%s)", sqlf.Join(conds, "AND"))
	}

	q := sqlf.Sprintf("SELECT COUNT(*) FROM repo %s WHERE repo.deleted_at IS NULL AND %s", sqlf.Join(joins, " "), predQ)
	tr.LazyPrintf("SQL: %v", q.Query(sqlf.PostgresBindVar))

	var count int

	if err := s.QueryRow(ctx, q).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

const getRepoByQueryFmtstr = `
SELECT %s
FROM %%s
WHERE
	repo.deleted_at IS NULL
AND %%s   -- Populates "queryConds"
AND (%%s) -- Populates "authzConds"
%%s       -- Populates "querySuffix"
`

const getSourcesByRepoQueryStr = `
(
	SELECT
		json_agg(
		json_build_object(
			'CloneURL', esr.clone_url,
			'ID', esr.external_service_id,
			'Kind', LOWER(svcs.kind)
		)
		)
	FROM external_service_repos AS esr
	JOIN external_services AS svcs ON esr.external_service_id = svcs.id
	WHERE
		esr.repo_id = repo.id
		AND
		svcs.deleted_at IS NULL
)
`

var getBySQLColumns = []string{
	"repo.id",
	"repo.name",
	"repo.private",
	"repo.external_id",
	"repo.external_service_type",
	"repo.external_service_id",
	"repo.uri",
	"repo.description",
	"repo.fork",
	"repo.archived",
	"repo.cloned",
	"repo.created_at",
	"repo.updated_at",
	"repo.deleted_at",
	"repo.metadata",
	getSourcesByRepoQueryStr,
}

func minimalColumns(columns []string) []string {
	return columns[:2]
}

func (s *RepoStore) getBySQL(ctx context.Context, queryConds, querySuffix *sqlf.Query) ([]*types.Repo, error) {
	var repos []*types.Repo
	err := s.getReposBySQL(ctx, false, nil, queryConds, querySuffix, func(rows *sql.Rows) error {
		var repo types.Repo

		if err := scanRepo(rows, &repo); err != nil {
			return err
		}

		repos = append(repos, &repo)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return repos, nil
}

func (s *RepoStore) getReposBySQL(ctx context.Context, minimal bool, fromClause, queryConds, querySuffix *sqlf.Query, scanRepo func(*sql.Rows) error) error {
	if fromClause == nil {
		fromClause = sqlf.Sprintf("repo")
	}
	if queryConds == nil {
		queryConds = sqlf.Sprintf("TRUE")
	}
	if querySuffix == nil {
		querySuffix = sqlf.Sprintf("")
	}

	authzConds, err := AuthzQueryConds(ctx, s.Handle().DB())
	if err != nil {
		return err
	}

	columns := getBySQLColumns
	if minimal {
		columns = minimalColumns(columns)
	}

	q := sqlf.Sprintf(
		fmt.Sprintf(getRepoByQueryFmtstr, strings.Join(columns, ",")),
		fromClause,
		queryConds,
		authzConds, // 🚨 SECURITY: Enforce repository permissions
		querySuffix,
	)

	rows, err := s.Query(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		if err := scanRepo(rows); err != nil {
			return err
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}

	return nil
}

func scanRepo(rows *sql.Rows, r *types.Repo) (err error) {
	var sources dbutil.NullJSONRawMessage
	var metadata json.RawMessage

	err = rows.Scan(
		&r.ID,
		&r.Name,
		&r.Private,
		&dbutil.NullString{S: &r.ExternalRepo.ID},
		&dbutil.NullString{S: &r.ExternalRepo.ServiceType},
		&dbutil.NullString{S: &r.ExternalRepo.ServiceID},
		&dbutil.NullString{S: &r.URI},
		&dbutil.NullString{S: &r.Description},
		&r.Fork,
		&r.Archived,
		&r.Cloned,
		&r.CreatedAt,
		&dbutil.NullTime{Time: &r.UpdatedAt},
		&dbutil.NullTime{Time: &r.DeletedAt},
		&metadata,
		&sources,
	)
	if err != nil {
		return err
	}

	type sourceInfo struct {
		ID       int64
		CloneURL string
		Kind     string
	}
	r.Sources = make(map[string]*types.SourceInfo)

	if sources.Raw != nil {
		var srcs []sourceInfo
		if err = json.Unmarshal(sources.Raw, &srcs); err != nil {
			return errors.Wrap(err, "scanRepo: failed to unmarshal sources")
		}
		for _, src := range srcs {
			urn := extsvc.URN(src.Kind, src.ID)
			r.Sources[urn] = &types.SourceInfo{
				ID:       urn,
				CloneURL: src.CloneURL,
			}
		}
	}

	typ, ok := extsvc.ParseServiceType(r.ExternalRepo.ServiceType)
	if !ok {
		return nil
	}
	switch typ {
	case extsvc.TypeGitHub:
		r.Metadata = new(github.Repository)
	case extsvc.TypeGitLab:
		r.Metadata = new(gitlab.Project)
	case extsvc.TypeBitbucketServer:
		r.Metadata = new(bitbucketserver.Repo)
	case extsvc.TypeBitbucketCloud:
		r.Metadata = new(bitbucketcloud.Repo)
	case extsvc.TypeAWSCodeCommit:
		r.Metadata = new(awscodecommit.Repository)
	case extsvc.TypeGitolite:
		r.Metadata = new(gitolite.Repo)
	default:
		return nil
	}

	if err = json.Unmarshal(metadata, r.Metadata); err != nil {
		return errors.Wrapf(err, "scanRepo: failed to unmarshal %q metadata", typ)
	}

	return nil
}

// ReposListOptions specifies the options for listing repositories.
//
// Query and IncludePatterns/ExcludePatterns may not be used together.
type ReposListOptions struct {
	// Query specifies a search query for repositories. If specified, then the Sort and
	// Direction options are ignored
	Query string

	// IncludePatterns is a list of regular expressions, all of which must match all
	// repositories returned in the list.
	IncludePatterns []string

	// ExcludePattern is a regular expression that must not match any repository
	// returned in the list.
	ExcludePattern string

	// Names is a list of repository names used to limit the results to that
	// set of repositories.
	// Note: This is currently used for version contexts. In future iterations,
	// version contexts may have their own table
	// and this may be replaced by the version context name.
	Names []string

	// IDs of repos to list. When zero-valued, this is omitted from the predicate set.
	IDs []api.RepoID

	// UserID, if non zero, will limit the set of results to repositories added by the user
	// through external services. Mutually exclusive with the ExternalServiceIDs option.
	UserID int32

	// ServiceTypes of repos to list. When zero-valued, this is omitted from the predicate set.
	ServiceTypes []string

	// ExternalServiceIDs, if non empty, will only return repos added by the given external services.
	// The id is that of the external_services table NOT the external_service_id in the repo table
	// Mutually exclusive with the UserID option.
	ExternalServiceIDs []int64

	// ExternalRepos of repos to list. When zero-valued, this is omitted from the predicate set.
	ExternalRepos []api.ExternalRepoSpec

	// ExternalRepoIncludePrefixes is the list of specs to include repos using
	// prefix matching. When zero-valued, this is omitted from the predicate set.
	ExternalRepoIncludePrefixes []api.ExternalRepoSpec

	// ExternalRepoExcludePrefixes is the list of specs to exclude repos using
	// prefix matching. When zero-valued, this is omitted from the predicate set.
	ExternalRepoExcludePrefixes []api.ExternalRepoSpec

	// PatternQuery is an expression tree of patterns to query. The atoms of
	// the query are strings which are regular expression patterns.
	PatternQuery query.Q

	// NoForks excludes forks from the list.
	NoForks bool

	// OnlyForks excludes non-forks from the lhist.
	OnlyForks bool

	// NoArchived excludes archived repositories from the list.
	NoArchived bool

	// OnlyArchived excludes non-archived repositories from the list.
	OnlyArchived bool

	// NoCloned excludes cloned repositories from the list.
	NoCloned bool

	// OnlyCloned excludes non-cloned repositories from the list.
	OnlyCloned bool

	// NoPrivate excludes private repositories from the list.
	NoPrivate bool

	// OnlyPrivate excludes non-private repositories from the list.
	OnlyPrivate bool

	// Index when set will only include repositories which should be indexed
	// if true. If false it will exclude repositories which should be
	// indexed. An example use case of this is for indexed search only
	// indexing a subset of repositories.
	Index *bool

	// List of fields by which to order the return repositories.
	OrderBy RepoListOrderBy

	// CursorColumn contains the relevant column for cursor-based pagination (e.g. "name")
	CursorColumn string

	// CursorValue contains the relevant value for cursor-based pagination (e.g. "Zaphod").
	CursorValue string

	// CursorDirection contains the comparison for cursor-based pagination, all possible values are: next, prev.
	CursorDirection string

	// UseOr decides between ANDing or ORing the predicates together.
	UseOr bool

	// IncludeUserPublicRepos will include repos from the user_public_repos table if this field is true, and the user_id
	// is non-zero. Note that these are not repos owned by this user, just ones they are interested in.
	IncludeUserPublicRepos bool

	*LimitOffset
}

type RepoListOrderBy []RepoListSort

func (r RepoListOrderBy) SQL() *sqlf.Query {
	if len(r) == 0 {
		return sqlf.Sprintf(`ORDER BY id ASC`)
	}

	clauses := make([]*sqlf.Query, 0, len(r))
	for _, s := range r {
		clauses = append(clauses, s.SQL())
	}
	return sqlf.Sprintf(`ORDER BY %s`, sqlf.Join(clauses, ", "))
}

// RepoListSort is a field by which to sort and the direction of the sorting.
type RepoListSort struct {
	Field      RepoListColumn
	Descending bool
}

func (r RepoListSort) SQL() *sqlf.Query {
	if r.Descending {
		return sqlf.Sprintf(string(r.Field) + ` DESC`)
	}
	return sqlf.Sprintf(string(r.Field))
}

// RepoListColumn is a column by which repositories can be sorted. These correspond to columns in the database.
type RepoListColumn string

const (
	RepoListCreatedAt RepoListColumn = "created_at"
	RepoListName      RepoListColumn = "name"
)

// List lists repositories in the Sourcegraph repository
//
// This will not return any repositories from external services that are not present in the Sourcegraph repository.
// The result list is unsorted and has a fixed maximum limit of 1000 items.
// Matching is done with fuzzy matching, i.e. "query" will match any repo name that matches the regexp `q.*u.*e.*r.*y`
func (s *RepoStore) List(ctx context.Context, opt ReposListOptions) (results []*types.Repo, err error) {
	tr, ctx := trace.New(ctx, "repos.List", "")
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	if Mocks.Repos.List != nil {
		return Mocks.Repos.List(ctx, opt)
	}
	s.ensureStore()

	var repos []*types.Repo
	err = s.list(ctx, tr, false, opt, func(rows *sql.Rows) error {
		var repo types.Repo

		if err := scanRepo(rows, &repo); err != nil {
			return err
		}

		repos = append(repos, &repo)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return repos, nil
}

// ListRepoNames returns a list of repositories names and ids.
func (s *RepoStore) ListRepoNames(ctx context.Context, opt ReposListOptions) (results []*types.RepoName, err error) {
	tr, ctx := trace.New(ctx, "repos.ListRepoNames", "")
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()
	if Mocks.Repos.ListRepoNames != nil {
		return Mocks.Repos.ListRepoNames(ctx, opt)
	}
	s.ensureStore()

	var repos []*types.RepoName
	err = s.list(ctx, tr, true, opt, func(rows *sql.Rows) error {
		var r types.RepoName
		err := rows.Scan(
			&r.ID,
			&r.Name,
		)
		if err != nil {
			return err
		}

		repos = append(repos, &r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return repos, nil
}

func (s *RepoStore) list(ctx context.Context, tr *trace.Trace, minimal bool, opt ReposListOptions, scanRepo func(rows *sql.Rows) error) error {
	if len(opt.ExternalServiceIDs) != 0 && opt.UserID != 0 {
		return errors.New("options ExternalServiceIDs and UserID are mutually exclusive")
	}

	conds, err := s.listSQL(opt)
	if err != nil {
		return err
	}

	fromClause := sqlf.Sprintf("repo")
	if len(opt.ExternalServiceIDs) != 0 {
		serviceIDQuery := []*sqlf.Query{}
		for _, id := range opt.ExternalServiceIDs {
			serviceIDQuery = append(serviceIDQuery, sqlf.Sprintf("%s", id))
		}
		fromClause = sqlf.Sprintf("repo JOIN external_service_repos e ON (repo.id = e.repo_id AND e.external_service_id IN (%s))", sqlf.Join(serviceIDQuery, ","))
	} else if opt.UserID != 0 {
		fromClause = sqlf.Sprintf(`
			repo
				JOIN external_service_repos esr ON repo.id = esr.repo_id
				JOIN external_services es ON esr.external_service_id = es.id
		`)
		if opt.IncludeUserPublicRepos {
			conds = append(conds, sqlf.Sprintf("(es.namespace_user_id = %d OR EXISTS (SELECT 1 FROM user_public_repos WHERE user_id = %d AND repo_id = repo.id)) AND es.deleted_at IS NULL", opt.UserID, opt.UserID))
		} else {
			conds = append(conds, sqlf.Sprintf("es.namespace_user_id = %d AND es.deleted_at IS NULL", opt.UserID))
		}
	}

	queryConds := sqlf.Sprintf("TRUE")
	if len(conds) > 0 {
		if opt.UseOr {
			queryConds = sqlf.Join(conds, "\n OR ")
		} else {
			queryConds = sqlf.Join(conds, "\n AND ")
		}
	}
	queryConds = sqlf.Sprintf("(%s)", queryConds)

	querySuffix := sqlf.Sprintf("%s %s", opt.OrderBy.SQL(), opt.LimitOffset.SQL())
	tr.LogFields(trace.SQL(queryConds), trace.SQL(querySuffix))

	return s.getReposBySQL(ctx, minimal, fromClause, queryConds, querySuffix, scanRepo)
}

type ListDefaultReposOptions struct {
	// If true, will only include uncloned default repos
	OnlyUncloned bool
}

// ListDefaultRepos returns a list of default repos. Default repos are a union of
// repos in our default_repos table and repos owned by users.
func (s *RepoStore) ListDefaultRepos(ctx context.Context, opts ListDefaultReposOptions) (results []*types.RepoName, err error) {
	tr, ctx := trace.New(ctx, "repos.ListDefaultRepos", "")
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()
	s.ensureStore()

	cloneClause := sqlf.Sprintf("TRUE")
	if opts.OnlyUncloned {
		cloneClause = sqlf.Sprintf("NOT repo.cloned")
	}

	q := sqlf.Sprintf(`
-- source: internal/database/repos.go:RepoStore.ListDefaultRepos
SELECT repo.id, repo.name FROM repo
JOIN default_repos dr ON repo.id = dr.repo_id
WHERE
      repo.deleted_at IS NULL
      AND %s

UNION

SELECT repo.id, repo.name FROM repo
JOIN external_service_repos esr ON repo.id = esr.repo_id
JOIN external_services es ON esr.external_service_id = es.id
WHERE
      NOT es.cloud_default
      AND es.deleted_at IS NULL
      AND repo.deleted_at IS NULL
      AND %s

UNION

SELECT repo.id, repo.name FROM repo
WHERE
	EXISTS (SELECT 1 FROM user_public_repos WHERE repo_id = repo.id)
	AND repo.deleted_at IS NULL
	AND %s
`, cloneClause, cloneClause, cloneClause)

	rows, err := s.Query(ctx, q)
	if err != nil {
		return nil, errors.Wrap(err, "querying for indexed repos")
	}
	defer rows.Close()
	for rows.Next() {
		var r types.RepoName
		if err := rows.Scan(&r.ID, &r.Name); err != nil {
			return nil, errors.Wrap(err, "scanning row from default_repos table")
		}
		results = append(results, &r)
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, "scanning rows for default repos")
	}

	return results, nil
}

// Create inserts repos and their sources, respectively in the repo and external_service_repos table.
// Associated external services must already exist.
func (s *RepoStore) Create(ctx context.Context, repos ...*types.Repo) (err error) {
	tr, ctx := trace.New(ctx, "repos.Create", "")
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()
	s.ensureStore()

	records := make([]*repoRecord, 0, len(repos))

	for _, r := range repos {
		repoRec, err := newRepoRecord(r)
		if err != nil {
			return err
		}

		records = append(records, repoRec)
	}

	encodedRepos, err := json.Marshal(records)
	if err != nil {
		return err
	}

	q := sqlf.Sprintf(insertReposQuery, string(encodedRepos))

	rows, err := s.Query(ctx, q)
	if err != nil {
		return errors.Wrap(err, "insert")
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	for i := 0; rows.Next(); i++ {
		if err := rows.Scan(&repos[i].ID); err != nil {
			return err
		}
	}

	return nil
}

// repoRecord is the json representation of a repository as used in this package
// Postgres CTEs.
type repoRecord struct {
	ID                  api.RepoID      `json:"id"`
	Name                string          `json:"name"`
	URI                 *string         `json:"uri,omitempty"`
	Description         string          `json:"description"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           *time.Time      `json:"updated_at,omitempty"`
	DeletedAt           *time.Time      `json:"deleted_at,omitempty"`
	ExternalServiceType *string         `json:"external_service_type,omitempty"`
	ExternalServiceID   *string         `json:"external_service_id,omitempty"`
	ExternalID          *string         `json:"external_id,omitempty"`
	Archived            bool            `json:"archived"`
	Fork                bool            `json:"fork"`
	Private             bool            `json:"private"`
	Metadata            json.RawMessage `json:"metadata"`
	Sources             json.RawMessage `json:"sources,omitempty"`
}

func newRepoRecord(r *types.Repo) (*repoRecord, error) {
	metadata, err := metadataColumn(r.Metadata)
	if err != nil {
		return nil, errors.Wrapf(err, "newRecord: metadata marshalling failed")
	}

	sources, err := sourcesColumn(r.ID, r.Sources)
	if err != nil {
		return nil, errors.Wrapf(err, "newRecord: sources marshalling failed")
	}

	return &repoRecord{
		ID:                  r.ID,
		Name:                string(r.Name),
		URI:                 nullStringColumn(r.URI),
		Description:         r.Description,
		CreatedAt:           r.CreatedAt.UTC(),
		UpdatedAt:           nullTimeColumn(r.UpdatedAt),
		DeletedAt:           nullTimeColumn(r.DeletedAt),
		ExternalServiceType: nullStringColumn(r.ExternalRepo.ServiceType),
		ExternalServiceID:   nullStringColumn(r.ExternalRepo.ServiceID),
		ExternalID:          nullStringColumn(r.ExternalRepo.ID),
		Archived:            r.Archived,
		Fork:                r.Fork,
		Private:             r.Private,
		Metadata:            metadata,
		Sources:             sources,
	}, nil
}

func nullTimeColumn(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func nullInt32Column(n int32) *int32 {
	if n == 0 {
		return nil
	}
	return &n
}

func nullStringColumn(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func metadataColumn(metadata interface{}) (msg json.RawMessage, err error) {
	switch m := metadata.(type) {
	case nil:
		msg = json.RawMessage("{}")
	case string:
		msg = json.RawMessage(m)
	case []byte:
		msg = m
	case json.RawMessage:
		msg = m
	default:
		msg, err = json.MarshalIndent(m, "        ", "    ")
	}
	return
}

func sourcesColumn(repoID api.RepoID, sources map[string]*types.SourceInfo) (json.RawMessage, error) {
	var records []externalServiceRepo
	for _, src := range sources {
		records = append(records, externalServiceRepo{
			ExternalServiceID: src.ExternalServiceID(),
			RepoID:            int64(repoID),
			CloneURL:          src.CloneURL,
		})
	}

	return json.MarshalIndent(records, "        ", "    ")
}

type externalServiceRepo struct {
	ExternalServiceID int64  `json:"external_service_id"`
	RepoID            int64  `json:"repo_id"`
	CloneURL          string `json:"clone_url"`
}

var insertReposQuery = `
WITH repos_list AS (
  SELECT * FROM ROWS FROM (
	json_to_recordset(%s)
	AS (
		name                  citext,
		uri                   citext,
		description           text,
		created_at            timestamptz,
		updated_at            timestamptz,
		deleted_at            timestamptz,
		external_service_type text,
		external_service_id   text,
		external_id           text,
		archived              boolean,
		fork                  boolean,
		private               boolean,
		metadata              jsonb,
		sources               jsonb
	  )
	)
	WITH ORDINALITY
),
inserted_repos AS (
  INSERT INTO repo (
	name,
	uri,
	description,
	created_at,
	updated_at,
	deleted_at,
	external_service_type,
	external_service_id,
	external_id,
	archived,
	fork,
	private,
	metadata
  )
  SELECT
	name,
	NULLIF(BTRIM(uri), ''),
	description,
	created_at,
	updated_at,
	deleted_at,
	external_service_type,
	external_service_id,
	external_id,
	archived,
	fork,
	private,
	metadata
  FROM repos_list
  RETURNING id
),
inserted_repos_rows AS (
  SELECT id, ROW_NUMBER() OVER () AS rn FROM inserted_repos
),
repos_list_rows AS (
  SELECT *, ROW_NUMBER() OVER () AS rn FROM repos_list
),
inserted_repos_with_ids AS (
  SELECT
	inserted_repos_rows.id,
	repos_list_rows.*
  FROM repos_list_rows
  JOIN inserted_repos_rows USING (rn)
),
sources_list AS (
  SELECT
    inserted_repos_with_ids.id AS repo_id,
	sources.external_service_id AS external_service_id,
	sources.clone_url AS clone_url
  FROM
    inserted_repos_with_ids,
	jsonb_to_recordset(inserted_repos_with_ids.sources)
	  AS sources(
		external_service_id bigint,
		repo_id             integer,
		clone_url           text
	  )
),
insert_sources AS (
  INSERT INTO external_service_repos (
    external_service_id,
    repo_id,
    clone_url
  )
  SELECT
    external_service_id,
    repo_id,
    clone_url
  FROM sources_list
  ON CONFLICT ON CONSTRAINT external_service_repos_repo_id_external_service_id_unique
  DO
    UPDATE SET clone_url = EXCLUDED.clone_url
    WHERE external_service_repos.clone_url != EXCLUDED.clone_url
)
SELECT id FROM inserted_repos_with_ids;
`

// Delete deletes repos associated with the given ids and their associated sources.
func (s *RepoStore) Delete(ctx context.Context, ids ...api.RepoID) error {
	if len(ids) == 0 {
		return nil
	}
	s.ensureStore()

	// The number of deleted repos can potentially be higher
	// than the maximum number of arguments we can pass to postgres.
	// We pass them as a json array instead to overcome this limitation.
	encodedIds, err := json.Marshal(ids)
	if err != nil {
		return err
	}

	q := sqlf.Sprintf(deleteReposQuery, string(encodedIds))

	err = s.Exec(ctx, q)
	if err != nil {
		return errors.Wrap(err, "delete")
	}

	return nil
}

const deleteReposQuery = `
WITH repo_ids AS (
  SELECT jsonb_array_elements_text(%s) AS id
)
UPDATE repo
SET
  name = soft_deleted_repository_name(name),
  deleted_at = transaction_timestamp()
FROM repo_ids
WHERE deleted_at IS NULL
AND repo.id = repo_ids.id::int
`

// ListEnabledNames returns a list of all enabled repo names. This is commonly
// requested information by other services (repo-updater and
// indexed-search). We special case just returning enabled names so that we
// read much less data into memory.
func (s *RepoStore) ListEnabledNames(ctx context.Context) ([]string, error) {
	s.ensureStore()
	q := sqlf.Sprintf("SELECT name FROM repo WHERE deleted_at IS NULL")
	return basestore.ScanStrings(s.Query(ctx, q))
}

func parsePattern(p string) ([]*sqlf.Query, error) {
	exact, like, pattern, err := parseIncludePattern(p)
	if err != nil {
		return nil, err
	}
	var conds []*sqlf.Query
	if exact != nil {
		if len(exact) == 0 || (len(exact) == 1 && exact[0] == "") {
			conds = append(conds, sqlf.Sprintf("TRUE"))
		} else {
			items := []*sqlf.Query{}
			for _, v := range exact {
				items = append(items, sqlf.Sprintf("%s", v))
			}
			conds = append(conds, sqlf.Sprintf("name IN (%s)", sqlf.Join(items, ",")))
		}
	}
	if len(like) > 0 {
		for _, v := range like {
			conds = append(conds, sqlf.Sprintf(`lower(name) LIKE %s`, strings.ToLower(v)))
		}
	}
	if pattern != "" {
		conds = append(conds, sqlf.Sprintf("lower(name) ~ lower(%s)", pattern))
	}
	return []*sqlf.Query{sqlf.Sprintf("(%s)", sqlf.Join(conds, "OR"))}, nil
}

func (*RepoStore) listSQL(opt ReposListOptions) (conds []*sqlf.Query, err error) {
	// Cursor-based pagination requires parsing a handful of extra fields, which
	// may result in additional query conditions.
	cursorConds, err := parseCursorConds(opt)
	if err != nil {
		return nil, err
	}
	conds = append(conds, cursorConds...)

	if opt.Query != "" && (len(opt.IncludePatterns) > 0 || opt.ExcludePattern != "") {
		return nil, errors.New("Repos.List: Query and IncludePatterns/ExcludePattern options are mutually exclusive")
	}
	if opt.Query != "" {
		conds = append(conds, sqlf.Sprintf("lower(name) LIKE %s", "%"+strings.ToLower(opt.Query)+"%"))
	}
	for _, includePattern := range opt.IncludePatterns {
		extraConds, err := parsePattern(includePattern)
		if err != nil {
			return nil, err
		}
		conds = append(conds, extraConds...)
	}
	if opt.ExcludePattern != "" {
		conds = append(conds, sqlf.Sprintf("lower(name) !~* %s", opt.ExcludePattern))
	}
	if opt.PatternQuery != nil {
		cond, err := query.Eval(opt.PatternQuery, func(q query.Q) (*sqlf.Query, error) {
			pattern, ok := q.(string)
			if !ok {
				return nil, errors.Errorf("unexpected token in repo listing query: %q", q)
			}
			extraConds, err := parsePattern(pattern)
			if err != nil {
				return nil, err
			}
			if len(extraConds) == 0 {
				return sqlf.Sprintf("TRUE"), nil
			}
			return sqlf.Join(extraConds, "AND"), nil
		})
		if err != nil {
			return nil, err
		}
		conds = append(conds, cond)
	}

	if len(opt.IDs) > 0 {
		ids := make([]*sqlf.Query, 0, len(opt.IDs))
		for _, id := range opt.IDs {
			if id != 0 {
				ids = append(ids, sqlf.Sprintf("%d", id))
			}
		}
		conds = append(conds, sqlf.Sprintf("id IN (%s)", sqlf.Join(ids, ",")))
	}

	if len(opt.ServiceTypes) > 0 {
		ks := make([]*sqlf.Query, 0, len(opt.ServiceTypes))
		for _, svcType := range opt.ServiceTypes {
			ks = append(ks, sqlf.Sprintf("%s", strings.ToLower(svcType)))
		}
		conds = append(conds,
			sqlf.Sprintf("LOWER(external_service_type) IN (%s)", sqlf.Join(ks, ",")))
	}

	if len(opt.ExternalRepos) > 0 {
		er := make([]*sqlf.Query, 0, len(opt.ExternalRepos))
		for _, spec := range opt.ExternalRepos {
			er = append(er, sqlf.Sprintf("(external_id = %s AND external_service_type = %s AND external_service_id = %s)", spec.ID, spec.ServiceType, spec.ServiceID))
		}
		conds = append(conds, sqlf.Sprintf("(%s)", sqlf.Join(er, "\n OR ")))
	}

	if len(opt.ExternalRepoIncludePrefixes) > 0 {
		er := make([]*sqlf.Query, 0, len(opt.ExternalRepoIncludePrefixes))
		for _, spec := range opt.ExternalRepoIncludePrefixes {
			er = append(er, sqlf.Sprintf("(external_id LIKE %s AND external_service_type = %s AND external_service_id = %s)", spec.ID+"%", spec.ServiceType, spec.ServiceID))
		}
		conds = append(conds, sqlf.Sprintf("(%s)", sqlf.Join(er, "\n OR ")))
	}

	if len(opt.ExternalRepoExcludePrefixes) > 0 {
		er := make([]*sqlf.Query, 0, len(opt.ExternalRepoExcludePrefixes))
		for _, spec := range opt.ExternalRepoExcludePrefixes {
			er = append(er, sqlf.Sprintf("(external_id NOT LIKE %s AND external_service_type = %s AND external_service_id = %s)", spec.ID+"%", spec.ServiceType, spec.ServiceID))
		}
		conds = append(conds, sqlf.Sprintf("(%s)", sqlf.Join(er, "\n AND ")))
	}

	if opt.NoForks {
		conds = append(conds, sqlf.Sprintf("NOT fork"))
	}
	if opt.OnlyForks {
		conds = append(conds, sqlf.Sprintf("fork"))
	}
	if opt.NoArchived {
		conds = append(conds, sqlf.Sprintf("NOT archived"))
	}
	if opt.OnlyArchived {
		conds = append(conds, sqlf.Sprintf("archived"))
	}
	if opt.NoCloned {
		conds = append(conds, sqlf.Sprintf("NOT cloned"))
	}
	if opt.OnlyCloned {
		conds = append(conds, sqlf.Sprintf("cloned"))
	}
	if opt.NoPrivate {
		conds = append(conds, sqlf.Sprintf("NOT private"))
	}
	if opt.OnlyPrivate {
		conds = append(conds, sqlf.Sprintf("private"))
	}
	if len(opt.Names) > 0 {
		queries := make([]*sqlf.Query, 0, len(opt.Names))
		for _, repo := range opt.Names {
			queries = append(queries, sqlf.Sprintf("%s", repo))
		}
		conds = append(conds, sqlf.Sprintf("NAME IN (%s)", sqlf.Join(queries, ", ")))
	}

	if opt.Index != nil {
		// We don't currently have an index column, but when we want the
		// indexable repositories to be a subset it will live in the database
		// layer. So we do the filtering here.
		indexAll := conf.SearchIndexEnabled()
		if indexAll != *opt.Index {
			conds = append(conds, sqlf.Sprintf("false"))
		}
	}

	return conds, nil
}

// parseCursorConds checks whether the query is using cursor-based pagination, and
// if so performs the necessary transformations for it to be successful.
func parseCursorConds(opt ReposListOptions) (conds []*sqlf.Query, err error) {
	if opt.CursorColumn == "" || opt.CursorValue == "" {
		return nil, nil
	}
	var direction string
	switch opt.CursorDirection {
	case "next":
		direction = ">="
	case "prev":
		direction = "<="
	default:
		return nil, fmt.Errorf("missing or invalid cursor direction: %q", opt.CursorDirection)
	}

	switch opt.CursorColumn {
	case string(RepoListName):
		conds = append(conds, sqlf.Sprintf("name "+direction+" %s", opt.CursorValue))
	case string(RepoListCreatedAt):
		conds = append(conds, sqlf.Sprintf("created_at "+direction+" %s", opt.CursorValue))
	default:
		return nil, fmt.Errorf("missing or invalid cursor: %q %q", opt.CursorColumn, opt.CursorValue)
	}
	return conds, nil
}

// parseIncludePattern either (1) parses the pattern into a list of exact possible
// string values and LIKE patterns if such a list can be determined from the pattern,
// and (2) returns the original regexp if those patterns are not equivalent to the
// regexp.
//
// It allows Repos.List to optimize for the common case where a pattern like
// `(^github.com/foo/bar$)|(^github.com/baz/qux$)` is provided. In that case,
// it's faster to query for "WHERE name IN (...)" the two possible exact values
// (because it can use an index) instead of using a "WHERE name ~*" regexp condition
// (which generally can't use an index).
//
// This optimization is necessary for good performance when there are many repos
// in the database. With this optimization, specifying a "repogroup:" in the query
// will be fast (even if there are many repos) because the query can be constrained
// efficiently to only the repos in the group.
func parseIncludePattern(pattern string) (exact, like []string, regexp string, err error) {
	re, err := regexpsyntax.Parse(pattern, regexpsyntax.OneLine)
	if err != nil {
		return nil, nil, "", err
	}
	exact, contains, prefix, suffix, err := allMatchingStrings(re.Simplify(), false)
	if err != nil {
		return nil, nil, "", err
	}
	for _, v := range contains {
		like = append(like, "%"+v+"%")
	}
	for _, v := range prefix {
		like = append(like, v+"%")
	}
	for _, v := range suffix {
		like = append(like, "%"+v)
	}
	if exact != nil || like != nil {
		return exact, like, "", nil
	}
	return nil, nil, pattern, nil
}

// allMatchingStrings returns a complete list of the strings that re
// matches, if it's possible to determine the list. The "last" argument
// indicates if this is the last part of the original regexp.
func allMatchingStrings(re *regexpsyntax.Regexp, last bool) (exact, contains, prefix, suffix []string, err error) {
	switch re.Op {
	case regexpsyntax.OpEmptyMatch:
		return []string{""}, nil, nil, nil, nil
	case regexpsyntax.OpLiteral:
		prog, err := regexpsyntax.Compile(re)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		prefix, complete := prog.Prefix()
		if complete {
			return nil, []string{prefix}, nil, nil, nil
		}
		return nil, nil, nil, nil, nil

	case regexpsyntax.OpCharClass:
		// Only handle simple case of one range.
		if len(re.Rune) == 2 {
			len := int(re.Rune[1] - re.Rune[0] + 1)
			if len > 26 {
				// Avoid large character ranges (which could blow up the number
				// of possible matches).
				return nil, nil, nil, nil, nil
			}
			chars := make([]string, len)
			for r := re.Rune[0]; r <= re.Rune[1]; r++ {
				chars[r-re.Rune[0]] = string(r)
			}
			return nil, chars, nil, nil, nil
		}
		return nil, nil, nil, nil, nil

	case regexpsyntax.OpStar:
		if len(re.Sub) == 1 && (re.Sub[0].Op == regexpsyntax.OpAnyCharNotNL || re.Sub[0].Op == regexpsyntax.OpAnyChar) {
			if last {
				return nil, []string{""}, nil, nil, nil
			}
			return nil, nil, nil, nil, nil
		}

	case regexpsyntax.OpBeginText:
		return nil, nil, []string{""}, nil, nil

	case regexpsyntax.OpEndText:
		return nil, nil, nil, []string{""}, nil

	case regexpsyntax.OpCapture:
		return allMatchingStrings(re.Sub0[0], false)

	case regexpsyntax.OpConcat:
		var begin, end bool
		for i, sub := range re.Sub {
			if sub.Op == regexpsyntax.OpBeginText && i == 0 {
				begin = true
				continue
			}
			if sub.Op == regexpsyntax.OpEndText && i == len(re.Sub)-1 {
				end = true
				continue
			}
			subexact, subcontains, subprefix, subsuffix, err := allMatchingStrings(sub, i == len(re.Sub)-1)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			if subexact == nil && subcontains == nil && subprefix == nil && subsuffix == nil {
				return nil, nil, nil, nil, nil
			}

			if subexact == nil {
				subexact = subcontains
			}
			if exact == nil {
				exact = subexact
			} else {
				size := len(exact) * len(subexact)
				if len(subexact) > 4 || size > 30 {
					// Avoid blowup in number of possible matches.
					return nil, nil, nil, nil, nil
				}
				combined := make([]string, 0, size)
				for _, match := range exact {
					for _, submatch := range subexact {
						combined = append(combined, match+submatch)
					}
				}
				exact = combined
			}
		}
		if exact == nil {
			exact = []string{""}
		}
		if begin && end {
			return exact, nil, nil, nil, nil
		} else if begin {
			return nil, nil, exact, nil, nil
		} else if end {
			return nil, nil, nil, exact, nil
		}
		return nil, exact, nil, nil, nil

	case regexpsyntax.OpAlternate:
		for _, sub := range re.Sub {
			subexact, subcontains, subprefix, subsuffix, err := allMatchingStrings(sub, false)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			exact = append(exact, subexact...)
			contains = append(contains, subcontains...)
			prefix = append(prefix, subprefix...)
			suffix = append(suffix, subsuffix...)
		}
		return exact, contains, prefix, suffix, nil
	}

	return nil, nil, nil, nil, nil
}
