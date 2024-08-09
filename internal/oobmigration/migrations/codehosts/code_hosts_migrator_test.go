package codehosts

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	et "github.com/sourcegraph/sourcegraph/internal/encryption/testing"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration/migrations/codehosts/schema"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

type testExtSvc struct {
	kind string
	cfg  any
}

// Empty limits for all types.
var testExtSvcs = []testExtSvc{
	{kind: "AWSCODECOMMIT", cfg: schema.AWSCodeCommitConnection{
		Region:      "us-east-1",
		AccessKeyID: "ABCDEF",
	}},
	{kind: "AZUREDEVOPS", cfg: schema.AzureDevOpsConnection{
		Url: "https://dev.azure.com",
	}},
	{kind: "BITBUCKETCLOUD", cfg: schema.BitbucketCloudConnection{
		Url: "https://bitbucket.org",
	}},
	{kind: "BITBUCKETSERVER", cfg: schema.BitbucketServerConnection{
		Url: "https://bitbucket.sgdev.org",
	}},
	{kind: "GERRIT", cfg: schema.GerritConnection{
		Url: "https://gerrit.sgdev.org",
	}},
	{kind: "GITHUB", cfg: schema.GitHubConnection{
		Url: "https://github.com",
	}},
	// Note: Here are two github services for the same host, to make sure that
	// this won't cause issues.
	{kind: "GITHUB", cfg: schema.GitHubConnection{
		Url: "https://github.com",
	}},
	{kind: "GITLAB", cfg: schema.GitLabConnection{
		Url: "https://gitlab.com",
	}},
	{kind: "GITOLITE", cfg: schema.GitoliteConnection{
		Host: "ssh://git@github.com",
	}},
	{kind: "GOMODULES", cfg: schema.GoModulesConnection{}},
	{kind: "JVMPACKAGES", cfg: schema.JVMPackagesConnection{}},
	{kind: "NPMPACKAGES", cfg: schema.NpmPackagesConnection{}},
	{kind: "OTHER", cfg: schema.OtherExternalServiceConnection{
		Url: "https://user:pass@sgdev.org/repo.git",
	}},
	{kind: "PAGURE", cfg: schema.PagureConnection{
		Url: "https://pagure.sgdev.org",
	}},
	{kind: "PERFORCE", cfg: schema.PerforceConnection{
		P4Port: "ssl:111.222.333.444:1666",
	}},
	{kind: "PHABRICATOR", cfg: schema.PhabricatorConnection{
		Url: "https://phabricator.sgdev.org",
	}},
	{kind: "PYTHONPACKAGES", cfg: schema.PythonPackagesConnection{}},
	{kind: "RUBYPACKAGES", cfg: schema.RubyPackagesConnection{}},
	{kind: "RUSTPACKAGES", cfg: schema.RustPackagesConnection{}},
	{kind: "LOCALGIT", cfg: schema.LocalGitExternalService{}},
}

// Set with limits for all types that support it.
var testExtSvcsWithLimits = []testExtSvc{
	{kind: "AWSCODECOMMIT", cfg: schema.AWSCodeCommitConnection{
		Region:      "us-east-1",
		AccessKeyID: "ABCDEF",
		// Doesn't support limiting.
	}},
	{kind: "AZUREDEVOPS", cfg: schema.AzureDevOpsConnection{
		Url: "https://dev.azure.com",
		// Doesn't support limiting.
	}},
	{kind: "BITBUCKETCLOUD", cfg: schema.BitbucketCloudConnection{
		Url: "https://bitbucket.org",
		RateLimit: &schema.BitbucketCloudRateLimit{
			Enabled:         true,
			RequestsPerHour: 1800,
		},
	}},
	{kind: "BITBUCKETSERVER", cfg: schema.BitbucketServerConnection{
		Url: "https://bitbucket.sgdev.org",
		RateLimit: &schema.BitbucketServerRateLimit{
			Enabled:         true,
			RequestsPerHour: 3600,
		},
	}},
	{kind: "GERRIT", cfg: schema.GerritConnection{
		Url: "https://gerrit.sgdev.org",
		// Doesn't support limiting.
	}},
	{kind: "GITHUB", cfg: schema.GitHubConnection{
		Url: "https://github.com",
		RateLimit: &schema.GitHubRateLimit{
			Enabled:         true,
			RequestsPerHour: 5000,
		},
	}},
	// Note: Here are two github services for the same host, to make sure that
	// this won't cause issues. Also note, that this service allows MORE requests
	// than the former one, so the test also verifies that we actually choose the
	// lowest limit.
	{kind: "GITHUB", cfg: schema.GitHubConnection{
		Url: "https://github.com",
		RateLimit: &schema.GitHubRateLimit{
			Enabled:         true,
			RequestsPerHour: 7500,
		},
	}},
	{kind: "GITLAB", cfg: schema.GitLabConnection{
		Url: "https://gitlab.com",
		RateLimit: &schema.GitLabRateLimit{
			Enabled:         true,
			RequestsPerHour: 6000,
		},
	}},
	{kind: "GITOLITE", cfg: schema.GitoliteConnection{
		Host: "ssh://git@github.com",
		// Doesn't support limiting.
	}},
	{kind: "GOMODULES", cfg: schema.GoModulesConnection{
		RateLimit: &schema.GoRateLimit{
			Enabled:         true,
			RequestsPerHour: 10000,
		},
	}},
	{kind: "JVMPACKAGES", cfg: schema.JVMPackagesConnection{
		Maven: schema.Maven{
			RateLimit: &schema.MavenRateLimit{
				Enabled:         true,
				RequestsPerHour: 11500,
			},
		},
	}},
	{kind: "NPMPACKAGES", cfg: schema.NpmPackagesConnection{
		RateLimit: &schema.NpmRateLimit{
			Enabled:         true,
			RequestsPerHour: 12000,
		},
	}},
	{kind: "OTHER", cfg: schema.OtherExternalServiceConnection{
		Url: "https://user:pass@sgdev.org/repo.git",
		// Doesn't support limiting.
	}},
	{kind: "PAGURE", cfg: schema.PagureConnection{
		Url: "https://pagure.sgdev.org",
		RateLimit: &schema.PagureRateLimit{
			Enabled:         true,
			RequestsPerHour: 13000,
		},
	}},
	{kind: "PERFORCE", cfg: schema.PerforceConnection{
		P4Port: "ssl:111.222.333.444:1666",
		RateLimit: &schema.PerforceRateLimit{
			Enabled:         true,
			RequestsPerHour: 14000,
		},
	}},
	{kind: "PHABRICATOR", cfg: schema.PhabricatorConnection{
		Url: "https://phabricator.sgdev.org",
		// Doesn't support limiting.
	}},
	{kind: "PYTHONPACKAGES", cfg: schema.PythonPackagesConnection{
		RateLimit: &schema.PythonRateLimit{
			Enabled:         true,
			RequestsPerHour: 15000,
		},
	}},
	{kind: "RUBYPACKAGES", cfg: schema.RubyPackagesConnection{
		RateLimit: &schema.RubyRateLimit{
			Enabled:         true,
			RequestsPerHour: 16000,
		},
	}},
	{kind: "RUSTPACKAGES", cfg: schema.RustPackagesConnection{
		RateLimit: &schema.RustRateLimit{
			Enabled:         true,
			RequestsPerHour: 17000,
		},
	}},
	{kind: "LOCALGIT", cfg: schema.LocalGitExternalService{
		// Doesn't support limiting.
	}},
}

func TestCodeHostsMigrator(t *testing.T) {
	logger := logtest.Scoped(t)
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(logger, t))
	store := basestore.NewWithHandle(db.Handle())
	key := et.TestKey{}
	m := NewMigratorWithDB(store, key)

	t.Run("EmptyConfigs", func(t *testing.T) {
		t.Run("Progress", func(t *testing.T) {
			ensureCleanDB(t, ctx, store)

			// With no data in the db, this migration should be done.
			progress, err := m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 1., progress)

			// Now insert data.
			created := createExternalServices(t, ctx, store, testExtSvcs, false)

			// Assume no progress now since none of the new records are migrated yet.
			progress, err = m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 0., progress)

			// For each service, run up.
			for i := 0; i < created; i++ {
				if err := m.Up(ctx); err != nil {
					t.Fatal(err)
				}
			}

			// Now we expect all services to be migrated.
			progress, err = m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 1., progress)

			// Now we'll clear the code hosts and expect progress to drop again.
			clearCodeHosts(t, ctx, store)
			progress, err = m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 0., progress)
		})

		t.Run("Up", func(t *testing.T) {
			ensureCleanDB(t, ctx, store)

			// To start with, there should be nothing to do, no external services exist.
			// Make sure no external services returns a nil error.
			assert.Nil(t, m.Up(ctx))

			// Now we'll create our external services.
			created := createExternalServices(t, ctx, store, testExtSvcs, false)

			// Now, we need to run Up up to created times, so every individual code
			// host URL has been considered.
			for i := 0; i < created; i++ {
				assert.Nil(t, m.Up(ctx))
			}

			// Check that we're actually done.
			progress, err := m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 1., progress)

			// Now check that we have all code_hosts in the DB that we would expect
			// to be there, one of each kind, and one for the deleted host:
			// For all of those, we expect that there will be no values stored
			// for either API or git requests.
			verifyCodeHostsExist(t, store, []codeHost{
				{
					Kind: "AWSCODECOMMIT",
					URL:  "us-east-1:ABCDEF",
				},
				{
					Kind: "AZUREDEVOPS",
					URL:  "https://dev.azure.com/",
				},
				{
					Kind: "BITBUCKETCLOUD",
					URL:  "https://bitbucket.org/",
				},
				{
					Kind: "BITBUCKETSERVER",
					URL:  "https://bitbucket.sgdev.org/",
				},
				{
					Kind: "GERRIT",
					URL:  "https://gerrit.sgdev.org/",
				},
				// Our deleted code host.
				{
					Kind: "GITHUB",
					URL:  "https://ghe.sgdev.org/",
				},
				{
					Kind: "GITHUB",
					URL:  "https://github.com/",
				},
				{
					Kind: "GITLAB",
					URL:  "https://gitlab.com/",
				},
				{
					Kind: "GITOLITE",
					URL:  "ssh://git@github.com/",
				},
				{
					Kind: "GOMODULES",
					URL:  "GOMODULES",
				},
				{
					Kind: "JVMPACKAGES",
					URL:  "JVMPACKAGES",
				},
				{
					Kind: "LOCALGIT",
					URL:  "LOCALGIT",
				},
				{
					Kind: "NPMPACKAGES",
					URL:  "NPMPACKAGES",
				},
				{
					Kind: "OTHER",
					URL:  "https://user:pass@sgdev.org/repo.git/",
				},
				{
					Kind: "PAGURE",
					URL:  "https://pagure.sgdev.org/",
				},
				{
					Kind: "PERFORCE",
					URL:  "ssl:111.222.333.444:1666",
				},
				{
					Kind: "PHABRICATOR",
					URL:  "https://phabricator.sgdev.org/",
				},
				{
					Kind: "PYTHONPACKAGES",
					URL:  "PYTHONPACKAGES",
				},
				{
					Kind: "RUBYPACKAGES",
					URL:  "RUBYPACKAGES",
				},
				{
					Kind: "RUSTPACKAGES",
					URL:  "RUSTPACKAGES",
				},
			})
		})
	})

	// Test that with a site config containing gitMaxCodehostRequestsPerSecond the
	// git limits will be written correctly.
	t.Run("GitConfig", func(t *testing.T) {
		// Create a site config entry with the gitMaxCodehostRequestsPerSecond field set.
		if err := store.Exec(ctx, sqlf.Sprintf("INSERT INTO critical_and_site_config (type, contents) VALUES ('site', %s)", `{"gitMaxCodehostRequestsPerSecond": 10}`)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := store.Exec(ctx, sqlf.Sprintf("DELETE FROM critical_and_site_config WHERE 1=1")); err != nil {
				t.Fatal(err)
			}
		})

		t.Run("Progress", func(t *testing.T) {
			ensureCleanDB(t, ctx, store)

			// With no data in the db, this migration should be done.
			progress, err := m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 1., progress)

			// Now insert data.
			created := createExternalServices(t, ctx, store, testExtSvcs, false)

			// Assume no progress now since none of the new records are migrated yet.
			progress, err = m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 0., progress)

			// For each service, run up.
			for i := 0; i < created; i++ {
				if err := m.Up(ctx); err != nil {
					t.Fatal(err)
				}
			}

			// Now we expect all services to be migrated.
			progress, err = m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 1., progress)

			// Now we'll clear the code hosts and expect progress to drop again.
			clearCodeHosts(t, ctx, store)
			progress, err = m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 0., progress)
		})

		t.Run("Up", func(t *testing.T) {
			ensureCleanDB(t, ctx, store)

			// To start with, there should be nothing to do, no external services exist.
			// Make sure no external services returns a nil error.
			assert.Nil(t, m.Up(ctx))

			// Now we'll create our external services.
			created := createExternalServices(t, ctx, store, testExtSvcs, false)

			// Now, we need to run Up up to created times, so every individual code
			// host URL has been considered.
			for i := 0; i < created; i++ {
				assert.Nil(t, m.Up(ctx))
			}

			// Check that we're actually done.
			progress, err := m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 1., progress)

			// Now check that we have all code_hosts in the DB that we would expect
			// to be there, one of each kind, and one for the deleted host:
			// For all of those, we expect that there will be no values stored
			// for either API or git requests.
			verifyCodeHostsExist(t, store, []codeHost{
				{
					Kind:                        "AWSCODECOMMIT",
					URL:                         "us-east-1:ABCDEF",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "AZUREDEVOPS",
					URL:                         "https://dev.azure.com/",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "BITBUCKETCLOUD",
					URL:                         "https://bitbucket.org/",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "BITBUCKETSERVER",
					URL:                         "https://bitbucket.sgdev.org/",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "GERRIT",
					URL:                         "https://gerrit.sgdev.org/",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				// Our deleted code host.
				{
					Kind:                        "GITHUB",
					URL:                         "https://ghe.sgdev.org/",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "GITHUB",
					URL:                         "https://github.com/",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "GITLAB",
					URL:                         "https://gitlab.com/",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "GITOLITE",
					URL:                         "ssh://git@github.com/",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "GOMODULES",
					URL:                         "GOMODULES",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "JVMPACKAGES",
					URL:                         "JVMPACKAGES",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "LOCALGIT",
					URL:                         "LOCALGIT",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "NPMPACKAGES",
					URL:                         "NPMPACKAGES",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "OTHER",
					URL:                         "https://user:pass@sgdev.org/repo.git/",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "PAGURE",
					URL:                         "https://pagure.sgdev.org/",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "PERFORCE",
					URL:                         "ssl:111.222.333.444:1666",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "PHABRICATOR",
					URL:                         "https://phabricator.sgdev.org/",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "PYTHONPACKAGES",
					URL:                         "PYTHONPACKAGES",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "RUBYPACKAGES",
					URL:                         "RUBYPACKAGES",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
				{
					Kind:                        "RUSTPACKAGES",
					URL:                         "RUSTPACKAGES",
					GitRateLimitQuota:           pointers.Ptr(int32(10)),
					GitRateLimitIntervalSeconds: pointers.Ptr(int32(1)),
				},
			})
		})
	})

	// Test that rate limits are extracted and written correctly.
	t.Run("RateLimits", func(t *testing.T) {
		t.Run("Progress", func(t *testing.T) {
			ensureCleanDB(t, ctx, store)

			// With no data in the db, this migration should be done.
			progress, err := m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 1., progress)

			// Now insert data.
			created := createExternalServices(t, ctx, store, testExtSvcsWithLimits, true)

			// Assume no progress now since none of the new records are migrated yet.
			progress, err = m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 0., progress)

			// For each service, run up.
			for i := 0; i < created; i++ {
				if err := m.Up(ctx); err != nil {
					t.Fatal(err)
				}
			}

			// Now we expect all services to be migrated.
			progress, err = m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 1., progress)

			// Now we'll clear the code hosts and expect progress to drop again.
			clearCodeHosts(t, ctx, store)
			progress, err = m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 0., progress)
		})

		t.Run("Up", func(t *testing.T) {
			ensureCleanDB(t, ctx, store)

			// To start with, there should be nothing to do, no external services exist.
			// Make sure no external services returns a nil error.
			assert.Nil(t, m.Up(ctx))

			// Now we'll create our external services.
			created := createExternalServices(t, ctx, store, testExtSvcsWithLimits, true)

			// Now, we need to run Up up to created times, so every individual code
			// host URL has been considered.
			for i := 0; i < created; i++ {
				assert.Nil(t, m.Up(ctx))
			}

			// Check that we're actually done.
			progress, err := m.Progress(ctx, false)
			assert.Nil(t, err)
			assert.EqualValues(t, 1., progress)

			// Now check that we have all code_hosts in the DB that we would expect
			// to be there, one of each kind, and one for the deleted host:
			// For all of those, we expect that there will be no values stored
			// for either API or git requests.
			verifyCodeHostsExist(t, store, []codeHost{
				{
					Kind: "AWSCODECOMMIT",
					URL:  "us-east-1:ABCDEF",
					// Doesn't support limiting.
				},
				{
					Kind: "AZUREDEVOPS",
					URL:  "https://dev.azure.com/",
					// Doesn't support limiting.
				},
				{
					Kind:                        "BITBUCKETCLOUD",
					URL:                         "https://bitbucket.org/",
					APIRateLimitQuota:           pointers.Ptr(int32(1800)),
					APIRateLimitIntervalSeconds: pointers.Ptr(int32(3600)),
				},
				{
					Kind:                        "BITBUCKETSERVER",
					URL:                         "https://bitbucket.sgdev.org/",
					APIRateLimitQuota:           pointers.Ptr(int32(3600)),
					APIRateLimitIntervalSeconds: pointers.Ptr(int32(3600)),
				},
				{
					Kind: "GERRIT",
					URL:  "https://gerrit.sgdev.org/",
					// Doesn't support limiting.
				},
				// Our deleted code host.
				{
					Kind:                        "GITHUB",
					URL:                         "https://ghe.sgdev.org/",
					APIRateLimitQuota:           pointers.Ptr(int32(1000)),
					APIRateLimitIntervalSeconds: pointers.Ptr(int32(3600)),
				},
				{
					Kind:                        "GITHUB",
					URL:                         "https://github.com/",
					APIRateLimitQuota:           pointers.Ptr(int32(5000)),
					APIRateLimitIntervalSeconds: pointers.Ptr(int32(3600)),
				},
				{
					Kind:                        "GITLAB",
					URL:                         "https://gitlab.com/",
					APIRateLimitQuota:           pointers.Ptr(int32(6000)),
					APIRateLimitIntervalSeconds: pointers.Ptr(int32(3600)),
				},
				{
					Kind: "GITOLITE",
					URL:  "ssh://git@github.com/",
					// Doesn't support limiting.
				},
				{
					Kind:                        "GOMODULES",
					URL:                         "GOMODULES",
					APIRateLimitQuota:           pointers.Ptr(int32(10000)),
					APIRateLimitIntervalSeconds: pointers.Ptr(int32(3600)),
				},
				{
					Kind:                        "JVMPACKAGES",
					URL:                         "JVMPACKAGES",
					APIRateLimitQuota:           pointers.Ptr(int32(11500)),
					APIRateLimitIntervalSeconds: pointers.Ptr(int32(3600)),
				},
				{
					Kind: "LOCALGIT",
					URL:  "LOCALGIT",
					// Doesn't support limiting.
				},
				{
					Kind:                        "NPMPACKAGES",
					URL:                         "NPMPACKAGES",
					APIRateLimitQuota:           pointers.Ptr(int32(12000)),
					APIRateLimitIntervalSeconds: pointers.Ptr(int32(3600)),
				},
				{
					Kind: "OTHER",
					URL:  "https://user:pass@sgdev.org/repo.git/",
					// Doesn't support limiting.
				},
				{
					Kind:                        "PAGURE",
					URL:                         "https://pagure.sgdev.org/",
					APIRateLimitQuota:           pointers.Ptr(int32(13000)),
					APIRateLimitIntervalSeconds: pointers.Ptr(int32(3600)),
				},
				{
					Kind:                        "PERFORCE",
					URL:                         "ssl:111.222.333.444:1666",
					APIRateLimitQuota:           pointers.Ptr(int32(14000)),
					APIRateLimitIntervalSeconds: pointers.Ptr(int32(3600)),
				},
				{
					Kind: "PHABRICATOR",
					URL:  "https://phabricator.sgdev.org/",
					// Doesn't support limiting.
				},
				{
					Kind:                        "PYTHONPACKAGES",
					URL:                         "PYTHONPACKAGES",
					APIRateLimitQuota:           pointers.Ptr(int32(15000)),
					APIRateLimitIntervalSeconds: pointers.Ptr(int32(3600)),
				},
				{
					Kind:                        "RUBYPACKAGES",
					URL:                         "RUBYPACKAGES",
					APIRateLimitQuota:           pointers.Ptr(int32(16000)),
					APIRateLimitIntervalSeconds: pointers.Ptr(int32(3600)),
				},
				{
					Kind:                        "RUSTPACKAGES",
					URL:                         "RUSTPACKAGES",
					APIRateLimitQuota:           pointers.Ptr(int32(17000)),
					APIRateLimitIntervalSeconds: pointers.Ptr(int32(3600)),
				},
			})
		})
	})

	// Test that an existing code host from the new code paths does still make it
	// so other external services get migrated.
	t.Run("Existing code host", func(t *testing.T) {
		ensureCleanDB(t, ctx, store)

		// Create an external service that already has a code host.
		require.NoError(t, store.Exec(ctx, sqlf.Sprintf("INSERT INTO code_hosts (kind, url) VALUES('GITHUB', 'https://github.com/')")))
		require.NoError(t, store.Exec(ctx, sqlf.Sprintf(`INSERT INTO external_services (kind, display_name, config, code_host_id, created_at) VALUES ('GITHUB', 'GH', '{"url": "https://github.com/"}', (SELECT id FROM code_hosts WHERE url = 'https://github.com/'), NOW())`)))

		// Create an additional external service that has no code host set yet but
		// uses the same URL. Well, the same after normalization, we also leave
		// the trailing slash out here to see if these two are still matched up against
		// each other.
		require.NoError(t, store.Exec(ctx, sqlf.Sprintf(`INSERT INTO external_services (kind, display_name, config, created_at) VALUES ('GITHUB', 'GH 2', '{"url": "https://github.com"}', NOW())`)))

		// Check that we're only 50% done.
		progress, err := m.Progress(ctx, false)
		assert.Nil(t, err)
		assert.EqualValues(t, .5, progress)

		// Now, we need to run Up to get our other service migrated.
		assert.Nil(t, m.Up(ctx))

		// Check that we're actually done.
		progress, err = m.Progress(ctx, false)
		assert.Nil(t, err)
		assert.EqualValues(t, 1., progress)

		// Check that only one code host exists.
		verifyCodeHostsExist(t, store, []codeHost{
			{
				Kind: "GITHUB",
				URL:  "https://github.com/",
			},
		})

		// Verify all external services now have code hosts set.
		row := store.QueryRow(ctx, sqlf.Sprintf(`SELECT COUNT(*) FROM external_services WHERE code_host_id IS NULL`))
		var count int
		require.NoError(t, row.Scan(&count))
		require.NoError(t, row.Err())
		assert.Equal(t, 0, count)
	})
}

type codeHost struct {
	Kind                        string
	URL                         string
	APIRateLimitQuota           *int32
	APIRateLimitIntervalSeconds *int32
	GitRateLimitQuota           *int32
	GitRateLimitIntervalSeconds *int32
}

func verifyCodeHostsExist(t *testing.T, store *basestore.Store, expectedCodeHosts []codeHost) {
	t.Helper()

	// Sort the expected hosts for a stable comparison.
	sort.Slice(expectedCodeHosts, func(i, j int) bool {
		return expectedCodeHosts[i].Kind < expectedCodeHosts[j].Kind && expectedCodeHosts[i].URL < expectedCodeHosts[j].URL
	})

	rows, err := store.Query(context.Background(), sqlf.Sprintf("SELECT kind, url, api_rate_limit_quota, api_rate_limit_interval_seconds, git_rate_limit_quota, git_rate_limit_interval_seconds FROM code_hosts ORDER BY kind, url"))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var haveCodeHosts []codeHost
	for rows.Next() {
		var ch codeHost
		if err := rows.Scan(
			&ch.Kind,
			&ch.URL,
			&nullInt32{N: &ch.APIRateLimitQuota},
			&nullInt32{N: &ch.APIRateLimitIntervalSeconds},
			&nullInt32{N: &ch.GitRateLimitQuota},
			&nullInt32{N: &ch.GitRateLimitIntervalSeconds},
		); err != nil {
			t.Fatal(err)
		}
		haveCodeHosts = append(haveCodeHosts, ch)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(haveCodeHosts, expectedCodeHosts); diff != "" {
		t.Fatalf("invalid code host configuration in database: %s", diff)
	}
}

// clearCodeHosts resets the migration state effectively.
func clearCodeHosts(t *testing.T, ctx context.Context, store *basestore.Store) {
	t.Helper()

	if err := store.Exec(
		ctx,
		sqlf.Sprintf("UPDATE external_services SET code_host_id = NULL"),
	); err != nil {
		t.Fatal(err)
	}
	if err := store.Exec(
		ctx,
		sqlf.Sprintf("DELETE FROM code_hosts WHERE 1=1"),
	); err != nil {
		t.Fatal(err)
	}
}

func ensureCleanDB(t *testing.T, ctx context.Context, store *basestore.Store) {
	t.Helper()

	clean := func() {
		if err := store.Exec(
			ctx,
			sqlf.Sprintf("DELETE FROM external_services WHERE 1=1"),
		); err != nil {
			t.Fatal(err)
		}
		if err := store.Exec(
			ctx,
			sqlf.Sprintf("DELETE FROM code_hosts WHERE 1=1"),
		); err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(clean)
	clean()
}

func createExternalServices(t *testing.T, ctx context.Context, store *basestore.Store, extSvcs []testExtSvc, deletedWithRateLimitConfig bool) (created int) {
	t.Helper()

	// Create a trivial external service of each kind
	for i, svc := range extSvcs {
		buf, err := json.MarshalIndent(svc.cfg, "", "  ")
		if err != nil {
			t.Fatal(err)
		}

		if err := store.Exec(ctx, sqlf.Sprintf(`
			INSERT INTO external_services (kind, display_name, config, created_at)
			VALUES (%s, %s, %s, NOW())
		`,
			svc.kind,
			svc.kind+strconv.Itoa(i),
			string(buf),
		)); err != nil {
			t.Fatal(err)
		}
		created++
	}

	configForDeleted := `{"url":"https://ghe.sgdev.org"}`
	if deletedWithRateLimitConfig {
		configForDeleted = `{"url":"https://ghe.sgdev.org", "ratelimit": {"enabled": true, "requestsPerHour": 1000}}`
	}
	// We'll also add another external service that is deleted, to make sure that
	// one is also getting an entry.
	if err := store.Exec(
		ctx,
		sqlf.Sprintf(`
			INSERT INTO external_services (kind, display_name, config, deleted_at)
			VALUES (%s, %s, %s, NOW())
		`,
			"GITHUB",
			"deleted",
			configForDeleted,
		),
	); err != nil {
		t.Fatal(err)
	}
	created++

	return created
}
