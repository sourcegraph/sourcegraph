package gitserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestClientSource_AddrMatchesTarget(t *testing.T) {
	repos := dbmocks.NewMockRepoStore()
	repos.GetByNameFunc.SetDefaultReturn(nil, nil)

	source := NewTestClientSource(t, []string{"localhost:1234", "localhost:4321"})
	testGitserverConns := source.(*testGitserverConns)
	conns := GitserverConns(*testGitserverConns.conns)

	ctx := context.Background()
	for _, repo := range []api.RepoName{"a", "b", "c", "d"} {
		addr := source.AddrForRepo(ctx, repo)
		conn, err := conns.ConnForRepo(ctx, repo)
		if err != nil {
			t.Fatal(err)
		}
		if addr != conn.Target() {
			t.Fatalf("expected addr (%q) to equal target (%q)", addr, conn.Target())
		}
	}
}

// mockGitserver implements both a gRPC server and an HTTP server that just tracks
// whether or not it was called.
type mockGitserver struct {
	called bool
	proto.UnimplementedGitserverServiceServer
}

func (m *mockGitserver) ResolveRevision(ctx context.Context, req *proto.ResolveRevisionRequest) (*proto.ResolveRevisionResponse, error) {
	m.called = true
	return &proto.ResolveRevisionResponse{}, nil
}

func (m *mockGitserver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.called = true
}

func TestClient_GRPCRouting(t *testing.T) {
	gs1 := grpc.NewServer()
	m1 := &mockGitserver{}
	proto.RegisterGitserverServiceServer(gs1, m1)
	srv1 := httptest.NewServer(internalgrpc.MultiplexHandlers(gs1, m1))

	gs2 := grpc.NewServer()
	m2 := &mockGitserver{}
	proto.RegisterGitserverServiceServer(gs2, m2)
	srv2 := httptest.NewServer(internalgrpc.MultiplexHandlers(gs2, m2))

	u1, _ := url.Parse(srv1.URL)
	u2, _ := url.Parse(srv2.URL)

	conf.Mock(&conf.Unified{
		ServiceConnectionConfig: conftypes.ServiceConnections{
			GitServers: []string{u1.Host, u2.Host},
		},
		SiteConfiguration: schema.SiteConfiguration{
			ExperimentalFeatures: &schema.ExperimentalFeatures{
				GitServerPinnedRepos: map[string]string{"a": u1.Host, "b": u2.Host},
			},
		},
	})
	t.Cleanup(func() { conf.Mock(nil) })

	client := NewClient("test")
	_, _ = client.ResolveRevision(context.Background(), "a", "HEAD", ResolveRevisionOptions{})

	if !(m1.called && !m2.called) {
		t.Fatalf("expected repo 'a' to hit srv1, got %v, %v", m1.called, m2.called)
	}

	m1.called, m2.called = false, false
	_, _ = client.ResolveRevision(context.Background(), "b", "HEAD", ResolveRevisionOptions{})

	if !(!m1.called && m2.called) {
		t.Fatalf("expected repo 'b' to hit srv2, got %v, %v", m1.called, m2.called)
	}
}

func TestClient_AddrForRepo_UsesConfToRead_PinnedRepos(t *testing.T) {
	client := NewClient("test")

	cfg := newConfig(
		[]string{"gitserver1", "gitserver2"},
		map[string]string{"repo1": "gitserver2"},
	)

	Conns.update(cfg)

	ctx := context.Background()
	addr := client.AddrForRepo(ctx, "repo1")
	require.Equal(t, "gitserver2", addr)

	// simulate config change - site admin manually changes the pinned repo config
	cfg = newConfig(
		[]string{"gitserver1", "gitserver2"},
		map[string]string{"repo1": "gitserver1"},
	)
	Conns.update(cfg)

	require.Equal(t, "gitserver1", client.AddrForRepo(ctx, "repo1"))
}

func newConfig(addrs []string, pinned map[string]string) *conf.Unified {
	return &conf.Unified{
		ServiceConnectionConfig: conftypes.ServiceConnections{
			GitServers: addrs,
		},
		SiteConfiguration: schema.SiteConfiguration{
			ExperimentalFeatures: &schema.ExperimentalFeatures{
				GitServerPinnedRepos: pinned,
			},
		},
	}
}
