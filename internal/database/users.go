package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/sourcegraph/sourcegraph/lib/pointers"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/crypto/bcrypt"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/randstring"
	"github.com/sourcegraph/sourcegraph/internal/security"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// User hooks
var (
	// BeforeCreateUser (if set) is a hook called before creating a new user in the DB by any means
	// (e.g., both directly via Users.Create or via Users.CreateWithExternalAccount).
	BeforeCreateUser func(ctx context.Context, db DB, spec *extsvc.AccountSpec) error
	// AfterCreateUser (if set) is a hook called after creating a new user in the DB by any means
	// (e.g., both directly via Users.Create or via Users.CreateWithExternalAccount).
	// Whatever this hook mutates in database should be reflected on the `user` argument as well.
	AfterCreateUser func(ctx context.Context, db DB, user *types.User) error
	// BeforeSetUserIsSiteAdmin (if set) is a hook called before promoting/revoking a user to be a
	// site admin.
	BeforeSetUserIsSiteAdmin func(ctx context.Context, isSiteAdmin bool) error
)

// UserStore provides access to the `users` table.
//
// For a detailed overview of the schema, see schema.md.
type UserStore interface {
	basestore.ShareableStore
	CheckAndDecrementInviteQuota(context.Context, int32) (ok bool, err error)
	Count(context.Context, *UsersListOptions) (int, error)
	CountForSCIM(context.Context, *UsersListOptions) (int, error)
	Create(context.Context, NewUser) (*types.User, error)
	// CreateWithExternalAccount is used to create a new Sourcegraph user account from an external account
	// (e.g., "signup from SAML").
	//
	// It creates a new user and associates it with the specified external account. If the user to
	// create already exists, it returns an error.
	CreateWithExternalAccount(ctx context.Context, newUser NewUser, acct *extsvc.Account) (*types.User, error)
	CreateInTransaction(context.Context, NewUser, *extsvc.AccountSpec) (*types.User, error)
	CreatePassword(ctx context.Context, id int32, password string) error
	Delete(context.Context, int32) error
	DeleteList(context.Context, []int32) error
	DeletePasswordResetCode(context.Context, int32) error
	Done(error) error
	Exec(ctx context.Context, query *sqlf.Query) error
	ExecResult(ctx context.Context, query *sqlf.Query) (sql.Result, error)
	GetByCurrentAuthUser(context.Context) (*types.User, error)
	GetByID(context.Context, int32) (*types.User, error)
	GetByUsername(context.Context, string) (*types.User, error)
	GetByUsernames(context.Context, ...string) ([]*types.User, error)
	GetByVerifiedEmail(context.Context, string) (*types.User, error)
	HardDelete(context.Context, int32) error
	HardDeleteList(context.Context, []int32) error
	InvalidateSessionsByID(context.Context, int32) (err error)
	InvalidateSessionsByIDs(context.Context, []int32) (err error)
	IsPassword(ctx context.Context, id int32, password string) (bool, error)
	List(context.Context, *UsersListOptions) (_ []*types.User, err error)
	ListForSCIM(context.Context, *UsersListOptions) (_ []*types.UserForSCIM, err error)
	ListDates(context.Context) ([]types.UserDates, error)
	ListByOrg(ctx context.Context, orgID int32, paginationArgs *PaginationArgs, query *string) ([]*types.User, error)
	RandomizePasswordAndClearPasswordResetRateLimit(context.Context, int32) error
	RecoverUsersList(context.Context, []int32) (_ []int32, err error)
	RenewPasswordResetCode(context.Context, int32) (string, error)
	SetIsSiteAdmin(ctx context.Context, id int32, isSiteAdmin bool) error
	SetPassword(ctx context.Context, id int32, resetCode, newPassword string) (bool, error)
	Transact(context.Context) (UserStore, error)
	Update(context.Context, int32, UserUpdate) error
	UpdatePassword(ctx context.Context, id int32, oldPassword, newPassword string) error
	ChangeCodyPlan(ctx context.Context, id int32, plan bool) error
	SetChatCompletionsQuota(ctx context.Context, id int32, quota *int) error
	GetChatCompletionsQuota(ctx context.Context, id int32) (*int, error)
	SetCodeCompletionsQuota(ctx context.Context, id int32, quota *int) error
	GetCodeCompletionsQuota(ctx context.Context, id int32) (*int, error)
	With(basestore.ShareableStore) UserStore
}

type userStore struct {
	logger log.Logger
	*basestore.Store
}

var _ UserStore = (*userStore)(nil)

// Users instantiates and returns a new RepoStore with prepared statements.
func Users(logger log.Logger) UserStore {
	return &userStore{
		logger: logger,
		Store:  &basestore.Store{},
	}
}

// UsersWith instantiates and returns a new RepoStore using the other store handle.
func UsersWith(logger log.Logger, other basestore.ShareableStore) UserStore {
	return &userStore{
		logger: logger,
		Store:  basestore.NewWithHandle(other.Handle()),
	}
}

func (u *userStore) With(other basestore.ShareableStore) UserStore {
	return &userStore{logger: u.logger, Store: u.Store.With(other)}
}

func (u *userStore) Transact(ctx context.Context) (UserStore, error) {
	return u.transact(ctx)
}

func (u *userStore) transact(ctx context.Context) (*userStore, error) {
	txBase, err := u.Store.Transact(ctx)
	return &userStore{logger: u.logger, Store: txBase}, err
}

// userNotFoundErr is the error that is returned when a user is not found.
type userNotFoundErr struct {
	args []any
}

func IsUserNotFoundErr(err error) bool {
	_, ok := err.(userNotFoundErr)
	return ok
}

func NewUserNotFoundErr(args ...any) userNotFoundErr {
	return userNotFoundErr{args: args}
}

func (err userNotFoundErr) Error() string {
	return fmt.Sprintf("user not found: %v", err.args)
}

func (err userNotFoundErr) NotFound() bool {
	return true
}

// NewUserNotFoundError returns a new error indicating that the user with the given user ID was not
// found.
func NewUserNotFoundError(userID int32) error {
	return userNotFoundErr{args: []any{"userID", userID}}
}

// ErrCannotCreateUser is the error that is returned when
// a user cannot be added to the DB due to a constraint.
type ErrCannotCreateUser struct {
	code string
}

const (
	ErrorCodeUsernameExists = "err_username_exists"
	ErrorCodeEmailExists    = "err_email_exists"
)

func (err ErrCannotCreateUser) Error() string {
	return fmt.Sprintf("cannot create user: %v", err.code)
}

func (err ErrCannotCreateUser) Code() string {
	return err.code
}

// IsUsernameExists reports whether err is an error indicating that the intended username exists.
func IsUsernameExists(err error) bool {
	var e ErrCannotCreateUser
	return errors.As(err, &e) && e.code == ErrorCodeUsernameExists
}

// IsEmailExists reports whether err is an error indicating that the intended email exists.
func IsEmailExists(err error) bool {
	var e ErrCannotCreateUser
	return errors.As(err, &e) && e.code == ErrorCodeEmailExists
}

// NewUser describes a new to-be-created user.
type NewUser struct {
	Email       string
	Username    string
	DisplayName string
	Password    string
	AvatarURL   string // the new user's avatar URL, if known

	// EmailVerificationCode, if given, causes the new user's email address to be unverified until
	// they perform the email verification process and provied this code.
	EmailVerificationCode string `json:"-"` // forbid this field being set by JSON, just in case

	// EmailIsVerified is whether the email address should be considered already verified.
	//
	// 🚨 SECURITY: Only site admins are allowed to create users whose email addresses are initially
	// verified (i.e., with EmailVerificationCode == "").
	EmailIsVerified bool `json:"-"` // forbid this field being set by JSON, just in case

	// FailIfNotInitialUser causes the (users).Create call to return an error and not create the
	// user if at least one of the following is true: (1) the site has already been initialized or
	// (2) any other user account already exists.
	FailIfNotInitialUser bool `json:"-"` // forbid this field being set by JSON, just in case

	// EnforcePasswordLength is whether should enforce minimum and maximum password length requirement.
	// Users created by non-builtin auth providers do not have a password thus no need to check.
	EnforcePasswordLength bool `json:"-"` // forbid this field being set by JSON, just in case

	// TosAccepted is whether the user is created with the terms of service accepted already.
	TosAccepted bool `json:"-"` // forbid this field being set by JSON, just in case
}

type NewUserForSCIM struct {
	NewUser
	AdditionalVerifiedEmails []string
	SCIMExternalID           string
}

// Create creates a new user in the database.
//
// If a password is given, then unauthenticated users can sign into the account using the
// username/email and password. If no password is given, a non-builtin auth provider must be used to
// sign into the account.
//
// # CREATION OF SITE ADMINS
//
// The new user is made to be a site admin if the following are both true: (1) this user would be
// the first and only user on the server, and (2) the site has not yet been initialized. Otherwise,
// the user is created as a normal (non-site-admin) user. After the call, the site is marked as
// having been initialized (so that no subsequent (users).Create calls will yield a site
// admin). This is used to create the initial site admin user during site initialization.
//
// It's implemented as part of the (users).Create call instead of relying on the caller to do it in
// order to avoid a race condition where multiple initial site admins could be created or zero site
// admins could be created.
func (u *userStore) Create(ctx context.Context, info NewUser) (newUser *types.User, err error) {
	tx, err := u.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()
	newUser, err = tx.CreateInTransaction(ctx, info, nil)
	if err == nil {
		logAccountCreatedEvent(ctx, NewDBWith(u.logger, u), newUser, "")
	}
	return newUser, err
}

func (u *userStore) CreateWithExternalAccount(ctx context.Context, newUser NewUser, acct *extsvc.Account) (_ *types.User, err error) {
	tx, err := u.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	createdUser, err := tx.CreateInTransaction(ctx, newUser, &acct.AccountSpec)
	if err != nil {
		return nil, err
	}

	acct.UserID = createdUser.ID
	acct, err = ExternalAccountsWith(u.logger, tx).Insert(ctx, acct)
	if err == nil {
		logAccountCreatedEvent(ctx, NewDBWith(u.logger, tx), createdUser, acct.AccountSpec.ServiceType)
	}
	return createdUser, err
}

// CheckPassword returns an error depending on the method used for validation
func CheckPassword(pw string) error {
	return security.ValidatePassword(pw)
}

// CreateInTransaction is like Create, except it is expected to be run from within a
// transaction. It must execute in a transaction because the post-user-creation
// hooks must run atomically with the user creation.
func (u *userStore) CreateInTransaction(ctx context.Context, info NewUser, spec *extsvc.AccountSpec) (newUser *types.User, err error) {
	if !u.InTransaction() {
		return nil, errors.New("must run within a transaction")
	}

	if info.EnforcePasswordLength {
		if err := security.ValidatePassword(info.Password); err != nil {
			return nil, err
		}
	}

	if info.Email != "" && info.EmailVerificationCode == "" && !info.EmailIsVerified {
		return nil, errors.New("no email verification code provided for new user with unverified email")
	}

	createdAt := timeutil.Now()
	updatedAt := createdAt
	invalidatedSessionsAt := createdAt
	var id int32

	var passwd sql.NullString
	if info.Password == "" {
		passwd = sql.NullString{Valid: false}
	} else {
		// Compute hash of password
		passwd, err = hashPassword(info.Password)
		if err != nil {
			return nil, err
		}
	}

	var avatarURL *string
	if info.AvatarURL != "" {
		avatarURL = &info.AvatarURL
	}

	// Creating the initial site admin user is equivalent to initializing the
	// site. ensureInitialized runs in the transaction, so we are guaranteed that the user account
	// creation and site initialization operations occur atomically (to guarantee to the legitimate
	// site admin that if they successfully initialize the server, then no attacker's account could
	// have been created as a site admin).
	alreadyInitialized, err := GlobalStateWith(u).EnsureInitialized(ctx)
	if err != nil {
		return nil, err
	}
	if alreadyInitialized && info.FailIfNotInitialUser {
		return nil, ErrCannotCreateUser{"site_already_initialized"}
	}

	// Run BeforeCreateUser hook.
	if BeforeCreateUser != nil {
		if err := BeforeCreateUser(ctx, NewDBWith(u.logger, u.Store), spec); err != nil {
			return nil, errors.Wrap(err, "pre create user hook")
		}
	}

	var siteAdmin bool
	err = u.QueryRow(
		ctx,
		sqlf.Sprintf("INSERT INTO users(username, display_name, avatar_url, created_at, updated_at, passwd, invalidated_sessions_at, tos_accepted, site_admin) VALUES(%s, %s, %s, %s, %s, %s, %s, %s, %s AND NOT EXISTS(SELECT * FROM users)) RETURNING id, site_admin",
			info.Username, info.DisplayName, avatarURL, createdAt, updatedAt, passwd, invalidatedSessionsAt, info.TosAccepted, !alreadyInitialized)).Scan(&id, &siteAdmin)
	if err != nil {
		var e *pgconn.PgError
		if errors.As(err, &e) {
			switch e.ConstraintName {
			case "users_username":
				return nil, ErrCannotCreateUser{ErrorCodeUsernameExists}
			case "users_username_max_length", "users_username_valid_chars", "users_display_name_max_length":
				return nil, ErrCannotCreateUser{e.ConstraintName}
			}
		}
		return nil, err
	}
	if info.FailIfNotInitialUser && !siteAdmin {
		// Refuse to make the user the initial site admin if there are other existing users.
		return nil, ErrCannotCreateUser{"initial_site_admin_must_be_first_user"}
	}

	// Reserve username in shared users+orgs namespace.
	if err := u.Exec(ctx, sqlf.Sprintf("INSERT INTO names(name, user_id) VALUES(%s, %s)", info.Username, id)); err != nil {
		return nil, ErrCannotCreateUser{ErrorCodeUsernameExists}
	}

	if info.Email != "" {
		// We don't allow adding a new user with an email address that has already been
		// verified by another user.
		exists, _, err := basestore.ScanFirstBool(u.Query(ctx, sqlf.Sprintf("SELECT TRUE WHERE EXISTS (SELECT FROM user_emails where email = %s AND verified_at IS NOT NULL)", info.Email)))
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrCannotCreateUser{ErrorCodeEmailExists}
		}

		// The first email address added should be their primary
		if info.EmailIsVerified {
			err = u.Exec(ctx, sqlf.Sprintf("INSERT INTO user_emails(user_id, email, verified_at, is_primary) VALUES (%s, %s, now(), true)", id, info.Email))
		} else {
			err = u.Exec(ctx, sqlf.Sprintf("INSERT INTO user_emails(user_id, email, verification_code, is_primary) VALUES (%s, %s, %s, true)", id, info.Email, info.EmailVerificationCode))
		}
		if err != nil {
			var e *pgconn.PgError
			if errors.As(err, &e) && e.ConstraintName == "user_emails_unique_verified_email" {
				return nil, ErrCannotCreateUser{ErrorCodeEmailExists}
			}
			return nil, err
		}

		// Cancel possible pending access request for this email
		accessRequestsStore := AccessRequestsWith(u, u.logger)
		ar, err := accessRequestsStore.GetByEmail(ctx, info.Email)

		if err != nil && !errors.Is(err, &ErrAccessRequestNotFound{Email: info.Email}) {
			return nil, err
		}

		if err == nil {
			ar.Status = types.AccessRequestStatusCanceled
			ar.UpdatedAt = time.Now()
			ar.DecisionByUserID = pointers.Ptr(id)
			_, err = accessRequestsStore.Update(ctx, ar)
			if err != nil {
				return nil, err
			}
		}
	}

	user := &types.User{
		ID:                    id,
		Username:              info.Username,
		DisplayName:           info.DisplayName,
		AvatarURL:             info.AvatarURL,
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
		SiteAdmin:             siteAdmin,
		BuiltinAuth:           info.Password != "",
		InvalidatedSessionsAt: invalidatedSessionsAt,
	}

	{
		// Assign roles to the created user. We do this in here to ensure role assign occurs in the same transaction as user creation occurs.
		// This ensures we don't have "zombie" users (users with no role assigned to them).
		// All users on a Sourcegraph instance must have the `USER` role, however depending on the value of user.SiteAdmin,
		// we assign them the `SITE_ADMINISTRATOR` role.
		roles := []types.SystemRole{types.UserSystemRole}
		if user.SiteAdmin {
			roles = append(roles, types.SiteAdministratorSystemRole)
		}

		db := NewDBWith(u.logger, u)
		if err := db.UserRoles().BulkAssignSystemRolesToUser(ctx, BulkAssignSystemRolesToUserOpts{
			UserID: user.ID,
			Roles:  roles,
		}); err != nil {
			return nil, err
		}
	}

	{
		// Run hooks.
		//
		// NOTE: If we need more hooks in the future, we should do something better than just
		// adding random calls here.

		// Ensure the user (all users, actually) is joined to the orgs specified in auth.userOrgMap.
		orgs, errs := orgsForAllUsersToJoin(conf.Get().AuthUserOrgMap)
		for _, err := range errs {
			u.logger.Warn("Error ensuring user is joined to orgs", log.Error(err))
		}
		if err := OrgMembersWith(u).CreateMembershipInOrgsForAllUsers(ctx, orgs); err != nil {
			return nil, err
		}

		// Run AfterCreateUser hook
		if AfterCreateUser != nil {
			if err := AfterCreateUser(ctx, NewDBWith(u.logger, u.Store), user); err != nil {
				return nil, errors.Wrap(err, "after create user hook")
			}
		}
	}

	return user, nil
}

func logAccountCreatedEvent(ctx context.Context, db DB, u *types.User, serviceType string) {
	a := actor.FromContext(ctx)
	arg := struct {
		Creator     int32  `json:"creator"`
		SiteAdmin   bool   `json:"site_admin"`
		ServiceType string `json:"service_type"`
	}{
		Creator:     a.UID,
		SiteAdmin:   u.SiteAdmin,
		ServiceType: serviceType,
	}
	if err := db.SecurityEventLogs().LogSecurityEvent(ctx, SecurityEventNameAccountCreated, "", uint32(u.ID), "", "BACKEND", arg); err != nil {
		log.Error(err)
	}

	eArg, _ := json.Marshal(struct {
		Creator     int32  `json:"creator"`
		Created     int32  `json:"created"`
		SiteAdmin   bool   `json:"site_admin"`
		ServiceType string `json:"service_type"`
	}{
		Creator:     a.UID,
		Created:     u.ID,
		SiteAdmin:   u.SiteAdmin,
		ServiceType: serviceType,
	})
	logEvent := &Event{
		Name:            "AccountCreated",
		URL:             "",
		AnonymousUserID: "backend",
		Argument:        eArg,
		Source:          "BACKEND",
		Timestamp:       time.Now(),
	}
	//lint:ignore SA1019 existing usage of deprecated functionality. Use EventRecorder from internal/telemetryrecorder instead.
	_ = db.EventLogs().Insert(ctx, logEvent)
}

// orgsForAllUsersToJoin returns the list of org names that all users should be joined to. The second return value
// is a list of errors encountered while generating this list. Note that even if errors are returned, the first
// return value is still valid.
func orgsForAllUsersToJoin(userOrgMap map[string][]string) ([]string, []error) {
	var errs []error
	for userPattern, orgs := range userOrgMap {
		if userPattern != "*" {
			errs = append(errs, errors.Errorf("unsupported auth.userOrgMap user pattern %q (only \"*\" is supported)", userPattern))
			continue
		}
		return orgs, errs
	}
	return nil, errs
}

// UserUpdate describes user fields to update.
type UserUpdate struct {
	Username string // update the Username to this value (if non-zero)

	// For the following fields:
	//
	// - If nil, the value in the DB is unchanged.
	// - If pointer to "" (empty string), the value in the DB is set to null.
	// - If pointer to a non-empty string, the value in the DB is set to the string.
	DisplayName, AvatarURL *string
	TosAccepted            *bool
	CompletedPostSignup    *bool
}

// Update updates a user's profile information.
func (u *userStore) Update(ctx context.Context, id int32, update UserUpdate) (err error) {
	tx, err := u.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	fieldUpdates := []*sqlf.Query{
		sqlf.Sprintf("updated_at=now()"), // always update updated_at timestamp
	}
	if update.Username != "" {
		fieldUpdates = append(fieldUpdates, sqlf.Sprintf("username=%s", update.Username))

		// Ensure new username is available in shared users+orgs namespace.
		if err := tx.Exec(ctx, sqlf.Sprintf("UPDATE names SET name=%s WHERE user_id=%s", update.Username, id)); err != nil {
			var e *pgconn.PgError
			if errors.As(err, &e) && e.ConstraintName == "names_pkey" {
				return errors.Errorf("Username is already in use.")
			}
			return err
		}
	}
	strOrNil := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	if update.DisplayName != nil {
		fieldUpdates = append(fieldUpdates, sqlf.Sprintf("display_name=%s", strOrNil(*update.DisplayName)))
	}
	if update.AvatarURL != nil {
		fieldUpdates = append(fieldUpdates, sqlf.Sprintf("avatar_url=%s", strOrNil(*update.AvatarURL)))
	}
	if update.TosAccepted != nil {
		fieldUpdates = append(fieldUpdates, sqlf.Sprintf("tos_accepted=%s", *update.TosAccepted))
	}
	if update.CompletedPostSignup != nil {
		fieldUpdates = append(fieldUpdates, sqlf.Sprintf("completed_post_signup=%s", *update.CompletedPostSignup))
	}
	query := sqlf.Sprintf("UPDATE users SET %s WHERE id=%d", sqlf.Join(fieldUpdates, ", "), id)
	res, err := tx.ExecResult(ctx, query)
	if err != nil {
		var e *pgconn.PgError
		if errors.As(err, &e) && e.ConstraintName == "users_username" {
			return ErrCannotCreateUser{ErrorCodeUsernameExists}
		}
		return err
	}
	nrows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if nrows == 0 {
		return userNotFoundErr{args: []any{id}}
	}
	return nil
}

// NOTE(naman): THIS IS TEMPORARY FOR DEC GA
// Upgrade user to cody pro plan
func (u *userStore) ChangeCodyPlan(ctx context.Context, id int32, pro bool) (err error) {
	query := sqlf.Sprintf("UPDATE users SET cody_pro_enabled_at=NULL WHERE id=%d AND cody_pro_enabled_at IS NOT NULL AND deleted_at IS NULL", id)

	if pro {
		query = sqlf.Sprintf("UPDATE users SET cody_pro_enabled_at=NOW() WHERE id=%d AND cody_pro_enabled_at IS NULL AND deleted_at IS NULL", id)
	}

	res, err := u.ExecResult(ctx, query)
	if err != nil {
		return err
	}

	nrows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if nrows == 0 {
		user, err := u.GetByID(ctx, id)
		if err != nil {
			return err
		}

		if user == nil {
			return userNotFoundErr{args: []any{id}}
		}

		// Intentionally not returning an error if the user is already pro/commiunity. This makes the mutation idempotent.
		return nil
	}

	return nil
}

// Delete performs a soft-delete of the user and all resources associated with this user.
// Permissions for soft-deleted users are removed from the user_repo_permissions table via a trigger.
func (u *userStore) Delete(ctx context.Context, id int32) (err error) {
	return u.DeleteList(ctx, []int32{id})
}

// DeleteList performs a bulk "Delete" action.
func (u *userStore) DeleteList(ctx context.Context, ids []int32) (err error) {
	tx, err := u.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	userIDs := make([]*sqlf.Query, len(ids))
	for i := range ids {
		userIDs[i] = sqlf.Sprintf("%d", ids[i])
	}

	idsCond := sqlf.Join(userIDs, ",")

	res, err := tx.ExecResult(ctx, sqlf.Sprintf("UPDATE users SET deleted_at=now() WHERE id IN (%s) AND deleted_at IS NULL", idsCond))
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows != int64(len(ids)) {
		return userNotFoundErr{args: []any{fmt.Sprintf("Some users were not found. Expected to delete %d users, but deleted only %d", +len(ids), rows)}}
	}

	// Release the username so it can be used by another user or org.
	if err := tx.Exec(ctx, sqlf.Sprintf("DELETE FROM names WHERE user_id IN (%s)", idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("UPDATE access_tokens SET deleted_at=now() WHERE deleted_at IS NULL AND (subject_user_id IN (%s) OR creator_user_id IN (%s))", idsCond, idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("DELETE FROM user_emails WHERE user_id IN (%s)", idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("UPDATE user_external_accounts SET deleted_at=now() WHERE deleted_at IS NULL AND user_id IN (%s)", idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("UPDATE org_invitations SET deleted_at=now() WHERE deleted_at IS NULL AND (sender_user_id IN (%s) OR recipient_user_id IN (%s))", idsCond, idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("UPDATE registry_extensions SET deleted_at=now() WHERE deleted_at IS NULL AND publisher_user_id IN (%s)", idsCond)); err != nil {
		return err
	}

	logUserDeletionEvents(ctx, NewDBWith(u.logger, u), ids, SecurityEventNameAccountDeleted)

	return nil
}

// HardDelete removes the user and all resources associated with this user.
func (u *userStore) HardDelete(ctx context.Context, id int32) (err error) {
	return u.HardDeleteList(ctx, []int32{id})
}

// HardDeleteList performs a bulk "HardDelete" action.
func (u *userStore) HardDeleteList(ctx context.Context, ids []int32) (err error) {
	if len(ids) == 0 {
		return nil
	}

	// Wrap in transaction because we delete from multiple tables.
	tx, err := u.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	userIDs := make([]*sqlf.Query, len(ids))
	for i := range ids {
		userIDs[i] = sqlf.Sprintf("%d", ids[i])
	}

	idsCond := sqlf.Join(userIDs, ",")

	if err := tx.Exec(ctx, sqlf.Sprintf("DELETE FROM names WHERE user_id IN (%s)", idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("DELETE FROM access_tokens WHERE subject_user_id IN (%s) OR creator_user_id IN (%s)", idsCond, idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("DELETE FROM user_emails WHERE user_id IN (%s)", idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("DELETE FROM user_external_accounts WHERE user_id IN (%s)", idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("DELETE FROM survey_responses WHERE user_id IN (%s)", idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("DELETE FROM registry_extension_releases WHERE registry_extension_id IN (SELECT id FROM registry_extensions WHERE publisher_user_id IN (%s))", idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("DELETE FROM registry_extensions WHERE publisher_user_id IN (%s)", idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("DELETE FROM org_invitations WHERE sender_user_id IN (%s) OR recipient_user_id IN (%s)", idsCond, idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("DELETE FROM org_members WHERE user_id IN (%s)", idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("DELETE FROM settings WHERE user_id IN (%s)", idsCond)); err != nil {
		return err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf("DELETE FROM saved_searches WHERE user_id IN (%s)", idsCond)); err != nil {
		return err
	}
	// Anonymize all entries for the deleted user
	for _, uid := range userIDs {
		if err := tx.Exec(ctx, sqlf.Sprintf("UPDATE event_logs SET user_id=0, anonymous_user_id=%s WHERE user_id=%s", uuid.New().String(), uid)); err != nil {
			return err
		}
	}
	// Settings that were merely authored by this user should not be deleted. They may be global or
	// org settings that apply to other users, too. There is currently no way to hard-delete
	// settings for an org or globally, but we can handle those rare cases manually.
	if err := tx.Exec(ctx, sqlf.Sprintf("UPDATE settings SET author_user_id=NULL WHERE author_user_id IN (%s)", idsCond)); err != nil {
		return err
	}

	res, err := tx.ExecResult(ctx, sqlf.Sprintf("DELETE FROM users WHERE id IN (%s)", idsCond))
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows != int64(len(ids)) {
		return userNotFoundErr{args: []any{fmt.Sprintf("Some users were not found. Expected to hard delete %d users, but deleted only %d", +len(ids), rows)}}
	}

	logUserDeletionEvents(ctx, NewDBWith(u.logger, u), ids, SecurityEventNameAccountNuked)

	return nil
}

func logUserDeletionEvents(ctx context.Context, db DB, ids []int32, name SecurityEventName) {
	// The actor deleting the user could be a different user, for example a site
	// admin
	a := actor.FromContext(ctx)
	arg, _ := json.Marshal(struct {
		Deleter int32 `json:"deleter"`
	}{
		Deleter: a.UID,
	})

	now := time.Now()
	events := make([]*SecurityEvent, len(ids))
	for index, id := range ids {
		events[index] = &SecurityEvent{
			Name:      name,
			UserID:    uint32(id),
			Argument:  arg,
			Source:    "BACKEND",
			Timestamp: now,
		}
	}
	db.SecurityEventLogs().LogEventList(ctx, events)

	logEvents := make([]*Event, len(ids))
	for index, id := range ids {
		eArg, _ := json.Marshal(struct {
			Deleter int32 `json:"deleter"`
			Deleted int32 `json:"deleted"`
		}{
			Deleter: a.UID,
			Deleted: id,
		})
		logEvents[index] = &Event{
			Name:            string(name),
			AnonymousUserID: "backend",
			Argument:        eArg,
			Source:          "BACKEND",
			Timestamp:       now,
		}
	}

	//lint:ignore SA1019 existing usage of deprecated functionality. Use EventRecorder from internal/telemetryrecorder instead.
	_ = db.EventLogs().BulkInsert(ctx, logEvents)
}

// RecoverUsersList recovers a list of users by their IDs.
func (u *userStore) RecoverUsersList(ctx context.Context, ids []int32) (_ []int32, err error) {
	tx, err := u.transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	userIDs := make([]*sqlf.Query, len(ids))
	for i := range ids {
		userIDs[i] = sqlf.Sprintf("%d", ids[i])
	}
	idsCond := sqlf.Join(userIDs, ",")

	if err := tx.Exec(ctx, sqlf.Sprintf("INSERT INTO names(name, user_id) SELECT username, id FROM users WHERE id IN(%s)", idsCond)); err != nil {
		return nil, err
	}

	const updateAccessTokensQuery = `
	UPDATE access_tokens as a
	SET deleted_at = null
	FROM users as u
	WHERE a.creator_user_id = u.id
	AND a.deleted_at >= u.deleted_at
	AND a.deleted_at <= u.deleted_at + interval '10 second'
	AND (a.creator_user_id IN (%s) OR a.subject_user_id IN (%s))
	`
	if err := tx.Exec(ctx, sqlf.Sprintf(updateAccessTokensQuery, idsCond, idsCond)); err != nil {
		return nil, err
	}

	const updateUserExtAccQuery = `
	UPDATE user_external_accounts AS a
	SET deleted_at = NULL, updated_at = now()
	FROM users AS u
	WHERE a.user_id = u.id
	AND a.deleted_at >= u.deleted_at
	AND a.deleted_at <= u.deleted_at + interval '10 second'
	AND a.user_id IN (%s)
	`
	if err := tx.Exec(ctx, sqlf.Sprintf(updateUserExtAccQuery, idsCond)); err != nil {
		return nil, err
	}
	const updateOrgInvQuery = `
	UPDATE org_invitations AS o
	SET deleted_at = NULL
	FROM users  AS u
	WHERE o.recipient_user_id = u.id
	AND o.deleted_at >= u.deleted_at
	AND o.deleted_at <= u.deleted_at + interval '10 second'
	AND (o.sender_user_id IN (%s) OR o.recipient_user_id IN (%s))
	`

	if err := tx.Exec(ctx, sqlf.Sprintf(updateOrgInvQuery, idsCond, idsCond)); err != nil {
		return nil, err
	}
	const updateRegistryExtQuery = `
	UPDATE registry_extensions AS r
	SET deleted_at = NULL, updated_at = now()
	FROM users AS u
	WHERE r.publisher_user_id = u.id
	AND r.deleted_at >= u.deleted_at
	AND r.deleted_at <= u.deleted_at + interval '10 second'
	AND r.publisher_user_id IN (%s)
	`

	if err := tx.Exec(ctx, sqlf.Sprintf(updateRegistryExtQuery, idsCond)); err != nil {
		return nil, err
	}

	updateIds, err := basestore.ScanInt32s(tx.Query(ctx, sqlf.Sprintf("UPDATE users SET deleted_at=NULL, updated_at=now() WHERE id IN (%s) AND deleted_at IS NOT NULL RETURNING id", idsCond)))
	if err != nil {
		return nil, err
	}

	return updateIds, nil
}

// SetIsSiteAdmin sets the user with the given ID to be or not to be the site admin. It also assigns the role `SITE_ADMINISTRATOR`
// to the user when `isSiteAdmin` is true and revokes the role when false.
func (u *userStore) SetIsSiteAdmin(ctx context.Context, id int32, isSiteAdmin bool) error {
	if BeforeSetUserIsSiteAdmin != nil {
		if err := BeforeSetUserIsSiteAdmin(ctx, isSiteAdmin); err != nil {
			return err
		}
	}

	db := NewDBWith(u.logger, u)
	return db.WithTransact(ctx, func(tx DB) error {
		userStore := tx.Users()
		err := userStore.Exec(ctx, sqlf.Sprintf("UPDATE users SET site_admin=%s WHERE id=%s", isSiteAdmin, id))
		if err != nil {
			return err
		}

		userRoleStore := tx.UserRoles()
		if isSiteAdmin {
			err := userRoleStore.AssignSystemRole(ctx, AssignSystemRoleOpts{
				UserID: id,
				Role:   types.SiteAdministratorSystemRole,
			})
			a := actor.FromContext(ctx)
			arg, _ := json.Marshal(struct {
				Assigner int32  `json:"assigner"`
				Assignee int32  `json:"assignee"`
				Role     string `json:"role"`
			}{
				Assigner: a.UID,
				Assignee: id,
				Role:     string(types.SiteAdministratorSystemRole),
			})
			logEvent := &Event{
				Name:            "RoleChangeGranted",
				AnonymousUserID: "backend",
				Argument:        arg,
				Source:          "BACKEND",
				Timestamp:       time.Now(),
			}
			//lint:ignore SA1019 existing usage of deprecated functionality. Use EventRecorder from internal/telemetryrecorder instead.
			_ = db.EventLogs().Insert(ctx, logEvent)
			return err
		}

		err = userRoleStore.RevokeSystemRole(ctx, RevokeSystemRoleOpts{
			UserID: id,
			Role:   types.SiteAdministratorSystemRole,
		})
		return err
	})
}

// CheckAndDecrementInviteQuota should be called before the user (identified
// by userID) is allowed to invite any other user. If ok is false, then the
// user is not allowed to invite any other user (either because they've
// invited too many users, or some other error occurred). If the user has
// quota remaining, their quota is decremented and ok is true.
func (u *userStore) CheckAndDecrementInviteQuota(ctx context.Context, userID int32) (ok bool, err error) {
	var quotaRemaining int32
	q := sqlf.Sprintf(`
	UPDATE users SET invite_quota=(invite_quota - 1)
	WHERE users.id=%s AND invite_quota>0 AND deleted_at IS NULL
	RETURNING invite_quota`, userID)
	row := u.QueryRow(ctx, q)
	if err := row.Scan(&quotaRemaining); err == sql.ErrNoRows {
		// It's possible that some other problem occurred, such as the user being deleted,
		// but treat that as a quota exceeded error, too.
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil // the user has remaining quota to send invites
}

func (u *userStore) GetByID(ctx context.Context, id int32) (*types.User, error) {
	return u.getOneBySQL(ctx, sqlf.Sprintf("WHERE id=%s AND deleted_at IS NULL LIMIT 1", id))
}

// GetByVerifiedEmail returns the user (if any) with the specified verified email address. If a user
// has a matching *unverified* email address, they will not be returned by this method. At most one
// user may have any given verified email address.
func (u *userStore) GetByVerifiedEmail(ctx context.Context, email string) (*types.User, error) {
	return u.getOneBySQL(ctx, sqlf.Sprintf("WHERE id=(SELECT user_id FROM user_emails WHERE email=%s AND verified_at IS NOT NULL) AND deleted_at IS NULL LIMIT 1", email))
}

func (u *userStore) GetByUsername(ctx context.Context, username string) (*types.User, error) {
	return u.getOneBySQL(ctx, sqlf.Sprintf("WHERE u.username=%s AND u.deleted_at IS NULL LIMIT 1", username))
}

// GetByUsernames returns a list of users by given usernames. The number of results list could be less
// than the candidate list due to no user is associated with some usernames.
func (u *userStore) GetByUsernames(ctx context.Context, usernames ...string) ([]*types.User, error) {
	if len(usernames) == 0 {
		return []*types.User{}, nil
	}

	items := make([]*sqlf.Query, len(usernames))
	for i := range usernames {
		items[i] = sqlf.Sprintf("%s", usernames[i])
	}
	q := sqlf.Sprintf("WHERE username IN (%s) AND deleted_at IS NULL ORDER BY id ASC", sqlf.Join(items, ","))
	return u.getBySQL(ctx, q)
}

var ErrNoCurrentUser = errors.New("no current user")

func (u *userStore) GetByCurrentAuthUser(ctx context.Context) (*types.User, error) {
	a := actor.FromContext(ctx)
	if !a.IsAuthenticated() {
		return nil, ErrNoCurrentUser
	}

	return a.User(ctx, u)
}

func (u *userStore) InvalidateSessionsByID(ctx context.Context, id int32) (err error) {
	return u.InvalidateSessionsByIDs(ctx, []int32{id})
}

func (u *userStore) InvalidateSessionsByIDs(ctx context.Context, ids []int32) (err error) {
	tx, err := u.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	userIDs := make([]*sqlf.Query, len(ids))
	for i := range ids {
		userIDs[i] = sqlf.Sprintf("%d", ids[i])
	}
	query := sqlf.Sprintf(`
		UPDATE users
		SET
			updated_at=now(),
			invalidated_sessions_at=now()
		WHERE id IN (%d)`, sqlf.Join(userIDs, ","))

	res, err := tx.ExecResult(ctx, query)
	if err != nil {
		return err
	}
	nrows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if nrows != int64(len(ids)) {
		return userNotFoundErr{args: []any{fmt.Sprintf("Some users were not found. Expected to invalidate sessions of %d users, but invalidated sessions only %d", +len(ids), nrows)}}
	}
	return nil
}

func (u *userStore) CountForSCIM(ctx context.Context, opt *UsersListOptions) (int, error) {
	if opt == nil {
		opt = &UsersListOptions{}
	}
	opt.includeDeleted = true
	return u.Count(ctx, opt)
}

func (u *userStore) Count(ctx context.Context, opt *UsersListOptions) (int, error) {
	if opt == nil {
		opt = &UsersListOptions{}
	}
	conds := u.listSQL(*opt)
	q := sqlf.Sprintf("SELECT COUNT(*) FROM users u WHERE %s", sqlf.Join(conds, "AND"))

	var count int
	if err := u.QueryRow(ctx, q).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// UsersListOptions specifies the options for listing users.
type UsersListOptions struct {
	// Query specifies a search query for users.
	Query string
	// UserIDs specifies a list of user IDs to include.
	UserIDs []int32
	// Usernames specifies a list of usernames to include.
	Usernames []string
	// Only show users inside this org
	OrgID int32

	// InactiveSince filters out users that have had an eventlog entry with a
	// `timestamp` greater-than-or-equal to the given timestamp.
	InactiveSince time.Time

	// Filter out users with a known Sourcegraph admin username
	//
	// Deprecated: Use ExcludeSourcegraphOperators instead. If you have to use this,
	// then set both fields with the same value at the same time.
	ExcludeSourcegraphAdmins bool
	// ExcludeSourcegraphOperators indicates whether to exclude Sourcegraph Operator
	// user accounts.
	ExcludeSourcegraphOperators bool
	// includeDeleted indicates whether to include soft deleted user accounts.
	//
	// The intention is that external consumers should not need to interact with soft deleted users but
	// internally there are valid reasons to include them.
	includeDeleted bool

	*LimitOffset
}

func (u *userStore) List(ctx context.Context, opt *UsersListOptions) (_ []*types.User, err error) {
	tr, ctx := trace.New(ctx, "database.Users.List", attribute.String("opt", fmt.Sprintf("%+v", opt)))
	defer tr.EndWithErr(&err)

	if opt == nil {
		opt = &UsersListOptions{}
	}
	conds := u.listSQL(*opt)

	q := sqlf.Sprintf("WHERE %s ORDER BY id ASC %s", sqlf.Join(conds, "AND"), opt.LimitOffset.SQL())
	return u.getBySQL(ctx, q)
}

// ListForSCIM lists users along with their email addresses and SCIM ExternalID.
func (u *userStore) ListForSCIM(ctx context.Context, opt *UsersListOptions) (_ []*types.UserForSCIM, err error) {
	tr, ctx := trace.New(ctx, "database.Users.ListForSCIM", attribute.String("opt", fmt.Sprintf("%+v", opt)))
	defer tr.EndWithErr(&err)

	if opt == nil {
		opt = &UsersListOptions{}
	}
	opt.includeDeleted = true
	conditions := u.listSQL(*opt)

	q := sqlf.Sprintf("WHERE %s ORDER BY id ASC %s", sqlf.Join(conditions, "AND"), opt.LimitOffset.SQL())
	return u.getBySQLForSCIM(ctx, q)
}

// ListDates lists all user's created and deleted dates, used by usage stats.
func (u *userStore) ListDates(ctx context.Context) (dates []types.UserDates, _ error) {
	rows, err := u.Query(ctx, sqlf.Sprintf(listDatesQuery))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var d types.UserDates

		err := rows.Scan(&d.UserID, &d.CreatedAt, &dbutil.NullTime{Time: &d.DeletedAt})
		if err != nil {
			return nil, err
		}

		dates = append(dates, d)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return dates, nil
}

func (u *userStore) ListByOrg(ctx context.Context, orgID int32, paginationArgs *PaginationArgs, query *string) ([]*types.User, error) {
	where := []*sqlf.Query{
		sqlf.Sprintf(orgMembershipCond, orgID),
		sqlf.Sprintf("u.deleted_at IS NULL"),
	}

	if cond := newQueryCond(query); cond != nil {
		where = append(where, cond)
	}

	p := paginationArgs.SQL()

	if p.Where != nil {
		where = append(where, p.Where)
	}

	q := sqlf.Sprintf("WHERE %s", sqlf.Join(where, "AND"))
	q = p.AppendOrderToQuery(q)
	q = p.AppendLimitToQuery(q)

	return u.getBySQL(ctx, q)
}

const listDatesQuery = `
SELECT id, created_at, deleted_at
FROM users
ORDER BY id ASC
`

const listUsersInactiveCond = `
(NOT EXISTS (
	SELECT 1 FROM event_logs
	WHERE
		event_logs.user_id = u.id
	AND
		timestamp >= %s
))
`

const orgMembershipCond = `
EXISTS (
	SELECT 1
	FROM org_members
	WHERE
		org_members.user_id = u.id
		AND org_members.org_id = %d)
`

func newQueryCond(query *string) *sqlf.Query {
	if query != nil && *query != "" {
		q := "%" + *query + "%"

		items := []*sqlf.Query{
			sqlf.Sprintf("username ILIKE %s", q),
			sqlf.Sprintf("display_name ILIKE %s", q),
		}

		// Query looks like an ID
		if id, ok := maybeQueryIsID(*query); ok {
			items = append(items, sqlf.Sprintf("id = %d", id))
		}

		return sqlf.Sprintf("(%s)", sqlf.Join(items, " OR "))
	}

	return nil
}

func (*userStore) listSQL(opt UsersListOptions) (conds []*sqlf.Query) {
	conds = []*sqlf.Query{sqlf.Sprintf("TRUE")}

	if !opt.includeDeleted {
		conds = append(conds, sqlf.Sprintf("deleted_at IS NULL"))
	}

	if cond := newQueryCond(&opt.Query); cond != nil {
		conds = append(conds, cond)
	}

	if opt.UserIDs != nil {
		if len(opt.UserIDs) == 0 {
			// Must return empty result set.
			conds = append(conds, sqlf.Sprintf("FALSE"))
		} else {
			items := []*sqlf.Query{}
			for _, id := range opt.UserIDs {
				items = append(items, sqlf.Sprintf("%d", id))
			}
			conds = append(conds, sqlf.Sprintf("u.id IN (%s)", sqlf.Join(items, ",")))
		}
	}

	if len(opt.Usernames) > 0 {
		conds = append(conds, sqlf.Sprintf("u.username = ANY(%s)", pq.Array(opt.Usernames)))
	}
	if opt.OrgID != 0 {
		conds = append(conds, sqlf.Sprintf(orgMembershipCond, opt.OrgID))
	}

	if !opt.InactiveSince.IsZero() {
		conds = append(conds, sqlf.Sprintf(listUsersInactiveCond, opt.InactiveSince))
	}

	// NOTE: This is a hack which should be replaced when we have proper user types.
	// However, for billing purposes and more accurate ping data, we need a way to
	// exclude Sourcegraph (employee) admins when counting users. The following
	// username patterns, in conjunction with the presence of a corresponding
	// "@sourcegraph.com" email address, are used to filter out Sourcegraph admins:
	//
	// - managed-*
	// - sourcegraph-management-*
	// - sourcegraph-admin
	//
	// This method of filtering is imperfect and may still incur false positives, but
	// the two together should help prevent that in the majority of cases, and we
	// acknowledge this risk as we would prefer to undercount rather than overcount.
	//
	// TODO(jchen): This hack will be removed as part of https://github.com/sourcegraph/customer/issues/1531
	if opt.ExcludeSourcegraphAdmins {
		conds = append(conds, sqlf.Sprintf(`
-- The user does not...
NOT(
	-- ...have a known Sourcegraph admin username pattern
	(u.username ILIKE 'managed-%%'
		OR u.username ILIKE 'sourcegraph-management-%%'
		OR u.username = 'sourcegraph-admin')
	-- ...and have a matching sourcegraph email address
	AND EXISTS (
		SELECT
			1 FROM user_emails
		WHERE
			user_emails.user_id = u.id
			AND user_emails.email ILIKE '%%@sourcegraph.com')
)
`))
	}

	if opt.ExcludeSourcegraphOperators {
		conds = append(conds, sqlf.Sprintf(`
NOT EXISTS (
	SELECT FROM user_external_accounts
	WHERE
		service_type = 'sourcegraph-operator'
	AND user_id = u.id
)
`))
	}
	return conds
}

func (u *userStore) getOneBySQL(ctx context.Context, q *sqlf.Query) (*types.User, error) {
	users, err := u.getBySQL(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(users) != 1 {
		return nil, userNotFoundErr{q.Args()}
	}
	return users[0], nil
}

// getBySQL returns users matching the SQL query, if any exist.
func (u *userStore) getBySQL(ctx context.Context, query *sqlf.Query) ([]*types.User, error) {
	q := sqlf.Sprintf(`
SELECT u.id,
	u.username,
	u.display_name,
	u.avatar_url,
	u.created_at,
	u.updated_at,
	u.site_admin,
	u.passwd IS NOT NULL,
	u.invalidated_sessions_at,
	u.tos_accepted,
	u.completed_post_signup,
	EXISTS (SELECT 1 FROM user_external_accounts WHERE service_type = 'scim' AND user_id = u.id AND deleted_at IS NULL) AS scim_controlled
FROM users u %s`, query)
	rows, err := u.Query(ctx, q)
	if err != nil {
		return nil, err
	}

	users := []*types.User{}
	defer rows.Close()
	for rows.Next() {
		var u types.User
		var displayName, avatarURL sql.NullString
		err := rows.Scan(&u.ID, &u.Username, &displayName, &avatarURL, &u.CreatedAt, &u.UpdatedAt, &u.SiteAdmin, &u.BuiltinAuth, &u.InvalidatedSessionsAt, &u.TosAccepted, &u.CompletedPostSignup, &u.SCIMControlled)
		if err != nil {
			return nil, err
		}
		u.DisplayName = displayName.String
		u.AvatarURL = avatarURL.String
		users = append(users, &u)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

const userForSCIMQueryFmtStr = `
WITH scim_accounts AS (
    SELECT
        user_id,
        account_id,
        account_data
    FROM user_external_accounts
    WHERE service_type = 'scim'
)
SELECT u.id,
       u.username,
       u.display_name,
       u.avatar_url,
       u.created_at,
       u.updated_at,
	   u.deleted_at,
       u.site_admin,
       u.passwd IS NOT NULL,
       u.invalidated_sessions_at,
       u.tos_accepted,
       ARRAY(SELECT email FROM user_emails WHERE user_id = u.id AND verified_at IS NOT NULL) AS emails,
       sa.account_id AS scim_external_id,
       sa.account_data AS scim_account_data
FROM users u
LEFT JOIN scim_accounts sa ON u.id = sa.user_id
%s`

// getBySQLForSCIM returns users matching the SQL query, along with their email addresses and SCIM ExternalID.
func (u *userStore) getBySQLForSCIM(ctx context.Context, query *sqlf.Query) ([]*types.UserForSCIM, error) {
	// NOTE: We use a separate query here because we want to fetch the emails and SCIM ExternalID in a single query.
	q := sqlf.Sprintf(userForSCIMQueryFmtStr, query)
	scanUsersForSCIM := basestore.NewSliceScanner(scanUserForSCIM)
	return scanUsersForSCIM(u.Query(ctx, q))
}

// scanUserForSCIM scans a UserForSCIM from the return of a *sql.Rows.
func scanUserForSCIM(s dbutil.Scanner) (*types.UserForSCIM, error) {
	var u types.UserForSCIM
	var displayName, avatarURL, scimExternalID, scimAccountData sql.NullString
	var deletedAt sql.NullTime
	err := s.Scan(&u.ID, &u.Username, &displayName, &avatarURL, &u.CreatedAt, &u.UpdatedAt, &deletedAt, &u.SiteAdmin, &u.BuiltinAuth, &u.InvalidatedSessionsAt, &u.TosAccepted, pq.Array(&u.Emails), &scimExternalID, &scimAccountData)
	if err != nil {
		return nil, err
	}
	u.DisplayName = displayName.String
	u.AvatarURL = avatarURL.String
	u.SCIMExternalID = scimExternalID.String
	u.SCIMAccountData = scimAccountData.String
	u.Active = !deletedAt.Valid
	return &u, nil
}

func (u *userStore) IsPassword(ctx context.Context, id int32, password string) (bool, error) {
	var passwd sql.NullString
	if err := u.QueryRow(ctx, sqlf.Sprintf("SELECT passwd FROM users WHERE deleted_at IS NULL AND id=%s", id)).Scan(&passwd); err != nil {
		return false, err
	}
	if !passwd.Valid {
		return false, nil
	}
	return validPassword(passwd.String, password), nil
}

var (
	passwordResetRateLimit    = "1 minute"
	ErrPasswordResetRateLimit = errors.New("password reset rate limit reached")
)

func (u *userStore) RenewPasswordResetCode(ctx context.Context, id int32) (string, error) {
	if _, err := u.GetByID(ctx, id); err != nil {
		return "", err
	}
	var b [40]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	code := base64.StdEncoding.EncodeToString(b[:])
	res, err := u.ExecResult(ctx, sqlf.Sprintf("UPDATE users SET passwd_reset_code=%s, passwd_reset_time=now() WHERE id=%s AND (passwd_reset_time IS NULL OR passwd_reset_time + interval '"+passwordResetRateLimit+"' < now())", code, id))
	if err != nil {
		return "", err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return "", err
	}
	if affected == 0 {
		return "", ErrPasswordResetRateLimit
	}

	return code, nil
}

// SetPassword sets the user's password given a new password and a password reset code
func (u *userStore) SetPassword(ctx context.Context, id int32, resetCode, newPassword string) (bool, error) {
	// 🚨 SECURITY: Check min and max password length
	if err := CheckPassword(newPassword); err != nil {
		return false, err
	}

	resetLinkExpiryDuration := conf.AuthPasswordResetLinkExpiry()

	// 🚨 SECURITY: check resetCode against what's in the DB and that it's not expired
	r := u.QueryRow(ctx, sqlf.Sprintf("SELECT count(*) FROM users WHERE id=%s AND deleted_at IS NULL AND passwd_reset_code=%s AND passwd_reset_time + interval '"+strconv.Itoa(resetLinkExpiryDuration)+" seconds' > now()", id, resetCode))

	var ct int
	if err := r.Scan(&ct); err != nil {
		return false, err
	}
	if ct > 1 {
		return false, errors.Errorf("illegal state: found more than one user matching ID %d", id)
	}
	if ct == 0 {
		return false, nil
	}
	passwd, err := hashPassword(newPassword)
	if err != nil {
		return false, err
	}
	// 🚨 SECURITY: set the new password and clear the reset code and expiry so the same code can't be reused.
	if err := u.Exec(ctx, sqlf.Sprintf("UPDATE users SET passwd_reset_code=NULL, passwd_reset_time=NULL, passwd=%s WHERE id=%s", passwd, id)); err != nil {
		return false, err
	}

	return true, nil
}

func (u *userStore) DeletePasswordResetCode(ctx context.Context, id int32) error {
	err := u.Exec(ctx, sqlf.Sprintf("UPDATE users SET passwd_reset_code=NULL, passwd_reset_time=NULL WHERE id=%s", id))
	return err
}

// UpdatePassword updates a user's password given the current password.
func (u *userStore) UpdatePassword(ctx context.Context, id int32, oldPassword, newPassword string) error {
	// 🚨 SECURITY: Old password cannot be blank
	if oldPassword == "" {
		return errors.New("old password was empty")
	}
	// 🚨 SECURITY: Make sure the caller provided the correct old password.
	if ok, err := u.IsPassword(ctx, id, oldPassword); err != nil {
		return err
	} else if !ok {
		return errors.New("wrong old password")
	}

	if err := CheckPassword(newPassword); err != nil {
		return err
	}

	passwd, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	// 🚨 SECURITY: Set the new password
	if err := u.Exec(ctx, sqlf.Sprintf("UPDATE users SET passwd_reset_code=NULL, passwd_reset_time=NULL, passwd=%s WHERE id=%s", passwd, id)); err != nil {
		return err
	}

	return nil
}

// SetChatCompletionsQuota sets the user's quota override for completions. Nil means unset.
func (u *userStore) SetChatCompletionsQuota(ctx context.Context, id int32, quota *int) error {
	if quota == nil {
		return u.Exec(ctx, sqlf.Sprintf("UPDATE users SET completions_quota = NULL WHERE id = %s", id))
	}
	return u.Exec(ctx, sqlf.Sprintf("UPDATE users SET completions_quota = %s WHERE id = %s", *quota, id))
}

// GetChatCompletionsQuota reads the user's quota override for completions. Nil means unset.
func (u *userStore) GetChatCompletionsQuota(ctx context.Context, id int32) (*int, error) {
	quota, found, err := basestore.ScanFirstInt(u.Query(ctx, sqlf.Sprintf("SELECT completions_quota FROM users WHERE id = %s AND completions_quota IS NOT NULL", id)))
	if err != nil {
		return nil, err
	}
	if found {
		return &quota, nil
	}
	return nil, nil
}

// SetCodeCompletionsQuota sets the user's quota override for code completions. Nil means unset.
func (u *userStore) SetCodeCompletionsQuota(ctx context.Context, id int32, quota *int) error {
	if quota == nil {
		return u.Exec(ctx, sqlf.Sprintf("UPDATE users SET code_completions_quota = NULL WHERE id = %s", id))
	}
	return u.Exec(ctx, sqlf.Sprintf("UPDATE users SET code_completions_quota = %s WHERE id = %s", *quota, id))
}

// GetCodeCompletionsQuota reads the user's quota override for code completions. Nil means unset.
func (u *userStore) GetCodeCompletionsQuota(ctx context.Context, id int32) (*int, error) {
	quota, found, err := basestore.ScanFirstInt(u.Query(ctx, sqlf.Sprintf("SELECT code_completions_quota FROM users WHERE id = %s AND code_completions_quota IS NOT NULL", id)))
	if err != nil {
		return nil, err
	}
	if found {
		return &quota, nil
	}
	return nil, nil
}

// CreatePassword creates a user's password if they don't have a password.
func (u *userStore) CreatePassword(ctx context.Context, id int32, password string) error {
	// 🚨 SECURITY: Check min and max password length
	if err := CheckPassword(password); err != nil {
		return err
	}

	passwd, err := hashPassword(password)
	if err != nil {
		return err
	}

	// 🚨 SECURITY: Create the password
	res, err := u.ExecResult(ctx, sqlf.Sprintf(`
UPDATE users
SET passwd=%s
WHERE id=%s
  AND deleted_at IS NULL
  AND passwd IS NULL
  AND passwd_reset_code IS NULL
  AND passwd_reset_time IS NULL
`, passwd, id))
	if err != nil {
		return errors.Wrap(err, "creating password")
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "checking rows affected when creating password")
	}

	if affected == 0 {
		return errors.New("password not created")
	}

	return nil
}

// RandomizePasswordAndClearPasswordResetRateLimit overwrites a user's password with a hard-to-guess
// random password and clears the password reset rate limit. It is intended to be used by site admins,
// who can subsequently generate a new password reset code for the user (in case the user has locked
// themselves out, or in case the site admin wants to initiate a password reset).
//
// A randomized password is used (instead of an empty password) to avoid bugs where an empty password
// is considered to be no password. The random password is expected to be irretrievable.
func (u *userStore) RandomizePasswordAndClearPasswordResetRateLimit(ctx context.Context, id int32) error {
	passwd, err := hashPassword(randstring.NewLen(36))
	if err != nil {
		return err
	}
	// 🚨 SECURITY: Set the new random password and clear the reset code/expiry, so the old code
	// can't be reused, and so a new valid reset code can be generated afterward.
	err = u.Exec(ctx, sqlf.Sprintf("UPDATE users SET passwd_reset_code=NULL, passwd_reset_time=NULL, passwd=%s WHERE id=%s", passwd, id))
	if err == nil {
		LogPasswordEvent(ctx, NewDBWith(u.logger, u), nil, SecurityEventNamPasswordRandomized, id)
	}
	return err
}

func LogPasswordEvent(ctx context.Context, db DB, r *http.Request, name SecurityEventName, userID int32) {
	a := actor.FromContext(ctx)
	args := struct {
		Requester int32 `json:"requester"`
	}{
		Requester: a.UID,
	}

	var path string
	var host string
	if r != nil {
		path = r.URL.Path
		host = r.URL.Host
	}
	if err := db.SecurityEventLogs().LogSecurityEvent(ctx, name, path, uint32(userID), "", "BACKEND", args); err != nil {
		log.Error(err)
	}

	eArgs, _ := json.Marshal(struct {
		Requester int32 `json:"requester"`
		Requestee int32 `json:"requestee"`
	}{
		Requester: a.UID,
		Requestee: userID,
	})
	logEvent := &Event{
		Name:            string(name),
		AnonymousUserID: "backend",
		URL:             host,
		Argument:        eArgs,
		Source:          "BACKEND",
		Timestamp:       time.Now(),
	}

	//lint:ignore SA1019 existing usage of deprecated functionality. Use EventRecorder from internal/telemetryrecorder instead.
	_ = db.EventLogs().Insert(ctx, logEvent)
}

func hashPassword(password string) (sql.NullString, error) {
	if MockHashPassword != nil {
		return MockHashPassword(password)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{Valid: true, String: string(hash)}, nil
}

func validPassword(hash, password string) bool {
	if MockValidPassword != nil {
		return MockValidPassword(hash, password)
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// MockHashPassword if non-nil is used instead of database.hashPassword. This is useful
// when running tests since we can use a faster implementation.
var (
	MockHashPassword  func(password string) (sql.NullString, error)
	MockValidPassword func(hash, password string) bool
)

//nolint:unused // used in tests
func useFastPasswordMocks() {
	// We can't care about security in tests, we care about speed.
	MockHashPassword = func(password string) (sql.NullString, error) {
		h := fnv.New64()
		_, _ = io.WriteString(h, password)
		return sql.NullString{Valid: true, String: strconv.FormatUint(h.Sum64(), 16)}, nil
	}
	MockValidPassword = func(hash, password string) bool {
		h := fnv.New64()
		_, _ = io.WriteString(h, password)
		return hash == strconv.FormatUint(h.Sum64(), 16)
	}
}
