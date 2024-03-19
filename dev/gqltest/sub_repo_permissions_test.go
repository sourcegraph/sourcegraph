package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/gqltestutil"
	"github.com/sourcegraph/sourcegraph/schema"
)

const (
	perforceRepoName = "perforce/test-perms"
	testPermsDepot   = "test-perms"
	aliceEmail       = "alice@perforce.sgdev.org"
	aliceUsername    = "alice"
)

func TestSubRepoPermissionsPerforce(t *testing.T) {
	checkPerforceEnvironment(t)
	enableSubRepoPermissions(t)
	cleanup := createPerforceExternalService(t, testPermsDepot, true)
	t.Cleanup(cleanup)
	userClient, repoName, err := createTestUserAndWaitForRepo(t)
	if err != nil {
		t.Fatalf("Failed to create user and wait for repo: %v", err)
	}

	// Test cases

	t.Run("can read README.md", func(t *testing.T) {
		blob, err := userClient.GitBlob(repoName, "main", "README.md")
		if err != nil {
			t.Fatal(err)
		}
		wantBlob := `This depot is used to test user and group permissions.
`
		if diff := cmp.Diff(wantBlob, blob); diff != "" {
			t.Fatalf("Blob mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("cannot read hack.sh", func(t *testing.T) {
		// Should not be able to read hack.sh
		blob, err := userClient.GitBlob(repoName, "main", "Security/hack.sh")
		if err != nil {
			t.Fatal(err)
		}

		// This is the desired behaviour at the moment, see where we check for
		// os.IsNotExist error in GitCommitResolver.Blob
		wantBlob := ``

		if diff := cmp.Diff(wantBlob, blob); diff != "" {
			t.Fatalf("Blob mismatch (-want +got):\n%s", diff)
		}
	})

	// flaky test
	t.Run("file list excludes excluded files", func(t *testing.T) {
		files, err := userClient.GitListFilenames(repoName, "main")
		if err != nil {
			t.Fatal(err)
		}

		// Notice that Security/hack.sh is excluded
		wantFiles := []string{
			"Backend/main.go",
			"Frontend/app.ts",
			"README.md",
		}

		if diff := cmp.Diff(wantFiles, files); diff != "" {
			t.Fatalf("fileNames mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestSubRepoPermissionsSymbols(t *testing.T) {
	checkPerforceEnvironment(t)
	enableSubRepoPermissions(t)
	cleanup := createPerforceExternalService(t, testPermsDepot, true)
	t.Cleanup(cleanup)
	userClient, repoName, err := createTestUserAndWaitForRepo(t)
	if err != nil {
		t.Fatalf("Failed to create user and wait for repo: %v", err)
	}

	err = client.WaitForRepoToBeIndexed(perforceRepoName)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("can read main.go and app.ts, but not hack.sh symbols", func(t *testing.T) {
		// Symbols are lazily indexed, that's why we need an initial request to search
		// for the revision, after which symbols of this revision are indexed. The search
		// is repeated 10 times and the test runs for ~50 seconds in total to increase
		// the probability of symbols being indexed.
		for range 10 {
			symbols, err := userClient.GitGetCommitSymbols(repoName, "main")
			if err != nil {
				t.Fatal(err)
			}
			// Should not be able to read hack.sh
			for _, symbol := range symbols {
				fileName := symbol.Location.Resource.Path
				if fileName == "Security/hack.sh" {
					t.Fatal("Shouldn't be able to read symbols of hack.sh")
				}
			}
			time.Sleep(5 * time.Second)
		}
	})
}

func TestSubRepoPermissionsSearch(t *testing.T) {
	checkPerforceEnvironment(t)
	enableSubRepoPermissions(t)
	enableStructuralSearch(t)
	cleanup := createPerforceExternalService(t, testPermsDepot, true)
	t.Cleanup(cleanup)
	userClient, _, err := createTestUserAndWaitForRepo(t)
	if err != nil {
		t.Fatalf("Failed to create user and wait for repo: %v", err)
	}

	t.Log("Wait for repo to be indexed")
	err = client.WaitForRepoToBeIndexed(perforceRepoName)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		query         string
		zeroResult    bool
		minMatchCount int64
	}{
		{
			name:          "indexed search, nonzero result",
			query:         `index:only This depot is used to test`,
			minMatchCount: 1,
		},
		{
			name:          "unindexed multiline search, nonzero result",
			query:         `index:no This depot is used to test`,
			minMatchCount: 1,
		},
		{
			name:       "indexed search of restricted content",
			query:      `index:only uploading your secrets`,
			zeroResult: true,
		},
		{
			name:       "unindexed search of restricted content",
			query:      `index:no uploading your secrets`,
			zeroResult: true,
		},
		{
			name:       "structural, indexed search of restricted content",
			query:      `repo:^perforce/test-perms$ echo "..." index:only patterntype:structural`,
			zeroResult: true,
		},
		{
			name:       "structural, unindexed search of restricted content",
			query:      `repo:^perforce/test-perms$ echo "..." index:no patterntype:structural`,
			zeroResult: true,
		},
		{
			name:          "structural, indexed search, nonzero result",
			query:         `println(...) index:only patterntype:structural`,
			minMatchCount: 1,
		},
		{
			name:          "structural, unindexed search, nonzero result",
			query:         `println(...) index:no patterntype:structural`,
			minMatchCount: 1,
		},
		{
			name:          "filename search, nonzero result",
			query:         `repo:^perforce/test-perms$ type:path app`,
			minMatchCount: 1,
		},
		{
			name:       "filename search of restricted content",
			query:      `repo:^perforce/test-perms$ type:path hack`,
			zeroResult: true,
		},
		{
			name:          "content search, nonzero result",
			query:         `repo:^perforce/test-perms$ type:file let`,
			minMatchCount: 1,
		},
		{
			name:       "content search of restricted content",
			query:      `repo:^perforce/test-perms$ type:file echo`,
			zeroResult: true,
		},
		{
			name:          "diff search, nonzero result",
			query:         `repo:^perforce/test-perms$ type:diff let`,
			minMatchCount: 1,
		},
		{
			name:       "diff search of restricted content",
			query:      `repo:^perforce/test-perms$ type:diff echo`,
			zeroResult: true,
		},
		{
			name:          "symbol search, nonzero result",
			query:         `repo:^perforce/test-perms$ type:symbol main`,
			minMatchCount: 1,
		},
		{
			name:       "symbol search of restricted content",
			query:      `repo:^perforce/test-perms$ type:symbol asdf`,
			zeroResult: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			results, err := userClient.SearchFiles(test.query)
			if err != nil {
				t.Fatal(err)
			}

			if test.zeroResult {
				if len(results.Results) > 0 {
					t.Fatalf("Want zero result but got %d", len(results.Results))
				}
			} else {
				if len(results.Results) == 0 {
					t.Fatal("Want non-zero results but got 0")
				}
			}

			if results.MatchCount < test.minMatchCount {
				t.Fatalf("Want at least %d match count but got %d", test.minMatchCount, results.MatchCount)
			}
		})
	}

	t.Run("commit search admin", func(t *testing.T) {
		results, err := client.SearchCommits(`repo:^perforce/test-perms$ type:commit`)
		if err != nil {
			t.Fatal(err)
		}
		// Admin should have access to ALL commits: there are 6 total
		commitsNumber := len(results.Results)
		expectedCommitsNumber := 6
		if commitsNumber != expectedCommitsNumber {
			t.Fatalf("Should have access to %d commits but got %d", expectedCommitsNumber, commitsNumber)
		}
	})

	t.Run("commit search", func(t *testing.T) {
		results, err := userClient.SearchCommits(`repo:^perforce/test-perms$ type:commit`)
		if err != nil {
			t.Fatal(err)
		}
		// Alice should have access to only 3 commits at the moment
		commitsNumber := len(results.Results)
		expectedCommitsNumber := 3
		if commitsNumber != expectedCommitsNumber {
			t.Fatalf("Should have access to %d commits but got %d", expectedCommitsNumber, commitsNumber)
		}
	})

	commitAccessTests := []struct {
		name      string
		revision  string
		hasAccess bool
	}{
		{
			name:     "direct access to inaccessible commit",
			revision: "ce09ed9ea31ffa71c8de2d6029b93d4bd9568af8",
		},
		{
			name:      "direct access to accessible commit",
			revision:  "de3fac9e20a01dbda7f354c82bd6ed77f16a778c",
			hasAccess: true,
		},
	}

	for _, test := range commitAccessTests {
		t.Run(test.name, func(t *testing.T) {
			t.Skip("Skipping until we have stable commit hashes")
			_, err := userClient.GitGetCommitMessage(perforceRepoName, test.revision)
			if err != nil {
				if test.hasAccess {
					t.Fatal(err)
				}
			} else {
				if !test.hasAccess {
					t.Fatal("No error during accessing restricted commit")
				}
			}
		})
	}

	t.Run("archive repo", func(t *testing.T) {
		url := fmt.Sprintf("%s/%s/-/raw/", *baseURL, perforceRepoName)
		response, err := userClient.GetWithHeaders(url, map[string][]string{"Accept": {"application/zip"}})
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode == http.StatusOK {
			t.Fatalf("Should not be able to get an archive of repo with enabled sub-repo perms")
		}
	})

	t.Run("code intel search", func(t *testing.T) {
		t.Skip("Skipping until we have stable commit hashes")
		result, err := userClient.SearchFiles("context:global \\bhack1337\\b type:file patternType:regexp count:500 case:yes file:\\.(go)$ repo:^perforce/test-perms$@2a30922ef9f214d44de60e719a64473e17321684")
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range result.Results {
			if strings.HasPrefix(file.File.Name, "hack") {
				t.Fatal("Should not find references for restricted files")
			}
		}
	})
}

func createTestUserAndWaitForRepo(t *testing.T) (*gqltestutil.Client, string, error) {
	t.Helper()

	// We need to create the `alice` user with a specific e-mail address. This user is
	// configured on our dogfood perforce instance with limited access to the
	// test-perms depot.
	// Alice has access to root, Backend and Frontend directories. (there are .md, .ts and .go files)
	// Alice doesn't have access to Security directory. (there is a .sh file)
	alicePassword := "alicessupersecurepassword"
	t.Log("Creating Alice")
	userClient, err := gqltestutil.SignUpOrSignIn(*baseURL, aliceEmail, aliceUsername, alicePassword)
	if err != nil {
		t.Fatal(err)
	}

	aliceID := userClient.AuthenticatedUserID()
	removeTestUserAfterTest(t, aliceID)

	if err := client.SetUserEmailVerified(aliceID, aliceEmail, true); err != nil {
		t.Fatal(err)
	}

	t.Log("Wait for repo to be cloned")
	err = userClient.WaitForReposToBeClonedWithin(120*time.Second, perforceRepoName)
	if err != nil {
		return nil, "", err
	}

	syncUserPerms(t, aliceID, aliceUsername)
	return userClient, perforceRepoName, nil
}

func syncUserPerms(t *testing.T, userID, userName string) {
	t.Helper()

	t.Log("Wait for Perforce to be added as an authz provider")
	// Wait up to 30 seconds for Perforce to be added as an authz provider
	err := gqltestutil.Retry(30*time.Second, func() error {
		authzProviders, err := client.AuthzProviderTypes()
		if err != nil {
			t.Fatal("failed to fetch list of authz providers", err)
		}
		if len(authzProviders) != 0 {
			for _, p := range authzProviders {
				if p == "perforce" {
					return nil
				}
			}
		}
		return gqltestutil.ErrContinueRetry
	})
	if err != nil {
		t.Fatal("Waiting for authz providers to be added:", err)
	}

	t.Log("Schedule permissions sync")
	err = client.ScheduleUserPermissionsSync(userID)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Wait for permissions to sync")
	// Wait up to 30 seconds for the user to have permissions synced
	// from the code host at least once.
	err = gqltestutil.Retry(30*time.Second, func() error {
		userPermsInfo, err := client.UserPermissionsInfo(userName)
		if err != nil {
			t.Fatal(err)
		}
		if userPermsInfo != nil && !userPermsInfo.UpdatedAt.IsZero() {
			return nil
		}
		return gqltestutil.ErrContinueRetry
	})
	// Try a second time if the first attempt failed.
	if err != nil {
		t.Log("First permissions sync failed. Trying a second time")
		err = client.ScheduleUserPermissionsSync(userID)
		if err != nil {
			t.Fatal(err)
		}

		// Wait up to 30 seconds for the user to have permissions synced
		// from the code host at least once.
		err = gqltestutil.Retry(30*time.Second, func() error {
			userPermsInfo, err := client.UserPermissionsInfo(userName)
			if err != nil {
				t.Fatal(err)
			}
			if userPermsInfo != nil && !userPermsInfo.UpdatedAt.IsZero() {
				return nil
			}
			return gqltestutil.ErrContinueRetry
		})
		if err != nil {
			t.Fatal("Waiting for user permissions to be synced:", err)
		}
	}
}

func enableSubRepoPermissions(t *testing.T) {
	t.Helper()
	t.Log("Enabling sub-repo permissions")

	siteConfig, lastID, err := client.SiteConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	oldSiteConfig := new(schema.SiteConfiguration)
	*oldSiteConfig = *siteConfig
	t.Cleanup(func() {
		_, lastID, err := client.SiteConfiguration()
		if err != nil {
			t.Fatal(err)
		}
		err = client.UpdateSiteConfiguration(oldSiteConfig, lastID)
		if err != nil {
			t.Fatal(err)
		}
	})

	siteConfig.ExperimentalFeatures = &schema.ExperimentalFeatures{
		Perforce: "enabled",
		SubRepoPermissions: &schema.SubRepoPermissions{
			Enabled: true,
		},
	}
	err = client.UpdateSiteConfiguration(siteConfig, lastID)
	if err != nil {
		t.Fatal(err)
	}
}

func enableStructuralSearch(t *testing.T) {
	t.Helper()
	t.Log("Enabling structural search")

	// Enable structural search.
	siteConfig, lastID, err := client.SiteConfiguration()
	if err != nil {
		t.Fatal(err)
	}

	oldSiteConfig := new(schema.SiteConfiguration)
	*oldSiteConfig = *siteConfig
	t.Cleanup(func() {
		_, lastID, err := client.SiteConfiguration()
		if err != nil {
			t.Fatal(err)
		}
		err = client.UpdateSiteConfiguration(oldSiteConfig, lastID)
		if err != nil {
			t.Fatal(err)
		}
	})

	if siteConfig.ExperimentalFeatures == nil {
		siteConfig.ExperimentalFeatures = &schema.ExperimentalFeatures{}
	}
	siteConfig.ExperimentalFeatures.StructuralSearch = "enabled"
	err = client.UpdateSiteConfiguration(siteConfig, lastID)
	if err != nil {
		t.Fatal(err)
	}
}
