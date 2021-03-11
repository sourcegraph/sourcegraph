package resolvers

import (
	"context"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/sourcegraph/go-diff/diff"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

func marshalChangesetSpecRandID(id string) graphql.ID {
	return relay.MarshalID("ChangesetSpec", id)
}

func unmarshalChangesetSpecID(id graphql.ID) (changesetSpecRandID string, err error) {
	err = relay.UnmarshalSpec(id, &changesetSpecRandID)
	return
}

var _ graphqlbackend.ChangesetSpecResolver = &changesetSpecResolver{}

type changesetSpecResolver struct {
	store *store.Store

	changesetSpec *batches.ChangesetSpec

	repo *types.Repo
}

func NewChangesetSpecResolver(ctx context.Context, store *store.Store, changesetSpec *batches.ChangesetSpec) (*changesetSpecResolver, error) {
	resolver := &changesetSpecResolver{
		store:         store,
		changesetSpec: changesetSpec,
	}

	// 🚨 SECURITY: database.Repos.GetByIDs uses the authzFilter under the hood and
	// filters out repositories that the user doesn't have access to.
	// In case we don't find a repository, it might be because it's deleted
	// or because the user doesn't have access.
	rs, err := store.Repos().GetByIDs(ctx, changesetSpec.RepoID)
	if err != nil {
		return nil, err
	}

	// Not found is ok, the resolver will disguise as a HiddenChangesetResolver.
	if len(rs) == 1 {
		resolver.repo = rs[0]
	}

	return resolver, nil
}

func NewChangesetSpecResolverWithRepo(store *store.Store, repo *types.Repo, changesetSpec *batches.ChangesetSpec) *changesetSpecResolver {
	return &changesetSpecResolver{
		store:         store,
		repo:          repo,
		changesetSpec: changesetSpec,
	}
}

func (r *changesetSpecResolver) ID() graphql.ID {
	// 🚨 SECURITY: This needs to be the RandID! We can't expose the
	// sequential, guessable ID.
	return marshalChangesetSpecRandID(r.changesetSpec.RandID)
}

func (r *changesetSpecResolver) Type() batches.ChangesetSpecDescriptionType {
	return r.changesetSpec.Spec.Type()
}

func (r *changesetSpecResolver) Description(ctx context.Context) (graphqlbackend.ChangesetDescription, error) {
	descriptionResolver := &changesetDescriptionResolver{
		store: r.store,
		desc:  r.changesetSpec.Spec,
		// Note: r.repo can never be nil, because Description is a VisibleChangesetSpecResolver-only field.
		repoResolver: graphqlbackend.NewRepositoryResolver(r.store.DB(), r.repo),
		diffStat:     r.changesetSpec.DiffStat(),
	}

	return descriptionResolver, nil
}

func (r *changesetSpecResolver) ExpiresAt() *graphqlbackend.DateTime {
	return &graphqlbackend.DateTime{Time: r.changesetSpec.ExpiresAt()}
}

func (r *changesetSpecResolver) repoAccessible() bool {
	// If the repository is not nil, it's accessible
	return r.repo != nil
}

func (r *changesetSpecResolver) ToHiddenChangesetSpec() (graphqlbackend.HiddenChangesetSpecResolver, bool) {
	if r.repoAccessible() {
		return nil, false
	}

	return r, true
}

func (r *changesetSpecResolver) ToVisibleChangesetSpec() (graphqlbackend.VisibleChangesetSpecResolver, bool) {
	if !r.repoAccessible() {
		return nil, false
	}

	return r, true
}

var _ graphqlbackend.ChangesetDescription = &changesetDescriptionResolver{}

// changesetDescriptionResolver implements both ChangesetDescription
// interfaces: ExistingChangesetReferenceResolver and
// GitBranchChangesetDescriptionResolver.
type changesetDescriptionResolver struct {
	store        *store.Store
	repoResolver *graphqlbackend.RepositoryResolver
	desc         *batches.ChangesetSpecDescription
	diffStat     diff.Stat
}

func (r *changesetDescriptionResolver) ToExistingChangesetReference() (graphqlbackend.ExistingChangesetReferenceResolver, bool) {
	if r.desc.IsImportingExisting() {
		return r, true
	}
	return nil, false
}
func (r *changesetDescriptionResolver) ToGitBranchChangesetDescription() (graphqlbackend.GitBranchChangesetDescriptionResolver, bool) {
	if r.desc.IsBranch() {
		return r, true
	}
	return nil, false
}

func (r *changesetDescriptionResolver) BaseRepository() *graphqlbackend.RepositoryResolver {
	return r.repoResolver
}
func (r *changesetDescriptionResolver) ExternalID() string { return r.desc.ExternalID }
func (r *changesetDescriptionResolver) BaseRef() string    { return git.AbbreviateRef(r.desc.BaseRef) }
func (r *changesetDescriptionResolver) BaseRev() string    { return r.desc.BaseRev }
func (r *changesetDescriptionResolver) HeadRepository() *graphqlbackend.RepositoryResolver {
	return r.repoResolver
}
func (r *changesetDescriptionResolver) HeadRef() string { return git.AbbreviateRef(r.desc.HeadRef) }
func (r *changesetDescriptionResolver) Title() string   { return r.desc.Title }
func (r *changesetDescriptionResolver) Body() string    { return r.desc.Body }
func (r *changesetDescriptionResolver) Published() batches.PublishedValue {
	return r.desc.Published
}

func (r *changesetDescriptionResolver) DiffStat() *graphqlbackend.DiffStat {
	return graphqlbackend.NewDiffStat(r.diffStat)
}

func (r *changesetDescriptionResolver) Diff(ctx context.Context) (graphqlbackend.PreviewRepositoryComparisonResolver, error) {
	diff, err := r.desc.Diff()
	if err != nil {
		return nil, err
	}
	return graphqlbackend.NewPreviewRepositoryComparisonResolver(ctx, r.store.DB(), r.repoResolver, r.desc.BaseRev, diff)
}

func (r *changesetDescriptionResolver) Commits() []graphqlbackend.GitCommitDescriptionResolver {
	var resolvers []graphqlbackend.GitCommitDescriptionResolver
	for _, c := range r.desc.Commits {
		resolvers = append(resolvers, &gitCommitDescriptionResolver{
			store:       r.store,
			message:     c.Message,
			diff:        c.Diff,
			authorName:  c.AuthorName,
			authorEmail: c.AuthorEmail,
		})
	}
	return resolvers
}

var _ graphqlbackend.GitCommitDescriptionResolver = &gitCommitDescriptionResolver{}

type gitCommitDescriptionResolver struct {
	store       *store.Store
	message     string
	diff        string
	authorName  string
	authorEmail string
}

func (r *gitCommitDescriptionResolver) Author() *graphqlbackend.PersonResolver {
	return graphqlbackend.NewPersonResolver(
		r.store.DB(),
		r.authorName,
		r.authorEmail,
		// Try to find the corresponding Sourcegraph user.
		true,
	)
}
func (r *gitCommitDescriptionResolver) Message() string { return r.message }
func (r *gitCommitDescriptionResolver) Subject() string {
	return git.Message(r.message).Subject()
}
func (r *gitCommitDescriptionResolver) Body() *string {
	body := git.Message(r.message).Body()
	if body == "" {
		return nil
	}
	return &body
}
func (r *gitCommitDescriptionResolver) Diff() string { return r.diff }
