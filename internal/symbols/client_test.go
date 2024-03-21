package symbols

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/endpoint"
	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"

	proto "github.com/sourcegraph/sourcegraph/internal/symbols/v1"
)

func init() {
	LoadConfig()
}

func TestSearchWithFiltering(t *testing.T) {
	ctx := context.Background()
	fixture := search.SymbolsResponse{
		Symbols: result.Symbols{
			result.Symbol{
				Name: "foo1",
				Path: "file1",
			},
			result.Symbol{
				Name: "foo2",
				Path: "file2",
			},
		},
		LimitHit: true,
	}

	mockServer := &mockSymbolsServer{
		mockSearchGRPC: func(_ context.Context, _ *proto.SearchRequest) (*proto.SearchResponse, error) {
			var response proto.SearchResponse
			response.FromInternal(&fixture)

			return &response, nil
		},

		restHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(fixture)
		}),
	}

	handler, cleanup := mockServer.NewHandler(logtest.Scoped(t))
	srv := httptest.NewServer(handler)
	t.Cleanup(func() {
		srv.Close()
		cleanup()
	})

	DefaultClient.Endpoints = endpoint.Static(srv.URL)

	results, limitHit, err := DefaultClient.Search(ctx, search.SymbolsParameters{
		Repo:     "foo",
		CommitID: "HEAD",
		Query:    "abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if limitHit != true {
		t.Fatal("expected limitHit to be true")
	}
	if results == nil {
		t.Fatal("nil result")
	}
	wantCount := 2
	if len(results) != wantCount {
		t.Fatalf("Want %d results, got %d", wantCount, len(results))
	}

	// With filtering
	ctx = actor.WithActor(ctx, &actor.Actor{
		UID: 1,
	})
	checker := authz.NewMockSubRepoPermissionChecker()
	checker.EnabledFunc.SetDefaultHook(func() bool {
		return true
	})
	checker.PermissionsFunc.SetDefaultHook(func(ctx context.Context, i int32, content authz.RepoContent) (authz.Perms, error) {
		if content.Path == "file1" {
			return authz.Read, nil
		}
		return authz.None, nil
	})
	authz.DefaultSubRepoPermsChecker = checker

	results, limitHit, err = DefaultClient.Search(ctx, search.SymbolsParameters{
		Repo:     "foo",
		CommitID: "HEAD",
		Query:    "abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if limitHit != true {
		t.Fatal("expected limitHit to be true")
	}
	if results == nil {
		t.Fatal("nil result")
	}
	wantCount = 1
	if len(results) != wantCount {
		t.Fatalf("Want %d results, got %d", wantCount, len(results))
	}
}

func TestDefinitionWithFiltering(t *testing.T) {
	// This test conflicts with the previous use of httptest.NewServer, but passes in isolation.
	t.Skip()

	path1 := types.RepoCommitPathPoint{
		RepoCommitPath: types.RepoCommitPath{
			Repo:   "somerepo",
			Commit: "somecommit",
			Path:   "path1",
		},
		Point: types.Point{Row: 0, Column: 0},
	}

	path2 := types.RepoCommitPathPoint{
		RepoCommitPath: types.RepoCommitPath{
			Repo:   "somerepo",
			Commit: "somecommit",
			Path:   "path2",
		},
		Point: types.Point{Row: 0, Column: 0},
	}

	mockServer := &mockSymbolsServer{
		mockSymbolInfoGRPC: func(_ context.Context, _ *proto.SymbolInfoRequest) (*proto.SymbolInfoResponse, error) {
			var info types.SymbolInfo
			info.Definition.RepoCommitPath = path1.RepoCommitPath

			var response proto.SymbolInfoResponse
			response.FromInternal(&info)

			return &response, nil
		},

		restHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(path1)
		}),
	}

	// Create a new HTTP server that response with path1.
	handler, cleanup := mockServer.NewHandler(logtest.Scoped(t))
	srv := httptest.NewServer(handler)
	t.Cleanup(func() {
		srv.Close()
		cleanup()
	})

	DefaultClient.Endpoints = endpoint.Static(srv.URL)

	ctx := context.Background()

	// Request path1.
	results, err := DefaultClient.SymbolInfo(ctx, path2)
	if err != nil {
		t.Fatal(err)
	}
	// Make sure we get results.
	if results == nil {
		t.Fatal("nil result")
	}

	// Now do the same but with perms filtering.
	ctx = actor.WithActor(ctx, &actor.Actor{
		UID: 1,
	})
	checker := authz.NewMockSubRepoPermissionChecker()
	checker.EnabledFunc.SetDefaultHook(func() bool {
		return true
	})
	checker.PermissionsFunc.SetDefaultHook(func(ctx context.Context, i int32, content authz.RepoContent) (authz.Perms, error) {
		return authz.None, nil
	})
	authz.DefaultSubRepoPermsChecker = checker
	results, err = DefaultClient.SymbolInfo(ctx, path2)
	if err != nil {
		t.Fatalf("unexpected error when getting a definition for an unauthorized path: %s", err)
	}
	// Make sure we do not get results.
	if results != nil {
		t.Fatal("expected nil result when getting a definition for an unauthorized path")
	}
}

type mockSymbolsServer struct {
	mockSearchGRPC         func(ctx context.Context, request *proto.SearchRequest) (*proto.SearchResponse, error)
	mockLocalCodeIntelGRPC func(request *proto.LocalCodeIntelRequest, ss proto.SymbolsService_LocalCodeIntelServer) error
	mockSymbolInfoGRPC     func(ctx context.Context, request *proto.SymbolInfoRequest) (*proto.SymbolInfoResponse, error)
	mockHealthzGRPC        func(ctx context.Context, request *proto.HealthzRequest) (*proto.HealthzResponse, error)

	restHandler http.Handler

	proto.UnimplementedSymbolsServiceServer
}

func (m *mockSymbolsServer) NewHandler(l log.Logger) (handler http.Handler, cleanup func()) {
	grpcServer := defaults.NewServer(l)
	proto.RegisterSymbolsServiceServer(grpcServer, m)

	handler = internalgrpc.MultiplexHandlers(grpcServer, http.HandlerFunc(m.serveRestHandler))
	cleanup = func() {
		grpcServer.Stop()
	}

	return handler, cleanup
}

func (m *mockSymbolsServer) serveRestHandler(w http.ResponseWriter, r *http.Request) {
	if m.restHandler != nil {
		m.restHandler.ServeHTTP(w, r)
		return
	}

	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockSymbolsServer) Search(ctx context.Context, r *proto.SearchRequest) (*proto.SearchResponse, error) {
	if m.mockSearchGRPC != nil {
		return m.mockSearchGRPC(ctx, r)
	}

	return nil, errors.Newf("grpc: method %q not implemented", "Search")
}

func (m *mockSymbolsServer) LocalCodeIntel(r *proto.LocalCodeIntelRequest, ss proto.SymbolsService_LocalCodeIntelServer) error {
	if m.mockLocalCodeIntelGRPC != nil {
		return m.LocalCodeIntel(r, ss)
	}

	return errors.Newf("grpc: method %q not implemented", "LocalCodeIntel")
}

func (m *mockSymbolsServer) SymbolInfo(ctx context.Context, r *proto.SymbolInfoRequest) (*proto.SymbolInfoResponse, error) {
	if m.mockSymbolInfoGRPC != nil {
		return m.mockSymbolInfoGRPC(ctx, r)
	}

	return nil, errors.Newf("grpc: method %q not implemented", "SymbolInfo")
}

func (m *mockSymbolsServer) Healthz(ctx context.Context, r *proto.HealthzRequest) (*proto.HealthzResponse, error) {
	if m.mockHealthzGRPC != nil {
		return m.mockHealthzGRPC(ctx, r)
	}

	return nil, errors.Newf("grpc: method %q not implemented", "Healthz")
}

var _ proto.SymbolsServiceServer = &mockSymbolsServer{}
