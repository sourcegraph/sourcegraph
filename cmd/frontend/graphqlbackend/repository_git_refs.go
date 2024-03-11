package graphqlbackend

import (
	"context"
	"strings"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
)

type refsArgs struct {
	graphqlutil.ConnectionArgs
	Query *string
	Type  *string
}

func (r *RepositoryResolver) Branches(ctx context.Context, args *refsArgs) (*gitRefConnectionResolver, error) {
	t := gitRefTypeBranch
	args.Type = &t
	return r.GitRefs(ctx, args)
}

func (r *RepositoryResolver) Tags(ctx context.Context, args *refsArgs) (*gitRefConnectionResolver, error) {
	t := gitRefTypeTag
	args.Type = &t
	return r.GitRefs(ctx, args)
}

func (r *RepositoryResolver) GitRefs(ctx context.Context, args *refsArgs) (*gitRefConnectionResolver, error) {
	gc := gitserver.NewClient("graphql.repo.refs")

	refs, err := gc.ListRefs(ctx, r.RepoName(), gitserver.ListRefsOpts{
		HeadsOnly: args.Type == nil || *args.Type == gitRefTypeBranch,
		TagsOnly:  args.Type == nil || *args.Type == gitRefTypeTag,
	})
	if err != nil {
		return nil, err
	}

	query := ""
	if args.Query != nil {
		query = strings.ToLower(*args.Query)
	}

	// Combine branches and tags.
	resolvers := make([]*GitRefResolver, 0, len(refs))
	for _, ref := range refs {
		if query != "" {
			if !strings.Contains(strings.ToLower(ref.ShortName), query) {
				continue
			}
		}
		resolvers = append(resolvers, &GitRefResolver{
			name:   ref.Name,
			repo:   r,
			target: GitObjectID(ref.CommitID),
		})
	}

	return &gitRefConnectionResolver{
		first: args.First,
		refs:  resolvers,
	}, nil
}

type gitRefConnectionResolver struct {
	first *int32
	refs  []*GitRefResolver
}

func (r *gitRefConnectionResolver) Nodes() []*GitRefResolver {
	var nodes []*GitRefResolver

	// Paginate.
	if r.first != nil && len(r.refs) > int(*r.first) {
		nodes = r.refs[:int(*r.first)]
	} else {
		nodes = r.refs
	}

	return nodes
}

func (r *gitRefConnectionResolver) TotalCount() int32 {
	return int32(len(r.refs))
}

func (r *gitRefConnectionResolver) PageInfo() *graphqlutil.PageInfo {
	return graphqlutil.HasNextPage(r.first != nil && int(*r.first) < len(r.refs))
}
