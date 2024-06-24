package graphqlbackend

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/graph-gophers/graphql-go"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type repositoryArgs struct {
	Query *string // Search query
	Names *[]string

	Cloned     bool
	NotCloned  bool
	Indexed    bool
	NotIndexed bool

	Embedded    bool
	NotEmbedded bool

	CloneStatus *string
	FailedFetch bool
	Corrupted   bool

	ExternalService *graphql.ID

	OrderBy    string
	Descending bool
	graphqlutil.ConnectionResolverArgs
}

func (args *repositoryArgs) toReposListOptions() (database.ReposListOptions, error) {
	opt := database.ReposListOptions{}
	if args.Names != nil {
		opt.Names = *args.Names
	}
	if args.Query != nil {
		opt.Query = *args.Query
	}

	if args.CloneStatus != nil {
		opt.CloneStatus = types.ParseCloneStatusFromGraphQL(*args.CloneStatus)
	}

	opt.FailedFetch = args.FailedFetch
	opt.OnlyCorrupted = args.Corrupted

	if !args.Cloned && !args.NotCloned {
		return database.ReposListOptions{}, errors.New("excluding cloned and not cloned repos leaves an empty set")
	}
	if !args.Cloned {
		opt.NoCloned = true
	}
	if !args.NotCloned {
		// notCloned is true by default.
		// this condition is valid only if it has been
		// explicitly set to false by the client.
		opt.OnlyCloned = true
	}

	if !args.Indexed && !args.NotIndexed {
		return database.ReposListOptions{}, errors.New("excluding indexed and not indexed repos leaves an empty set")
	}
	if !args.Indexed {
		opt.NoIndexed = true
	}
	if !args.NotIndexed {
		opt.OnlyIndexed = true
	}

	if !args.Embedded && !args.NotEmbedded {
		return database.ReposListOptions{}, errors.New("excluding embedded and not embedded repos leaves an empty set")
	}
	if !args.Embedded {
		opt.NoEmbedded = true
	}
	if !args.NotEmbedded {
		opt.OnlyEmbedded = true
	}

	if args.ExternalService != nil {
		extSvcID, err := UnmarshalExternalServiceID(*args.ExternalService)
		if err != nil {
			return opt, err
		}
		opt.ExternalServiceIDs = append(opt.ExternalServiceIDs, extSvcID)
	}

	return opt, nil
}

func (r *schemaResolver) Repositories(ctx context.Context, args *repositoryArgs) (*graphqlutil.ConnectionResolver[*RepositoryResolver], error) {
	opt, err := args.toReposListOptions()
	if err != nil {
		return nil, err
	}

	connectionStore := &repositoriesConnectionStore{
		ctx:    ctx,
		db:     r.db,
		logger: r.logger.Scoped("repositoryConnectionResolver"),
		opt:    opt,
	}

	maxPageSize := 1000

	// `REPOSITORY_NAME` is the enum value in the graphql schema.
	orderBy := "REPOSITORY_NAME"
	if args.OrderBy != "" {
		orderBy = args.OrderBy
	}

	connectionOptions := graphqlutil.ConnectionResolverOptions{
		MaxPageSize: maxPageSize,
		OrderBy:     database.OrderBy{{Field: string(toDBRepoListColumn(orderBy))}, {Field: "id"}},
		Ascending:   !args.Descending,
	}

	return graphqlutil.NewConnectionResolver[*RepositoryResolver](connectionStore, &args.ConnectionResolverArgs, &connectionOptions)
}

type repositoriesConnectionStore struct {
	ctx    context.Context
	logger log.Logger
	db     database.DB
	opt    database.ReposListOptions
}

func (s *repositoriesConnectionStore) MarshalCursor(node *RepositoryResolver, orderBy database.OrderBy) (*string, error) {
	column := orderBy[0].Field
	var value string

	switch database.RepoListColumn(column) {
	case database.RepoListName:
		value = node.Name()
	case database.RepoListCreatedAt:
		value = node.RawCreatedAt()
	case database.RepoListSize:
		size, err := node.DiskSizeBytes(s.ctx)
		if err != nil {
			return nil, err
		}
		value = strconv.FormatInt(int64(*size), 10)
	default:
		return nil, errors.New(fmt.Sprintf("invalid OrderBy.Field. Expected: one of (name, created_at, gr.repo_size_bytes). Actual: %s", column))
	}

	cursor := MarshalRepositoryCursor(
		&types.Cursor{
			Column: column,
			Value:  fmt.Sprintf("%s@%d", value, node.IDInt32()),
		},
	)

	return &cursor, nil
}

func (s *repositoriesConnectionStore) UnmarshalCursor(cursor string, orderBy database.OrderBy) ([]any, error) {
	repoCursor, err := UnmarshalRepositoryCursor(&cursor)
	if err != nil {
		return nil, err
	}

	if len(orderBy) == 0 {
		return nil, errors.New("no orderBy provided")
	}

	column := orderBy[0].Field
	if repoCursor.Column != column {
		return nil, errors.New(fmt.Sprintf("Invalid cursor. Expected: %s Actual: %s", column, repoCursor.Column))
	}

	values := strings.Split(repoCursor.Value, "@")
	if len(values) != 2 {
		return nil, errors.New(fmt.Sprintf("Invalid cursor. Expected Value: <%s>@<id> Actual Value: %s", column, repoCursor.Value))
	}

	repoID, err := strconv.Atoi(values[1])
	if err != nil {
		return nil, err
	}

	switch database.RepoListColumn(column) {
	case database.RepoListName:
		return []any{values[0], repoID}, nil
	case database.RepoListCreatedAt:
		return []any{values[0], repoID}, nil
	case database.RepoListSize:
		size, err := strconv.ParseInt(values[0], 10, 64)
		if err != nil {
			return nil, err
		}
		return []any{size, repoID}, nil
	default:
		return nil, errors.New("Invalid OrderBy Field.")
	}
}

func (s *repositoriesConnectionStore) ComputeTotal(ctx context.Context) (int32, error) {
	// 🚨 SECURITY: Only site admins can list all repos, because a total repository
	// count does not respect repository permissions.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, s.db); err != nil {
		return 0, nil
	}

	if dotcom.LazilySyncsIndefinitelyManyRepositories() {
		return 0, nil
	}

	count, err := s.db.Repos().Count(ctx, s.opt)
	return int32(count), err
}

func (s *repositoriesConnectionStore) ComputeNodes(ctx context.Context, args *database.PaginationArgs) ([]*RepositoryResolver, error) {
	opt := s.opt
	opt.PaginationArgs = args

	client := gitserver.NewClient("graphql.repos")
	repos, err := backend.NewRepos(s.logger, s.db, client).List(ctx, opt)
	if err != nil {
		return nil, err
	}

	resolvers := make([]*RepositoryResolver, 0, len(repos))
	for _, repo := range repos {
		resolvers = append(resolvers, NewRepositoryResolver(s.db, client, repo))
	}

	return resolvers, nil
}

type TotalCountArgs struct {
	Precise bool
}

type RepositoryConnectionResolver interface {
	Nodes(ctx context.Context) ([]*RepositoryResolver, error)
	TotalCount(ctx context.Context, args *TotalCountArgs) (*int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

func toDBRepoListColumn(ob string) database.RepoListColumn {
	switch ob {
	case "REPO_URI", "REPOSITORY_NAME":
		return database.RepoListName
	case "REPO_CREATED_AT", "REPOSITORY_CREATED_AT":
		return database.RepoListCreatedAt
	case "SIZE":
		return database.RepoListSize
	default:
		return ""
	}
}
