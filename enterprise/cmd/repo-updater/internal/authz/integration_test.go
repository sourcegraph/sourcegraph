package authz

import (
	"context"
	"database/sql"
	"flag"
	"net/url"
	"os"
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"

	edb "github.com/sourcegraph/sourcegraph/enterprise/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	authzGitHub "github.com/sourcegraph/sourcegraph/internal/authz/github"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	extsvcGitHub "github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/httptestutil"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

var updateRegex = flag.String("update", "", "Update testdata of tests matching the given regex")

func update(name string) bool {
	if updateRegex == nil || *updateRegex == "" {
		return false
	}
	return regexp.MustCompile(*updateRegex).MatchString(name)
}

var dsn = flag.String("dsn", "", "Database connection string to use in integration tests")

// This integration test performs a repository-centric permissions syncing against
// https://github.com, then check if permissions are correctly granted for the test
// user "sourcegraph-vcr-bob", who is a outside collaborator of the repository
// "sourcegraph-vcr-repos/private-org-repo-1".
//
// NOTE: To update VCR for this test, please use the token of "sourcegraph-vcr"
// for GITHUB_TOKEN, which can be found in 1Password.
func TestIntegration_GitHubPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	const name = "Integration_GitHubPermissions"
	cf, save := httptestutil.NewGitHubRecorderFactory(t, update(name), name)
	defer save()

	uri, err := url.Parse("https://github.com")
	if err != nil {
		t.Fatal(err)
	}

	doer, err := cf.Doer()
	if err != nil {
		t.Fatal(err)
	}

	token := os.Getenv("GITHUB_TOKEN")
	cli := extsvcGitHub.NewV3Client(uri, &auth.OAuthBearerToken{Token: token}, doer)

	testDB := dbtest.NewDB(t, *dsn)
	ctx := actor.WithInternalActor(context.Background())

	reposStore := repos.NewStore(testDB, sql.TxOptions{})

	svc := types.ExternalService{
		Kind:      extsvc.KindGitHub,
		CreatedAt: timeutil.Now(),
		Config:    `{"url": "https://github.com", "authorization": {}}`,
	}
	err = reposStore.ExternalServiceStore.Upsert(ctx, &svc)
	if err != nil {
		t.Fatal(err)
	}

	provider := authzGitHub.NewProvider(svc.URN(), uri, token, cli)

	authz.SetProviders(false, []authz.Provider{provider})
	defer authz.SetProviders(true, nil)

	repo := types.Repo{
		Name:    "github.com/sourcegraph-vcr-repos/private-org-repo-1",
		Private: true,
		URI:     "github.com/sourcegraph-vcr-repos/private-org-repo-1",
		ExternalRepo: api.ExternalRepoSpec{
			ServiceType: extsvc.TypeGitHub,
			ServiceID:   "https://github.com/",
		},
		Sources: map[string]*types.SourceInfo{
			svc.URN(): {
				ID: svc.URN(),
			},
		},
	}
	err = reposStore.RepoStore.Create(ctx, &repo)
	if err != nil {
		t.Fatal(err)
	}

	dbconn.Global = testDB
	newUser := database.NewUser{
		Email:           "sourcegraph-vcr-bob@sourcegraph.com",
		Username:        "sourcegraph-vcr-bob",
		EmailIsVerified: true,
	}
	spec := extsvc.AccountSpec{
		ServiceType: extsvc.TypeGitHub,
		ServiceID:   "https://github.com/",
		AccountID:   "66464926",
	}
	userID, err := database.GlobalExternalAccounts.CreateUserAndSave(ctx, newUser, spec, extsvc.AccountData{})
	if err != nil {
		t.Fatal(err)
	}

	permsStore := edb.Perms(testDB, timeutil.Now)
	syncer := NewPermsSyncer(reposStore, permsStore, timeutil.Now, nil)

	err = syncer.syncRepoPerms(ctx, repo.ID, false)
	if err != nil {
		t.Fatal(err)
	}

	p := &authz.UserPermissions{
		UserID: userID,
		Perm:   authz.Read,
		Type:   authz.PermRepos,
	}
	err = permsStore.LoadUserPermissions(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	wantIDs := []uint32{1}
	if diff := cmp.Diff(wantIDs, p.IDs.ToArray()); diff != "" {
		t.Fatalf("IDs mismatch (-want +got):\n%s", diff)
	}
}
