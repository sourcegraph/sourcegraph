package graphqlbackend

import (
	"context"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type savedSearchResolver struct {
	db database.DB
	s  types.SavedSearch
}

func marshalSavedSearchID(savedSearchID int32) graphql.ID {
	return relay.MarshalID("SavedSearch", savedSearchID)
}

func unmarshalSavedSearchID(id graphql.ID) (savedSearchID int32, err error) {
	err = relay.UnmarshalSpec(id, &savedSearchID)
	return
}

func (r *schemaResolver) savedSearchByID(ctx context.Context, id graphql.ID) (*savedSearchResolver, error) {
	intID, err := unmarshalSavedSearchID(id)
	if err != nil {
		return nil, err
	}

	ss, err := r.db.SavedSearches().GetByID(ctx, intID)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Make sure the current user has permission to get the saved search.
	if err := checkAuthorizedForNamespaceByIDs(ctx, r.db, ss.Owner); err != nil {
		return nil, err
	}

	savedSearch := &savedSearchResolver{
		db: r.db,
		s:  *ss,
	}
	return savedSearch, nil
}

func (r savedSearchResolver) ID() graphql.ID {
	return marshalSavedSearchID(r.s.ID)
}

func (r savedSearchResolver) Description() string { return r.s.Description }

func (r savedSearchResolver) Query() string { return r.s.Query }

func (r savedSearchResolver) Owner(ctx context.Context) (*NamespaceResolver, error) {
	if r.s.Owner.User != nil {
		n, err := NamespaceByID(ctx, r.db, MarshalUserID(*r.s.Owner.User))
		if err != nil {
			return nil, err
		}
		return &NamespaceResolver{n}, nil
	}
	if r.s.Owner.Org != nil {
		n, err := NamespaceByID(ctx, r.db, MarshalOrgID(*r.s.Owner.Org))
		if err != nil {
			return nil, err
		}
		return &NamespaceResolver{n}, nil
	}
	return nil, nil
}

func (r *schemaResolver) toSavedSearchResolver(entry types.SavedSearch) *savedSearchResolver {
	return &savedSearchResolver{db: r.db, s: entry}
}

type savedSearchesArgs struct {
	graphqlutil.ConnectionResolverArgs
	Owner              *graphql.ID
	ViewerIsAffiliated *bool
}

func (r *schemaResolver) SavedSearches(ctx context.Context, args savedSearchesArgs) (*graphqlutil.ConnectionResolver[*savedSearchResolver], error) {
	connectionStore := &savedSearchesConnectionStore{db: r.db}

	if args.Owner != nil {
		// 🚨 SECURITY: Make sure the current user has permission to view saved searches of the
		// specified owner.
		owner, err := checkAuthorizedForNamespace(ctx, r.db, *args.Owner)
		if err != nil {
			return nil, err
		}
		connectionStore.listArgs.Owner = owner
	}

	if args.ViewerIsAffiliated != nil && *args.ViewerIsAffiliated {
		currentUser, err := auth.CurrentUser(ctx, r.db)
		if err != nil {
			return nil, err
		}
		connectionStore.listArgs.AffiliatedUser = &currentUser.ID
	}

	// 🚨 SECURITY: Only site admins can list all saved searches.
	if connectionStore.listArgs.Owner == nil && connectionStore.listArgs.AffiliatedUser == nil {
		if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
			return nil, errors.Wrap(err, "must specify owner or viewerIsAffiliated args")
		}
	}

	return graphqlutil.NewConnectionResolver[*savedSearchResolver](connectionStore, &args.ConnectionResolverArgs, nil)
}

type savedSearchesConnectionStore struct {
	db       database.DB
	listArgs database.SavedSearchListArgs
}

func (s *savedSearchesConnectionStore) MarshalCursor(node *savedSearchResolver, _ database.OrderBy) (*string, error) {
	cursor := string(node.ID())

	return &cursor, nil
}

func (s *savedSearchesConnectionStore) UnmarshalCursor(cursor string, _ database.OrderBy) ([]any, error) {
	nodeID, err := unmarshalSavedSearchID(graphql.ID(cursor))
	if err != nil {
		return nil, err
	}

	return []any{nodeID}, nil
}

func (s *savedSearchesConnectionStore) ComputeTotal(ctx context.Context) (int32, error) {
	count, err := s.db.SavedSearches().Count(ctx, s.listArgs)
	return int32(count), err
}

func (s *savedSearchesConnectionStore) ComputeNodes(ctx context.Context, pgArgs *database.PaginationArgs) ([]*savedSearchResolver, error) {
	dbResults, err := s.db.SavedSearches().List(ctx, s.listArgs, pgArgs)
	if err != nil {
		return nil, err
	}

	var results []*savedSearchResolver
	for _, savedSearch := range dbResults {
		results = append(results, &savedSearchResolver{db: s.db, s: *savedSearch})
	}

	return results, nil
}

type savedSearchInput struct {
	Owner       graphql.ID
	Description string
	Query       string
}

func (r *schemaResolver) CreateSavedSearch(ctx context.Context, args *struct {
	Input savedSearchInput
}) (*savedSearchResolver, error) {
	// 🚨 SECURITY: Make sure the current user has permission to create a saved search in the
	// specified owner namespace.
	namespace, err := checkAuthorizedForNamespace(ctx, r.db, args.Input.Owner)
	if err != nil {
		return nil, err
	}

	if !queryHasPatternType(args.Input.Query) {
		return nil, errMissingPatternType
	}

	ss, err := r.db.SavedSearches().Create(ctx, &types.SavedSearch{
		Description: args.Input.Description,
		Query:       args.Input.Query,
		Owner:       *namespace,
	})
	if err != nil {
		return nil, err
	}

	return r.toSavedSearchResolver(*ss), nil
}

type savedSearchUpdateInput struct {
	Description string
	Query       string
}

func (r *schemaResolver) UpdateSavedSearch(ctx context.Context, args *struct {
	ID    graphql.ID
	Input savedSearchUpdateInput
}) (*savedSearchResolver, error) {
	id, err := unmarshalSavedSearchID(args.ID)
	if err != nil {
		return nil, err
	}

	old, err := r.db.SavedSearches().GetByID(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, "get existing saved search")
	}

	// 🚨 SECURITY: Make sure the current user has permission to update a saved search for the
	// specified owner namespace.
	if err := checkAuthorizedForNamespaceByIDs(ctx, r.db, old.Owner); err != nil {
		return nil, err
	}

	if !queryHasPatternType(args.Input.Query) {
		return nil, errMissingPatternType
	}

	ss, err := r.db.SavedSearches().Update(ctx, &types.SavedSearch{
		ID:          id,
		Description: args.Input.Description,
		Query:       args.Input.Query,
		Owner:       old.Owner,
	})
	if err != nil {
		return nil, err
	}

	return r.toSavedSearchResolver(*ss), nil
}

func (r *schemaResolver) DeleteSavedSearch(ctx context.Context, args *struct {
	ID graphql.ID
}) (*EmptyResponse, error) {
	id, err := unmarshalSavedSearchID(args.ID)
	if err != nil {
		return nil, err
	}
	ss, err := r.db.SavedSearches().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Make sure the current user has permission to delete a saved search for the
	// specified owner namespace.
	if err := checkAuthorizedForNamespaceByIDs(ctx, r.db, ss.Owner); err != nil {
		return nil, err
	}

	if err := r.db.SavedSearches().Delete(ctx, id); err != nil {
		return nil, err
	}
	return &EmptyResponse{}, nil
}

var patternType = lazyregexp.New(`(?i)\bpatternType:(literal|regexp|structural|standard|keyword)\b`)

func queryHasPatternType(query string) bool {
	return patternType.Match([]byte(query))
}

var errMissingPatternType = errors.New("a `patternType:` filter is required in the query for all saved searches. `patternType` can be \"keyword\", \"standard\", \"literal\", or \"regexp\"")
