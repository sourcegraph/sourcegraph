package symbols

import (
	"context"
	"io"

	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/endpoint"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/limiter"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	proto "github.com/sourcegraph/sourcegraph/internal/symbols/v1"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func defaultEndpoints() *endpoint.Map {
	return endpoint.ConfBased(func(conns conftypes.ServiceConnections) []string {
		return conns.Symbols
	})
}

func LoadConfig() {
	DefaultClient = &Client{
		Endpoints:           defaultEndpoints(),
		GRPCConnectionCache: defaults.NewConnectionCache(log.Scoped("symbolsConnectionCache")),
		HTTPLimiter:         limiter.New(500),
		SubRepoPermsChecker: func() authz.SubRepoPermissionChecker { return authz.DefaultSubRepoPermsChecker },
	}
}

// DefaultClient is the default Client. Unless overwritten, it is connected to the server specified by the
// SYMBOLS_URL environment variable.
var DefaultClient *Client

// Client is a symbols service client.
type Client struct {
	// Endpoints to symbols service.
	Endpoints *endpoint.Map

	GRPCConnectionCache *defaults.ConnectionCache

	// Limits concurrency of outstanding HTTP posts
	HTTPLimiter limiter.Limiter

	// SubRepoPermsChecker is function to return the checker to use. It needs to be a
	// function since we expect the client to be set at runtime once we have a
	// database connection.
	SubRepoPermsChecker func() authz.SubRepoPermissionChecker
}

// Search performs a symbol search on the symbols service.
func (c *Client) Search(ctx context.Context, args search.SymbolsParameters) (symbols result.Symbols, limitHit bool, err error) {
	tr, ctx := trace.New(ctx, "symbols.Search",
		args.Repo.Attr(),
		args.CommitID.Attr())
	defer tr.EndWithErr(&err)

	response, err := c.searchGRPC(ctx, args)
	if err != nil {
		return nil, false, errors.Wrap(err, "executing symbols search request")
	}
	if response.Err != "" {
		return nil, false, errors.New(response.Err)
	}

	symbols = response.Symbols
	limitHit = response.LimitHit

	// 🚨 SECURITY: We have valid results, so we need to apply sub-repo permissions
	// filtering.
	if c.SubRepoPermsChecker == nil {
		return symbols, limitHit, err
	}

	checker := c.SubRepoPermsChecker()
	if !authz.SubRepoEnabled(checker) {
		return symbols, limitHit, err
	}

	a := actor.FromContext(ctx)
	// Filter in place
	filtered := symbols[:0]
	for _, r := range symbols {
		rc := authz.RepoContent{
			Repo: args.Repo,
			Path: r.Path,
		}
		perm, err := authz.ActorPermissions(ctx, checker, a, rc)
		if err != nil {
			return nil, false, errors.Wrap(err, "checking sub-repo permissions")
		}
		if perm.Include(authz.Read) {
			filtered = append(filtered, r)
		}
	}

	return filtered, limitHit, nil
}

func (c *Client) searchGRPC(ctx context.Context, args search.SymbolsParameters) (search.SymbolsResponse, error) {
	grpcClient, err := c.gRPCClient(string(args.Repo))
	if err != nil {
		return search.SymbolsResponse{}, errors.Wrap(err, "getting gRPC symbols client")
	}

	var protoArgs proto.SearchRequest
	protoArgs.FromInternal(&args)

	protoResponse, err := grpcClient.Search(ctx, &protoArgs)
	if err != nil {
		return search.SymbolsResponse{}, translateGRPCError(err)
	}

	response := protoResponse.ToInternal()
	return response, nil
}

func (c *Client) LocalCodeIntel(ctx context.Context, path types.RepoCommitPath) (result *types.LocalCodeIntelPayload, err error) {
	tr, ctx := trace.New(ctx, "symbols.LocalCodeIntel",
		attribute.String("repo", path.Repo),
		attribute.String("commitID", path.Commit))
	defer tr.EndWithErr(&err)

	grpcClient, err := c.gRPCClient(path.Repo)
	if err != nil {
		return nil, errors.Wrap(err, "getting gRPC symbols client")
	}

	var rcp proto.RepoCommitPath
	rcp.FromInternal(&path)

	protoArgs := proto.LocalCodeIntelRequest{RepoCommitPath: &rcp}

	client, err := grpcClient.LocalCodeIntel(ctx, &protoArgs)
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			// This ignores errors from LocalCodeIntel to match the behavior found here:
			// https://sourcegraph.com/github.com/sourcegraph/sourcegraph@a1631d58604815917096acc3356447c55baebf22/-/blob/cmd/symbols/squirrel/http_handlers.go?L57-57
			//
			// This is weird, and maybe not intentional, but things break if we return an error.
			return nil, nil
		}
		return nil, translateGRPCError(err)
	}

	var out types.LocalCodeIntelPayload
	for {
		resp, err := client.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) { // end of stream
				return &out, nil
			}

			if status.Code(err) == codes.Unimplemented {
				// This ignores errors from LocalCodeIntel to match the behavior found here:
				// https://sourcegraph.com/github.com/sourcegraph/sourcegraph@a1631d58604815917096acc3356447c55baebf22/-/blob/cmd/symbols/squirrel/http_handlers.go?L57-57
				//
				// This is weird, and maybe not intentional, but things break if we return an error.
				return nil, nil
			}

			return nil, translateGRPCError(err)
		}

		partial := resp.ToInternal()
		if partial != nil {
			out.Symbols = append(out.Symbols, partial.Symbols...)
		}
	}
}

func (c *Client) SymbolInfo(ctx context.Context, args types.RepoCommitPathPoint) (result *types.SymbolInfo, err error) {
	tr, ctx := trace.New(ctx, "squirrel.SymbolInfo",
		attribute.String("repo", args.Repo),
		attribute.String("commitID", args.Commit))
	defer tr.EndWithErr(&err)

	result, err = c.symbolInfoGRPC(ctx, args)
	if err != nil {
		return nil, errors.Wrap(err, "executing symbol info request")
	}

	// 🚨 SECURITY: We have a valid result, so we need to apply sub-repo permissions filtering.
	if c.SubRepoPermsChecker == nil {
		return result, err
	}

	checker := c.SubRepoPermsChecker()
	if !authz.SubRepoEnabled(checker) {
		return result, err
	}

	a := actor.FromContext(ctx)
	// Filter in place
	rc := authz.RepoContent{
		Repo: api.RepoName(args.Repo),
		Path: args.Path,
	}
	perm, err := authz.ActorPermissions(ctx, checker, a, rc)
	if err != nil {
		return nil, errors.Wrap(err, "checking sub-repo permissions")
	}
	if !perm.Include(authz.Read) {
		return nil, nil
	}

	return result, nil
}

func (c *Client) symbolInfoGRPC(ctx context.Context, args types.RepoCommitPathPoint) (result *types.SymbolInfo, err error) {
	client, err := c.gRPCClient(args.Repo)
	if err != nil {
		return nil, errors.Wrap(err, "getting gRPC symbols client")
	}

	var rcp proto.RepoCommitPath
	rcp.FromInternal(&args.RepoCommitPath)

	var point proto.Point
	point.FromInternal(&args.Point)

	protoArgs := proto.SymbolInfoRequest{
		RepoCommitPath: &rcp,
		Point:          &point,
	}

	protoResponse, err := client.SymbolInfo(ctx, &protoArgs)
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			// This ignores unimplemented errors from SymbolInfo to match the behavior here:
			// https://sourcegraph.com/github.com/sourcegraph/sourcegraph@b039aa70fbd155b5b1eddc4b5deede739626a978/-/blob/cmd/symbols/squirrel/http_handlers.go?L114-114
			return nil, nil
		}
		return nil, translateGRPCError(err)
	}

	return protoResponse.ToInternal(), nil
}

func (c *Client) gRPCClient(repo string) (proto.SymbolsServiceClient, error) {
	address, err := c.Endpoints.Get(repo)
	if err != nil {
		return nil, errors.Wrapf(err, "getting symbols server address for repo %q", repo)
	}

	conn, err := c.GRPCConnectionCache.GetConnection(address)
	if err != nil {
		return nil, errors.Wrapf(err, "getting gRPC connection to symbols server at %q", address)
	}

	return &automaticRetryClient{base: proto.NewSymbolsServiceClient(conn)}, nil
}

// translateGRPCError translates gRPC errors to their corresponding context errors, if applicable.
func translateGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	switch st.Code() {
	case codes.Canceled:
		return context.Canceled
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	default:
		return err
	}
}
