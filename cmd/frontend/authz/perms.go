package authz

import (
	"fmt"
	"time"

	"github.com/RoaringBitmap/roaring"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/types"
)

var ErrPermsNotFound = errors.New("permissions not found")

// RepoPerms contains a repo and the permissions a given user
// has associated with it.
type RepoPerms struct {
	Repo  *types.Repo
	Perms Perms
}

// Perms is a permission set represented as bitset.
type Perms uint32

// Perm constants.
const (
	None Perms = 0
	Read Perms = 1 << iota
	Write
)

// Include is a convenience method to test if Perms
// includes all the other Perms.
func (p Perms) Include(other Perms) bool {
	return p&other == other
}

// String implements the fmt.Stringer interface.
func (p Perms) String() string {
	switch p {
	case Read:
		return "read"
	case Write:
		return "write"
	case Read | Write:
		return "read,write"
	default:
		return "none"
	}
}

// PermType is the object type of the user permissions.
type PermType string

// The list of available user permission types.
const (
	PermRepos PermType = "repos"
)

// RepoPermsSort sorts a slice of RepoPerms to guarantee a stable ordering.
type RepoPermsSort []RepoPerms

func (s RepoPermsSort) Len() int      { return len(s) }
func (s RepoPermsSort) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s RepoPermsSort) Less(i, j int) bool {
	if s[i].Repo.ID != s[j].Repo.ID {
		return s[i].Repo.ID < s[j].Repo.ID
	}
	if s[i].Repo.ExternalRepo.ID != s[j].Repo.ExternalRepo.ID {
		return s[i].Repo.ExternalRepo.ID < s[j].Repo.ExternalRepo.ID
	}
	return s[i].Repo.Name < s[j].Repo.Name
}

// ErrStalePermissions is returned by LoadPermissions when the stored
// permissions are stale (e.g. the first time a user needs them and they haven't
// been fetched yet). Callers should pass this error up to the user and show a
// more friendly prompt message in the UI.
type ErrStalePermissions struct {
	UserID int32
	Perm   Perms
	Type   PermType
}

// Error implements the error interface.
func (e ErrStalePermissions) Error() string {
	return fmt.Sprintf("%s:%s permissions for user=%d are stale and being updated", e.Perm, e.Type, e.UserID)
}

// UserPermissions are the permissions of a user to perform an action
// on the given set of object IDs of the defined type that is scoped by
// the provider.
type UserPermissions struct {
	UserID    int32
	Perm      Perms
	Type      PermType
	IDs       *roaring.Bitmap
	UpdatedAt time.Time
}

// Expired returns true if these UserPermissions have elapsed the given ttl.
func (p *UserPermissions) Expired(ttl time.Duration, now time.Time) bool {
	return !now.Before(p.UpdatedAt.Add(ttl))
}

// AuthorizedRepos returns the intersection of the given repository IDs with
// the authorized IDs.
func (p *UserPermissions) AuthorizedRepos(repos []*types.Repo) []RepoPerms {
	// Return directly if it's used for wrong permissions type or no permissions available.
	if p.Type != PermRepos ||
		p.IDs == nil || p.IDs.GetCardinality() == 0 {
		return []RepoPerms{}
	}

	perms := make([]RepoPerms, 0, len(repos))
	for _, r := range repos {
		if r.ID != 0 && p.IDs.Contains(uint32(r.ID)) {
			perms = append(perms, RepoPerms{Repo: r, Perms: p.Perm})
		}
	}
	return perms
}

// TracingFields returns tracing fields for the opentracing log.
func (p *UserPermissions) TracingFields() []otlog.Field {
	fs := []otlog.Field{
		otlog.Int32("UserPermissions.UserID", p.UserID),
		otlog.String("UserPermissions.Perm", string(p.Perm)),
		otlog.String("UserPermissions.Type", string(p.Type)),
	}

	if p.IDs != nil {
		fs = append(fs,
			otlog.Uint64("UserPermissions.IDs.Count", p.IDs.GetCardinality()),
			otlog.String("UserPermissions.UpdatedAt", p.UpdatedAt.String()),
		)
	}

	return fs
}

// RepoPermissions declares which users have access to a given repository, as
// defined by the permissions provider (either Bitbucket Server or Sourcegraph).
type RepoPermissions struct {
	RepoID    int32
	Perm      Perms
	UserIDs   *roaring.Bitmap
	UpdatedAt time.Time
}

// Expired returns true if these RepoPermissions have elapsed the given ttl.
func (p *RepoPermissions) Expired(ttl time.Duration, now time.Time) bool {
	return !now.Before(p.UpdatedAt.Add(ttl))
}

// TracingFields returns tracing fields for the opentracing log.
func (p *RepoPermissions) TracingFields() []otlog.Field {
	fs := []otlog.Field{
		otlog.Int32("RepoPermissions.RepoID", p.RepoID),
		otlog.String("RepoPermissions.Perm", string(p.Perm)),
	}

	if p.UserIDs != nil {
		fs = append(fs,
			otlog.Uint64("RepoPermissions.UserIDs.Count", p.UserIDs.GetCardinality()),
			otlog.String("RepoPermissions.UpdatedAt", p.UpdatedAt.String()),
		)
	}

	return fs
}

// UserPendingPermissions defines permissions that a not-yet-created user has to
// perform on a given set of object IDs. Not-yet-created users may exist on the
// code host but not yet in Sourcegraph. `BindID` is used to map this stub user
// to an actual user when the actual user is created; it can either be a username
// or email.
type UserPendingPermissions struct {
	ID        int32
	BindID    string
	Perm      Perms
	Type      PermType
	IDs       *roaring.Bitmap
	UpdatedAt time.Time
}

// TracingFields returns tracing fields for the opentracing log.
func (p *UserPendingPermissions) TracingFields() []otlog.Field {
	fs := []otlog.Field{
		otlog.Int32("UserPendingPermissions.ID", p.ID),
		otlog.String("UserPendingPermissions.BindID", p.BindID),
		otlog.String("UserPendingPermissions.Perm", string(p.Perm)),
		otlog.String("UserPendingPermissions.Type", string(p.Type)),
	}

	if p.IDs != nil {
		fs = append(fs,
			otlog.Uint64("UserPendingPermissions.IDs.Count", p.IDs.GetCardinality()),
			otlog.String("UserPendingPermissions.UpdatedAt", p.UpdatedAt.String()),
		)
	}

	return fs
}
