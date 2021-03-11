package search

import (
	"context"
	"reflect"
	"strings"
	"sync"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

// RevisionSpecifier represents either a revspec or a ref glob. At most one
// field is set. The default branch is represented by all fields being empty.
type RevisionSpecifier struct {
	// RevSpec is a revision range specifier suitable for passing to git. See
	// the manpage gitrevisions(7).
	RevSpec string

	// RefGlob is a reference glob to pass to git. See the documentation for
	// "--glob" in git-log.
	RefGlob string

	// ExcludeRefGlob is a glob for references to exclude. See the
	// documentation for "--exclude" in git-log.
	ExcludeRefGlob string
}

func (r1 RevisionSpecifier) String() string {
	if r1.ExcludeRefGlob != "" {
		return "*!" + r1.ExcludeRefGlob
	}
	if r1.RefGlob != "" {
		return "*" + r1.RefGlob
	}
	return r1.RevSpec
}

// Less compares two revspecOrRefGlob entities, suitable for use
// with sort.Slice()
//
// possibly-undesired: this results in treating an entity with
// no revspec, but a refGlob, as "earlier" than any revspec.
func (r1 RevisionSpecifier) Less(r2 RevisionSpecifier) bool {
	if r1.RevSpec != r2.RevSpec {
		return r1.RevSpec < r2.RevSpec
	}
	if r1.RefGlob != r2.RefGlob {
		return r1.RefGlob < r2.RefGlob
	}
	return r1.ExcludeRefGlob < r2.ExcludeRefGlob
}

// RepositoryRevisions specifies a repository and 0 or more revspecs and ref
// globs.  If no revspecs and no ref globs are specified, then the
// repository's default branch is used.
type RepositoryRevisions struct {
	Repo *types.RepoName
	Revs []RevisionSpecifier

	// resolveOnce protects resolvedRevs
	resolveOnce sync.Once

	// resolvedRevs is set by ExpandedRevSpecs and contains all revisions
	// including resolved ref-globs.
	resolvedRevs []string

	// resolveErr stores the error returned by the first call to ExpandedRevSpecs. It
	// gives the caller the chance to distinguish between an error and an empty resolvedRevs.
	resolveErr error

	// ListRefs is called to list all Git refs for a repository. It is intended to be mocked by
	// tests. If nil, git.ListRefs is used.
	ListRefs func(context.Context, api.RepoName) ([]git.Ref, error)
}

// Equal provides custom comparison which is used by go-cmp
func (r *RepositoryRevisions) Equal(other *RepositoryRevisions) bool {
	return reflect.DeepEqual(r.Repo, other.Repo) && reflect.DeepEqual(r.Revs, other.Revs)
}

// ParseRepositoryRevisions parses strings that refer to a repository and 0
// or more revspecs. The format is:
//
//   repo@revs
//
// where repo is a repository regex and revs is a ':'-separated list of revspecs
// and/or ref globs. A ref glob is a revspec prefixed with '*' (which is not a
// valid revspec or ref itself; see `man git-check-ref-format`). The '@' and revs
// may be omitted to refer to the default branch.
//
// For example:
//
// - 'foo' refers to the 'foo' repo at the default branch
// - 'foo@bar' refers to the 'foo' repo and the 'bar' revspec.
// - 'foo@bar:baz:qux' refers to the 'foo' repo and 3 revspecs: 'bar', 'baz',
//   and 'qux'.
// - 'foo@*bar' refers to the 'foo' repo and all refs matching the glob 'bar/*',
//   because git interprets the ref glob 'bar' as being 'bar/*' (see `man git-log`
//   section on the --glob flag)
func ParseRepositoryRevisions(repoAndOptionalRev string) (string, []RevisionSpecifier) {
	i := strings.Index(repoAndOptionalRev, "@")
	if i == -1 {
		// return an empty slice to indicate that there's no revisions; callers
		// have to distinguish between "none specified" and "default" to handle
		// cases where two repo specs both match the same repository, and only one
		// specifies a revspec, which normally implies "master" but in that case
		// really means "didn't specify"
		return repoAndOptionalRev, []RevisionSpecifier{}
	}

	repo := repoAndOptionalRev[:i]
	var revs []RevisionSpecifier
	for _, part := range strings.Split(repoAndOptionalRev[i+1:], ":") {
		if part == "" {
			continue
		}
		revs = append(revs, parseRev(part))
	}
	if len(revs) == 0 {
		revs = []RevisionSpecifier{{RevSpec: ""}} // default branch
	}
	return repo, revs
}

func parseRev(spec string) RevisionSpecifier {
	if strings.HasPrefix(spec, "*!") {
		return RevisionSpecifier{ExcludeRefGlob: spec[2:]}
	} else if strings.HasPrefix(spec, "*") {
		return RevisionSpecifier{RefGlob: spec[1:]}
	}
	return RevisionSpecifier{RevSpec: spec}
}

// GitserverRepo is a convenience function to return the api.RepoName for
// r.Repo. The returned Repo will not have the URL set, only the name.
func (r *RepositoryRevisions) GitserverRepo() api.RepoName {
	return r.Repo.Name
}

func (r *RepositoryRevisions) String() string {
	if len(r.Revs) == 0 {
		return string(r.Repo.Name)
	}

	parts := make([]string, len(r.Revs))
	for i, rev := range r.Revs {
		parts[i] = rev.String()
	}
	return string(r.Repo.Name) + "@" + strings.Join(parts, ":")
}

// OnlyExplicit returns true if all revspecs in Revs are explicit.
func (r *RepositoryRevisions) OnlyExplicit() bool {
	for _, rev := range r.Revs {
		if rev.RefGlob != "" || rev.ExcludeRefGlob != "" {
			return false
		}
	}
	return true
}

// RevSpecs returns a list of all explicitly listed Git revspecs. It does not expand ref globs to
// their matching revspecs.
func (r *RepositoryRevisions) RevSpecs() []string {
	var revspecs []string
	for _, rev := range r.Revs {
		if rev.RefGlob == "" && rev.ExcludeRefGlob == "" {
			revspecs = append(revspecs, rev.RevSpec)
		}
	}
	return revspecs
}

// ExpandedRevSpecs is a wrapper around expandedRevSpecs. It uses a sync.Once
// to ensure we only resolve revisions once. The resolved revisions and the error response
// are stored in r and returned to future callers.
//
// Note that storing the error causes all callers to return the same error. For example,
// if the first caller has a context error, all other callers will return a context error, too.
//
// Not all callers need to expand ref glob expressions. If a caller is passing the ref globs as
// command-line args to `git` directly (e.g., to `git log --glob ... --exclude ...`), it does not
// need to use this function.
func (r *RepositoryRevisions) ExpandedRevSpecs(ctx context.Context) ([]string, error) {
	r.resolveOnce.Do(func() {
		revSpecsList, err := expandedRevSpec(ctx, r)
		if err != nil {
			r.resolveErr = err
			return
		}
		r.resolvedRevs = revSpecsList
	})
	return r.resolvedRevs, r.resolveErr
}

// expandedRevSpecs evaluates all of r's ref glob expressions and returns the full, current list of
// refs matched or resolved by them, plus the explicitly listed Git revspecs. See
// git.CompileRefGlobs for information on how ref include/exclude globs are handled.
func expandedRevSpec(ctx context.Context, r *RepositoryRevisions) ([]string, error) {
	listRefs := r.ListRefs
	if listRefs == nil {
		listRefs = git.ListRefs
	}

	var (
		revSpecs = map[string]struct{}{}
		globs    []git.RefGlob
	)
	for _, rev := range r.Revs {
		switch {
		case rev.RefGlob != "":
			globs = append(globs, git.RefGlob{Include: rev.RefGlob})
		case rev.ExcludeRefGlob != "":
			globs = append(globs, git.RefGlob{Exclude: rev.ExcludeRefGlob})
		default:
			revSpecs[rev.RevSpec] = struct{}{}
		}
	}
	if len(globs) > 0 {
		allRefs, err := listRefs(ctx, r.GitserverRepo())
		if err != nil {
			return nil, err
		}

		rg, err := git.CompileRefGlobs(globs)
		if err != nil {
			return nil, err
		}

		for _, ref := range allRefs {
			if rg.Match(ref.Name) {
				revSpecs[strings.TrimPrefix(ref.Name, "refs/heads/")] = struct{}{}
			}
		}
	}

	revSpecsList := make([]string, 0, len(revSpecs))
	for revSpec := range revSpecs {
		revSpecsList = append(revSpecsList, revSpec)
	}
	return revSpecsList, nil
}
