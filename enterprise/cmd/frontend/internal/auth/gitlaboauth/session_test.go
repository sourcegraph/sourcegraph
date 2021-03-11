package gitlaboauth

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestSessionIssuerHelper_CreateCodeHostConnection(t *testing.T) {
	ctx := context.Background()
	s := &sessionIssuerHelper{}
	t.Run("Unauthenticated request", func(t *testing.T) {
		_, err := s.CreateCodeHostConnection(ctx, nil, "")
		if err == nil {
			t.Fatal("Want error but got nil")
		}
	})

	mockGitLabCom := newMockProvider(t, "gitlabcomclient", "gitlabcomsecret", "https://gitlab.com/")
	providers.MockProviders = []providers.Provider{mockGitLabCom.Provider}
	defer func() { providers.MockProviders = nil }()

	var got *types.ExternalService
	database.Mocks.ExternalServices.Create = func(ctx context.Context, confGet func() *conf.Unified, externalService *types.ExternalService) error {
		got = externalService
		return nil
	}
	defer func() { database.Mocks.ExternalServices.Create = nil }()

	act := &actor.Actor{UID: 1}
	glUser := &gitlab.User{
		ID:       101,
		Username: "alice",
	}

	ctx = actor.WithActor(ctx, act)
	ctx = WithUser(ctx, glUser)
	tok := &oauth2.Token{
		AccessToken: "dummy-value-that-isnt-relevant-to-unit-correctness",
	}
	_, err := s.CreateCodeHostConnection(ctx, tok, mockGitLabCom.ConfigID().ID)
	if err != nil {
		t.Fatal(err)
	}

	want := &types.ExternalService{
		Kind:        extsvc.KindGitLab,
		DisplayName: fmt.Sprintf("GitLab (%s)", glUser.Username),
		Config: fmt.Sprintf(`
{
  "url": "%s",
  "token": "%s",
  "projectQuery": ["projects?visibility=public"]
}
`, mockGitLabCom.ServiceID, tok.AccessToken),
		NamespaceUserID: act.UID,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Mismatch (-want +got):\n%s", diff)
	}
}
