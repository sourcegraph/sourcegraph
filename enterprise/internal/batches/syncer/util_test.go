package syncer

import (
	"context"
	"testing"
	"time"

	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestLoadExternalService(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	db := dbtesting.GetDB(t)
	esStore := database.ExternalServices(db)
	repoStore := database.Repos(db)
	user := ct.CreateTestUser(t, db, false)

	noToken := types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "GitHub no token",
		Config:      `{"url": "https://github.com", "authorization": {}}`,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	userOwnedWithToken := types.ExternalService{
		Kind:            extsvc.KindGitHub,
		DisplayName:     "GitHub user owned",
		NamespaceUserID: user.ID,
		Config:          `{"url": "https://github.com", "token": "123", "authorization": {}}`,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	withToken := types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "GitHub token",
		Config:      `{"url": "https://github.com", "token": "123", "authorization": {}}`,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	withTokenNewer := types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "GitHub newer token",
		Config:      `{"url": "https://github.com", "token": "123456", "authorization": {}}`,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := esStore.Upsert(ctx, &noToken, &userOwnedWithToken, &withToken, &withTokenNewer); err != nil {
		t.Fatalf("failed to insert external service: %v", err)
	}

	repo := &types.Repo{
		Name:    api.RepoName("test-repo"),
		URI:     "test-repo",
		Private: true,
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "external-id-123",
			ServiceType: extsvc.TypeGitHub,
			ServiceID:   "https://github.com/",
		},
		Sources: map[string]*types.SourceInfo{
			noToken.URN(): {
				ID:       noToken.URN(),
				CloneURL: "https://github.com/sourcegraph/sourcegraph",
			},
			userOwnedWithToken.URN(): {
				ID:       userOwnedWithToken.URN(),
				CloneURL: "https://123@github.com/sourcegraph/sourcegraph",
			},
			withToken.URN(): {
				ID:       withToken.URN(),
				CloneURL: "https://123@github.com/sourcegraph/sourcegraph",
			},
			withTokenNewer.URN(): {
				ID:       withTokenNewer.URN(),
				CloneURL: "https://123456@github.com/sourcegraph/sourcegraph",
			},
		},
	}

	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatalf("failed to insert repo: %v", err)
	}

	// Expect the newest public external service with a token to be returned.
	svc, err := loadExternalService(ctx, esStore, repo)
	if err != nil {
		t.Fatalf("invalid error, expected nil, got %v", err)
	}
	if have, want := svc.ID, withTokenNewer.ID; have != want {
		t.Fatalf("invalid external service returned, want=%d have=%d", want, have)
	}

	// Now delete the global external services and expect the user owned external service to be returned.
	if err := esStore.Delete(ctx, withTokenNewer.ID); err != nil {
		t.Fatalf("failed to delete external service: %v", err)
	}
	if err := esStore.Delete(ctx, withToken.ID); err != nil {
		t.Fatalf("failed to delete external service: %v", err)
	}
	svc, err = loadExternalService(ctx, esStore, repo)
	if err != nil {
		t.Fatalf("invalid error, expected nil, got %v", err)
	}
	if have, want := svc.ID, userOwnedWithToken.ID; have != want {
		t.Fatalf("invalid external service returned, want=%d have=%d", want, have)
	}
}
