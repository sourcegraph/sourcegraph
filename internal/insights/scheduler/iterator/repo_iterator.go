package iterator

import (
	"context"
	"math"
	"time"

	"github.com/derision-test/glock"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type FinishFunc func(ctx context.Context, store *basestore.Store, maybeErr error) error
type FinishNFunc func(ctx context.Context, store *basestore.Store, maybeErr map[int32]error) error

// PersistentRepoIterator represents a durable (persisted) iterator over a set of repositories. This iteration is not
// concurrency safe and only one consumer should have access to this resource at a time.
type PersistentRepoIterator struct {
	Id              int
	CreatedAt       time.Time
	StartedAt       time.Time
	CompletedAt     time.Time
	RuntimeDuration time.Duration
	PercentComplete float64
	TotalCount      int
	SuccessCount    int
	repos           []int32
	Cursor          int

	errors         errorMap
	terminalErrors errorMap
	retryRepos     []int32
	retryCursor    int

	glock glock.Clock
}

type errorMap map[int32]*IterationError

func (em errorMap) FailureCount(repo int32) int {
	v, ok := em[repo]
	if !ok {
		return 0
	}
	return v.FailureCount
}

type IterationError struct {
	id            int
	RepoId        int32
	FailureCount  int
	ErrorMessages []string
}

var repoIteratorCols = []*sqlf.Query{
	sqlf.Sprintf("repo_iterator.Id"),
	sqlf.Sprintf("repo_iterator.created_at"),
	sqlf.Sprintf("repo_iterator.started_at"),
	sqlf.Sprintf("repo_iterator.completed_at"),
	sqlf.Sprintf("repo_iterator.runtime_duration"),
	sqlf.Sprintf("repo_iterator.percent_complete"),
	sqlf.Sprintf("repo_iterator.total_count"),
	sqlf.Sprintf("repo_iterator.success_count"),
	sqlf.Sprintf("repo_iterator.repos"),
	sqlf.Sprintf("repo_iterator.repo_cursor"),
}
var iteratorJoinCols = sqlf.Join(repoIteratorCols, ", ")

var repoIteratorErrorCols = []*sqlf.Query{
	sqlf.Sprintf("repo_iterator_errors.Id"),
	sqlf.Sprintf("repo_iterator_errors.repo_id"),
	sqlf.Sprintf("repo_iterator_errors.error_message"),
	sqlf.Sprintf("repo_iterator_errors.failure_count"),
}
var errorJoinCols = sqlf.Join(repoIteratorErrorCols, ", ")

// New returns a new (durable) repo iterator starting from cursor position 0.
func New(ctx context.Context, store *basestore.Store, repos []int32) (*PersistentRepoIterator, error) {
	return NewWithClock(ctx, store, glock.NewRealClock(), repos)
}

// NewWithClock returns a new (durable) repo iterator starting from cursor position 0 and optionally overrides the internal clock. Useful for tests.
func NewWithClock(ctx context.Context, store *basestore.Store, clock glock.Clock, repos []int32) (*PersistentRepoIterator, error) {
	q := "INSERT INTO repo_iterator(repos, total_count, created_at) VALUES (%S, %S, %S) RETURNING Id"
	id, err := basestore.ScanInt(store.QueryRow(ctx, sqlf.Sprintf(q, pq.Int32Array(repos), len(repos), clock.Now())))
	if err != nil {
		return nil, err
	}

	loaded, err := Load(ctx, store, id)
	if err != nil {
		return nil, err
	}
	loaded.glock = clock
	return loaded, nil
}

// Load will load a repo iterator that has been persisted and prepare it at the current cursor state.
func Load(ctx context.Context, store *basestore.Store, id int) (got *PersistentRepoIterator, err error) {
	return LoadWithClock(ctx, store, id, glock.NewRealClock())
}

func LoadWithClock(ctx context.Context, store *basestore.Store, id int, clock glock.Clock) (_ *PersistentRepoIterator, err error) {
	baseQuery := "SELECT %S FROM repo_iterator WHERE repo_iterator.Id = %S"
	row := store.QueryRow(ctx, sqlf.Sprintf(baseQuery, iteratorJoinCols, id))
	var repos pq.Int32Array
	var tmp PersistentRepoIterator
	if err = row.Scan(
		&tmp.Id,
		&tmp.CreatedAt,
		&dbutil.NullTime{Time: &tmp.StartedAt},
		&dbutil.NullTime{Time: &tmp.CompletedAt},
		&tmp.RuntimeDuration,
		&tmp.PercentComplete,
		&tmp.TotalCount,
		&tmp.SuccessCount,
		&repos,
		&tmp.Cursor,
	); err != nil {
		return nil, errors.Wrap(err, "ScanRepoIterator")
	}
	tmp.repos = repos
	if tmp.Cursor > len(tmp.repos) {
		return nil, errors.Newf("invalid repo iterator state Id:%d cursor:%d length:%d", tmp.Id, tmp.Cursor, len(repos))
	}

	tmp.errors, err = loadRepoIteratorErrors(ctx, store, &tmp)
	if err != nil {
		return nil, errors.Wrap(err, "loadRepoIteratorErrors")
	}
	tmp.terminalErrors = make(errorMap)

	tmp.glock = clock
	return &tmp, nil
}

type IterationConfig struct {
	MaxFailures int
	OnTerminal  OnTerminalFunc
}

type OnTerminalFunc func(ctx context.Context, store *basestore.Store, repoId int32, terminalErr error) error

// NextWithFinish will iterate the repository set from the current cursor position. If the iterator is marked complete
// or has no more repositories this will do nothing. The finish function returned is a mechanism to have atomic updates,
// callers will need to call the finish function when complete with work. Errors during work processing can be passed
// into the finish function and will be marked as errors on the repo iterator. Calling NextWithFinish without calling the
// finish function will infinitely loop on the current cursor. This iteration for a given repo iterator is not
// concurrency safe and should only be called from a single thread. Care should be taken to ensure in a distributed
// environment only one consumer is able to access this resource at a time.
func (p *PersistentRepoIterator) NextWithFinish(config IterationConfig) (api.RepoID, bool, FinishFunc) {
	current, got := peek(p.Cursor, p.repos)
	if !p.CompletedAt.IsZero() || !got {
		return 0, false, func(ctx context.Context, store *basestore.Store, err error) error {
			return nil
		}
	}
	itrStart := p.glock.Now()
	return api.RepoID(current), true, func(ctx context.Context, store *basestore.Store, maybeErr error) error {
		itrEnd := p.glock.Now()
		maybeErrs := map[int32]error{}
		if maybeErr != nil {
			maybeErrs[current] = maybeErr
		}
		if err := p.doFinishN(ctx, store, maybeErrs, []int32{current}, false, config, itrStart, itrEnd); err != nil {
			return err
		}
		return nil
	}
}

// NextPageWithFinish is like NextWithFinish but grabs the next pageSize number of repos.
func (p *PersistentRepoIterator) NextPageWithFinish(pageSize int, config IterationConfig) ([]api.RepoID, bool, FinishNFunc) {
	currentRepos, got := peekN(p.Cursor, pageSize, p.repos)
	if !p.CompletedAt.IsZero() || !got {
		return []api.RepoID{}, false, func(ctx context.Context, store *basestore.Store, maybeErrors map[int32]error) error {
			return nil
		}
	}
	itrStart := p.glock.Now()
	repoIds := make([]api.RepoID, 0, len(currentRepos))
	for i := range len(currentRepos) {
		repoIds = append(repoIds, api.RepoID(currentRepos[i]))
	}
	return repoIds, true, func(ctx context.Context, store *basestore.Store, maybeErrs map[int32]error) error {
		itrEnd := p.glock.Now()
		if err := p.doFinishN(ctx, store, maybeErrs, currentRepos, false, config, itrStart, itrEnd); err != nil {
			return err
		}
		return nil
	}
}

func (p *PersistentRepoIterator) NextRetryWithFinish(config IterationConfig) (api.RepoID, bool, FinishFunc) {
	if len(p.retryRepos) == 0 {
		p.resetRetry(config)
	}
	var current int32
	var got bool
	for {
		current, got = peek(p.retryCursor, p.retryRepos)
		if !p.CompletedAt.IsZero() || !got {
			return 0, false, func(ctx context.Context, store *basestore.Store, err error) error {
				return nil
			}
		} else if config.MaxFailures > 0 && p.errors.FailureCount(current) >= config.MaxFailures {
			// this repo has exceeded its retry count, skip it
			p.advanceRetry()
			p.setRepoTerminal(current)
			continue
		}
		break
	}

	itrStart := p.glock.Now()
	return api.RepoID(current), true, func(ctx context.Context, store *basestore.Store, maybeErr error) error {
		itrEnd := p.glock.Now()
		p.advanceRetry()
		maybeErrs := map[int32]error{}
		if maybeErr != nil {
			maybeErrs[current] = maybeErr
		}
		if err := p.doFinishN(ctx, store, maybeErrs, []int32{current}, true, config, itrStart, itrEnd); err != nil {
			return err
		}
		return nil
	}
}

// MarkComplete will mark the repo iterator as complete. Once marked complete the iterator is no longer eligible for iteration.
// This can be called at any time to mark the iterator as complete, and does not require the cursor have passed all the way through the set.
func (p *PersistentRepoIterator) MarkComplete(ctx context.Context, store *basestore.Store) error {
	now := p.glock.Now()
	err := store.Exec(ctx, sqlf.Sprintf("UPDATE repo_iterator SET percent_complete = 1, completed_at = %S, last_updated_at = %S where id = %s", now, now, p.Id))
	if err != nil {
		return err
	}
	p.CompletedAt = now
	p.PercentComplete = 1
	return nil
}

// Restart the iterator to the initial state.
func (p *PersistentRepoIterator) Restart(ctx context.Context, store *basestore.Store) (err error) {
	tx, err := store.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	err = tx.Exec(ctx, sqlf.Sprintf("UPDATE repo_iterator SET percent_complete = 0, runtime_duration = 0, success_count = 0, repo_cursor = 0, completed_at = null, started_at = null, last_updated_at = now() where id = %s", p.Id))
	if err != nil {
		return err
	}

	err = tx.Exec(ctx, sqlf.Sprintf("DELETE FROM repo_iterator_errors WHERE id = %s", p.Id))
	if err != nil {
		return err
	}
	p.CompletedAt = time.Time{}
	p.StartedAt = time.Time{}
	p.PercentComplete = 0
	p.retryCursor = 0
	p.retryCursor = 0
	p.retryRepos = []int32{}
	p.terminalErrors = make(errorMap)
	p.Cursor = 0
	p.RuntimeDuration = 0
	p.SuccessCount = 0
	p.errors = make(errorMap)

	return nil
}

func (p *PersistentRepoIterator) HasMore() bool {
	_, has := peek(p.Cursor, p.repos)
	return has
}

func (p *PersistentRepoIterator) HasErrors() bool {
	return len(p.errors) > 0
}

func (p *PersistentRepoIterator) HasTerminalErrors() bool {
	return len(p.errors) > 0
}

func (p *PersistentRepoIterator) ErroredRepos() int {
	return len(p.errors)
}

func (p *PersistentRepoIterator) TotalErrors() int {
	count := 0
	for _, iterationError := range p.errors {
		count += iterationError.FailureCount
	}
	for _, iterationError := range p.terminalErrors {
		count += iterationError.FailureCount
	}
	return count
}

func (p *PersistentRepoIterator) Errors() []IterationError {
	itErrors := []IterationError{}
	for _, iterationError := range p.errors {
		itErrors = append(itErrors, *iterationError)
	}
	return itErrors
}

func stampStartedAt(ctx context.Context, store *basestore.Store, itrId int, stampTime time.Time) error {
	return store.Exec(ctx, sqlf.Sprintf("UPDATE repo_iterator SET started_at = %S WHERE Id = %S", stampTime, itrId))
}

func peek(offset int, repos []int32) (int32, bool) {
	if offset >= len(repos) {
		return 0, false
	}
	return repos[offset], true
}

func peekN(offset, num int, repos []int32) ([]int32, bool) {
	if offset >= len(repos) {
		return []int32{}, false
	}
	end := int32(math.Min(float64(offset+num), float64(len(repos))))
	return repos[offset:end], true
}

func (p *PersistentRepoIterator) advanceRetry() {
	p.retryCursor += 1
}

func (p *PersistentRepoIterator) insertIterationError(ctx context.Context, store *basestore.Store, repoId int32, msg string) (err error) {
	var query *sqlf.Query
	if p.Id == 0 {
		return errors.New("invalid iterator to insert iterator error")
	}

	v, ok := p.errors[repoId]
	if !ok {
		// The db defaults the failure count to 1
		query = sqlf.Sprintf("INSERT INTO repo_iterator_errors(repo_iterator_id, repo_id, error_message) VALUES (%S, %S, %S) RETURNING %S", p.Id, repoId, pq.Array([]string{msg}), errorJoinCols)
		row := store.QueryRow(ctx, query)
		var tmp IterationError
		if err = row.Scan(
			&tmp.id,
			&tmp.RepoId,
			pq.Array(&tmp.ErrorMessages),
			&tmp.FailureCount,
		); err != nil {
			return errors.Wrap(err, "InsertIterationError")
		}
		p.errors[tmp.RepoId] = &tmp
	} else {
		v.FailureCount += 1
		query = sqlf.Sprintf("UPDATE repo_iterator_errors SET failure_count = %S, error_message = array_append(error_message, %S) WHERE Id = %S", v.FailureCount, msg, v.id)
		if err = store.Exec(ctx, query); err != nil {
			return errors.Wrap(err, "UpdateIterationError")
		}
	}
	return nil
}

func (p *PersistentRepoIterator) doFinishN(ctx context.Context, store *basestore.Store, maybeErrs map[int32]error, repos []int32, isRetry bool, config IterationConfig, start, end time.Time) (err error) {
	cursorOffset := len(repos)
	errorsCount := 0
	for _, repoErr := range maybeErrs {
		if repoErr != nil {
			errorsCount++
		}
	}
	successfulRepoCount := int(math.Max(float64(len(repos)-errorsCount), 0))
	if isRetry {
		cursorOffset = 0
	}
	itrDuration := end.Sub(start)

	tx, err := store.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	if p.StartedAt.IsZero() {
		if err = stampStartedAt(ctx, tx, p.Id, start); err != nil {
			return errors.Wrap(err, "stampStartedAt")
		}
		p.StartedAt = start
	}
	// This updates the iterator moving it ahead by the current offset (number of repos being "finished")
	err = p.updateRepoIterator(ctx, tx, successfulRepoCount, cursorOffset, itrDuration)

	// For each repo that is being finished check if it errored
	//    If errored - record the error
	//    No error - clear any previous errors since it it now successful
	for _, repoID := range repos {
		if maybeErr, ok := maybeErrs[repoID]; ok && maybeErr != nil {
			if err = p.insertIterationError(ctx, tx, repoID, maybeErr.Error()); err != nil {
				return errors.Wrapf(err, "unable to upsert error for repo iterator id: %d", p.Id)
			}
			if config.MaxFailures != 0 && p.errors.FailureCount(repoID) >= config.MaxFailures {
				// the condition is if there was an error, and we have configured both a max attempts, and the total attempts exceeds the config
				if config.OnTerminal != nil {
					err = config.OnTerminal(ctx, tx, repoID, maybeErr)
					if err != nil {
						return errors.Wrap(err, "iterator.OnTerminal")
					}
					p.setRepoTerminal(repoID)
				}
			}
		} else if isRetry {
			// delete the error for this repo
			err = tx.Exec(ctx, sqlf.Sprintf(`DELETE FROM repo_iterator_errors WHERE id = %s`, p.errors[repoID].id))
			if err != nil {
				return errors.Wrap(err, "deleteIteratorError")
			}
			delete(p.errors, repoID)
		}
	}

	return nil
}

// setRepoTerminal sets a repository to a terminal error state
func (p *PersistentRepoIterator) setRepoTerminal(repoId int32) {
	p.terminalErrors[repoId] = p.errors[repoId]
	delete(p.errors, repoId)
}

func (p *PersistentRepoIterator) updateRepoIterator(ctx context.Context, store *basestore.Store, successCount, cursorOffset int, duration time.Duration) error {
	updateQ := `UPDATE repo_iterator
    SET percent_complete = COALESCE(((%s + success_count)::float / NULLIF(total_count, 0)::float), 0),
    success_count    = success_count + %s,
    repo_cursor      = repo_cursor + %s,
    last_updated_at  = NOW(),
    runtime_duration = runtime_duration + %s
    WHERE id = %s RETURNING percent_complete, success_count, repo_cursor, runtime_duration;`

	q := sqlf.Sprintf(updateQ, successCount, successCount, cursorOffset, duration, p.Id)

	var pct float64
	var successCnt int
	var cursor int
	var runtime time.Duration

	row := store.QueryRow(ctx, q)
	if err := row.Scan(
		&pct,
		&successCnt,
		&cursor,
		&runtime,
	); err != nil {
		return errors.Wrapf(err, "unable to update repo iterator id: %d", p.Id)
	}

	p.Cursor = cursor
	p.SuccessCount = successCnt
	p.PercentComplete = pct
	p.RuntimeDuration = runtime

	return nil
}

func loadRepoIteratorErrors(ctx context.Context, store *basestore.Store, iterator *PersistentRepoIterator) (got errorMap, err error) {
	baseQuery := "SELECT %S FROM repo_iterator_errors WHERE repo_iterator_id = %S"
	rows, err := store.Query(ctx, sqlf.Sprintf(baseQuery, errorJoinCols, iterator.Id))
	if err != nil {
		return nil, err
	}
	got = make(errorMap)
	for rows.Next() {
		var tmp IterationError
		if err := rows.Scan(
			&tmp.id,
			&tmp.RepoId,
			pq.Array(&tmp.ErrorMessages),
			&tmp.FailureCount,
		); err != nil {
			return nil, err
		}
		got[tmp.RepoId] = &tmp
	}

	return got, err
}

func (p *PersistentRepoIterator) resetRetry(config IterationConfig) {
	p.retryCursor = 0
	p.terminalErrors = make(errorMap)
	var retry []int32
	for repo, val := range p.errors {
		if config.MaxFailures > 0 && val.FailureCount >= config.MaxFailures {
			p.terminalErrors[repo] = val
			delete(p.errors, repo)
			continue
		}
		retry = append(retry, repo)
	}
	p.retryRepos = retry
}
