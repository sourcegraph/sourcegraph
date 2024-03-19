package sources

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	mockassert "github.com/derision-test/go-mockgen/v2/testutil/assert"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches/store"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	ghaauth "github.com/sourcegraph/sourcegraph/internal/github_apps/auth"
	ghastore "github.com/sourcegraph/sourcegraph/internal/github_apps/store"
	ghatypes "github.com/sourcegraph/sourcegraph/internal/github_apps/types"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

// Random valid private key generated for this test and nothing else
const testGHAppPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA1LqMqnchtoTHiRfFds2RWWji43R5mHT65WZpZXpBuBwsgSWr
rN5VwTHZ4dxWk+XlyVDYsri7vlVWNX4EIt0jwxvh/OBXCFJXTL+byNHCimRIKvur
ofoT1eF3z+5WpH5ddHNPOkGZV0Chyd5kvUcNuFA7q203HRVEOloHEs4fqJrGPHIF
zc8Sug5qkOtZTS5xiHgTtmZkDLuLZ26H5Gfx3zZk5Gv2Jy/+fLsGninaiwTRsZf6
RgPgmdlkuM8OhSm4GtlpzoK0D3iZEhf4pITo1CK2U4Cs7vzkU0UkQ+J/z6dDmVBJ
gkalH1SHsboRqjNxkStEqGnbWwdtal01skbGOwIDAQABAoIBAQCls54oll17V5g5
0Htu3BdxBsNdG3gv6kcY85n7gqy4ZbHA83/zSsiPkW4/gasqzzQbiU8Sf9U2IDDj
wAImygy2SPzSRklk4QbBcKs/VSztMcoJOTprFGno+xShsexpe0j+kWdQYJK6JU0g
+ouL6FHmlRC1qn/4tn0L2t6Rpl+Aq4peDLqdwFHXj8GxGv0S10qMQ4/ER7onP6f0
99WDTvNQR5DugKqHxooOV5HfUP70scqhCcFhp2zc7/aYQFVt/k4hDOMu/w4HhkD3
r34y4EJoZsugGD1kPaJCw2rbSdoTtQHCqG5tfidY+XUIoC9mfmN8243jeRrhayeT
4ewiDuNhAoGBAPszeqN/+V8EVrlbBiBG+xVYGzWU0KgHu1TUiIrOSmKa6rTwaYMb
dKI8N4gYwFtb24AeDLfGnpaZAKSvNnrf8OapyLik7zXDylY0DBU7tRxQiUvNsVTs
7CYjxih5GWzUeP/xgpfVbHIIGdTHaZ6JWiDHWOolAw3hQyw6V/uQTDtxAoGBANjK
6vlctX55MjE2tuPk3ZCtCjgDFmWQjvFuiYYE/2cP4v4UBqgZn1vOaLRCnFm10ycl
peBLxPVpeeNBWc2ep2YNnJ+hm+SavhIDesLJTxuhC4wtcKMVAtq83VQmMQTU5wRO
KcUpviXLv2Z0UfbMWcohR4fJY1SkREwaxneHZc5rAoGBAIpT8c/BNBhPslYFutzh
WXiKeQlLdo9hGpZ/JuWQ7cNY3bBfxyqMXvDLyiSmxJ5KehgV9BjrRf9WJ9WIKq8F
TIooqsCLCrMHqw9HP/QdWgFKlCBrF6DVisEB6Cf3b7nPUwZV/v0PaNVugpL6cL39
kuUEAYGGeiUVi8D6K+L6tg/xAoGATlQqyAQ+Mz8Y6n0pYXfssfxDh+9dpT6w1vyo
RbsCiLtNuZ2EtjHjySjv3cl/ck5mx2sr3rmhpUYB2yFekBN1ykK6x1Z93AApEpsd
PMm9gm8SnAhC/Tl3OY8prODLr0I5Ye3X27v0TvWp5xu6DaDSBF032hDiic98Ob8m
3EMYfpcCgYBySPGnPmwqimiSyZRn+gJh+cZRF1aOKBqdqsfdcQrNpaZuZuQ4aYLo
cEoKFPr8HjXXUVCa3Q84tf9nGb4iUFslRSbS6RCP6Nm+JsfbCTtzyglYuPRKITGm
jSzka5UER3Dj1lSLMk9DkU+jrBxUsFeeiQOYhzQBaZxguvwYRIYHpg==
-----END RSA PRIVATE KEY-----`

func TestGetCloneURL(t *testing.T) {
	tcs := []struct {
		name      string
		want      string
		cloneURLs []string
	}{
		{
			name: "https",
			want: "https://github.com/sourcegraph/sourcegraph",
			cloneURLs: []string{
				`https://github.com/sourcegraph/sourcegraph`,
			},
		},
		{
			name: "ssh",
			want: "git@github.com:sourcegraph/sourcegraph.git",
			cloneURLs: []string{
				`git@github.com:sourcegraph/sourcegraph.git`,
			},
		},
		{
			name: "https and ssh, favoring https",
			want: "https://github.com/sourcegraph/sourcegraph",
			cloneURLs: []string{
				`https://github.com/sourcegraph/sourcegraph`,
				`git@github.com:sourcegraph/sourcegraph.git`,
			},
		},
		{
			name: "https and ssh, favoring https different order",
			want: "https://github.com/sourcegraph/sourcegraph",
			cloneURLs: []string{
				`git@github.com:sourcegraph/sourcegraph.git`,
				`https://github.com/sourcegraph/sourcegraph`,
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			sources := map[string]*types.SourceInfo{}
			for i, c := range tc.cloneURLs {
				sources[strconv.Itoa(i)] = &types.SourceInfo{
					ID:       strconv.Itoa(i),
					CloneURL: c,
				}
			}
			repo := &types.Repo{
				Name:    api.RepoName("github.com/sourcegraph/sourcegraph"),
				URI:     "github.com/sourcegraph/sourcegraph",
				Sources: sources,
				Metadata: &github.Repository{
					NameWithOwner: "sourcegraph/sourcegraph",
					URL:           "https://github.com/sourcegraph/sourcegraph",
				},
			}

			have, err := getCloneURL(repo)
			if err != nil {
				t.Fatal(err)
			}
			if have.String() != tc.want {
				t.Fatalf("invalid cloneURL returned, want=%q have=%q", tc.want, have)
			}
		})
	}
}

func TestLoadExternalService(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	externalService := types.ExternalService{
		ID:          1,
		Kind:        extsvc.KindGitHub,
		DisplayName: "GitHub",
		Config:      extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "authorization": {}}`),
	}
	newerExternalService := types.ExternalService{
		ID:          2,
		Kind:        extsvc.KindGitHub,
		DisplayName: "GitHub newer",
		Config:      extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "authorization": {}}`),
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
			externalService.URN(): {
				ID:       externalService.URN(),
				CloneURL: "https://github.com/sourcegraph/sourcegraph",
			},
			newerExternalService.URN(): {
				ID:       newerExternalService.URN(),
				CloneURL: "https://123456@github.com/sourcegraph/sourcegraph",
			},
		},
	}

	ess := dbmocks.NewMockExternalServiceStore()
	ess.ListFunc.SetDefaultHook(func(ctx context.Context, options database.ExternalServicesListOptions) ([]*types.ExternalService, error) {
		sources := make([]*types.ExternalService, 0)
		if _, ok := repo.Sources[newerExternalService.URN()]; ok {
			sources = append(sources, &newerExternalService)
		}
		// Simulate original ORDER BY ID DESC.
		sort.SliceStable(sources, func(i, j int) bool { return sources[i].ID > sources[j].ID })
		return sources, nil
	})

	// Expect the newest public external service with a token to be returned.
	ids := repo.ExternalServiceIDs()
	svc, err := loadExternalService(ctx, ess, database.ExternalServicesListOptions{IDs: ids})
	if err != nil {
		t.Fatalf("invalid error, expected nil, got %v", err)
	}
	if have, want := svc.ID, newerExternalService.ID; have != want {
		t.Fatalf("invalid external service returned, want=%d have=%d", want, have)
	}
}

func TestGitserverPushConfig(t *testing.T) {
	oauthHTTPSAuthenticator := auth.OAuthBearerToken{Token: "bearer-test"}
	oauthSSHAuthenticator := auth.OAuthBearerTokenWithSSH{
		OAuthBearerToken: oauthHTTPSAuthenticator,
		PrivateKey:       "private-key",
		Passphrase:       "passphrase",
		PublicKey:        "public-key",
	}
	basicHTTPSAuthenticator := auth.BasicAuth{Username: "basic", Password: "pw"}
	basicSSHAuthenticator := auth.BasicAuthWithSSH{
		BasicAuth:  basicHTTPSAuthenticator,
		PrivateKey: "private-key",
		Passphrase: "passphrase",
		PublicKey:  "public-key",
	}
	tcs := []struct {
		name                string
		repoName            string
		externalServiceType string
		cloneURLs           []string
		authenticator       auth.Authenticator
		wantPushConfig      *protocol.PushConfig
		wantErr             error
	}{
		// Without authenticator:
		{
			name:                "GitHub HTTPS no token",
			repoName:            "github.com/sourcegraph/sourcegraph",
			externalServiceType: extsvc.TypeGitHub,
			cloneURLs:           []string{"https://github.com/sourcegraph/sourcegraph"},
			authenticator:       nil,
			wantErr:             ErrNoPushCredentials{},
		},
		// With authenticator:
		{
			name:                "GitHub HTTPS with authenticator",
			repoName:            "github.com/sourcegraph/sourcegraph",
			externalServiceType: extsvc.TypeGitHub,
			cloneURLs:           []string{"https://github.com/sourcegraph/sourcegraph"},
			authenticator:       &oauthHTTPSAuthenticator,
			wantPushConfig: &protocol.PushConfig{
				RemoteURL: "https://bearer-test@github.com/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "GitHub SSH with authenticator",
			repoName:            "github.com/sourcegraph/sourcegraph",
			externalServiceType: extsvc.TypeGitHub,
			cloneURLs:           []string{"git@github.com:sourcegraph/sourcegraph.git"},
			authenticator:       &oauthSSHAuthenticator,
			wantPushConfig: &protocol.PushConfig{
				RemoteURL:  "git@github.com:sourcegraph/sourcegraph.git",
				PrivateKey: "private-key",
				Passphrase: "passphrase",
			},
		},
		{
			name:                "GitLab HTTPS with authenticator",
			repoName:            "sourcegraph/sourcegraph",
			externalServiceType: extsvc.TypeGitLab,
			cloneURLs:           []string{"https://gitlab.com/sourcegraph/sourcegraph"},
			authenticator:       &oauthHTTPSAuthenticator,
			wantPushConfig: &protocol.PushConfig{
				RemoteURL: "https://git:bearer-test@gitlab.com/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "GitLab SSH with authenticator",
			repoName:            "sourcegraph/sourcegraph",
			externalServiceType: extsvc.TypeGitLab,
			cloneURLs:           []string{"git@gitlab.com:sourcegraph/sourcegraph.git"},
			authenticator:       &oauthSSHAuthenticator,
			wantPushConfig: &protocol.PushConfig{
				RemoteURL:  "git@gitlab.com:sourcegraph/sourcegraph.git",
				PrivateKey: "private-key",
				Passphrase: "passphrase",
			},
		},
		{
			name:                "Bitbucket server HTTPS with authenticator",
			repoName:            "bitbucket.sgdev.org/sourcegraph/sourcegraph",
			externalServiceType: extsvc.TypeBitbucketServer,
			cloneURLs:           []string{"https://bitbucket.sgdev.org/sourcegraph/sourcegraph"},
			authenticator:       &basicHTTPSAuthenticator,
			wantPushConfig: &protocol.PushConfig{
				RemoteURL: "https://basic:pw@bitbucket.sgdev.org/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "Bitbucket server SSH with authenticator",
			repoName:            "bitbucket.sgdev.org/sourcegraph/sourcegraph",
			externalServiceType: extsvc.TypeBitbucketServer,
			authenticator:       &basicSSHAuthenticator,
			cloneURLs:           []string{"git@bitbucket.sgdev.org:7999/sourcegraph/sourcegraph.git"},
			wantPushConfig: &protocol.PushConfig{
				RemoteURL:  "git@bitbucket.sgdev.org:7999/sourcegraph/sourcegraph.git",
				PrivateKey: "private-key",
				Passphrase: "passphrase",
			},
		},
		// Errors
		{
			name:                "Bitbucket server SSH no keypair",
			externalServiceType: extsvc.TypeBitbucketServer,
			cloneURLs:           []string{"git@bitbucket.sgdev.org:7999/sourcegraph/sourcegraph.git"},
			authenticator:       &basicHTTPSAuthenticator,
			wantErr:             ErrNoSSHCredential,
		},
		{
			name:                "Invalid credential type",
			externalServiceType: extsvc.TypeBitbucketServer,
			cloneURLs:           []string{"https://bitbucket.sgdev.org/sourcegraph/sourcegraph"},
			authenticator:       &auth.OAuthClient{},
			wantErr:             ErrNoPushCredentials{CredentialsType: "*auth.OAuthClient"},
		},
	}
	for _, tt := range tcs {
		t.Run(tt.name, func(t *testing.T) {
			sources := map[string]*types.SourceInfo{}
			for i, c := range tt.cloneURLs {
				sources[strconv.Itoa(i)] = &types.SourceInfo{
					ID:       strconv.Itoa(i),
					CloneURL: c,
				}
			}
			repo := &types.Repo{
				Name:    api.RepoName(tt.repoName),
				URI:     tt.repoName,
				Sources: sources,
				ExternalRepo: api.ExternalRepoSpec{
					ServiceType: tt.externalServiceType,
				},
			}

			havePushConfig, haveErr := GitserverPushConfig(repo, tt.authenticator)
			if haveErr != tt.wantErr {
				t.Fatalf("invalid error returned, want=%v have=%v", tt.wantErr, haveErr)
			}
			if diff := cmp.Diff(havePushConfig, tt.wantPushConfig); diff != "" {
				t.Fatalf("invalid push config returned: %s", diff)
			}
		})
	}
}

func TestSourcer_ForChangeset(t *testing.T) {
	ctx := context.Background()

	es := &types.ExternalService{
		ID:          1,
		Kind:        extsvc.KindGitHub,
		DisplayName: "GitHub.com",
		Config:      extsvc.NewUnencryptedConfig(`{"url": "https://github.com/", "repos": ["sourcegraph/sourcegraph"], "token": "secret"}`),
	}
	cfg, err := es.Configuration(ctx)
	if err != nil {
		t.Fatal("could not get config for external service: ", err)
	}
	config, ok := cfg.(*schema.GitHubConnection)
	if !ok {
		t.Fatal("got wrong config type for external service")
	}

	repo := &types.Repo{
		Name:    api.RepoName("some-org/test-repo"),
		URI:     "test-repo",
		Private: true,
		Metadata: &github.Repository{
			ID:            "external-id-123",
			NameWithOwner: "some-org/test-repo",
		},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "external-id-123",
			ServiceType: extsvc.TypeGitHub,
			ServiceID:   "https://github.com/",
		},
		Sources: map[string]*types.SourceInfo{
			es.URN(): {
				ID:       es.URN(),
				CloneURL: "https://123@github.com/sourcegraph/sourcegraph",
			},
		},
	}

	siteToken := &auth.OAuthBearerToken{Token: "site"}
	userToken := &auth.OAuthBearerToken{Token: "user"}

	t.Run("created changesets", func(t *testing.T) {
		bc := &btypes.BatchChange{ID: 1, LastApplierID: 3}
		ch := &btypes.Changeset{ID: 2, OwnedByBatchChangeID: 1}

		t.Run("with user credential", func(t *testing.T) {
			credStore := dbmocks.NewMockUserCredentialsStore()
			credStore.GetByScopeFunc.SetDefaultHook(func(ctx context.Context, opts database.UserCredentialScope) (*database.UserCredential, error) {
				assert.EqualValues(t, repo.ExternalRepo.ServiceID, opts.ExternalServiceID)
				assert.EqualValues(t, repo.ExternalRepo.ServiceType, opts.ExternalServiceType)
				assert.EqualValues(t, bc.LastApplierID, opts.UserID)
				cred := &database.UserCredential{Credential: database.NewEmptyCredential()}
				cred.SetAuthenticator(ctx, userToken)
				return cred, nil
			})
			extsvcStore := dbmocks.NewMockExternalServiceStore()
			extsvcStore.ListFunc.SetDefaultReturn([]*types.ExternalService{es}, nil)

			tx := NewMockSourcerStore()
			rs := dbmocks.NewMockRepoStore()
			rs.GetFunc.SetDefaultReturn(repo, nil)
			tx.ReposFunc.SetDefaultReturn(rs)
			tx.GetBatchChangeFunc.SetDefaultHook(func(ctx context.Context, opts store.GetBatchChangeOpts) (*btypes.BatchChange, error) {
				assert.EqualValues(t, bc.ID, opts.ID)
				return bc, nil
			})
			tx.UserCredentialsFunc.SetDefaultReturn(credStore)
			tx.ExternalServicesFunc.SetDefaultReturn(extsvcStore)

			css := NewMockChangesetSource()
			want := NewMockChangesetSource()
			css.WithAuthenticatorFunc.SetDefaultHook(func(a auth.Authenticator) (ChangesetSource, error) {
				assert.Equal(t, userToken, a)
				return want, nil
			})

			have, err := newMockSourcer(css).ForChangeset(ctx, tx, ch, AuthenticationStrategyUserCredential, repo)
			assert.NoError(t, err)
			assert.Same(t, want, have)
		})

		t.Run("with site credential", func(t *testing.T) {
			credStore := dbmocks.NewMockUserCredentialsStore()
			credStore.GetByScopeFunc.SetDefaultHook(func(ctx context.Context, opts database.UserCredentialScope) (*database.UserCredential, error) {
				assert.EqualValues(t, repo.ExternalRepo.ServiceID, opts.ExternalServiceID)
				assert.EqualValues(t, repo.ExternalRepo.ServiceType, opts.ExternalServiceType)
				assert.EqualValues(t, bc.LastApplierID, opts.UserID)
				return nil, &errcode.Mock{IsNotFound: true}
			})
			extsvcStore := dbmocks.NewMockExternalServiceStore()
			extsvcStore.ListFunc.SetDefaultReturn([]*types.ExternalService{es}, nil)

			tx := NewMockSourcerStore()
			rs := dbmocks.NewMockRepoStore()
			rs.GetFunc.SetDefaultReturn(repo, nil)
			tx.ReposFunc.SetDefaultReturn(rs)
			tx.GetBatchChangeFunc.SetDefaultHook(func(ctx context.Context, opts store.GetBatchChangeOpts) (*btypes.BatchChange, error) {
				assert.EqualValues(t, bc.ID, opts.ID)
				return bc, nil
			})
			tx.GetSiteCredentialFunc.SetDefaultHook(func(ctx context.Context, opts store.GetSiteCredentialOpts) (*btypes.SiteCredential, error) {
				assert.EqualValues(t, repo.ExternalRepo.ServiceID, opts.ExternalServiceID)
				assert.EqualValues(t, repo.ExternalRepo.ServiceType, opts.ExternalServiceType)
				cred := &btypes.SiteCredential{Credential: database.NewEmptyCredential()}
				cred.SetAuthenticator(ctx, siteToken)
				return cred, nil
			})
			tx.UserCredentialsFunc.SetDefaultReturn(credStore)
			tx.ExternalServicesFunc.SetDefaultReturn(extsvcStore)

			css := NewMockChangesetSource()
			want := NewMockChangesetSource()
			css.WithAuthenticatorFunc.SetDefaultHook(func(a auth.Authenticator) (ChangesetSource, error) {
				assert.Equal(t, siteToken, a)
				return want, nil
			})

			have, err := newMockSourcer(css).ForChangeset(ctx, tx, ch, AuthenticationStrategyUserCredential, repo)
			assert.NoError(t, err)
			assert.Same(t, want, have)
		})

		t.Run("without site credential", func(t *testing.T) {
			// When we remove the fallback to the external service
			// configuration, this test is expected to fail.
			credStore := dbmocks.NewMockUserCredentialsStore()
			credStore.GetByScopeFunc.SetDefaultHook(func(ctx context.Context, opts database.UserCredentialScope) (*database.UserCredential, error) {
				assert.EqualValues(t, repo.ExternalRepo.ServiceID, opts.ExternalServiceID)
				assert.EqualValues(t, repo.ExternalRepo.ServiceType, opts.ExternalServiceType)
				assert.EqualValues(t, bc.LastApplierID, opts.UserID)
				return nil, &errcode.Mock{IsNotFound: true}
			})
			extsvcStore := dbmocks.NewMockExternalServiceStore()
			extsvcStore.ListFunc.SetDefaultReturn([]*types.ExternalService{es}, nil)

			tx := NewMockSourcerStore()
			rs := dbmocks.NewMockRepoStore()
			rs.GetFunc.SetDefaultReturn(repo, nil)
			tx.ReposFunc.SetDefaultReturn(rs)
			tx.GetBatchChangeFunc.SetDefaultHook(func(ctx context.Context, opts store.GetBatchChangeOpts) (*btypes.BatchChange, error) {
				assert.EqualValues(t, bc.ID, opts.ID)
				return bc, nil
			})
			tx.GetSiteCredentialFunc.SetDefaultHook(func(ctx context.Context, opts store.GetSiteCredentialOpts) (*btypes.SiteCredential, error) {
				assert.EqualValues(t, repo.ExternalRepo.ServiceID, opts.ExternalServiceID)
				assert.EqualValues(t, repo.ExternalRepo.ServiceType, opts.ExternalServiceType)
				return nil, store.ErrNoResults
			})
			tx.UserCredentialsFunc.SetDefaultReturn(credStore)
			tx.ExternalServicesFunc.SetDefaultReturn(extsvcStore)

			css := NewMockChangesetSource()
			_, err := newMockSourcer(css).ForChangeset(ctx, tx, ch, AuthenticationStrategyUserCredential, repo)
			assert.Error(t, err)
		})

		t.Run("with GH App", func(t *testing.T) {
			ghaStore := ghastore.NewMockGitHubAppsStore()
			ghaStore.GetByDomainFunc.SetDefaultHook(func(ctx context.Context, domain types.GitHubAppDomain, baseUrl string) (*ghatypes.GitHubApp, error) {
				assert.EqualValues(t, types.BatchesGitHubAppDomain, domain)
				assert.EqualValues(t, config.Url, baseUrl)
				ghApp := &ghatypes.GitHubApp{
					BaseURL:    config.Url,
					Domain:     types.BatchesGitHubAppDomain,
					AppID:      1234,
					PrivateKey: testGHAppPrivateKey,
				}
				return ghApp, nil
			})
			ghaStore.GetInstallIDFunc.SetDefaultHook(func(ctx context.Context, appId int, account string) (int, error) {
				assert.EqualValues(t, "some-org", account)
				assert.EqualValues(t, 1234, appId)
				return 5678, nil
			})
			extsvcStore := dbmocks.NewMockExternalServiceStore()
			extsvcStore.ListFunc.SetDefaultReturn([]*types.ExternalService{es}, nil)

			tx := NewMockSourcerStore()
			rs := dbmocks.NewMockRepoStore()
			rs.GetFunc.SetDefaultReturn(repo, nil)
			tx.ReposFunc.SetDefaultReturn(rs)
			tx.GetBatchChangeFunc.SetDefaultHook(func(ctx context.Context, opts store.GetBatchChangeOpts) (*btypes.BatchChange, error) {
				assert.EqualValues(t, bc.ID, opts.ID)
				return bc, nil
			})
			tx.ExternalServicesFunc.SetDefaultReturn(extsvcStore)
			tx.GitHubAppsStoreFunc.SetDefaultReturn(ghaStore)
			tx.GetChangesetSpecByIDFunc.SetDefaultReturn(&btypes.ChangesetSpec{
				ForkNamespace: nil,
			}, nil)

			css := NewMockChangesetSource()
			want := NewMockChangesetSource()
			css.WithAuthenticatorFunc.SetDefaultHook(func(a auth.Authenticator) (ChangesetSource, error) {
				au, ok := a.(*ghaauth.InstallationAuthenticator)
				if !ok {
					t.Fatalf("unexpected authenticator type: %T", a)
				}
				assert.EqualValues(t, 5678, au.InstallationID())
				return want, nil
			})

			have, err := newMockSourcer(css).ForChangeset(ctx, tx, ch, AuthenticationStrategyGitHubApp, repo)
			assert.NoError(t, err)
			assert.Same(t, want, have)
		})

		t.Run("with GH Ap (forked changeset)", func(t *testing.T) {
			forkedRepoNamespace := "some-forked-org"
			ghaStore := ghastore.NewMockGitHubAppsStore()
			ghaStore.GetByDomainFunc.SetDefaultHook(func(ctx context.Context, domain types.GitHubAppDomain, baseUrl string) (*ghatypes.GitHubApp, error) {
				assert.EqualValues(t, types.BatchesGitHubAppDomain, domain)
				assert.EqualValues(t, config.Url, baseUrl)
				ghApp := &ghatypes.GitHubApp{
					BaseURL:    config.Url,
					Domain:     types.BatchesGitHubAppDomain,
					AppID:      1234,
					PrivateKey: testGHAppPrivateKey,
				}
				return ghApp, nil
			})
			ghaStore.GetInstallIDFunc.SetDefaultHook(func(ctx context.Context, appId int, account string) (int, error) {
				assert.EqualValues(t, forkedRepoNamespace, account)
				assert.EqualValues(t, 1234, appId)
				return 5678, nil
			})
			extsvcStore := dbmocks.NewMockExternalServiceStore()
			extsvcStore.ListFunc.SetDefaultReturn([]*types.ExternalService{es}, nil)

			tx := NewMockSourcerStore()
			rs := dbmocks.NewMockRepoStore()
			rs.GetFunc.SetDefaultReturn(repo, nil)
			tx.ReposFunc.SetDefaultReturn(rs)
			tx.GetBatchChangeFunc.SetDefaultHook(func(ctx context.Context, opts store.GetBatchChangeOpts) (*btypes.BatchChange, error) {
				assert.EqualValues(t, bc.ID, opts.ID)
				return bc, nil
			})
			tx.ExternalServicesFunc.SetDefaultReturn(extsvcStore)
			tx.GitHubAppsStoreFunc.SetDefaultReturn(ghaStore)
			tx.GetChangesetSpecByIDFunc.SetDefaultReturn(&btypes.ChangesetSpec{
				ForkNamespace: pointers.Ptr("<user>"),
			}, nil)

			css := NewMockChangesetSource()
			want := NewMockChangesetSource()
			css.WithAuthenticatorFunc.SetDefaultHook(func(a auth.Authenticator) (ChangesetSource, error) {
				au, ok := a.(*ghaauth.InstallationAuthenticator)
				if !ok {
					t.Fatalf("unexpected authenticator type: %T", a)
				}
				assert.EqualValues(t, 5678, au.InstallationID())
				return want, nil
			})

			// because it's a forked changeset, the target repo should be pointing to a fork instead
			// of the actual repo.
			targetRepo := &types.Repo{
				Sources: map[string]*types.SourceInfo{
					"fork": {
						ID:       es.URN(),
						CloneURL: fmt.Sprintf("https://github.com/%s/sourcegraph", forkedRepoNamespace),
					},
				},
			}

			have, err := newMockSourcer(css).ForChangeset(ctx, tx, ch, AuthenticationStrategyGitHubApp, targetRepo)
			assert.NoError(t, err)
			assert.Same(t, want, have)
		})
	})

	t.Run("imported changesets", func(t *testing.T) {
		ch := &btypes.Changeset{ID: 2, OwnedByBatchChangeID: 0}

		t.Run("with site credential", func(t *testing.T) {
			extsvcStore := dbmocks.NewMockExternalServiceStore()
			extsvcStore.ListFunc.SetDefaultReturn([]*types.ExternalService{es}, nil)

			tx := NewMockSourcerStore()
			rs := dbmocks.NewMockRepoStore()
			rs.GetFunc.SetDefaultReturn(repo, nil)
			tx.ReposFunc.SetDefaultReturn(rs)
			tx.GetSiteCredentialFunc.SetDefaultHook(func(ctx context.Context, opts store.GetSiteCredentialOpts) (*btypes.SiteCredential, error) {
				assert.EqualValues(t, repo.ExternalRepo.ServiceID, opts.ExternalServiceID)
				assert.EqualValues(t, repo.ExternalRepo.ServiceType, opts.ExternalServiceType)
				cred := &btypes.SiteCredential{Credential: database.NewEmptyCredential()}
				cred.SetAuthenticator(ctx, siteToken)
				return cred, nil
			})
			tx.ExternalServicesFunc.SetDefaultReturn(extsvcStore)

			css := NewMockChangesetSource()
			want := NewMockChangesetSource()
			css.WithAuthenticatorFunc.SetDefaultHook(func(a auth.Authenticator) (ChangesetSource, error) {
				assert.Equal(t, siteToken, a)
				return want, nil
			})

			have, err := newMockSourcer(css).ForChangeset(ctx, tx, ch, AuthenticationStrategyUserCredential, repo)
			assert.NoError(t, err)
			assert.Same(t, want, have)
		})

		// When we remove the fallback to the external service configuration, this test is
		// expected to fail.
		t.Run("without site credential", func(t *testing.T) {
			extsvcStore := dbmocks.NewMockExternalServiceStore()
			extsvcStore.ListFunc.SetDefaultReturn([]*types.ExternalService{es}, nil)

			tx := NewMockSourcerStore()
			rs := dbmocks.NewMockRepoStore()
			rs.GetFunc.SetDefaultReturn(repo, nil)
			tx.ReposFunc.SetDefaultReturn(rs)
			tx.GetSiteCredentialFunc.SetDefaultHook(func(ctx context.Context, opts store.GetSiteCredentialOpts) (*btypes.SiteCredential, error) {
				assert.EqualValues(t, repo.ExternalRepo.ServiceID, opts.ExternalServiceID)
				assert.EqualValues(t, repo.ExternalRepo.ServiceType, opts.ExternalServiceType)
				return nil, store.ErrNoResults
			})
			tx.ExternalServicesFunc.SetDefaultReturn(extsvcStore)

			css := NewMockChangesetSource()
			want := errors.New("validator was called")
			css.ValidateAuthenticatorFunc.SetDefaultReturn(want)

			_, err := newMockSourcer(css).ForChangeset(ctx, tx, ch, AuthenticationStrategyUserCredential, repo)
			assert.Error(t, err)
		})
	})
}

func TestGetRemoteRepo(t *testing.T) {
	ctx := context.Background()
	targetRepo := &types.Repo{}

	t.Run("forks disabled", func(t *testing.T) {
		t.Run("unforked changeset", func(t *testing.T) {
			// Set up a changeset source that will panic if any methods are invoked.
			css := NewStrictMockChangesetSource()

			// This should succeed, since loadRemoteRepo() should early return with
			// forks disabled.
			remoteRepo, err := GetRemoteRepo(ctx, css, targetRepo, &btypes.Changeset{}, &btypes.ChangesetSpec{
				ForkNamespace: nil,
			})
			assert.Nil(t, err)
			assert.Same(t, targetRepo, remoteRepo)
		})

		t.Run("forked changeset", func(t *testing.T) {
			forkNamespace := "fork"
			want := &types.Repo{}
			css := NewMockForkableChangesetSource()
			css.GetForkFunc.SetDefaultReturn(want, nil)

			// This should succeed, since loadRemoteRepo() should early return with
			// forks disabled.
			remoteRepo, err := GetRemoteRepo(ctx, css, targetRepo, &btypes.Changeset{}, &btypes.ChangesetSpec{
				ForkNamespace: &forkNamespace,
			})
			assert.Nil(t, err)
			assert.Same(t, want, remoteRepo)
			mockassert.CalledOnce(t, css.GetForkFunc)
		})
	})

	t.Run("forks enabled", func(t *testing.T) {
		forkNamespace := "<user>"

		t.Run("unforkable changeset source", func(t *testing.T) {
			css := NewMockChangesetSource()

			repo, err := GetRemoteRepo(ctx, css, targetRepo, &btypes.Changeset{}, &btypes.ChangesetSpec{
				ForkNamespace: &forkNamespace,
			})
			assert.Nil(t, repo)
			assert.ErrorContains(t, err, ErrChangesetSourceCannotFork.Error())
		})

		t.Run("forkable changeset source", func(t *testing.T) {
			t.Run("success", func(t *testing.T) {
				want := &types.Repo{}
				css := NewMockForkableChangesetSource()
				css.GetForkFunc.SetDefaultReturn(want, nil)

				have, err := GetRemoteRepo(ctx, css, targetRepo, &btypes.Changeset{}, &btypes.ChangesetSpec{
					ForkNamespace: &forkNamespace,
				})
				assert.Nil(t, err)
				assert.Same(t, want, have)
				mockassert.CalledOnce(t, css.GetForkFunc)
			})

			t.Run("error from the source", func(t *testing.T) {
				want := errors.New("source error")
				css := NewMockForkableChangesetSource()
				css.GetForkFunc.SetDefaultReturn(nil, want)

				repo, err := GetRemoteRepo(ctx, css, targetRepo, &btypes.Changeset{}, &btypes.ChangesetSpec{
					ForkNamespace: &forkNamespace,
				})
				assert.Nil(t, repo)
				assert.Contains(t, err.Error(), want.Error())
				mockassert.CalledOnce(t, css.GetForkFunc)
			})
		})
	})
}

func newMockSourcer(css ChangesetSource) Sourcer {
	return newSourcer(nil, func(ctx context.Context, tx SourcerStore, cf *httpcli.Factory, extSvc *types.ExternalService) (ChangesetSource, error) {
		return css, nil
	})
}
