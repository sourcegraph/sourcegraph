package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

// UserCredential represents a row in the `user_credentials` table.
type UserCredential struct {
	ID                  int64
	Domain              string
	UserID              int32
	ExternalServiceType string
	ExternalServiceID   string
	Credential          auth.Authenticator
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// UserCredentialType defines all possible types of authenticators stored in the database.
type UserCredentialType string

// Define credential type strings that we'll use when encoding credentials.
const (
	UserCredentialTypeOAuthClient                        UserCredentialType = "OAuthClient"
	UserCredentialTypeBasicAuth                          UserCredentialType = "BasicAuth"
	UserCredentialTypeBasicAuthWithSSH                   UserCredentialType = "BasicAuthWithSSH"
	UserCredentialTypeOAuthBearerToken                   UserCredentialType = "OAuthBearerToken"
	UserCredentialTypeOAuthBearerTokenWithSSH            UserCredentialType = "OAuthBearerTokenWithSSH"
	UserCredentialTypeBitbucketServerSudoableOAuthClient UserCredentialType = "BitbucketSudoableOAuthClient"
	UserCredentialTypeGitLabSudoableToken                UserCredentialType = "GitLabSudoableToken"
)

// This const block contains the valid domain values for user credentials.
const (
	UserCredentialDomainBatches = "batches"
)

// UserCredentialNotFoundErr is returned when a credential cannot be found from
// its ID or scope.
type UserCredentialNotFoundErr struct{ args []interface{} }

func (err UserCredentialNotFoundErr) Error() string {
	return fmt.Sprintf("user credential not found: %v", err.args)
}

func (UserCredentialNotFoundErr) NotFound() bool {
	return true
}

// UserCredentialsStore provides access to the `user_credentials` table.
type UserCredentialsStore struct {
	*basestore.Store
	once sync.Once
}

// NewUserStoreWithDB instantiates and returns a new UserCredentialsStore with prepared statements.
func UserCredentials(db dbutil.DB) *UserCredentialsStore {
	return &UserCredentialsStore{Store: basestore.NewWithDB(db, sql.TxOptions{})}
}

// NewUserStoreWith instantiates and returns a new UserCredentialsStore using the other store handle.
func UserCredentialsWith(other basestore.ShareableStore) *UserCredentialsStore {
	return &UserCredentialsStore{Store: basestore.NewWithHandle(other.Handle())}
}

func (s *UserCredentialsStore) With(other basestore.ShareableStore) *UserCredentialsStore {
	return &UserCredentialsStore{Store: s.Store.With(other)}
}

func (s *UserCredentialsStore) Transact(ctx context.Context) (*UserCredentialsStore, error) {
	s.ensureStore()

	txBase, err := s.Store.Transact(ctx)
	return &UserCredentialsStore{Store: txBase}, err
}

// ensureStore instantiates a basestore.Store if necessary, using the dbconn.Global handle.
// This function ensures access to dbconn happens after the rest of the code or tests have
// initialized it.
func (s *UserCredentialsStore) ensureStore() {
	s.once.Do(func() {
		if s.Store == nil {
			s.Store = basestore.NewWithDB(dbconn.Global, sql.TxOptions{})
		}
	})
}

// UserCredentialScope represents the unique scope for a credential. Only one
// credential may exist within a scope.
type UserCredentialScope struct {
	Domain              string
	UserID              int32
	ExternalServiceType string
	ExternalServiceID   string
}

// Create creates a new user credential based on the given scope and
// authenticator. If the scope already has a credential, an error will be
// returned.
func (s *UserCredentialsStore) Create(ctx context.Context, scope UserCredentialScope, credential auth.Authenticator) (*UserCredential, error) {
	if Mocks.UserCredentials.Create != nil {
		return Mocks.UserCredentials.Create(ctx, scope, credential)
	}
	s.ensureStore()

	raw, err := marshalCredential(credential)
	if err != nil {
		return nil, errors.Wrap(err, "marshalling credential")
	}

	q := sqlf.Sprintf(
		userCredentialsCreateQueryFmtstr,
		scope.Domain,
		scope.UserID,
		scope.ExternalServiceType,
		scope.ExternalServiceID,
		raw,
		sqlf.Join(userCredentialsColumns, ", "),
	)

	cred := UserCredential{}
	row := s.QueryRow(ctx, q)
	if err := scanUserCredential(&cred, row); err != nil {
		return nil, err
	}

	return &cred, nil
}

// Update updates a user credential in the database. If the credential cannot be found,
// an error ist returned
func (s *UserCredentialsStore) Update(ctx context.Context, credential *UserCredential) error {
	if Mocks.UserCredentials.Update != nil {
		return Mocks.UserCredentials.Update(ctx, credential)
	}
	s.ensureStore()

	raw, err := marshalCredential(credential.Credential)
	if err != nil {
		return errors.Wrap(err, "marshalling credential")
	}

	credential.UpdatedAt = timeutil.Now()

	q := sqlf.Sprintf(
		userCredentialsUpdateQueryFmtstr,
		credential.Domain,
		credential.UserID,
		credential.ExternalServiceType,
		credential.ExternalServiceID,
		raw,
		credential.UpdatedAt,
		credential.ID,
		sqlf.Join(userCredentialsColumns, ", "),
	)

	row := s.QueryRow(ctx, q)
	if err := scanUserCredential(credential, row); err != nil {
		return err
	}

	return nil
}

// Delete deletes the given user credential. Note that there is no concept of a
// soft delete with user credentials: once deleted, the relevant records are
// _gone_, so that we don't hold any sensitive data unexpectedly. 💀
func (s *UserCredentialsStore) Delete(ctx context.Context, id int64) error {
	if Mocks.UserCredentials.Delete != nil {
		return Mocks.UserCredentials.Delete(ctx, id)
	}
	s.ensureStore()

	q := sqlf.Sprintf("DELETE FROM user_credentials WHERE id = %s", id)
	res, err := s.ExecResult(ctx, q)
	if err != nil {
		return err
	}

	if rows, err := res.RowsAffected(); err != nil {
		return err
	} else if rows == 0 {
		return UserCredentialNotFoundErr{args: []interface{}{id}}
	}

	return nil
}

// GetByID returns the user credential matching the given ID, or
// UserCredentialNotFoundErr if no such credential exists.
func (s *UserCredentialsStore) GetByID(ctx context.Context, id int64) (*UserCredential, error) {
	if Mocks.UserCredentials.GetByID != nil {
		return Mocks.UserCredentials.GetByID(ctx, id)
	}
	s.ensureStore()

	q := sqlf.Sprintf(
		"SELECT %s FROM user_credentials WHERE id = %s",
		sqlf.Join(userCredentialsColumns, ", "),
		id,
	)

	cred := UserCredential{}
	row := s.QueryRow(ctx, q)
	if err := scanUserCredential(&cred, row); err == sql.ErrNoRows {
		return nil, UserCredentialNotFoundErr{args: []interface{}{id}}
	} else if err != nil {
		return nil, err
	}

	return &cred, nil
}

// GetByScope returns the user credential matching the given scope, or
// UserCredentialNotFoundErr if no such credential exists.
func (s *UserCredentialsStore) GetByScope(ctx context.Context, scope UserCredentialScope) (*UserCredential, error) {
	if Mocks.UserCredentials.GetByScope != nil {
		return Mocks.UserCredentials.GetByScope(ctx, scope)
	}
	s.ensureStore()

	q := sqlf.Sprintf(
		userCredentialsGetByScopeQueryFmtstr,
		sqlf.Join(userCredentialsColumns, ", "),
		scope.Domain,
		scope.UserID,
		scope.ExternalServiceType,
		scope.ExternalServiceID,
	)

	cred := UserCredential{}
	row := s.QueryRow(ctx, q)
	if err := scanUserCredential(&cred, row); err == sql.ErrNoRows {
		return nil, UserCredentialNotFoundErr{args: []interface{}{scope}}
	} else if err != nil {
		return nil, err
	}

	return &cred, nil
}

// UserCredentialsListOpts provide the options when listing credentials. At
// least one field in Scope must be set.
type UserCredentialsListOpts struct {
	*LimitOffset
	Scope             UserCredentialScope
	AuthenticatorType []UserCredentialType
	ForUpdate         bool
}

// sql overrides LimitOffset.SQL() to give a LIMIT clause with one extra value
// so we can populate the next cursor.
func (opts *UserCredentialsListOpts) sql() *sqlf.Query {
	if opts.LimitOffset == nil || opts.Limit == 0 {
		return &sqlf.Query{}
	}

	return (&LimitOffset{Limit: opts.Limit + 1, Offset: opts.Offset}).SQL()
}

// List returns all user credentials matching the given options.
func (s *UserCredentialsStore) List(ctx context.Context, opts UserCredentialsListOpts) ([]*UserCredential, int, error) {
	if Mocks.UserCredentials.List != nil {
		return Mocks.UserCredentials.List(ctx, opts)
	}
	s.ensureStore()

	preds := []*sqlf.Query{}
	if opts.Scope.Domain != "" {
		preds = append(preds, sqlf.Sprintf("domain = %s", opts.Scope.Domain))
	}
	if opts.Scope.UserID != 0 {
		preds = append(preds, sqlf.Sprintf("user_id = %s", opts.Scope.UserID))
	}
	if opts.Scope.ExternalServiceType != "" {
		preds = append(preds, sqlf.Sprintf("external_service_type = %s", opts.Scope.ExternalServiceType))
	}
	if opts.Scope.ExternalServiceID != "" {
		preds = append(preds, sqlf.Sprintf("external_service_id = %s", opts.Scope.ExternalServiceID))
	}
	if len(opts.AuthenticatorType) > 0 {
		values := make([]*sqlf.Query, 0, len(opts.AuthenticatorType))
		for _, t := range opts.AuthenticatorType {
			// The JSON text fields are quoted, so we need to quote the type here too.
			values = append(values, sqlf.Sprintf(`%s`, fmt.Sprintf(`"%s"`, t)))
		}
		preds = append(preds, sqlf.Sprintf("(credential::json->'Type')::text IN (%s)", sqlf.Join(values, ",")))
	}

	if len(preds) == 0 {
		preds = append(preds, sqlf.Sprintf("TRUE"))
	}

	forUpdate := &sqlf.Query{}
	if opts.ForUpdate {
		forUpdate = sqlf.Sprintf("FOR UPDATE")
	}

	q := sqlf.Sprintf(
		userCredentialsListQueryFmtstr,
		sqlf.Join(userCredentialsColumns, ", "),
		sqlf.Join(preds, "\n AND "),
		opts.sql(),
		forUpdate,
	)

	rows, err := s.Query(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var creds []*UserCredential
	for rows.Next() {
		cred := UserCredential{}
		if err := scanUserCredential(&cred, rows); err != nil {
			return nil, 0, err
		}
		creds = append(creds, &cred)
	}

	// Check if there were more results than the limit: if so, then we need to
	// set the return cursor and lop off the extra credential that we retrieved.
	next := 0
	if opts.LimitOffset != nil && opts.Limit != 0 && len(creds) == opts.Limit+1 {
		next = opts.Offset + opts.Limit
		creds = creds[:len(creds)-1]
	}

	return creds, next, nil
}

// 🐉 This marks the end of the public API. Beyond here are dragons.

// userCredentialsColumns are the columns that must be selected by
// user_credentials queries in order to use scanUserCredential().
var userCredentialsColumns = []*sqlf.Query{
	sqlf.Sprintf("id"),
	sqlf.Sprintf("domain"),
	sqlf.Sprintf("user_id"),
	sqlf.Sprintf("external_service_type"),
	sqlf.Sprintf("external_service_id"),
	sqlf.Sprintf("credential"),
	sqlf.Sprintf("created_at"),
	sqlf.Sprintf("updated_at"),
}

// The more unwieldy queries are below rather than inline in the above methods
// in a vain attempt to improve their readability.

const userCredentialsGetByScopeQueryFmtstr = `
-- source: internal/database/user_credentials.go:GetByScope
SELECT %s
FROM user_credentials
WHERE
	domain = %s AND
	user_id = %s AND
	external_service_type = %s AND
	external_service_id = %s
`

const userCredentialsListQueryFmtstr = `
-- source: internal/database/user_credentials.go:List
SELECT %s
FROM user_credentials
WHERE %s
ORDER BY created_at ASC, domain ASC, user_id ASC, external_service_id ASC
%s  -- LIMIT clause
%s  -- optional FOR UPDATE
`

const userCredentialsCreateQueryFmtstr = `
-- source: internal/database/user_credentials.go:Create
INSERT INTO
	user_credentials (
		domain,
		user_id,
		external_service_type,
		external_service_id,
		credential,
		created_at,
		updated_at
	)
	VALUES (
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

const userCredentialsUpdateQueryFmtstr = `
-- source: internal/database/user_credentials.go:Update
UPDATE user_credentials
SET
	domain = %s,
	user_id = %s,
	external_service_type = %s,
	external_service_id = %s,
	credential = %s,
	updated_at = %s
WHERE
	id = %s
RETURNING %s
`

// scanUserCredential scans a credential from the given scanner into the given
// credential.
//
// s is inspired by the BatchChange scanner type, but also matches sql.Row, which
// is generally used directly in this module.
func scanUserCredential(cred *UserCredential, s interface {
	Scan(...interface{}) error
}) error {
	// Set up a string for the credential to be decrypted into.
	raw := ""

	if err := s.Scan(
		&cred.ID,
		&cred.Domain,
		&cred.UserID,
		&cred.ExternalServiceType,
		&cred.ExternalServiceID,
		&raw,
		&cred.CreatedAt,
		&cred.UpdatedAt,
	); err != nil {
		return err
	}

	// Now we have the credential, we need to unmarshal it into the right Go
	// type.
	var err error
	if cred.Credential, err = unmarshalCredential(raw); err != nil {
		return errors.Wrap(err, "unmarshalling credential")
	}

	return nil
}

// marshalCredential encodes an Authenticator into a JSON string.
func marshalCredential(a auth.Authenticator) (string, error) {
	var t UserCredentialType
	switch a.(type) {
	case *auth.OAuthClient:
		t = UserCredentialTypeOAuthClient
	case *auth.BasicAuth:
		t = UserCredentialTypeBasicAuth
	case *auth.BasicAuthWithSSH:
		t = UserCredentialTypeBasicAuthWithSSH
	case *auth.OAuthBearerToken:
		t = UserCredentialTypeOAuthBearerToken
	case *auth.OAuthBearerTokenWithSSH:
		t = UserCredentialTypeOAuthBearerTokenWithSSH
	case *bitbucketserver.SudoableOAuthClient:
		t = UserCredentialTypeBitbucketServerSudoableOAuthClient
	case *gitlab.SudoableToken:
		t = UserCredentialTypeGitLabSudoableToken
	default:
		return "", errors.Errorf("unknown Authenticator implementation type: %T", a)
	}

	raw, err := json.Marshal(struct {
		Type UserCredentialType
		Auth auth.Authenticator
	}{
		Type: t,
		Auth: a,
	})
	if err != nil {
		return "", err
	}

	return string(raw), nil
}

// unmarshalCredential decodes a JSON string into an Authenticator.
func unmarshalCredential(raw string) (auth.Authenticator, error) {
	// We do two unmarshals: the first just to get the type, and then the second
	// to actually unmarshal the authenticator itself.
	var partial struct {
		Type UserCredentialType
		Auth json.RawMessage
	}
	if err := json.Unmarshal([]byte(raw), &partial); err != nil {
		return nil, err
	}

	var a interface{}
	switch partial.Type {
	case UserCredentialTypeOAuthClient:
		a = &auth.OAuthClient{}
	case UserCredentialTypeBasicAuth:
		a = &auth.BasicAuth{}
	case UserCredentialTypeBasicAuthWithSSH:
		a = &auth.BasicAuthWithSSH{}
	case UserCredentialTypeOAuthBearerToken:
		a = &auth.OAuthBearerToken{}
	case UserCredentialTypeOAuthBearerTokenWithSSH:
		a = &auth.OAuthBearerTokenWithSSH{}
	case UserCredentialTypeBitbucketServerSudoableOAuthClient:
		a = &bitbucketserver.SudoableOAuthClient{}
	case UserCredentialTypeGitLabSudoableToken:
		a = &gitlab.SudoableToken{}
	default:
		return nil, errors.Errorf("unknown credential type: %s", partial.Type)
	}

	if err := json.Unmarshal(partial.Auth, &a); err != nil {
		return nil, err
	}

	return a.(auth.Authenticator), nil
}
