package database

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/jackc/pgconn"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"google.golang.org/protobuf/proto"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	codeownerspb "github.com/sourcegraph/sourcegraph/internal/own/codeowners/v1"
	"github.com/sourcegraph/sourcegraph/internal/own/types"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type CodeownersStore interface {
	basestore.ShareableStore
	Done(error) error

	// CreateCodeownersFile creates a given Codeowners file in the database.
	CreateCodeownersFile(ctx context.Context, codeowners *types.CodeownersFile) error
	// UpdateCodeownersFile updates a manually ingested Codeowners file in the database, matched by repo.
	UpdateCodeownersFile(ctx context.Context, codeowners *types.CodeownersFile) error
	// GetCodeownersForRepo gets a manually ingested Codeowners file for the given repo if it exists.
	GetCodeownersForRepo(ctx context.Context, id api.RepoID) (*types.CodeownersFile, error)
	// DeleteCodeownersForRepos deletes manually ingested Codeowners files for the given repos if it exists.
	DeleteCodeownersForRepos(ctx context.Context, ids ...api.RepoID) error
	// ListCodeowners lists manually ingested Codeowners files given the options.
	ListCodeowners(ctx context.Context, opts ListCodeownersOpts) ([]*types.CodeownersFile, int32, error)
	// CountCodeownersFiles counts the number of manually ingested Codeowners files.
	CountCodeownersFiles(context.Context) (int32, error)
}

type codeownersStore struct {
	*basestore.Store
}

type CodeownersFileNotFoundError struct {
	args any
}

func (e CodeownersFileNotFoundError) Error() string {
	return fmt.Sprintf("codeowners file not found: %v", e.args)
}

func (CodeownersFileNotFoundError) NotFound() bool {
	return true
}

var ErrCodeownersFileAlreadyExists = errors.New("codeowners file has already been ingested for this repository")

func (s *codeownersStore) CreateCodeownersFile(ctx context.Context, file *types.CodeownersFile) error {
	return s.WithTransact(ctx, func(tx CodeownersStore) error {
		if file.CreatedAt.IsZero() {
			file.CreatedAt = timeutil.Now()
		}
		if file.UpdatedAt.IsZero() {
			file.UpdatedAt = file.CreatedAt
		}

		protoBytes, err := proto.Marshal(file.Proto)
		if err != nil {
			return err
		}

		q := sqlf.Sprintf(
			createCodeownersQueryFmtStr,
			sqlf.Join(codeownersColumns, ","),
			file.Contents,
			protoBytes,
			file.RepoID,
			file.CreatedAt,
			file.UpdatedAt,
		)

		if _, err := tx.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
			var e *pgconn.PgError
			if errors.As(err, &e) {
				switch e.ConstraintName {
				case "codeowners_repo_id_key":
					return ErrCodeownersFileAlreadyExists
				}
			}
			return err
		}
		return nil
	})
}

var codeownersColumns = []*sqlf.Query{
	sqlf.Sprintf("contents"),
	sqlf.Sprintf("contents_proto"),
	sqlf.Sprintf("repo_id"),
	sqlf.Sprintf("created_at"),
	sqlf.Sprintf("updated_at"),
}

const createCodeownersQueryFmtStr = `
INSERT INTO codeowners
(%s)
VALUES (%s, %s, %s, %s, %s)
`

func (s *codeownersStore) UpdateCodeownersFile(ctx context.Context, file *types.CodeownersFile) error {
	return s.WithTransact(ctx, func(tx CodeownersStore) error {
		if file.UpdatedAt.IsZero() {
			file.UpdatedAt = timeutil.Now()
		}

		conds := []*sqlf.Query{
			sqlf.Sprintf("repo_id = %s", file.RepoID),
		}

		protoBytes, err := proto.Marshal(file.Proto)
		if err != nil {
			return err
		}

		q := sqlf.Sprintf(
			updateCodeownersQueryFmtStr,
			file.Contents,
			protoBytes,
			file.UpdatedAt,
			sqlf.Join(conds, "AND"),
		)

		res, err := tx.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return CodeownersFileNotFoundError{args: file.RepoID}
		}
		return nil
	})
}

const updateCodeownersQueryFmtStr = `
UPDATE codeowners
SET
    contents = %s,
    contents_proto = %s,
    updated_at = %s
WHERE
    %s
`

func (s *codeownersStore) GetCodeownersForRepo(ctx context.Context, id api.RepoID) (*types.CodeownersFile, error) {
	q := sqlf.Sprintf(
		getCodeownersFileQueryFmtStr,
		sqlf.Join(codeownersColumns, ", "),
		sqlf.Sprintf("repo_id = %s", id),
	)
	codeownersFiles, err := scanCodeowners(s.Query(ctx, q))
	if err != nil {
		return nil, err
	}
	if len(codeownersFiles) != 1 {
		return nil, CodeownersFileNotFoundError{args: id}
	}
	return codeownersFiles[0], nil
}

const getCodeownersFileQueryFmtStr = `
SELECT %s
FROM codeowners
WHERE %s
LIMIT 1
`

func (s *codeownersStore) DeleteCodeownersForRepos(ctx context.Context, ids ...api.RepoID) error {
	return s.WithTransact(ctx, func(tx CodeownersStore) error {
		conds := []*sqlf.Query{
			sqlf.Sprintf(
				"repo_id = ANY (%s)",
				// We use this trick to convert the []api.RepoID to []int32 (the underlying
				// type of api.RepoID). This is safe because the underlying type of api.RepoID
				// is always int32.
				// When passing an []api.RepoID to pq.Array directly, a more costly
				// conversion when Value is called is required.
				pq.Array(*(*[]int32)(unsafe.Pointer(&ids))),
			),
		}

		q := sqlf.Sprintf(deleteCodeownersFileQueryFmtStr, sqlf.Join(conds, "AND"))

		res, err := tx.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return CodeownersFileNotFoundError{args: conds}
		}
		return nil
	})
}

const deleteCodeownersFileQueryFmtStr = `
DELETE FROM codeowners
WHERE %s
`

type ListCodeownersOpts struct {
	*LimitOffset

	// Only return codeowners past this cursor (repoID).
	Cursor int32
}

func (s *codeownersStore) ListCodeowners(ctx context.Context, opts ListCodeownersOpts) (_ []*types.CodeownersFile, next int32, err error) {
	if opts.LimitOffset != nil && opts.Limit > 0 {
		opts.Limit++
	}
	where := []*sqlf.Query{
		sqlf.Sprintf("repo_id >= %s", opts.Cursor),
	}

	q := sqlf.Sprintf(
		listCodeownersFilesQueryFmtStr,
		sqlf.Join(codeownersColumns, ","),
		sqlf.Join(where, "AND"),
		opts.LimitOffset.SQL(),
	)

	codeownersFiles, err := scanCodeowners(s.Query(ctx, q))
	if err != nil {
		return nil, 0, err
	}

	if opts.LimitOffset != nil && opts.Limit > 0 && len(codeownersFiles) == opts.Limit {
		next = int32(codeownersFiles[len(codeownersFiles)-1].RepoID)
		codeownersFiles = codeownersFiles[:len(codeownersFiles)-1]
	}

	return codeownersFiles, next, nil
}

const listCodeownersFilesQueryFmtStr = `
SELECT %s
FROM codeowners
WHERE %s
ORDER BY
    repo_id ASC
%s
`

func (s *codeownersStore) CountCodeownersFiles(ctx context.Context) (int32, error) {
	q := sqlf.Sprintf(countCodeownersFilesQueryFmtStr)

	count, _, err := basestore.ScanFirstInt(s.Query(ctx, q))
	return int32(count), err
}

const countCodeownersFilesQueryFmtStr = `
SELECT COUNT(*)
FROM codeowners
`

func CodeownersWith(other basestore.ShareableStore) CodeownersStore {
	return &codeownersStore{
		Store: basestore.NewWithHandle(other.Handle()),
	}
}

func (s *codeownersStore) With(other basestore.ShareableStore) CodeownersStore {
	return &codeownersStore{
		Store: s.Store.With(other),
	}
}

func (s *codeownersStore) WithTransact(ctx context.Context, f func(store CodeownersStore) error) error {
	return s.Store.WithTransact(ctx, func(tx *basestore.Store) error {
		return f(&codeownersStore{
			Store: tx,
		})
	})
}

var scanCodeowners = basestore.NewSliceScanner(func(s dbutil.Scanner) (*types.CodeownersFile, error) {
	var c types.CodeownersFile
	c.Proto = new(codeownerspb.File)
	err := scanCodeownersRow(s, &c)
	return &c, err
})

func scanCodeownersRow(sc dbutil.Scanner, c *types.CodeownersFile) error {
	var protoBytes []byte
	if err := sc.Scan(
		&c.Contents,
		&protoBytes,
		&c.RepoID,
		&c.CreatedAt,
		&c.UpdatedAt,
	); err != nil {
		return err
	}
	return proto.Unmarshal(protoBytes, c.Proto)
}
