package database

import (
	"context"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
)

type BypassAuthzReasonsMap struct {
	SiteAdmin       bool
	IsInternal      bool
	NoAuthzProvider bool
}

type AuthzQueryParameters struct {
	BypassAuthz               bool
	BypassAuthzReasons        BypassAuthzReasonsMap
	UsePermissionsUserMapping bool
	AuthenticatedUserID       int32
	AuthzEnforceForSiteAdmins bool
}

func (p *AuthzQueryParameters) ToAuthzQuery() *sqlf.Query {
	return authzQuery(p.BypassAuthz, p.AuthenticatedUserID)
}

func GetAuthzQueryParameters(ctx context.Context, db DB) (params *AuthzQueryParameters, err error) {
	params = &AuthzQueryParameters{}
	authzAllowByDefault, authzProviders := authz.GetProviders()
	params.UsePermissionsUserMapping = conf.PermissionsUserMapping().Enabled
	params.AuthzEnforceForSiteAdmins = conf.Get().AuthzEnforceForSiteAdmins

	a := actor.FromContext(ctx)

	// Authz is bypassed when the request is coming from an internal actor or
	// there is no authz provider configured and access to all repositories are
	// allowed by default. Authz can be bypassed by site admins unless
	// conf.AuthEnforceForSiteAdmins is set to "true".
	//
	// 🚨 SECURITY: internal requests bypass authz provider permissions checks,
	// so correctness is important here.
	if a.IsInternal() {
		params.BypassAuthz = true
		params.BypassAuthzReasons.IsInternal = true
	}

	// 🚨 SECURITY: If explicit permissions API is ON, we want to enforce authz
	// even if there are no authz providers configured.
	// Otherwise bypass authorization with no authz providers.
	if !params.UsePermissionsUserMapping && authzAllowByDefault && len(authzProviders) == 0 {
		params.BypassAuthz = true
		params.BypassAuthzReasons.NoAuthzProvider = true
	}

	if a.IsAuthenticated() {
		currentUser, err := db.Users().GetByCurrentAuthUser(ctx)
		if err != nil {
			if !params.BypassAuthz {
				return nil, err
			} else {
				return params, nil
			}
		}

		params.AuthenticatedUserID = currentUser.ID

		if currentUser.SiteAdmin && !params.AuthzEnforceForSiteAdmins {
			params.BypassAuthz = true
			params.BypassAuthzReasons.SiteAdmin = true
		}
	}

	return params, err
}

// AuthzQueryConds returns a query clause for enforcing repository permissions.
// It uses `repo` as the table name to filter out repository IDs and should be
// used as an AND condition in a complete SQL query.
func AuthzQueryConds(ctx context.Context, db DB) (*sqlf.Query, error) {
	params, err := GetAuthzQueryParameters(ctx, db)
	if err != nil {
		return nil, err
	}

	return params.ToAuthzQuery(), nil
}

func GetUnrestrictedReposCond() *sqlf.Query {
	return sqlf.Sprintf(`
			-- Unrestricted repos are visible to all users
			EXISTS (
				SELECT
				FROM user_repo_permissions
				WHERE repo_id = repo.id AND user_id IS NULL
			)
		`)
}

var ExternalServiceUnrestrictedCondition = sqlf.Sprintf(`
(
    NOT repo.private          -- Happy path of non-private repositories
    OR  EXISTS (              -- Each external service defines if repositories are unrestricted
        SELECT
        FROM external_services AS es
        JOIN external_service_repos AS esr ON (
                esr.external_service_id = es.id
            AND esr.repo_id = repo.id
            AND es.unrestricted = TRUE
            AND es.deleted_at IS NULL
        )
	)
)
`)

func getRestrictedReposCond(userID int32) *sqlf.Query {
	return sqlf.Sprintf(`
	-- Restricted repositories require checking permissions
	EXISTS (
		SELECT repo_id FROM user_repo_permissions
		WHERE
			repo_id = repo.id
		AND user_id = %s
	)
	`, userID)
}

func authzQuery(bypassAuthz bool, authenticatedUserID int32) *sqlf.Query {
	if bypassAuthz {
		// if bypassAuthz is true, we don't care about any of the checks
		return sqlf.Sprintf(`
(
    -- Bypass authz
    TRUE
)
`)
	}
	conditions := []*sqlf.Query{GetUnrestrictedReposCond(), ExternalServiceUnrestrictedCondition, getRestrictedReposCond(authenticatedUserID)}

	// Have to manually wrap the result in parenthesis so that they're evaluated together
	return sqlf.Sprintf("(%s)", sqlf.Join(conditions, "\nOR\n"))
}
