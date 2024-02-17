package bitbucketcloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketcloud"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func mustURL(t *testing.T, u string) *url.URL {
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func createTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/repositories") {
			json.NewEncoder(w).Encode(struct {
				Values []bitbucketcloud.Repo `json:"values"`
			}{
				Values: []bitbucketcloud.Repo{
					{UUID: "1"},
					{UUID: "2"},
					{UUID: "3"},
				},
			})
			return
		}

		if strings.HasSuffix(r.URL.Path, "/permissions-config/users") {
			json.NewEncoder(w).Encode(struct {
				Values []bitbucketcloud.ExplicitUserPermsResponse `json:"values"`
			}{
				Values: []bitbucketcloud.ExplicitUserPermsResponse{
					{User: &bitbucketcloud.Account{UUID: "1"}},
					{User: &bitbucketcloud.Account{UUID: "2"}},
					{User: &bitbucketcloud.Account{UUID: "3"}},
				},
			})
			return
		}

		if strings.HasSuffix(r.URL.Path, "/repositories/user/repo") {
			json.NewEncoder(w).Encode(bitbucketcloud.Repo{
				Owner: &bitbucketcloud.Account{UUID: "4"},
			})
			return
		}
	}))
}

func TestProvider_FetchUserPerms(t *testing.T) {
	ratelimit.SetupForTest(t)

	db := dbmocks.NewMockDB()
	t.Run("nil account", func(t *testing.T) {
		p := NewProvider(db,
			&types.BitbucketCloudConnection{
				BitbucketCloudConnection: &schema.BitbucketCloudConnection{
					ApiURL: "https://bitbucket.org",
					Url:    "https://bitbucket.org",
				},
			}, ProviderOptions{})
		_, err := p.FetchUserPerms(context.Background(), nil, authz.FetchPermsOptions{})
		want := "no account provided"
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	t.Run("not the code host of the account", func(t *testing.T) {
		p := NewProvider(db,
			&types.BitbucketCloudConnection{
				BitbucketCloudConnection: &schema.BitbucketCloudConnection{
					ApiURL: "https://bitbucket.org",
					Url:    "https://bitbucket.org",
				},
			}, ProviderOptions{})
		_, err := p.FetchUserPerms(context.Background(),
			&extsvc.Account{
				AccountSpec: extsvc.AccountSpec{
					ServiceType: extsvc.TypeGitHub,
					ServiceID:   "https://github.com/",
				},
			},
			authz.FetchPermsOptions{},
		)
		want := `not a code host of the account: want "https://bitbucket.org/" but have "https://github.com/"`
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	t.Run("no account data provided", func(t *testing.T) {
		p := NewProvider(db,
			&types.BitbucketCloudConnection{
				BitbucketCloudConnection: &schema.BitbucketCloudConnection{
					ApiURL: "https://bitbucket.org",
					Url:    "https://bitbucket.org",
				},
			}, ProviderOptions{})
		_, err := p.FetchUserPerms(context.Background(),
			&extsvc.Account{
				AccountSpec: extsvc.AccountSpec{
					ServiceType: extsvc.TypeBitbucketCloud,
					ServiceID:   "https://bitbucket.org/",
				},
			},
			authz.FetchPermsOptions{},
		)
		want := `no account data provided`
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	server := createTestServer()
	defer server.Close()

	t.Run("fetch user permissions", func(t *testing.T) {
		conn := &schema.BitbucketCloudConnection{
			ApiURL: server.URL,
			Url:    server.URL,
		}
		client, err := bitbucketcloud.NewClient(server.URL, conn, httpcli.NewFactory(nil))
		if err != nil {
			t.Fatal(err)
		}

		p := NewProvider(db,
			&types.BitbucketCloudConnection{
				BitbucketCloudConnection: conn,
			}, ProviderOptions{BitbucketCloudClient: client})

		var acctData extsvc.AccountData
		err = bitbucketcloud.SetExternalAccountData(&acctData, &bitbucketcloud.Account{}, &oauth2.Token{AccessToken: "my-access-token"})
		if err != nil {
			t.Fatal(err)
		}

		account := &extsvc.Account{
			AccountSpec: extsvc.AccountSpec{
				ServiceType: extsvc.TypeBitbucketCloud,
				ServiceID:   extsvc.NormalizeBaseURL(mustURL(t, server.URL)).String(),
			},
			AccountData: acctData,
		}
		userPerms, err := p.FetchUserPerms(context.Background(), account, authz.FetchPermsOptions{})
		if err != nil {
			t.Fatal(err)
		}

		expRepoIDs := []extsvc.RepoID{"1", "2", "3"}
		if diff := cmp.Diff(expRepoIDs, userPerms.Exacts); diff != "" {
			t.Fatal(diff)
		}
	})
}

func TestProvider_FetchRepoPerms(t *testing.T) {
	ratelimit.SetupForTest(t)

	server := createTestServer()
	defer server.Close()
	db := dbmocks.NewMockDB()

	conn := &schema.BitbucketCloudConnection{
		ApiURL: server.URL,
		Url:    server.URL,
	}
	client, err := bitbucketcloud.NewClient(server.URL, conn, httpcli.NewFactory(nil))
	if err != nil {
		t.Fatal(err)
	}

	p := NewProvider(db,
		&types.BitbucketCloudConnection{
			BitbucketCloudConnection: conn,
		}, ProviderOptions{BitbucketCloudClient: client})

	perms, err := p.FetchRepoPerms(context.Background(), &extsvc.Repository{
		URI: "bitbucket.org/user/repo",
	}, authz.FetchPermsOptions{})

	if err != nil {
		t.Fatal(err)
	}

	expUserIDs := []extsvc.AccountID{"1", "2", "3", "4"}
	if diff := cmp.Diff(expUserIDs, perms); diff != "" {
		t.Fatal(diff)
	}
}
