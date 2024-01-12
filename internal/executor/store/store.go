package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/hashutil"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// JobTokenStore is the store for interacting with the executor_job_tokens table.
type JobTokenStore interface {
	// Create creates a new JobToken.
	Create(ctx context.Context, jobId int, queue string, repo string) (string, error)
	// Regenerate creates a new value for the matching JobToken.
	Regenerate(ctx context.Context, jobId int, queue string) (string, error)
	// Exists checks if the JobToken exists.
	Exists(ctx context.Context, jobId int, queue string) (bool, error)
	// Get retrieves the JobToken matching the specified values.
	Get(ctx context.Context, jobId int, queue string) (JobToken, error)
	// GetByToken retrieves the JobToken matching the value of token.
	GetByToken(ctx context.Context, tokenHexEncoded string) (JobToken, error)
	// Delete deletes the matching JobToken.
	Delete(ctx context.Context, jobId int, queue string) error
}

// JobToken is the token for the specific Job.
type JobToken struct {
	Id     int64
	Value  []byte
	JobID  int64
	Queue  string
	RepoID int64
	Repo   string
}

type jobTokenStore struct {
	*basestore.Store
	logger         log.Logger
	operations     *operations
	observationCtx *observation.Context
}

// NewJobTokenStore creates a new JobTokenStore.
func NewJobTokenStore(observationCtx *observation.Context, db database.DB) JobTokenStore {
	return &jobTokenStore{
		Store:          basestore.NewWithHandle(db.Handle()),
		logger:         observationCtx.Logger,
		operations:     newOperations(observationCtx),
		observationCtx: observationCtx,
	}
}

func (s *jobTokenStore) Create(ctx context.Context, jobId int, queue string, repo string) (string, error) {
	if jobId == 0 {
		return "", errors.New("missing jobId")
	}
	if len(queue) == 0 {
		return "", errors.New("missing queue")
	}
	if len(repo) == 0 {
		return "", errors.New("missing repo")
	}

	var b [20]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	err := s.Exec(
		ctx,
		sqlf.Sprintf(
			createExecutorJobTokenFmtstr,
			hashutil.ToSHA256Bytes(b[:]), jobId, queue, repo,
		),
	)
	if err != nil {
		if isUniqueConstraintViolation(err, "executor_job_tokens_job_id_queue_repo_id_key") {
			return "", ErrJobTokenAlreadyCreated
		}
		return "", err
	}

	return hex.EncodeToString(b[:]), nil
}

const createExecutorJobTokenFmtstr = `
INSERT INTO executor_job_tokens (value_sha256, job_id, queue, repo_id)
SELECT %s, %s, %s, id from repo r where r.name = %s;
`

func isUniqueConstraintViolation(err error, constraintName string) bool {
	var e *pgconn.PgError
	return errors.As(err, &e) && e.Code == "23505" && e.ConstraintName == constraintName
}

// ErrJobTokenAlreadyCreated is a specific error when a token has already been created for a Job.
var ErrJobTokenAlreadyCreated = errors.New("job token already exists")

func (s *jobTokenStore) Regenerate(ctx context.Context, jobId int, queue string) (string, error) {
	var b [20]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	err := s.Exec(
		ctx,
		sqlf.Sprintf(
			regenerateExecutorJobTokenFmtstr,
			hashutil.ToSHA256Bytes(b[:]), jobId, queue,
		),
	)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(b[:]), nil
}

const regenerateExecutorJobTokenFmtstr = `
UPDATE executor_job_tokens SET value_sha256 = %s, updated_at = NOW()
WHERE job_id = %s AND queue = %s
`

func (s *jobTokenStore) Exists(ctx context.Context, jobId int, queue string) (bool, error) {
	exists, _, err := basestore.ScanFirstBool(s.Query(ctx, sqlf.Sprintf(existsExecutorJobTokenFmtstr, jobId, queue)))
	return exists, err
}

const existsExecutorJobTokenFmtstr = `
SELECT EXISTS(SELECT 1 FROM executor_job_tokens WHERE job_id=%s AND queue=%s)
`

func (s *jobTokenStore) Get(ctx context.Context, jobId int, queue string) (JobToken, error) {
	row := s.QueryRow(
		ctx,
		sqlf.Sprintf(
			getExecutorJobTokenFmtstr,
			jobId, queue,
		),
	)
	return scanJobToken(row)
}

const getExecutorJobTokenFmtstr = `
SELECT id, value_sha256, job_id, queue, repo_id, (select name from repo where id = t.repo_id) as repo
FROM executor_job_tokens t
WHERE job_id = %s AND queue = %s
`

func (s *jobTokenStore) GetByToken(ctx context.Context, tokenHexEncoded string) (JobToken, error) {
	token, err := hex.DecodeString(tokenHexEncoded)
	if err != nil {
		return JobToken{}, errors.New("invalid token")
	}
	row := s.QueryRow(
		ctx,
		sqlf.Sprintf(
			getByTokenExecutorJobTokenFmtstr,
			hashutil.ToSHA256Bytes(token),
		),
	)
	return scanJobToken(row)
}

const getByTokenExecutorJobTokenFmtstr = `
SELECT id, value_sha256, job_id, queue, repo_id, (select name from repo where id = t.repo_id) as repo
FROM executor_job_tokens t
WHERE value_sha256 = %s
`

func scanJobToken(row *sql.Row) (JobToken, error) {
	jobToken := JobToken{}
	err := row.Scan(
		&jobToken.Id,
		&jobToken.Value,
		&jobToken.JobID,
		&jobToken.Queue,
		&jobToken.RepoID,
		&jobToken.Repo,
	)
	if err != nil {
		return jobToken, err
	}
	return jobToken, nil
}

func (s *jobTokenStore) Delete(ctx context.Context, jobId int, queue string) error {
	return s.Store.Exec(ctx, sqlf.Sprintf(deleteExecutorJobTokenFmtstr, jobId, queue))
}

const deleteExecutorJobTokenFmtstr = `
DELETE FROM executor_job_tokens WHERE job_id = %s AND queue = %s
`
