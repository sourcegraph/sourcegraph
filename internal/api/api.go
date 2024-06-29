// Package api contains an API client and types for cross-service communication.
package api

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// RepoID is the unique identifier for a repository.
type RepoID int32

// RepoName is the name of a repository, consisting of one or more "/"-separated path components.
//
// Previously, this was called RepoURI.
type RepoName string

func (r RepoName) Attr() attribute.KeyValue {
	return attribute.String("repo", string(r))
}

func (r RepoName) Equal(o RepoName) bool {
	return strings.EqualFold(string(r), string(o))
}

// RepoHashedName is the hashed name of a repo
type RepoHashedName string

var deletedRegex = lazyregexp.New("^DELETED-[0-9]+\\.[0-9]+-")

// UndeletedRepoName will "undelete" a repo name. When we soft-delete a repo we
// change its name in the database, this function extracts the original repo
// name.
func UndeletedRepoName(name RepoName) RepoName {
	return RepoName(deletedRegex.ReplaceAllString(string(name), ""))
}

var validCommitIDRegexp = lazyregexp.New(`^[a-fA-F0-9]{40}$`)

// NewCommitID creates a new CommitID and validates that it conforms to the
// requirements of the type.
func NewCommitID(s string) (CommitID, error) {
	if validCommitIDRegexp.MatchString(s) {
		return CommitID(s), nil
	}
	return "", errors.Newf("invalid CommitID %q", s)
}

// CommitID is the 40-character SHA-1 hash for a Git commit.
type CommitID string

func (c CommitID) Attr() attribute.KeyValue {
	return attribute.String("commitID", string(c))
}

// Short returns the SHA-1 commit hash truncated to 7 characters
func (c CommitID) Short() string {
	if len(c) >= 7 {
		return string(c)[:7]
	}
	return string(c)
}

// ExternalRepoSpec specifies a repository on an external service (such as GitHub or GitLab).
type ExternalRepoSpec struct {
	// ID is the repository's ID on the external service. Its value is opaque except to the repo-updater.
	//
	// For GitHub, this is the GitHub GraphQL API's node ID for the repository.
	ID string

	// ServiceType is the type of external service. Its value is opaque except to the repo-updater.
	//
	// Example: "github", "gitlab", etc.
	ServiceType string

	// ServiceID is the particular instance of the external service where this repository resides. Its value is
	// opaque but typically consists of the canonical base URL to the service.
	//
	// Implementations must take care to normalize this URL. For example, if different GitHub.com repository code
	// paths used slightly different values here (such as "https://github.com/" and "https://github.com", note the
	// lack of trailing slash), then the same logical repository would be incorrectly treated as multiple distinct
	// repositories depending on the code path that provided its ServiceID value.
	//
	// Example: "https://github.com/", "https://github-enterprise.example.com/"
	ServiceID string
}

// Equal returns true if r is equal to s.
func (r ExternalRepoSpec) Equal(s *ExternalRepoSpec) bool {
	return r.ID == s.ID && r.ServiceType == s.ServiceType && r.ServiceID == s.ServiceID
}

// Compare returns -1 if r < s, 0 if r == s or 1 if r > s
func (r ExternalRepoSpec) Compare(s ExternalRepoSpec) int {
	if r.ServiceType != s.ServiceType {
		return cmp.Compare(r.ServiceType, s.ServiceType)
	}
	if r.ServiceID != s.ServiceID {
		return cmp.Compare(r.ServiceID, s.ServiceID)
	}
	return cmp.Compare(r.ID, s.ID)
}

func (r ExternalRepoSpec) String() string {
	return fmt.Sprintf("ExternalRepoSpec{%s %s %s}", r.ServiceID, r.ServiceType, r.ID)
}

// A SettingsSubject is something that can have settings. Exactly 1 field must be nonzero.
type SettingsSubject struct {
	Default bool   // whether this is for default settings
	Site    bool   // whether this is for global settings
	Org     *int32 // the org's ID
	User    *int32 // the user's ID
}

func (s SettingsSubject) String() string {
	switch {
	case s.Default:
		return "DefaultSettings"
	case s.Site:
		return "site"
	case s.Org != nil:
		return fmt.Sprintf("org %d", *s.Org)
	case s.User != nil:
		return fmt.Sprintf("user %d", *s.User)
	default:
		return "unknown settings subject"
	}
}

// Settings contains settings for a subject.
type Settings struct {
	ID           int32           // the unique ID of this settings value
	Subject      SettingsSubject // the subject of these settings
	AuthorUserID *int32          // the ID of the user who authored this settings value
	Contents     string          // the raw JSON (with comments and trailing commas allowed)
	CreatedAt    time.Time       // the date when this settings value was created
}
