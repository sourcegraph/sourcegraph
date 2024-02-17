package sources

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/testutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestGithubSource_CreateChangeset(t *testing.T) {
	// Repository used: https://github.com/sourcegraph/automation-testing
	//
	// The requests here cannot be easily rerun with `-update` since you can only open a
	// pull request once. To update, push a new branch with at least one commit to
	// automation-testing, and put the branch names into the `success` case below.
	//
	// You can update just this test with `-update GithubSource_CreateChangeset`.
	repo := &types.Repo{
		Metadata: &github.Repository{
			ID:            "MDEwOlJlcG9zaXRvcnkyMjExNDc1MTM=",
			NameWithOwner: "sourcegraph/automation-testing",
		},
	}

	testCases := []struct {
		name   string
		cs     *Changeset
		err    string
		exists bool
	}{
		{
			name: "success",
			cs: &Changeset{
				Title:      "This is a test PR",
				Body:       "This is the description of the test PR",
				HeadRef:    "refs/heads/test-review-decision",
				BaseRef:    "refs/heads/master",
				RemoteRepo: repo,
				TargetRepo: repo,
				Changeset:  &btypes.Changeset{},
			},
			err: "<nil>",
		},
		{
			name: "already exists",
			cs: &Changeset{
				Title:      "This is a test PR",
				Body:       "This is the description of the test PR",
				HeadRef:    "refs/heads/always-open-pr",
				BaseRef:    "refs/heads/master",
				RemoteRepo: repo,
				TargetRepo: repo,
				Changeset:  &btypes.Changeset{},
			},
			// If PR already exists we'll just return it, no error
			err:    "<nil>",
			exists: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		tc.name = "GithubSource_CreateChangeset_" + strings.ReplaceAll(tc.name, " ", "_")

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			src, save := setup(t, ctx, tc.name)
			defer save(t)

			exists, err := src.CreateChangeset(ctx, tc.cs)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			if have, want := exists, tc.exists; have != want {
				t.Errorf("exists:\nhave: %t\nwant: %t", have, want)
			}

			pr, ok := tc.cs.Changeset.Metadata.(*github.PullRequest)
			if !ok {
				t.Fatal("Metadata does not contain PR")
			}

			testutil.AssertGolden(t, "testdata/golden/"+tc.name, update(tc.name), pr)
		})
	}
}

func TestGithubSource_CreateChangeset_CreationLimit(t *testing.T) {
	rt := new(mockTransport)
	// Version lookup
	versionMatchedBy := func(req *http.Request) bool {
		return req.Method == http.MethodGet && req.URL.Path == "/"
	}
	rt.On("RoundTrip", mock.MatchedBy(versionMatchedBy)).
		Once().
		Return(
			&http.Response{
				StatusCode: http.StatusOK,
				Header: map[string][]string{
					"X-GitHub-Enterprise-Version": {"99"},
				},
				Body: io.NopCloser(bytes.NewReader([]byte{})),
			},
			nil,
		)
	// Create Changeset mutation
	createChangesetMatchedBy := func(req *http.Request) bool {
		return req.Method == http.MethodPost && req.URL.Path == "/graphql"
	}
	rt.On("RoundTrip", mock.MatchedBy(createChangesetMatchedBy)).
		Once().
		Return(
			&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"errors": [{"message": "error in GraphQL response: was submitted too quickly"}]}`))),
			},
			nil,
		)

	apiURL, err := url.Parse("https://fake.api.github.com")
	require.NoError(t, err)
	client, err := github.NewV4Client("extsvc:github:0", apiURL, nil, httpcli.NewFactory(nil, func(c *http.Client) error {
		c.Transport = httpcli.WrapTransport(rt, http.DefaultTransport)
		return nil
	}))
	require.NoError(t, err)
	source := &GitHubSource{
		client: client,
	}

	repo := &types.Repo{
		Metadata: &github.Repository{
			ID:            "bLAhBLAh",
			NameWithOwner: "some-org/some-repo",
		},
	}
	cs := &Changeset{
		Title:      "This is a test PR",
		Body:       "This is the description of the test PR",
		HeadRef:    "refs/heads/always-open-pr",
		BaseRef:    "refs/heads/master",
		RemoteRepo: repo,
		TargetRepo: repo,
		Changeset:  &btypes.Changeset{},
	}

	exists, err := source.CreateChangeset(context.Background(), cs)
	assert.False(t, exists)
	assert.Error(t, err)
	assert.Equal(
		t,
		"reached GitHub's internal creation limit: see https://sourcegraph.com/docs/admin/config/batch_changes#avoiding-hitting-rate-limits: error in GraphQL response: error in GraphQL response: was submitted too quickly",
		err.Error(),
	)
}

type mockTransport struct {
	mock.Mock
}

func (d *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	args := d.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestGithubSource_CloseChangeset(t *testing.T) {
	// Repository used: https://github.com/sourcegraph/automation-testing
	//
	// This test can be updated with `-update GithubSource_CloseChangeset`, provided this
	// PR is open: https://github.com/sourcegraph/automation-testing/pull/468
	testCases := []struct {
		name string
		cs   *Changeset
		err  string
	}{
		{
			name: "success",
			cs: &Changeset{
				Changeset: &btypes.Changeset{
					Metadata: &github.PullRequest{
						ID: "PR_kwDODS5xec4waMkR",
					},
				},
			},
			err: "<nil>",
		},
	}

	for _, tc := range testCases {
		tc := tc
		tc.name = "GithubSource_CloseChangeset_" + strings.ReplaceAll(tc.name, " ", "_")

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			src, save := setup(t, ctx, tc.name)
			defer save(t)

			err := src.CloseChangeset(ctx, tc.cs)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			pr := tc.cs.Changeset.Metadata.(*github.PullRequest)
			testutil.AssertGolden(t, "testdata/golden/"+tc.name, update(tc.name), pr)
		})
	}
}

func TestGithubSource_CloseChangeset_DeleteSourceBranch(t *testing.T) {
	// Repository used: https://github.com/sourcegraph/automation-testing
	//
	// This test can be updated with `-update GithubSource_CloseChangeset_DeleteSourceBranch`,
	// provided this PR is open: https://github.com/sourcegraph/automation-testing/pull/468
	repo := &types.Repo{
		Metadata: &github.Repository{
			ID:            "MDEwOlJlcG9zaXRvcnkyMjExNDc1MTM=",
			NameWithOwner: "sourcegraph/automation-testing",
		},
	}

	testCases := []struct {
		name string
		cs   *Changeset
		err  string
	}{
		{
			name: "success",
			cs: &Changeset{
				Changeset: &btypes.Changeset{
					Metadata: &github.PullRequest{
						ID:          "PR_kwDODS5xec5TsclN",
						HeadRefName: "refs/heads/test-review-decision",
					},
				},
				RemoteRepo: repo,
				TargetRepo: repo,
			},
			err: "<nil>",
		},
	}

	for _, tc := range testCases {
		tc := tc
		tc.name = "GithubSource_CloseChangeset_DeleteSourceBranch_" + strings.ReplaceAll(tc.name, " ", "_")

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			src, save := setup(t, ctx, tc.name)
			defer save(t)

			conf.Mock(&conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					BatchChangesAutoDeleteBranch: true,
				},
			})
			defer conf.Mock(nil)

			err := src.CloseChangeset(ctx, tc.cs)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			pr := tc.cs.Changeset.Metadata.(*github.PullRequest)
			testutil.AssertGolden(t, "testdata/golden/"+tc.name, update(tc.name), pr)
		})
	}
}

func TestGithubSource_ReopenChangeset(t *testing.T) {
	// Repository used: https://github.com/sourcegraph/automation-testing
	//
	// This test can be updated with `-update GithubSource_ReopenChangeset`, provided this
	// PR is closed but _not_ merged: https://github.com/sourcegraph/automation-testing/pull/468
	testCases := []struct {
		name string
		cs   *Changeset
		err  string
	}{
		{
			name: "success",
			cs: &Changeset{
				Changeset: &btypes.Changeset{
					Metadata: &github.PullRequest{
						// https://github.com/sourcegraph/automation-testing/pull/353
						ID: "MDExOlB1bGxSZXF1ZXN0NDg4MDI2OTk5",
					},
				},
			},
			err: "<nil>",
		},
	}

	for _, tc := range testCases {
		tc := tc
		tc.name = "GithubSource_ReopenChangeset_" + strings.ReplaceAll(tc.name, " ", "_")

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			src, save := setup(t, ctx, tc.name)
			defer save(t)

			err := src.ReopenChangeset(ctx, tc.cs)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			pr := tc.cs.Changeset.Metadata.(*github.PullRequest)
			testutil.AssertGolden(t, "testdata/golden/"+tc.name, update(tc.name), pr)
		})
	}
}

func TestGithubSource_CreateComment(t *testing.T) {
	testCases := []struct {
		name string
		cs   *Changeset
		err  string
	}{
		{
			name: "success",
			cs: &Changeset{
				Changeset: &btypes.Changeset{
					Metadata: &github.PullRequest{
						ID: "MDExOlB1bGxSZXF1ZXN0MzQ5NTIzMzE0",
					},
				},
			},
			err: "<nil>",
		},
	}

	for _, tc := range testCases {
		tc := tc
		tc.name = "GithubSource_CreateComment_" + strings.ReplaceAll(tc.name, " ", "_")

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			src, save := setup(t, ctx, tc.name)
			defer save(t)

			err := src.CreateComment(ctx, tc.cs, "test-comment")
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}
		})
	}
}

func TestGithubSource_UpdateChangeset(t *testing.T) {
	// Repository used: https://github.com/sourcegraph/automation-testing
	//
	// This test can be updated with `-update GithubSource_UpdateChangeset`, provided this
	// PR is open: https://github.com/sourcegraph/automation-testing/pull/1
	testCases := []struct {
		name string
		cs   *Changeset
		err  string
	}{
		{
			name: "success",
			cs: &Changeset{
				Title:   "This is a test PR that is always open (keep it open!)",
				Body:    "Feel free to ignore this. This is a test PR that is always open and is sometimes updated.",
				BaseRef: "refs/heads/master",
				Changeset: &btypes.Changeset{
					Metadata: &github.PullRequest{
						ID: "MDExOlB1bGxSZXF1ZXN0MzM5NzUyNDQy",
					},
				},
			},
			err: "<nil>",
		},
	}

	for _, tc := range testCases {
		tc := tc
		tc.name = "GithubSource_UpdateChangeset_" + strings.ReplaceAll(tc.name, " ", "_")

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			src, save := setup(t, ctx, tc.name)
			defer save(t)

			err := src.UpdateChangeset(ctx, tc.cs)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			pr := tc.cs.Changeset.Metadata.(*github.PullRequest)
			testutil.AssertGolden(t, "testdata/golden/"+tc.name, update(tc.name), pr)
		})
	}
}

func TestGithubSource_LoadChangeset(t *testing.T) {
	testCases := []struct {
		name string
		cs   *Changeset
		err  string
	}{
		{
			name: "found",
			cs: &Changeset{
				RemoteRepo: &types.Repo{Metadata: &github.Repository{NameWithOwner: "sourcegraph/sourcegraph"}},
				TargetRepo: &types.Repo{Metadata: &github.Repository{NameWithOwner: "sourcegraph/sourcegraph"}},
				Changeset:  &btypes.Changeset{ExternalID: "5550"},
			},
			err: "<nil>",
		},
		{
			name: "not-found",
			cs: &Changeset{
				RemoteRepo: &types.Repo{Metadata: &github.Repository{NameWithOwner: "sourcegraph/sourcegraph"}},
				TargetRepo: &types.Repo{Metadata: &github.Repository{NameWithOwner: "sourcegraph/sourcegraph"}},
				Changeset:  &btypes.Changeset{ExternalID: "100000"},
			},
			err: "Changeset with external ID 100000 not found",
		},
	}

	for _, tc := range testCases {
		tc := tc
		tc.name = "GithubSource_LoadChangeset_" + tc.name

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			src, save := setup(t, ctx, tc.name)
			defer save(t)

			err := src.LoadChangeset(ctx, tc.cs)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			meta := tc.cs.Changeset.Metadata.(*github.PullRequest)
			testutil.AssertGolden(t, "testdata/golden/"+tc.name, update(tc.name), meta)
		})
	}
}

func TestGithubSource_WithAuthenticator(t *testing.T) {
	svc := &types.ExternalService{
		Kind: extsvc.KindGitHub,
		Config: extsvc.NewUnencryptedConfig(marshalJSON(t, &schema.GitHubConnection{
			Url:   "https://github.com",
			Token: os.Getenv("GITHUB_TOKEN"),
		})),
	}

	ctx := context.Background()
	githubSrc, err := NewGitHubSource(ctx, dbmocks.NewMockDB(), svc, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("supported", func(t *testing.T) {
		src, err := githubSrc.WithAuthenticator(&auth.OAuthBearerToken{})
		if err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}

		if gs, ok := src.(*GitHubSource); !ok {
			t.Error("cannot coerce Source into GithubSource")
		} else if gs == nil {
			t.Error("unexpected nil Source")
		}
	})
}

func TestGithubSource_GetFork(t *testing.T) {
	ctx := context.Background()
	urn := extsvc.URN(extsvc.KindGitHub, 1)

	t.Run("vcr tests", func(t *testing.T) {
		newGitHubRepo := func(urn, nameWithOwner, id string) *types.Repo {
			return &types.Repo{
				Metadata: &github.Repository{
					ID:            id,
					NameWithOwner: nameWithOwner,
				},
				Sources: map[string]*types.SourceInfo{
					urn: {
						ID:       urn,
						CloneURL: "https://github.com/" + nameWithOwner,
					},
				},
			}
		}

		type TCRepo struct{ name, namespace string }

		failTestCases := []struct {
			name   string
			target TCRepo
			fork   TCRepo
			err    string
		}{
			// This test expects that:
			// - The repo sourcegraph-testing/vcr-fork-test-repo exists and is not a fork.
			// - The repo sourcegraph-vcr/vcr-fork-test-repo exists and is not a fork.
			// Use credentials in 1password for "sourcegraph-vcr" to access or update this test.
			{
				name:   "not a fork",
				target: TCRepo{name: "vcr-fork-test-repo", namespace: "sourcegraph-testing"},
				fork:   TCRepo{name: "vcr-fork-test-repo", namespace: "sourcegraph-vcr"},
				err:    "repo is not a fork",
			},
		}

		for _, tc := range failTestCases {
			tc := tc
			tc.name = "GithubSource_GetFork_" + strings.ReplaceAll(tc.name, " ", "_")
			t.Run(tc.name, func(t *testing.T) {
				src, save := setup(t, ctx, tc.name)
				defer save(t)
				target := newGitHubRepo(urn, tc.target.namespace+"/"+tc.target.name, "123")

				fork, err := src.GetFork(ctx, target, pointers.Ptr(tc.fork.namespace), pointers.Ptr(tc.fork.name))

				assert.Nil(t, fork)
				assert.ErrorContains(t, err, tc.err)
			})
		}

		successTestCases := []struct {
			name string
			// True if changeset is already created on code host.
			externalNameAndNamespace bool
			target                   TCRepo
			fork                     TCRepo
		}{
			// This test validates the behavior when `GetFork` is called without a
			// namespace or name set, but a fork of the repo already exists in the user's
			// namespace with the default fork name. `GetFork` should return the existing
			// fork.
			//
			// This test expects that:
			// - The repo sourcegraph-testing/vcr-fork-test-repo exists and is not a fork.
			// - The repo sourcegraph-vcr/sourcegraph-testing-vcr-fork-test-repo-already-forked
			//   exists and is a fork of it.
			// - The current user is sourcegraph-vcr and the default fork naming convention
			//   would produce the fork name "sourcegraph-testing-vcr-fork-test-repo-already-forked".
			// Use credentials in 1password for "sourcegraph-vcr" to access or update this test.
			{
				name:                     "success with new changeset and existing fork",
				externalNameAndNamespace: false,
				target:                   TCRepo{name: "vcr-fork-test-repo-already-forked", namespace: "sourcegraph-testing"},
				fork:                     TCRepo{name: "sourcegraph-testing-vcr-fork-test-repo-already-forked", namespace: "sourcegraph-vcr"},
			},

			// This test validates the behavior when `GetFork` is called without a
			// namespace or name set and no fork of the repo exists in the user's
			// namespace with the default fork name. `GetFork` should return the
			// newly-created fork.
			//
			// This test expects that:
			// - The repo sourcegraph-testing/vcr-fork-test-repo-not-forked exists and
			//   is not a fork.
			// - The repo sourcegraph-vcr/sourcegraph-testing-vcr-fork-test-repo-not-forked
			//   does not exist.
			// - The current user is sourcegraph-vcr and the default fork naming convention
			//   would produce the fork name "sourcegraph-testing-vcr-fork-test-repo-not-forked".
			// Use credentials in 1password for "sourcegraph-vcr" to access or update this test.
			//
			// NOTE: It is not possible to update this test and "success with existing
			// changeset and new fork" at the same time.
			{
				name:                     "success with new changeset and new fork",
				externalNameAndNamespace: false,
				target:                   TCRepo{name: "vcr-fork-test-repo-not-forked", namespace: "sourcegraph-testing"},
				fork:                     TCRepo{name: "sourcegraph-testing-vcr-fork-test-repo-not-forked", namespace: "sourcegraph-vcr"},
			},

			// This test validates the behavior when `GetFork` is called with a namespace
			// and name both already set, and a fork of the repo already exists at that
			// destination. `GetFork` should return the existing fork.
			//
			// This test expects that:
			// - The repo sourcegraph-testing/vcr-fork-test-repo exists and is not a fork.
			// - The repo sourcegraph-vcr/sourcegraph-testing-vcr-fork-test-repo-already-forked
			//   exists and is a fork of it.
			// Use credentials in 1password for "sourcegraph-vcr" to access or update this test.
			{
				name:                     "success with existing changeset and existing fork",
				externalNameAndNamespace: true,
				target:                   TCRepo{name: "vcr-fork-test-repo-already-forked", namespace: "sourcegraph-testing"},
				fork:                     TCRepo{name: "sourcegraph-testing-vcr-fork-test-repo-already-forked", namespace: "sourcegraph-vcr"},
			},

			// This test validates the behavior when `GetFork` is called with a namespace
			// and name both already set, but no fork of the repo already exists at that
			// destination. This situation is only possible if the changeset and fork repo
			// have been deleted on the code host since the changeset was created.
			// `GetFork` should return the newly-created fork.
			//
			// This test expects that:
			// - The repo sourcegraph-testing/vcr-fork-test-repo-not-forked exists and
			//   is not a fork.
			// - The repo sgtest/sourcegraph-testing-vcr-fork-test-repo-not-forked
			//   does not exist.
			// Use credentials in 1password for "sourcegraph-vcr" to access or update this test.
			//
			// NOTE: It is not possible to update this test and "success with existing
			// changeset and new fork" at the same time.
			{
				name:                     "success with existing changeset and new fork",
				externalNameAndNamespace: true,
				target:                   TCRepo{name: "vcr-fork-test-repo-not-forked", namespace: "sourcegraph-testing"},
				fork:                     TCRepo{name: "sourcegraph-testing-vcr-fork-test-repo-not-forked", namespace: "sgtest"},
			},
		}

		for _, tc := range successTestCases {
			tc := tc
			tc.name = "GithubSource_GetFork_" + strings.ReplaceAll(tc.name, " ", "_")
			t.Run(tc.name, func(t *testing.T) {
				src, save := setup(t, ctx, tc.name)
				defer save(t)
				target := newGitHubRepo(urn, tc.target.namespace+"/"+tc.target.name, "123")

				var fork *types.Repo
				var err error
				if tc.externalNameAndNamespace {
					fork, err = src.GetFork(ctx, target, pointers.Ptr(tc.fork.namespace), pointers.Ptr(tc.fork.name))
				} else {
					fork, err = src.GetFork(ctx, target, nil, nil)
				}

				assert.Nil(t, err)
				assert.NotNil(t, fork)
				assert.NotEqual(t, fork, target)
				assert.Equal(t, tc.fork.namespace+"/"+tc.fork.name, fork.Metadata.(*github.Repository).NameWithOwner)
				assert.Equal(t, fork.Sources[urn].CloneURL, "https://github.com/"+tc.fork.namespace+"/"+tc.fork.name)

				testutil.AssertGolden(t, "testdata/golden/"+tc.name, update(tc.name), fork)
			})
		}
	})

	t.Run("failures", func(t *testing.T) {
		for name, tc := range map[string]struct {
			targetRepo *types.Repo
			client     githubClientFork
		}{
			"invalid NameWithOwner": {
				targetRepo: &types.Repo{
					Metadata: &github.Repository{
						NameWithOwner: "foo",
					},
				},
				client: nil,
			},
			"client error": {
				targetRepo: &types.Repo{
					Metadata: &github.Repository{
						NameWithOwner: "foo/bar",
					},
				},
				client: &mockGithubClientFork{err: errors.New("hello!")},
			},
		} {
			t.Run(name, func(t *testing.T) {
				fork, err := getGitHubForkInternal(ctx, tc.targetRepo, tc.client, nil, nil)
				assert.Nil(t, fork)
				assert.NotNil(t, err)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		org := "org"
		user := "user"
		urn := extsvc.URN(extsvc.KindGitHub, 1)

		for name, tc := range map[string]struct {
			targetRepo    *types.Repo
			forkRepo      *github.Repository
			namespace     *string
			wantNamespace string
			name          *string
			wantName      string
			client        githubClientFork
		}{
			"no namespace": {
				targetRepo: &types.Repo{
					Metadata: &github.Repository{
						NameWithOwner: "foo/bar",
					},
					Sources: map[string]*types.SourceInfo{
						urn: {
							ID:       urn,
							CloneURL: "https://github.com/foo/bar",
						},
					},
				},
				forkRepo:      &github.Repository{NameWithOwner: user + "/user-bar", IsFork: true},
				namespace:     nil,
				wantNamespace: user,
				wantName:      user + "-bar",
				client:        &mockGithubClientFork{fork: &github.Repository{NameWithOwner: user + "/user-bar", IsFork: true}},
			},
			"with namespace": {
				targetRepo: &types.Repo{
					Metadata: &github.Repository{
						NameWithOwner: "foo/bar",
					},
					Sources: map[string]*types.SourceInfo{
						urn: {
							ID:       urn,
							CloneURL: "https://github.com/foo/bar",
						},
					},
				},
				forkRepo:      &github.Repository{NameWithOwner: org + "/" + org + "-bar", IsFork: true},
				namespace:     &org,
				wantNamespace: org,
				wantName:      org + "-bar",
				client: &mockGithubClientFork{
					fork:    &github.Repository{NameWithOwner: org + "/" + org + "-bar", IsFork: true},
					wantOrg: &org,
				},
			},
			"with namespace and name": {
				targetRepo: &types.Repo{
					Metadata: &github.Repository{
						NameWithOwner: "foo/bar",
					},
					Sources: map[string]*types.SourceInfo{
						urn: {
							ID:       urn,
							CloneURL: "https://github.com/foo/bar",
						},
					},
				},
				forkRepo:      &github.Repository{NameWithOwner: org + "/custom-bar", IsFork: true},
				namespace:     &org,
				wantNamespace: org,
				name:          pointers.Ptr("custom-bar"),
				wantName:      "custom-bar",
				client: &mockGithubClientFork{
					fork:    &github.Repository{NameWithOwner: org + "/custom-bar", IsFork: true},
					wantOrg: &org,
				},
			},
		} {
			t.Run(name, func(t *testing.T) {
				fork, err := getGitHubForkInternal(ctx, tc.targetRepo, tc.client, tc.namespace, tc.name)
				assert.Nil(t, err)
				assert.NotNil(t, fork)
				assert.NotEqual(t, fork, tc.targetRepo)
				assert.Equal(t, tc.forkRepo, fork.Metadata)
				assert.Equal(t, fork.Sources[urn].CloneURL, "https://github.com/"+tc.wantNamespace+"/"+tc.wantName)
			})
		}
	})
}

func TestGithubSource_DuplicateCommit(t *testing.T) {
	// This test uses the branch "duplicate-commits-on-me" on the repository
	// https://github.com/sourcegraph/automation-testing. The branch contains a single
	// commit, to mimic the state after gitserver pushes the commit for Batch Changes.
	//
	// The requests here cannot be easily rerun with `-update` since you can only open a
	// pull request once. To update, push a new branch with at least one commit to
	// automation-testing, and put the branch names into the `success` case below.
	//
	// You can update just this test with `-update GithubSource_DuplicateCommit`.
	repo := &types.Repo{
		Metadata: &github.Repository{
			ID:            "MDEwOlJlcG9zaXRvcnkyMjExNDc1MTM=",
			NameWithOwner: "sourcegraph/automation-testing",
		},
	}

	testCases := []struct {
		name string
		rev  string
		err  *string
	}{
		{
			name: "success",
			rev:  "refs/heads/duplicate-commits-on-me",
		},
		{
			name: "invalid ref",
			rev:  "refs/heads/some-non-existent-branch-naturally",
			err:  pointers.Ptr("No commit found for SHA: refs/heads/some-non-existent-branch-naturally"),
		},
	}

	opts := protocol.CreateCommitFromPatchRequest{
		CommitInfo: protocol.PatchCommitInfo{
			Messages: []string{"Test commit from VCR tests"},
		},
	}

	for _, tc := range testCases {
		tc := tc

		tc.name = "GithubSource_DuplicateCommit_" + strings.ReplaceAll(tc.name, " ", "_")

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			src, save := setup(t, ctx, tc.name)
			defer save(t)

			_, err := src.DuplicateCommit(ctx, opts, repo, tc.rev)
			if err != nil && tc.err == nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if err == nil && tc.err != nil {
				t.Fatalf("expected error %q but got none", *tc.err)
			}
			if err != nil && tc.err != nil {
				assert.ErrorContains(t, err, *tc.err)
			}
		})
	}
}

type mockGithubClientFork struct {
	wantOrg *string
	fork    *github.Repository
	err     error
}

var _ githubClientFork = &mockGithubClientFork{}

func (mock *mockGithubClientFork) Fork(ctx context.Context, owner, repo string, org *string, forkName string) (*github.Repository, error) {
	if (mock.wantOrg == nil && org != nil) || (mock.wantOrg != nil && org == nil) || (mock.wantOrg != nil && org != nil && *mock.wantOrg != *org) {
		return nil, errors.Newf("unexpected organisation: have=%v want=%v", org, mock.wantOrg)
	}

	return mock.fork, mock.err
}

func (mock *mockGithubClientFork) GetRepo(ctx context.Context, owner, repo string) (*github.Repository, error) {
	return nil, nil
}

func setup(t *testing.T, ctx context.Context, tName string) (src *GitHubSource, save func(testing.TB)) {
	// The GithubSource uses the github.Client under the hood, which uses rcache, a
	// caching layer that uses Redis. We need to clear the cache before we run the tests
	rcache.SetupForTest(t)

	cf, save := newClientFactory(t, tName)

	svc := &types.ExternalService{
		Kind: extsvc.KindGitHub,
		Config: extsvc.NewUnencryptedConfig(marshalJSON(t, &schema.GitHubConnection{
			Url:   "https://github.com",
			Token: os.Getenv("GITHUB_TOKEN"),
		})),
	}

	src, err := NewGitHubSource(ctx, dbmocks.NewMockDB(), svc, cf)
	if err != nil {
		t.Fatal(err)
	}
	return src, save
}
