package auth

import (
	"context"
	"fmt"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var ErrMustBeSiteAdmin = errors.New("must be site admin")
var ErrMustBeSiteAdminOrSameUser = &InsufficientAuthorizationError{"must be authenticated as the user or be a site admin"}

// CheckCurrentUserIsSiteAdmin returns an error if the current user is NOT a site admin.
func CheckCurrentUserIsSiteAdmin(ctx context.Context, db database.DB) error {
	if actor.FromContext(ctx).IsInternal() {
		return nil
	}
	user, err := CurrentUser(ctx, db)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotAuthenticated
	}
	if !user.SiteAdmin {
		return ErrMustBeSiteAdmin
	}
	return nil
}

// CheckUserIsSiteAdmin returns an error if the user is NOT a site admin.
func CheckUserIsSiteAdmin(ctx context.Context, db database.DB, userID int32) error {
	if actor.FromContext(ctx).IsInternal() {
		return nil
	}
	user, err := db.Users().GetByID(ctx, userID)
	if err != nil {
		if errcode.IsNotFound(err) || err == database.ErrNoCurrentUser {
			return ErrNotAuthenticated
		}
		return err
	}
	if !user.SiteAdmin {
		return ErrMustBeSiteAdmin
	}
	return nil
}

// InsufficientAuthorizationError is an error that occurs when the authentication is technically valid
// (e.g., the token is not expired) but does not yield a user with privileges to perform a certain
// action.
type InsufficientAuthorizationError struct {
	Message string
}

func (e *InsufficientAuthorizationError) Error() string      { return e.Message }
func (e *InsufficientAuthorizationError) Unauthorized() bool { return true }

// CheckSiteAdminOrSameUser returns an error if the user is NEITHER (1) a
// site admin NOR (2) the user specified by subjectUserID.
//
// It is used when an action on a user can be performed by site admins and the
// user themselves, but nobody else.
//
// Returns an error without the name of the given user.
func CheckSiteAdminOrSameUser(ctx context.Context, db database.DB, subjectUserID int32) error {
	a := actor.FromContext(ctx)
	return CheckSiteAdminOrSameUserFromActor(a, db, subjectUserID)
}

// CheckSameUser returns an error if the user is not the user specified by
// subjectUserID.
func CheckSameUser(ctx context.Context, subjectUserID int32) error {
	a := actor.FromContext(ctx)
	if a.IsInternal() || (a.IsAuthenticated() && a.UID == subjectUserID) {
		return nil
	}
	return &InsufficientAuthorizationError{Message: fmt.Sprintf("must be authenticated as user with id %d", subjectUserID)}
}

// CurrentUser gets the current authenticated user
// It returns nil, nil if no user is found
func CurrentUser(ctx context.Context, db database.DB) (*types.User, error) {
	user, err := db.Users().GetByCurrentAuthUser(ctx)
	if err != nil {
		if errcode.IsNotFound(err) || err == database.ErrNoCurrentUser {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

// CheckCurrentActorIsSiteAdmin returns an error if the current user derived from
// actor is NOT a site admin.
func CheckCurrentActorIsSiteAdmin(a *actor.Actor, db database.DB) error {
	if a.IsInternal() {
		return nil
	}
	// If actor already contains a user, then no DB query will be made. Background
	// context here is fine, because it is used only for a DB query.
	user, err := a.User(context.Background(), db.Users())
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotAuthenticated
	}
	if !user.SiteAdmin {
		return ErrMustBeSiteAdmin
	}
	return nil
}

// CheckSiteAdminOrSameUserFromActor returns an error if the user derived from actor is
// NEITHER (1) a site admin NOR (2) the user specified by subjectUserID.
//
// It is used when an action on a user can be performed by site admins and the
// user themselves, but nobody else.
//
// Returns an error without the name of the given user.
func CheckSiteAdminOrSameUserFromActor(a *actor.Actor, db database.DB, subjectUserID int32) error {
	if a.IsInternal() || (a.IsAuthenticated() && a.UID == subjectUserID) {
		return nil
	}
	isSiteAdminErr := CheckCurrentActorIsSiteAdmin(a, db)
	if isSiteAdminErr == nil {
		return nil
	}
	return ErrMustBeSiteAdminOrSameUser
}

// CheckSameUserFromActor returns an error if the user derived from actor is not the user
// specified by subjectUserID.
func CheckSameUserFromActor(a *actor.Actor, subjectUserID int32) error {
	if a.IsInternal() || (a.IsAuthenticated() && a.UID == subjectUserID) {
		return nil
	}
	return &InsufficientAuthorizationError{Message: fmt.Sprintf("must be authenticated as user with id %d", subjectUserID)}
}
