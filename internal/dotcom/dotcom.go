package dotcom

import (
	"strconv"

	"github.com/sourcegraph/sourcegraph/internal/env"
)

var sourcegraphDotComMode, _ = strconv.ParseBool(env.Get("SOURCEGRAPHDOTCOM_MODE", "false", "run as Sourcegraph.com, with add'l marketing and redirects"))

// SourcegraphDotComMode is true if this server is running Sourcegraph.com
// (solely by checking the SOURCEGRAPHDOTCOM_MODE env var). Sourcegraph.com shows
// additional marketing and sets up some additional redirects.
func SourcegraphDotComMode() bool {
	return sourcegraphDotComMode
}

// ProvidesCodySelfServe is true if this instance provides Cody self-serve (PLG) user and team
// management.
//
// This is currently equivalent to `SourcegraphDotComMode()`;  i.e., it is enabled on
// Sourcegraph.com and no other instances.
func ProvidesCodySelfServe() bool {
	return SourcegraphDotComMode()
}

// IsAbusePreventionEnabled is whether abuse prevention is enabled.
//
// This is currently equivalent to `SourcegraphDotComMode()`;  i.e., it is enabled on
// Sourcegraph.com and no other instances.
func IsAbusePreventionEnabled() bool {
	return SourcegraphDotComMode()
}

// IsUserAndOrgProfileDataPrivate is whether user and organization profile and membership data is
// private.
//
//   - If private (true), for example, organization membership is only visible to
//     members of the organization.
//   - If not private (false), for example, user's email addresses, committer email addresses,
//     organization membership, etc., are visible to all users. Sensitive data (such as
//     user credentials and user/org settings) is treated as private.
//
// This is currently equivalent to `SourcegraphDotComMode()`; i.e., it is enabled on Sourcegraph.com
// and no other instances.
func IsUserAndOrgProfileDataPrivate() bool {
	return SourcegraphDotComMode()
}

// SiteAdminCanViewAllUserData is whether the instance's site admin can view all user data,
// including sensitive user data.
//
// This is currently equivalent to `!SourcegraphDotComMode()`; i.e., it is DISABLED on Sourcegraph.com
// and enabled on all other instances.
func SiteAdminCanViewAllUserData() bool {
	return !SourcegraphDotComMode()
}

// LazilySyncsIndefinitelyManyRepositories is true if the instance has a very large and indefinite
// number of synced repositories, such as on Sourcegraph.com where we sync repositories from
// GitHub.com.
//
// This is currently equivalent to `SourcegraphDotComMode()`; i.e., it is enabled on Sourcegraph.com
// and no other instances.
func LazilySyncsIndefinitelyManyRepositories() bool {
	return SourcegraphDotComMode()
}

type TB interface {
	Cleanup(func())
}

// MockSourcegraphDotComMode is used by tests to mock the result of SourcegraphDotComMode.
func MockSourcegraphDotComMode(t TB, value bool) {
	orig := sourcegraphDotComMode
	sourcegraphDotComMode = value
	t.Cleanup(func() { sourcegraphDotComMode = orig })
}
