package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgconn"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// ExecutorSecret represents a row in the `executor_secrets` table.
type ExecutorSecret struct {
	ID                     int64
	Key                    string
	Scope                  ExecutorSecretScope
	OverwritesGlobalSecret bool
	CreatorID              int32
	NamespaceUserID        int32
	NamespaceOrgID         int32

	CreatedAt time.Time
	UpdatedAt time.Time

	// unexported so that there's no direct access. Use `Value` to access it
	// which will generate the access log entries as well.
	encryptedValue *encryption.Encryptable
}

type ExecutorSecretAccessLogCreator interface {
	Create(ctx context.Context, log *ExecutorSecretAccessLog) error
}

// Value decrypts the contained value and logs an access log event. Calling Value
// multiple times will not require another decryption call, but will create an
// additional access log entry.
func (e ExecutorSecret) Value(ctx context.Context, s ExecutorSecretAccessLogCreator) (string, error) {
	var userID *int32
	if uid := actor.FromContext(ctx).UID; uid != 0 {
		userID = &uid
	}
	if err := s.Create(ctx, &ExecutorSecretAccessLog{
		ExecutorSecretID: e.ID,
		UserID:           userID,
	}); err != nil {
		return "", errors.Wrap(err, "creating secret access log entry")
	}
	return e.encryptedValue.Decrypt(ctx)
}

type ExecutorSecretScope string

const (
	ExecutorSecretScopeBatches   ExecutorSecretScope = "batches"
	ExecutorSecretScopeCodeIntel ExecutorSecretScope = "codeintel"
)

// ExecutorSecretNotFoundErr is returned when a secret cannot be found.
type ExecutorSecretNotFoundErr struct {
	id int64
}

func (err ExecutorSecretNotFoundErr) Error() string {
	return fmt.Sprintf("executor secret not found: id=%d", err.id)
}

func (ExecutorSecretNotFoundErr) NotFound() bool {
	return true
}

// ExecutorSecretStore provides access to the `executor_secrets` table.
type ExecutorSecretStore interface {
	basestore.ShareableStore
	With(basestore.ShareableStore) ExecutorSecretStore
	WithTransact(context.Context, func(ExecutorSecretStore) error) error
	Done(err error) error
	ExecResult(ctx context.Context, query *sqlf.Query) (sql.Result, error)

	// Create inserts the given ExecutorSecret into the database.
	Create(ctx context.Context, scope ExecutorSecretScope, secret *ExecutorSecret, value string) error
	// Update updates a secret in the database. If the secret cannot be found,
	// an error is returned.
	Update(ctx context.Context, scope ExecutorSecretScope, secret *ExecutorSecret, value string) error
	// Delete deletes the given executor secret.
	Delete(ctx context.Context, scope ExecutorSecretScope, id int64) error
	// GetByID returns the executor secret matching the given ID, or
	// ExecutorSecretNotFoundErr if no such secret exists.
	GetByID(ctx context.Context, scope ExecutorSecretScope, id int64) (*ExecutorSecret, error)
	// List returns all secrets matching the given options.
	List(context.Context, ExecutorSecretScope, ExecutorSecretsListOpts) ([]*ExecutorSecret, int, error)
	// Count counts all secrets matching the given options.
	Count(context.Context, ExecutorSecretScope, ExecutorSecretsListOpts) (int, error)
}

// ExecutorSecretsListOpts provide the options when listing secrets. If no namespace
// scoping is provided, only global credentials are returned (no namespace set).
type ExecutorSecretsListOpts struct {
	*LimitOffset

	// Keys, if set limits the returned secrets to the list of provided keys.
	Keys []string

	// NamespaceUserID, when set, returns secrets accessible in the user namespace.
	// These may include global secrets.
	NamespaceUserID int32
	// NamespaceOrgID, when set, returns secrets accessible in the user namespace.
	// These may include global secrets.
	NamespaceOrgID int32
}

func (opts ExecutorSecretsListOpts) sqlConds(ctx context.Context, scope ExecutorSecretScope) *sqlf.Query {
	authz := executorSecretsAuthzQueryConds(ctx)

	globalSecret := sqlf.Sprintf("namespace_user_id IS NULL AND namespace_org_id IS NULL")

	preds := []*sqlf.Query{
		authz,
		sqlf.Sprintf("scope = %s", scope),
	}

	if opts.NamespaceOrgID != 0 {
		preds = append(preds, sqlf.Sprintf("(namespace_org_id = %s OR (%s))", opts.NamespaceOrgID, globalSecret))
	} else if opts.NamespaceUserID != 0 {
		preds = append(preds, sqlf.Sprintf("(namespace_user_id = %s OR (%s))", opts.NamespaceUserID, globalSecret))
	} else {
		preds = append(preds, globalSecret)
	}

	if len(opts.Keys) > 0 {
		preds = append(preds, sqlf.Sprintf("key = ANY(%s)", pq.Array(opts.Keys)))
	}

	return sqlf.Join(preds, "\n AND ")
}

// limitSQL overrides LimitOffset.SQL() to give a LIMIT clause with one extra value
// so we can populate the next cursor.
func (opts *ExecutorSecretsListOpts) limitSQL() *sqlf.Query {
	if opts.LimitOffset == nil || opts.Limit == 0 {
		return &sqlf.Query{}
	}

	return (&LimitOffset{Limit: opts.Limit + 1, Offset: opts.Offset}).SQL()
}

type executorSecretStore struct {
	logger log.Logger
	*basestore.Store
	key encryption.Key
}

// ExecutorSecretsWith instantiates and returns a new ExecutorSecretStore using the other store handle.
func ExecutorSecretsWith(logger log.Logger, other basestore.ShareableStore, key encryption.Key) ExecutorSecretStore {
	return &executorSecretStore{
		logger: logger,
		Store:  basestore.NewWithHandle(other.Handle()),
		key:    key,
	}
}

func (s *executorSecretStore) With(other basestore.ShareableStore) ExecutorSecretStore {
	return &executorSecretStore{
		logger: s.logger,
		Store:  s.Store.With(other),
		key:    s.key,
	}
}

func (s *executorSecretStore) Transact(ctx context.Context) (ExecutorSecretStore, error) {
	txBase, err := s.Store.Transact(ctx)
	return &executorSecretStore{
		logger: s.logger,
		Store:  txBase,
		key:    s.key,
	}, err
}

func (s *executorSecretStore) WithTransact(ctx context.Context, f func(tx ExecutorSecretStore) error) error {
	return s.Store.WithTransact(ctx, func(tx *basestore.Store) error {
		return f(&executorSecretStore{
			logger: s.logger,
			Store:  tx,
			key:    s.key,
		})
	})
}

var (
	ErrEmptyExecutorSecretKey   = errors.New("empty executor secret key is not allowed")
	ErrEmptyExecutorSecretValue = errors.New("empty executor secret value is not allowed")
)

var ErrDuplicateExecutorSecret = errors.New("duplicate executor secret")

func (s *executorSecretStore) Create(ctx context.Context, scope ExecutorSecretScope, secret *ExecutorSecret, value string) error {
	if len(secret.Key) == 0 {
		return ErrEmptyExecutorSecretKey
	}

	if len(value) == 0 {
		return ErrEmptyExecutorSecretValue
	}

	// SECURITY: check that the current user is authorized to create a secret for the given namespace.
	if err := EnsureActorHasNamespaceWriteAccess(ctx, NewDBWith(s.logger, s), secret); err != nil {
		return err
	}

	// Set the current actor as the secret creator if not set.
	if secret.CreatorID == 0 {
		secret.CreatorID = actor.FromContext(ctx).UID
	}

	encryptedValue, keyID, err := encryptExecutorSecret(ctx, s.key, value)
	if err != nil {
		return err
	}

	q := sqlf.Sprintf(
		executorSecretCreateQueryFmtstr,
		scope,
		secret.Key,
		encryptedValue, // N.B.: is already a []byte
		keyID,
		dbutil.NewNullInt(int(secret.NamespaceUserID)),
		dbutil.NewNullInt(int(secret.NamespaceOrgID)),
		secret.CreatorID,
		sqlf.Join(executorSecretsColumns, ", "),
	)

	row := s.QueryRow(ctx, q)
	if err := scanExecutorSecret(secret, s.key, row); err != nil {
		var e *pgconn.PgError
		if errors.As(err, &e) && e.Code == "23505" {
			return ErrDuplicateExecutorSecret
		}
		return err
	}

	return nil
}

func (s *executorSecretStore) Update(ctx context.Context, scope ExecutorSecretScope, secret *ExecutorSecret, value string) error {
	if len(value) == 0 {
		return ErrEmptyExecutorSecretValue
	}

	// SECURITY: check that the current user is authorized to update a secret in the given namespace.
	if err := EnsureActorHasNamespaceWriteAccess(ctx, NewDBWith(s.logger, s), secret); err != nil {
		return err
	}

	secret.UpdatedAt = timeutil.Now()
	encryptedValue, keyID, err := encryptExecutorSecret(ctx, s.key, value)
	if err != nil {
		return err
	}

	authz := executorSecretsAuthzQueryConds(ctx)

	q := sqlf.Sprintf(
		executorSecretUpdateQueryFmtstr,
		encryptedValue,
		keyID,
		secret.UpdatedAt,
		secret.ID,
		scope,
		authz,
		sqlf.Join(executorSecretsColumns, ", "),
	)

	row := s.QueryRow(ctx, q)
	if err := scanExecutorSecret(secret, s.key, row); err != nil {
		return err
	}

	return nil
}

func (s *executorSecretStore) Delete(ctx context.Context, scope ExecutorSecretScope, id int64) error {
	return s.WithTransact(ctx, func(tx ExecutorSecretStore) error {
		secret, err := tx.GetByID(ctx, scope, id)
		if err != nil {
			return err
		}

		// SECURITY: check that the current user is authorized to delete a secret in the given namespace.
		if err := EnsureActorHasNamespaceWriteAccess(ctx, NewDBWith(s.logger, tx), secret); err != nil {
			return err
		}

		authz := executorSecretsAuthzQueryConds(ctx)

		q := sqlf.Sprintf("DELETE FROM executor_secrets WHERE id = %s AND scope = %s AND %s", id, scope, authz)
		res, err := tx.ExecResult(ctx, q)
		if err != nil {
			return err
		}

		if rows, err := res.RowsAffected(); err != nil {
			return err
		} else if rows == 0 {
			return ExecutorSecretNotFoundErr{id: id}
		}

		return nil
	})
}

func (s *executorSecretStore) GetByID(ctx context.Context, scope ExecutorSecretScope, id int64) (*ExecutorSecret, error) {
	authz := executorSecretsAuthzQueryConds(ctx)

	q := sqlf.Sprintf(
		"SELECT %s FROM executor_secrets WHERE id = %s AND %s",
		sqlf.Join(executorSecretsColumns, ", "),
		id,
		authz,
	)

	secret := ExecutorSecret{}
	row := s.QueryRow(ctx, q)
	if err := scanExecutorSecret(&secret, s.key, row); err == sql.ErrNoRows {
		return nil, ExecutorSecretNotFoundErr{id: id}
	} else if err != nil {
		return nil, err
	}

	return &secret, nil
}

func (s *executorSecretStore) List(ctx context.Context, scope ExecutorSecretScope, opts ExecutorSecretsListOpts) ([]*ExecutorSecret, int, error) {
	conds := opts.sqlConds(ctx, scope)

	q := sqlf.Sprintf(
		executorSecretsListQueryFmtstr,
		sqlf.Join(executorSecretsColumns, ", "),
		sqlf.Join(executorSecretsColumns, ", "),
		conds,
		opts.limitSQL(),
	)

	rows, err := s.Query(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	var secrets []*ExecutorSecret
	for rows.Next() {
		secret := ExecutorSecret{}
		if err := scanExecutorSecret(&secret, s.key, rows); err != nil {
			return nil, 0, err
		}
		secrets = append(secrets, &secret)
	}

	// Check if there were more results than the limit: if so, then we need to
	// set the return cursor and lop off the extra secret that we retrieved.
	next := 0
	if opts.LimitOffset != nil && opts.Limit != 0 && len(secrets) == opts.Limit+1 {
		next = opts.Offset + opts.Limit
		secrets = secrets[:len(secrets)-1]
	}

	return secrets, next, nil
}

func (s *executorSecretStore) Count(ctx context.Context, scope ExecutorSecretScope, opts ExecutorSecretsListOpts) (int, error) {
	conds := opts.sqlConds(ctx, scope)

	q := sqlf.Sprintf(
		executorSecretsCountQueryFmtstr,
		conds,
	)

	totalCount, _, err := basestore.ScanFirstInt(s.Query(ctx, q))
	if err != nil {
		return 0, err
	}

	return totalCount, nil
}

// executorSecretsColumns are the columns that must be selected by
// executor_secrets queries in order to use scanExecutorSecret().
var executorSecretsColumns = []*sqlf.Query{
	sqlf.Sprintf("id"),
	sqlf.Sprintf("scope"),
	sqlf.Sprintf("key"),
	sqlf.Sprintf("value"),
	sqlf.Sprintf("encryption_key_id"),
	sqlf.Sprintf("COALESCE((SELECT o.id FROM executor_secrets o WHERE o.key = executor_secrets.key AND o.scope = executor_secrets.scope AND o.namespace_user_id IS NULL AND o.namespace_org_id IS NULL AND o.id != executor_secrets.id)::boolean, false) AS overwrites_global"),
	sqlf.Sprintf("namespace_user_id"),
	sqlf.Sprintf("namespace_org_id"),
	sqlf.Sprintf("creator_id"),
	sqlf.Sprintf("created_at"),
	sqlf.Sprintf("updated_at"),
}

const executorSecretsListQueryFmtstr = `
SELECT %s
FROM (
	SELECT
		%s,
		RANK() OVER(
			PARTITION BY key
			ORDER BY
				namespace_user_id NULLS LAST,
				namespace_org_id NULLS LAST
		)
	FROM executor_secrets
	WHERE %s
) executor_secrets
WHERE
	executor_secrets.rank = 1
ORDER BY key ASC
%s  -- LIMIT clause
`

const executorSecretsCountQueryFmtstr = `
SELECT COUNT(*)
FROM (
	SELECT
		RANK() OVER(
			PARTITION BY key
			ORDER BY
				namespace_user_id NULLS LAST,
				namespace_org_id NULLS LAST
		)
	FROM executor_secrets
	WHERE %s
) executor_secrets
WHERE
	executor_secrets.rank = 1
`

const executorSecretCreateQueryFmtstr = `
INSERT INTO
	executor_secrets (
		scope,
		key,
		value,
		encryption_key_id,
		namespace_user_id,
		namespace_org_id,
		creator_id,
		created_at,
		updated_at
	)
	VALUES (
		%s,
		%s,
		%s,
		%s,
		%s,
		%s,
		%s,
		NOW(),
		NOW()
	)
	RETURNING %s
`

const executorSecretUpdateQueryFmtstr = `
UPDATE executor_secrets
SET
	value = %s,
	encryption_key_id = %s,
	updated_at = %s
WHERE
	id = %s AND
	scope = %s AND
	%s -- authz query conds
RETURNING %s
`

// scanExecutorSecret scans a secret from the given scanner into the given
// ExecutorSecret.
func scanExecutorSecret(secret *ExecutorSecret, key encryption.Key, s interface {
	Scan(...any) error
},
) error {
	var (
		value []byte
		keyID string
	)

	if err := s.Scan(
		&secret.ID,
		&secret.Scope,
		&secret.Key,
		&value,
		&dbutil.NullString{S: &keyID},
		&secret.OverwritesGlobalSecret,
		&dbutil.NullInt32{N: &secret.NamespaceUserID},
		&dbutil.NullInt32{N: &secret.NamespaceOrgID},
		&dbutil.NullInt32{N: &secret.CreatorID},
		&secret.CreatedAt,
		&secret.UpdatedAt,
	); err != nil {
		return err
	}

	secret.encryptedValue = NewEncryptedCredential(string(value), keyID, key)
	return nil
}

func EnsureActorHasNamespaceWriteAccess(ctx context.Context, db DB, secret *ExecutorSecret) error {
	a := actor.FromContext(ctx)
	if a.IsInternal() {
		return nil
	}
	if !a.IsAuthenticated() {
		return errors.New("not logged in")
	}

	// TODO: This should use the helpers from the auth package, but that package
	// today depends on the database package, so that would be an import cycle.
	if secret.NamespaceOrgID != 0 {
		// Check if the current user is org member.
		resp, err := db.OrgMembers().GetByOrgIDAndUserID(ctx, secret.NamespaceOrgID, a.UID)
		if err != nil {
			if !errcode.IsNotFound(err) {
				return err
			}
			// Not found case: Fall through and eventually end up down at the site-admin
			// check.
		}
		// If membership is found, the user may pass.
		if resp != nil {
			return nil
		}
		// Not a member case: Fall through and eventually end up down at the site-admin
		// check.
	} else if secret.NamespaceUserID != 0 {
		// If the actor is the same user as the namespace user, pass. Otherwise
		// fall through and check if they're site-admin.
		if a.UID == secret.NamespaceUserID {
			return nil
		}
	}

	// Check user is site admin.
	user, err := db.Users().GetByID(ctx, a.UID)
	if err != nil {
		return err
	}
	if user == nil || !user.SiteAdmin {
		return errors.New("not site-admin")
	}
	return nil
}

// executorSecretsAuthzQueryConds generates authz query conditions for checking
// access to the secret at the database level.
// Internal actors will always pass.
func executorSecretsAuthzQueryConds(ctx context.Context) *sqlf.Query {
	a := actor.FromContext(ctx)
	if a.IsInternal() {
		return sqlf.Sprintf("(TRUE)")
	}

	return sqlf.Sprintf(
		executorSecretsAuthzQueryCondsFmtstr,
		a.UID,
		a.UID,
		a.UID,
	)
}

// executorSecretsAuthzQueryCondsFmtstr contains the SQL used to determine if a user
// has access to the given secret value. It is used in every query to ensure that
// the store never returns secrets that are not meant to be seen by them.
const executorSecretsAuthzQueryCondsFmtstr = `
(
	(
		-- the secret is a global secret
		executor_secrets.namespace_user_id IS NULL
		AND
		executor_secrets.namespace_org_id IS NULL
	)
	OR
	(
		-- user is the same as the actor
		executor_secrets.namespace_user_id = %s
	)
	OR
	(
		-- actor is part of the org
		executor_secrets.namespace_org_id IS NOT NULL
		AND
		EXISTS (
			SELECT 1
			FROM orgs
			JOIN org_members ON org_members.org_id = orgs.id
			WHERE org_members.user_id = %s
		)
	)
	OR
	(
		-- actor is site admin
		EXISTS (
			SELECT 1
			FROM users
			WHERE site_admin = TRUE AND id = %s  -- actor user ID
		)
	)
)
`

// encryptExecutorSecret encrypts the given raw secret value if encryption is enabled
// and returns the encrypted data and the associated encryption key ID.
func encryptExecutorSecret(ctx context.Context, key encryption.Key, raw string) ([]byte, string, error) {
	if len(raw) == 0 {
		return nil, "", errors.New("got empty secret")
	}
	data, keyID, err := encryption.MaybeEncrypt(ctx, key, raw)
	return []byte(data), keyID, err
}

// NewMockExecutorSecret can be used in tests to create an executor secret with a
// set inner value. DO NOT USE THIS OUTSIDE OF TESTS.
func NewMockExecutorSecret(s *ExecutorSecret, v string) *ExecutorSecret {
	s.encryptedValue = NewUnencryptedCredential([]byte(v))
	return s
}
