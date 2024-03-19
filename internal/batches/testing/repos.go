package testing

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/awscodecommit"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestRepo(t *testing.T, store database.ExternalServiceStore, serviceKind string) *types.Repo {
	t.Helper()

	clock := timeutil.NewFakeClock(time.Now(), 0)
	now := clock.Now()

	svc := types.ExternalService{
		Kind:        serviceKind,
		DisplayName: serviceKind + " - Test",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	switch serviceKind {
	case extsvc.KindGitHub:
		svc.Config = extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "authorization": {}, "token": "abc", "repos": ["owner/name"]}`)
	case extsvc.KindGitLab:
		svc.Config = extsvc.NewUnencryptedConfig(`{"url": "https://gitlab.com", "token": "abc", "projectQuery": ["repo"]}`)
	case extsvc.KindBitbucketCloud:
		svc.Config = extsvc.NewUnencryptedConfig(`{"url": "https://bitbucket.org", "username": "user", "appPassword": "pass"}`)
	case extsvc.KindBitbucketServer:
		svc.Config = extsvc.NewUnencryptedConfig(`{"url": "https://bitbucket.org", "username": "user", "token": "abc", "repos": ["owner/name"]}`)
	case extsvc.KindAWSCodeCommit:
		svc.Config = extsvc.NewUnencryptedConfig(`{"region": "us-east-1", "accessKeyID": "abc", "secretAccessKey": "abc", "gitCredentials": {"username": "user", "password": "pass"}}`)
	default:
		panic(fmt.Sprintf("unhandled kind: %q", serviceKind))
	}

	if err := store.Upsert(context.Background(), &svc); err != nil {
		t.Fatalf("failed to insert external services: %v", err)
	}

	repo := TestRepoWithService(t, store, fmt.Sprintf("repo-%d", svc.ID), &svc)

	repo.Sources[svc.URN()].CloneURL = "https://github.com/sourcegraph/sourcegraph"
	return repo
}

func TestRepoWithService(t *testing.T, store database.ExternalServiceStore, name string, svc *types.ExternalService) *types.Repo {
	t.Helper()

	return &types.Repo{
		Name:    api.RepoName(name),
		URI:     name,
		Private: true,
		ExternalRepo: api.ExternalRepoSpec{
			ID:          fmt.Sprintf("external-id-%s", name),
			ServiceType: extsvc.KindToType(svc.Kind),
			ServiceID:   fmt.Sprintf("https://%s.com/", strings.ToLower(svc.Kind)),
		},
		Sources: map[string]*types.SourceInfo{
			svc.URN(): {
				ID: svc.URN(),
			},
		},
	}
}

func CreateTestRepo(t *testing.T, ctx context.Context, db database.DB) (*types.Repo, *types.ExternalService) {
	repos, extSvc := CreateTestRepos(t, ctx, db, 1)
	return repos[0], extSvc
}

func CreateTestRepos(t *testing.T, ctx context.Context, db database.DB, count int) ([]*types.Repo, *types.ExternalService) {
	t.Helper()

	repoStore := db.Repos()
	esStore := db.ExternalServices()

	ext := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "GitHub",
		Config: extsvc.NewUnencryptedConfig(MarshalJSON(t, &schema.GitHubConnection{
			Url:             "https://github.com",
			Token:           "SECRETTOKEN",
			RepositoryQuery: []string{"none"},
			// This field is needed to enforce permissions
			Authorization: &schema.GitHubAuthorization{},
		})),
	}

	confGet := func() *conf.Unified {
		return &conf.Unified{}
	}

	if err := esStore.Create(ctx, confGet, ext); err != nil {
		t.Fatal(err)
	}

	var rs []*types.Repo
	for i := range count {
		r := TestRepoWithService(t, esStore, fmt.Sprintf("repo-%d-%d", ext.ID, i+1), ext)
		r.Metadata = &github.Repository{
			NameWithOwner: string(r.Name),
			URL:           fmt.Sprintf("https://github.com/sourcegraph/%s", string(r.Name)),
		}

		r.Sources[ext.URN()].CloneURL = fmt.Sprintf("https://github.com/sourcegraph/%s", string(r.Name))

		rs = append(rs, r)
	}

	err := repoStore.Create(ctx, rs...)
	if err != nil {
		t.Fatal(err)
	}

	return rs, ext
}

func CreateGitlabTestRepos(t *testing.T, ctx context.Context, db database.DB, count int) ([]*types.Repo, *types.ExternalService) {
	t.Helper()

	repoStore := db.Repos()
	esStore := db.ExternalServices()

	ext := &types.ExternalService{
		Kind:        extsvc.KindGitLab,
		DisplayName: "GitLab",
		Config: extsvc.NewUnencryptedConfig(MarshalJSON(t, &schema.GitLabConnection{
			Url:          "https://gitlab.com",
			Token:        "SECRETTOKEN",
			ProjectQuery: []string{"none"},
		})),
	}
	if err := esStore.Upsert(ctx, ext); err != nil {
		t.Fatal(err)
	}

	var rs []*types.Repo
	for i := range count {
		r := TestRepoWithService(t, esStore, fmt.Sprintf("repo-%d-%d", ext.ID, i+1), ext)
		r.Metadata = &gitlab.Project{
			ProjectCommon: gitlab.ProjectCommon{
				HTTPURLToRepo: fmt.Sprintf("https://gitlab.com/sourcegraph/%s", string(r.Name)),
			},
		}

		r.Sources[ext.URN()].CloneURL = fmt.Sprintf("https://gitlab.com/sourcegraph/%s", string(r.Name))

		rs = append(rs, r)
	}

	err := repoStore.Create(ctx, rs...)
	if err != nil {
		t.Fatal(err)
	}

	return rs, ext
}

func CreateBbsTestRepos(t *testing.T, ctx context.Context, db database.DB, count int) ([]*types.Repo, *types.ExternalService) {
	t.Helper()

	ext := &types.ExternalService{
		Kind:        extsvc.KindBitbucketServer,
		DisplayName: "Bitbucket Server",
		Config: extsvc.NewUnencryptedConfig(MarshalJSON(t, &schema.BitbucketServerConnection{
			Url:   "https://bitbucket.sourcegraph.com",
			Token: "SECRETTOKEN",
			Repos: []string{"owner/name"},
		})),
	}

	return createBbsRepos(t, ctx, db, ext, count, "https://bbs-user:bbs-token@bitbucket.sourcegraph.com/scm")
}

func CreateGitHubSSHTestRepos(t *testing.T, ctx context.Context, db database.DB, count int) ([]*types.Repo, *types.ExternalService) {
	t.Helper()

	ext := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "GitHub SSH",
		Config: extsvc.NewUnencryptedConfig(MarshalJSON(t, &schema.GitHubConnection{
			Url:        "https://github.com",
			Token:      "SECRETTOKEN",
			GitURLType: "ssh",
			Repos:      []string{"owner/name"},
		})),
	}
	esStore := db.ExternalServices()
	if err := esStore.Upsert(ctx, ext); err != nil {
		t.Fatal(err)
	}

	var rs []*types.Repo
	for range count {
		r := TestRepo(t, esStore, extsvc.KindGitHub)
		r.Sources = map[string]*types.SourceInfo{ext.URN(): {
			ID:       ext.URN(),
			CloneURL: "git@github.com:" + string(r.Name) + ".git",
		}}

		rs = append(rs, r)
	}

	err := db.Repos().Create(ctx, rs...)
	if err != nil {
		t.Fatal(err)
	}
	return rs, ext
}

func CreateBbsSSHTestRepos(t *testing.T, ctx context.Context, db database.DB, count int) ([]*types.Repo, *types.ExternalService) {
	t.Helper()

	ext := &types.ExternalService{
		Kind:        extsvc.KindBitbucketServer,
		DisplayName: "Bitbucket Server SSH",
		Config: extsvc.NewUnencryptedConfig(MarshalJSON(t, &schema.BitbucketServerConnection{
			Url:        "https://bitbucket.sgdev.org",
			Token:      "SECRETTOKEN",
			GitURLType: "ssh",
			Repos:      []string{"owner/name"},
		})),
	}

	return createBbsRepos(t, ctx, db, ext, count, "ssh://git@bitbucket.sgdev.org:7999")
}

func createBbsRepos(t *testing.T, ctx context.Context, db database.DB, ext *types.ExternalService, count int, cloneBaseURL string) ([]*types.Repo, *types.ExternalService) {
	t.Helper()

	repoStore := db.Repos()
	esStore := db.ExternalServices()

	if err := esStore.Upsert(ctx, ext); err != nil {
		t.Fatal(err)
	}

	var rs []*types.Repo
	for i := range count {
		r := TestRepoWithService(t, esStore, fmt.Sprintf("repo-%d-%d", ext.ID, i+1), ext)
		var metadata bitbucketserver.Repo
		urlType := "http"
		if strings.HasPrefix(cloneBaseURL, "ssh") {
			urlType = "ssh"
		}
		metadata.Links.Clone = append(metadata.Links.Clone, struct {
			Href string "json:\"href\""
			Name string "json:\"name\""
		}{
			Name: urlType,
			Href: cloneBaseURL + "/" + string(r.Name),
		})
		r.Metadata = &metadata
		r.Sources[ext.URN()].CloneURL = fmt.Sprintf("%s/%s", cloneBaseURL, string(r.Name))
		rs = append(rs, r)
	}

	err := repoStore.Create(ctx, rs...)
	if err != nil {
		t.Fatal(err)
	}

	return rs, ext
}

func CreateAWSCodeCommitTestRepos(t *testing.T, ctx context.Context, db database.DB, count int) ([]*types.Repo, *types.ExternalService) {
	t.Helper()

	repoStore := db.Repos()
	esStore := db.ExternalServices()

	ext := &types.ExternalService{
		Kind:        extsvc.KindAWSCodeCommit,
		DisplayName: "AWS CodeCommit",
		Config: extsvc.NewUnencryptedConfig(MarshalJSON(t, &schema.AWSCodeCommitConnection{
			AccessKeyID: "horse-key",
			Region:      "us-east-1",
			GitCredentials: schema.AWSCodeCommitGitCredentials{
				Username: "horse",
				Password: "graph",
			},
		})),
	}
	if err := esStore.Upsert(ctx, ext); err != nil {
		t.Fatal(err)
	}

	var rs []*types.Repo
	for i := range count {
		r := TestRepoWithService(t, esStore, fmt.Sprintf("repo-%d-%d", ext.ID, i+1), ext)
		r.Metadata = &awscodecommit.Repository{
			ARN:          fmt.Sprintf("arn:aws:codecommit:us-west-1:%d:%s", i, r.Name),
			AccountID:    "999999999999",
			ID:           "%s",
			Name:         string(r.Name),
			HTTPCloneURL: fmt.Sprintf("https://git-codecommit.us-west-1.amazonaws.com/v1/repos/%s", r.Name),
		}

		rs = append(rs, r)
	}

	err := repoStore.Create(ctx, rs...)
	if err != nil {
		t.Fatal(err)
	}

	return rs, ext
}
