package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/webhooks"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketcloud"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	gitlabwebhooks "github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab/webhooks"
	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	proto "github.com/sourcegraph/sourcegraph/internal/repoupdater/v1"
	v1 "github.com/sourcegraph/sourcegraph/internal/repoupdater/v1"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

type mockGRPCServer struct {
	f func(*proto.EnqueueRepoUpdateRequest) (*proto.EnqueueRepoUpdateResponse, error)
	proto.UnimplementedRepoUpdaterServiceServer
}

func (m *mockGRPCServer) EnqueueRepoUpdate(_ context.Context, req *proto.EnqueueRepoUpdateRequest) (*proto.EnqueueRepoUpdateResponse, error) {
	return m.f(req)
}

func TestGitHubHandler(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)

	db := database.NewDB(logger, dbtest.NewDB(t))
	store := repos.NewStore(logger, db)
	repoStore := store.RepoStore()
	esStore := store.ExternalServiceStore()

	repo := &types.Repo{
		ID:   1,
		Name: "ghe.sgdev.org/milton/test",
	}
	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	conn := schema.GitHubConnection{
		Url:      "https://ghe.sgdev.org",
		Token:    "token",
		Repos:    []string{"milton/test"},
		Webhooks: []*schema.GitHubWebhook{{Org: "ghe.sgdev.org", Secret: "secret"}},
	}

	config, err := json.Marshal(conn)
	if err != nil {
		t.Fatal(err)
	}

	svc := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "TestService",
		Config:      extsvc.NewUnencryptedConfig(string(config)),
	}
	if err := esStore.Upsert(ctx, svc); err != nil {
		t.Fatal(err)
	}

	handler := NewGitHubHandler()
	router := &webhooks.GitHubWebhook{
		Router: &webhooks.Router{
			DB: db,
		},
	}
	handler.Register(router.Router)

	gs := grpc.NewServer(defaults.ServerOptions(logger)...)
	v1.RegisterRepoUpdaterServiceServer(gs, &mockGRPCServer{
		f: func(req *v1.EnqueueRepoUpdateRequest) (*v1.EnqueueRepoUpdateResponse, error) {
			repositories, err := repoStore.List(ctx, database.ReposListOptions{Names: []string{req.Repo}})
			if err != nil {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			if len(repositories) != 1 {
				return nil, status.Error(codes.NotFound, fmt.Sprintf("expected 1 repo, got %v", len(repositories)))
			}

			repo := repositories[0]
			return &proto.EnqueueRepoUpdateResponse{
				Id:   int32(repo.ID),
				Name: string(repo.Name),
			}, nil
		},
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/enqueue-repo-update", func(w http.ResponseWriter, r *http.Request) {
		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		var req protocol.RepoUpdateRequest
		if err := json.Unmarshal(reqBody, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		repositories, err := repoStore.List(ctx, database.ReposListOptions{Names: []string{string(req.Repo)}})
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
		}
		if len(repositories) != 1 {
			http.Error(w, fmt.Sprintf("expected 1 repo, got %v", len(repositories)), http.StatusNotFound)
		}

		repo := repositories[0]
		res := &protocol.RepoUpdateResponse{
			ID:   repo.ID,
			Name: string(repo.Name),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(res)
	})

	server := httptest.NewServer(internalgrpc.MultiplexHandlers(gs, mux))
	defer server.Close()

	repoupdater.DefaultClient = repoupdater.NewClient(server.URL)

	payload, err := os.ReadFile(filepath.Join("testdata", "github-push.json"))
	if err != nil {
		t.Fatal(err)
	}

	targetURL := fmt.Sprintf("%s/github-webhooks", globals.ExternalURL())
	req, err := http.NewRequest("POST", targetURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature", sign(t, payload, []byte("secret")))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	resp := rec.Result()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status code: 200, got %v", resp.StatusCode)
	}
}

func sign(t *testing.T, message, secret []byte) string {
	t.Helper()

	mac := hmac.New(sha256.New, secret)
	_, err := mac.Write(message)
	if err != nil {
		t.Fatalf("writing hmac message failed: %s", err)
	}

	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestGitLabHandler(t *testing.T) {
	repoName := "gitlab.com/ryanslade/ryan-test-private"

	db := dbmocks.NewMockDB()
	repositories := dbmocks.NewMockRepoStore()
	repositories.GetFirstRepoNameByCloneURLFunc.SetDefaultHook(func(ctx context.Context, s string) (api.RepoName, error) {
		return api.RepoName(repoName), nil
	})
	db.ReposFunc.SetDefaultReturn(repositories)

	handler := NewGitLabHandler()
	data, err := os.ReadFile("testdata/gitlab-push.json")
	if err != nil {
		t.Fatal(err)
	}
	var payload gitlabwebhooks.PushEvent
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}

	var updateQueued string
	repoupdater.MockEnqueueRepoUpdate = func(ctx context.Context, repo api.RepoName) (*protocol.RepoUpdateResponse, error) {
		updateQueued = string(repo)
		return &protocol.RepoUpdateResponse{
			ID:   1,
			Name: string(repo),
		}, nil
	}
	t.Cleanup(func() { repoupdater.MockEnqueueRepoUpdate = nil })

	if err := handler.handlePushEvent(context.Background(), db, &payload); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, repoName, updateQueued)
}

func TestBitbucketServerHandler(t *testing.T) {
	repoName := "bitbucket.sgdev.org/private/test-2020-06-01"

	db := dbmocks.NewMockDB()
	repositories := dbmocks.NewMockRepoStore()
	repositories.GetFirstRepoNameByCloneURLFunc.SetDefaultHook(func(ctx context.Context, s string) (api.RepoName, error) {
		return "bitbucket.sgdev.org/private/test-2020-06-01", nil
	})
	db.ReposFunc.SetDefaultReturn(repositories)

	handler := NewBitbucketServerHandler()
	data, err := os.ReadFile("testdata/bitbucket-server-push.json")
	if err != nil {
		t.Fatal(err)
	}
	var payload bitbucketserver.PushEvent
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}

	var updateQueued string
	repoupdater.MockEnqueueRepoUpdate = func(ctx context.Context, repo api.RepoName) (*protocol.RepoUpdateResponse, error) {
		updateQueued = string(repo)
		return &protocol.RepoUpdateResponse{
			ID:   1,
			Name: string(repo),
		}, nil
	}
	t.Cleanup(func() { repoupdater.MockEnqueueRepoUpdate = nil })

	if err := handler.handlePushEvent(context.Background(), db, &payload); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, repoName, updateQueued)
}

func TestBitbucketCloudHandler(t *testing.T) {
	repoName := "bitbucket.org/sourcegraph-testing/sourcegraph"

	db := dbmocks.NewMockDB()
	repositories := dbmocks.NewMockRepoStore()
	repositories.GetFirstRepoNameByCloneURLFunc.SetDefaultHook(func(ctx context.Context, s string) (api.RepoName, error) {
		return "bitbucket.org/sourcegraph-testing/sourcegraph", nil
	})
	db.ReposFunc.SetDefaultReturn(repositories)

	handler := NewBitbucketCloudHandler()
	data, err := os.ReadFile("testdata/bitbucket-cloud-push.json")
	if err != nil {
		t.Fatal(err)
	}
	var payload bitbucketcloud.PushEvent
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}

	var updateQueued string
	repoupdater.MockEnqueueRepoUpdate = func(ctx context.Context, repo api.RepoName) (*protocol.RepoUpdateResponse, error) {
		updateQueued = string(repo)
		return &protocol.RepoUpdateResponse{
			ID:   1,
			Name: string(repo),
		}, nil
	}
	t.Cleanup(func() { repoupdater.MockEnqueueRepoUpdate = nil })

	if err := handler.handlePushEvent(context.Background(), db, &payload); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, repoName, updateQueued)
}
