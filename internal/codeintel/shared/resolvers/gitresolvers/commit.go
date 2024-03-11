package gitresolvers

import (
	"context"
	"fmt"
	"sync"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/internal/api"
	resolverstubs "github.com/sourcegraph/sourcegraph/internal/codeintel/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
)

type commitResolver struct {
	gitserverClient gitserver.Client
	repo            resolverstubs.RepositoryResolver
	oid             resolverstubs.GitObjectID
	rev             string

	tags     []string
	tagsErr  error
	tagsOnce sync.Once
}

func NewGitCommitResolver(
	gitserverClient gitserver.Client,
	repo resolverstubs.RepositoryResolver,
	commitID api.CommitID,
	inputRev string,
) resolverstubs.GitCommitResolver {
	rev := string(commitID)
	if inputRev != "" {
		rev = inputRev
	}

	return &commitResolver{
		gitserverClient: gitserverClient,
		repo:            repo,
		oid:             resolverstubs.GitObjectID(commitID),
		rev:             rev,
	}
}

func (r *commitResolver) ID() graphql.ID {
	return resolverstubs.MarshalID("GitCommit", map[string]any{
		"r": r.repo.ID(),
		"c": r.oid,
	})
}

func (r *commitResolver) Repository() resolverstubs.RepositoryResolver { return r.repo }
func (r *commitResolver) OID() resolverstubs.GitObjectID               { return r.oid }
func (r *commitResolver) AbbreviatedOID() string                       { return string(r.oid)[:7] }
func (r *commitResolver) URL() string                                  { return fmt.Sprintf("/%s/-/commit/%s", r.repo.Name(), r.rev) }
func (r *commitResolver) URI() string                                  { return fmt.Sprintf("/%s@%s", r.repo.Name(), r.rev) }

func (r *commitResolver) Tags(ctx context.Context) ([]string, error) {
	r.tagsOnce.Do(func() {
		rawTags, err := r.gitserverClient.ListRefs(ctx, api.RepoName(r.repo.Name()), gitserver.ListRefsOpts{
			TagsOnly:       true,
			PointsAtCommit: []api.CommitID{api.CommitID(r.oid)},
		})
		if err != nil {
			r.tagsErr = err
			return
		}

		r.tags = make([]string, 0, len(rawTags))
		for _, tag := range rawTags {
			r.tags = append(r.tags, tag.ShortName)
		}
	})

	return r.tags, r.tagsErr
}
