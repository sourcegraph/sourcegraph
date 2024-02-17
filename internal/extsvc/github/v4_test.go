package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/httptestutil"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/testutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestUnmarshal(t *testing.T) {
	type result struct {
		FieldA string
		FieldB string
	}
	cases := map[string]string{
		// Valid
		`[]`:                                  "",
		`[{"FieldA": "hi"}]`:                  "",
		`[{"FieldA": "hi", "FieldB": "bye"}]`: "",

		// Error
		`[[]]`:            `graphql: cannot unmarshal at offset 2: before "[["; after "]]": json: cannot unmarshal array into Go value of type github.result`,
		`[{"FieldA": 1}]`: `graphql: cannot unmarshal at offset 13: before "[{\"FieldA\": 1"; after "}]": json: cannot unmarshal number`,
	}
	// Large body
	repeated := strings.Repeat(`{"FieldA": "hi", "FieldB": "bye"},`, 100)
	cases[fmt.Sprintf(`[%s {"FieldA": 1}, %s]`, repeated, repeated[:len(repeated)-1])] = `graphql: cannot unmarshal at offset 3414: before ", \"FieldB\": \"bye\"},{\"FieldA\": \"hi\", \"FieldB\": \"bye\"},{\"FieldA\": \"hi\", \"FieldB\": \"bye\"}, {\"FieldA\": 1"; after "}, {\"FieldA\": \"hi\", \"FieldB\": \"bye\"},{\"FieldA\": \"hi\", \"FieldB\": \"bye\"},{\"FieldA\": \"hi\", \"FieldB\": \"b": json: cannot unmarshal number`

	for data, errStr := range cases {
		var a []result
		var b []result
		errA := json.Unmarshal([]byte(data), &a)
		errB := unmarshal([]byte(data), &b)

		if len(data) > 50 {
			data = data[:50] + "..."
		}

		if !reflect.DeepEqual(a, b) {
			t.Errorf("Expected the same result unmarshalling %v\na: %v\nb: %v", data, a, b)
		}

		if !errors.Is(errA, errors.Cause(errB)) {
			t.Errorf("Expected the same underlying error unmarshalling %v\na: %v\nb: %v", data, errA, errB)
		}
		got := ""
		if errB != nil {
			got = errB.Error()
		}
		if !strings.HasPrefix(got, errStr) {
			t.Errorf("Unexpected error message %v\ngot:  %s\nwant: %s", data, got, errStr)
		}
	}
}

func TestGetAuthenticatedUserV4(t *testing.T) {
	cli, save := newTestV4Client(t, "GetAuthenticatedUserV4")
	defer save()

	ctx := context.Background()

	user, err := cli.GetAuthenticatedUser(ctx)
	if err != nil {
		t.Fatal(err)
	}

	testutil.AssertGolden(t,
		"testdata/golden/GetAuthenticatedUserV4",
		update("GetAuthenticatedUserV4"),
		user,
	)
}

func TestV4Client_RateLimitRetry(t *testing.T) {
	rcache.SetupForTest(t)
	ratelimit.SetupForTest(t)

	ctx := context.Background()

	tests := map[string]struct {
		secondaryLimitWasHit bool
		primaryLimitWasHit   bool
		succeeded            bool
		numRequests          int
	}{
		"hit secondary limit": {
			secondaryLimitWasHit: true,
			succeeded:            true,
			numRequests:          2,
		},
		"hit primary limit": {
			primaryLimitWasHit: true,
			succeeded:          true,
			numRequests:        2,
		},
		"no rate limit hit": {
			succeeded:   true,
			numRequests: 1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			numRequests := 0
			succeeded := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				numRequests++
				if tt.secondaryLimitWasHit {
					simulateGitHubSecondaryRateLimitHit(w)
					tt.secondaryLimitWasHit = false
					return
				}

				if tt.primaryLimitWasHit {
					simulateGitHubPrimaryRateLimitHit(w)
					tt.primaryLimitWasHit = false
					return
				}

				succeeded = true
				w.Write([]byte(`{"message": "Very nice"}`))
			}))

			t.Cleanup(srv.Close)

			srvURL, err := url.Parse(srv.URL)
			require.NoError(t, err)

			client, err := NewV4Client("test", srvURL, nil, httpcli.NewFactory(nil, func(c *http.Client) error {
				transport := http.DefaultTransport.(*http.Transport).Clone()
				transport.DisableKeepAlives = true // Disable keep-alives otherwise the read of the request body is cached
				c.Transport = transport
				return nil
			}))
			require.NoError(t, err)
			client.internalRateLimiter = ratelimit.NewInstrumentedLimiter("githubv4", rate.NewLimiter(100, 10))
			client.githubDotCom = true // Otherwise it will make an extra request to determine GH version
			_, err = client.SearchRepos(ctx, SearchReposParams{Query: "test"})
			require.NoError(t, err)

			assert.Equal(t, tt.numRequests, numRequests)
			assert.Equal(t, tt.succeeded, succeeded)
		})
	}
}

func simulateGitHubSecondaryRateLimitHit(w http.ResponseWriter) {
	w.Header().Add("retry-after", "1")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"message": "Secondary rate limit hit"}`))
}

func simulateGitHubPrimaryRateLimitHit(w http.ResponseWriter) {
	w.Header().Add("x-ratelimit-remaining", "0")
	w.Header().Add("x-ratelimit-limit", "5000")
	resetTime := time.Now().Add(time.Second)
	w.Header().Add("x-ratelimit-reset", strconv.Itoa(int(resetTime.Unix())))
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"message": "Primary rate limit hit"}`))
}

func TestV4Client_RequestGraphQL_RequestUnmutated(t *testing.T) {
	rcache.SetupForTest(t)
	ratelimit.SetupForTest(t)

	query := `query Foobar { foobar }`
	vars := map[string]any{}
	result := struct{}{}

	ctx := context.Background()

	numRequests := 0
	requestPaths := []string{}
	requestBodies := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		numRequests++

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		requestPaths = append(requestPaths, r.URL.Path)
		requestBodies = append(requestBodies, string(body))

		if numRequests == 1 {
			simulateGitHubPrimaryRateLimitHit(w)
			return
		}

		w.Write([]byte(`{"message": "Very nice"}`))
	}))

	t.Cleanup(srv.Close)

	srvURL, err := url.Parse(srv.URL)
	require.NoError(t, err)

	// Now, this is IMPORTANT: we use `APIRoot` to simulate a real setup in which
	// we append the "API path" to the base URL configured by an admin.
	apiURL, _ := APIRoot(srvURL)

	// Now we create a client to talk to our test server with the API path
	// appended.
	client, err := NewV4Client("test", apiURL, nil, httpcli.NewFactory(nil, func(c *http.Client) error {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DisableKeepAlives = true // Disable keep-alives otherwise the read of the request body is cached
		c.Transport = transport
		return nil
	}))
	require.NoError(t, err)

	// Now we send a request that should run into rate limiting error.
	err = client.requestGraphQL(ctx, query, vars, &result)
	require.NoError(t, err)

	// Two requests should have been sent
	assert.Equal(t, numRequests, 2)

	// We want the same data to have been sent, twice
	wantPath := "/api/graphql"
	wantBody := `{"query":"query Foobar { foobar }","variables":{}}`
	assert.Equal(t, []string{wantPath, wantPath}, requestPaths)
	assert.Equal(t, []string{wantBody, wantBody}, requestBodies)
}

func TestV4Client_SearchRepos(t *testing.T) {
	rcache.SetupForTest(t)
	ratelimit.SetupForTest(t)
	cli, save := newTestV4Client(t, "SearchRepos")
	t.Cleanup(save)

	for _, tc := range []struct {
		name   string
		ctx    context.Context
		params SearchReposParams
		err    string
	}{
		{
			name: "narrow-query",
			params: SearchReposParams{
				Query: "repo:tsenart/vegeta",
				First: 1,
			},
		},
		{
			name: "huge-query",
			params: SearchReposParams{
				Query: "stars:5..500000 sort:stars-desc",
				First: 5,
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			if tc.err == "" {
				tc.err = "<nil>"
			}

			results, err := cli.SearchRepos(tc.ctx, tc.params)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			testutil.AssertGolden(t,
				fmt.Sprintf("testdata/golden/SearchRepos-%s", tc.name),
				update("SearchRepos"),
				results,
			)
		})
	}
}

func TestLoadPullRequest(t *testing.T) {
	cli, save := newTestV4Client(t, "LoadPullRequest")
	defer save()

	for i, tc := range []struct {
		name string
		ctx  context.Context
		pr   *PullRequest
		err  string
	}{
		{
			name: "non-existing-repo",
			pr:   &PullRequest{RepoWithOwner: "whoisthis/sourcegraph", Number: 5550},
			err:  "GitHub repository not found",
		},
		{
			name: "non-existing-pr",
			pr:   &PullRequest{RepoWithOwner: "sourcegraph/sourcegraph", Number: 0},
			err:  "GitHub pull request not found: 0",
		},
		{
			name: "success",
			pr:   &PullRequest{RepoWithOwner: "sourcegraph/sourcegraph", Number: 5550},
		},
		{
			name: "with more than 250 events",
			pr:   &PullRequest{RepoWithOwner: "sourcegraph/sourcegraph", Number: 596},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			if tc.err == "" {
				tc.err = "<nil>"
			}

			err := cli.LoadPullRequest(tc.ctx, tc.pr)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			testutil.AssertGolden(t,
				"testdata/golden/LoadPullRequest-"+strconv.Itoa(i),
				update("LoadPullRequest"),
				tc.pr,
			)
		})
	}
}

func TestCreatePullRequest(t *testing.T) {
	cli, save := newTestV4Client(t, "CreatePullRequest")
	defer save()

	// Repository used: https://github.com/sourcegraph/automation-testing
	//
	// The requests here cannot be easily rerun with `-update` since you can only open a
	// pull request once. To update, push two new branches with at least one commit each to
	// automation-testing, and put the branch names into the `success` and 'draft-pr' cases below.
	//
	// You can update just this test with `-update CreatePullRequest`.
	for i, tc := range []struct {
		name  string
		ctx   context.Context
		input *CreatePullRequestInput
		err   string
	}{
		{
			name: "success",
			input: &CreatePullRequestInput{
				RepositoryID: "MDEwOlJlcG9zaXRvcnkyMjExNDc1MTM=",
				BaseRefName:  "master",
				HeadRefName:  "test-pr-02",
				Title:        "This is a test PR, feel free to ignore",
				Body:         "I'm opening this PR to test something. Please ignore.",
			},
		},
		{
			name: "already-existing-pr",
			input: &CreatePullRequestInput{
				RepositoryID: "MDEwOlJlcG9zaXRvcnkyMjExNDc1MTM=",
				BaseRefName:  "master",
				HeadRefName:  "always-open-pr",
				Title:        "This is a test PR that is always open (keep it open!)",
				Body:         "Feel free to ignore this. This is a test PR that is always open and is sometimes updated.",
			},
			err: ErrPullRequestAlreadyExists.Error(),
		},
		{
			name: "invalid-head-ref",
			input: &CreatePullRequestInput{
				RepositoryID: "MDEwOlJlcG9zaXRvcnkyMjExNDc1MTM=",
				BaseRefName:  "master",
				HeadRefName:  "this-head-ref-should-not-exist",
				Title:        "Test",
			},
			err: "error in GraphQL response: Head sha can't be blank, Base sha can't be blank, No commits between master and this-head-ref-should-not-exist, Head ref must be a branch",
		},
		{
			name: "draft-pr",
			input: &CreatePullRequestInput{
				RepositoryID: "MDEwOlJlcG9zaXRvcnkyMjExNDc1MTM=",
				BaseRefName:  "master",
				HeadRefName:  "test-pr-15",
				Title:        "This is a test PR, feel free to ignore",
				Body:         "I'm opening this PR to test something. Please ignore.",
				Draft:        true,
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			if tc.err == "" {
				tc.err = "<nil>"
			}

			pr, err := cli.CreatePullRequest(tc.ctx, tc.input)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			testutil.AssertGolden(t,
				"testdata/golden/CreatePullRequest-"+strconv.Itoa(i),
				update("CreatePullRequest"),
				pr,
			)
		})
	}
}

func TestCreatePullRequest_Archived(t *testing.T) {
	ctx := context.Background()

	cli, save := newTestV4Client(t, "CreatePullRequest_Archived")
	defer save()

	// Repository used: sourcegraph-testing/archived
	//
	// This test can be updated at any time with `-update`, provided
	// `sourcegraph-testing/archived` is still archived.
	//
	// You can update just this test with `-update CreatePullRequest_Archived`.
	input := &CreatePullRequestInput{
		RepositoryID: "R_kgDOHpFg8A",
		BaseRefName:  "main",
		HeadRefName:  "branch-without-pr",
		Title:        "This is a PR that will never open",
		Body:         "This PR should not be open, as the repository is supposed to be archived!",
	}

	pr, err := cli.CreatePullRequest(ctx, input)
	assert.Nil(t, pr)
	assert.Error(t, err)
	assert.True(t, errcode.IsArchived(err))

	testutil.AssertGolden(t,
		"testdata/golden/CreatePullRequest_Archived",
		update("CreatePullRequest_Archived"),
		pr,
	)
}

func TestClosePullRequest(t *testing.T) {
	cli, save := newTestV4Client(t, "ClosePullRequest")
	defer save()

	// Repository used: https://github.com/sourcegraph/automation-testing
	//
	// This test can be updated with `-update ClosePullRequest`, provided:
	//
	// 1. https://github.com/sourcegraph/automation-testing/pull/44 must be open.
	// 2. https://github.com/sourcegraph/automation-testing/pull/29 must be
	//    closed, but _not_ merged.
	for i, tc := range []struct {
		name string
		ctx  context.Context
		pr   *PullRequest
		err  string
	}{
		{
			name: "success",
			// github.com/sourcegraph/automation-testing/pull/44
			pr: &PullRequest{ID: "MDExOlB1bGxSZXF1ZXN0MzQxMDU5OTY5"},
		},
		{
			name: "already closed",
			// github.com/sourcegraph/automation-testing/pull/29
			pr: &PullRequest{ID: "MDExOlB1bGxSZXF1ZXN0MzQxMDU5OTY5"},
			// Doesn't return an error
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			if tc.err == "" {
				tc.err = "<nil>"
			}

			err := cli.ClosePullRequest(tc.ctx, tc.pr)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			testutil.AssertGolden(t,
				"testdata/golden/ClosePullRequest-"+strconv.Itoa(i),
				update("ClosePullRequest"),
				tc.pr,
			)
		})
	}
}

func TestReopenPullRequest(t *testing.T) {
	cli, save := newTestV4Client(t, "ReopenPullRequest")
	defer save()

	// Repository used: https://github.com/sourcegraph/automation-testing
	//
	// This test can be updated with `-update ReopenPullRequest`, provided:
	//
	// 1. https://github.com/sourcegraph/automation-testing/pull/355 must be
	//    open.
	// 2. https://github.com/sourcegraph/automation-testing/pull/356 must be
	//    closed, but _not_ merged.
	for i, tc := range []struct {
		name string
		ctx  context.Context
		pr   *PullRequest
	}{
		{
			name: "success",
			// https://github.com/sourcegraph/automation-testing/pull/356
			pr: &PullRequest{ID: "MDExOlB1bGxSZXF1ZXN0NDg4NjEzODA3"},
		},
		{
			name: "already open",
			// https://github.com/sourcegraph/automation-testing/pull/355
			pr: &PullRequest{ID: "MDExOlB1bGxSZXF1ZXN0NDg4NjA0NTQ5"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			err := cli.ReopenPullRequest(tc.ctx, tc.pr)
			if err != nil {
				t.Fatalf("ReopenPullRequest returned unexpected error: %s", err)
			}

			testutil.AssertGolden(t,
				"testdata/golden/ReopenPullRequest-"+strconv.Itoa(i),
				update("ReopenPullRequest"),
				tc.pr,
			)
		})
	}
}

func TestMarkPullRequestReadyForReview(t *testing.T) {
	cli, save := newTestV4Client(t, "MarkPullRequestReadyForReview")
	defer save()

	// Repository used: https://github.com/sourcegraph/automation-testing
	//
	// This test can be updated with `-update MarkPullRequestReadyForReview`, provided:
	//
	// 1. https://github.com/sourcegraph/automation-testing/pull/467 must be
	//    open as a draft.
	// 2. https://github.com/sourcegraph/automation-testing/pull/466 must be
	//    open and ready for review.
	for i, tc := range []struct {
		name string
		ctx  context.Context
		pr   *PullRequest
	}{
		{
			name: "success",
			// https://github.com/sourcegraph/automation-testing/pull/467
			pr: &PullRequest{ID: "PR_kwDODS5xec4waL43"},
		},
		{
			name: "already ready for review",
			// https://github.com/sourcegraph/automation-testing/pull/466
			pr: &PullRequest{ID: "PR_kwDODS5xec4waL4w"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			err := cli.MarkPullRequestReadyForReview(tc.ctx, tc.pr)
			if err != nil {
				t.Fatalf("MarkPullRequestReadyForReview returned unexpected error: %s", err)
			}

			testutil.AssertGolden(t,
				"testdata/golden/MarkPullRequestReadyForReview-"+strconv.Itoa(i),
				update("MarkPullRequestReadyForReview"),
				tc.pr,
			)
		})
	}
}

func TestCreatePullRequestComment(t *testing.T) {
	cli, save := newTestV4Client(t, "CreatePullRequestComment")
	defer save()

	pr := &PullRequest{
		// https://github.com/sourcegraph/automation-testing/pull/44
		ID: "MDExOlB1bGxSZXF1ZXN0MzQxMDU5OTY5",
	}

	err := cli.CreatePullRequestComment(context.Background(), pr, "test-comment")
	if err != nil {
		t.Fatal(err)
	}
}

func TestMergePullRequest(t *testing.T) {
	cli, save := newTestV4Client(t, "TestMergePullRequest")
	defer save()

	t.Run("success", func(t *testing.T) {
		pr := &PullRequest{
			// https://github.com/sourcegraph/automation-testing/pull/506
			ID: "PR_kwDODS5xec5TxsRF",
		}

		err := cli.MergePullRequest(context.Background(), pr, true)
		if err != nil {
			t.Fatal(err)
		}

		testutil.AssertGolden(t,
			"testdata/golden/MergePullRequest-success",
			update("MergePullRequest"),
			pr,
		)
	})

	t.Run("not mergeable", func(t *testing.T) {
		pr := &PullRequest{
			// https://github.com/sourcegraph/automation-testing/pull/419
			ID: "MDExOlB1bGxSZXF1ZXN0NTY1Mzk1NTc3",
		}

		err := cli.MergePullRequest(context.Background(), pr, true)
		if err == nil {
			t.Fatal("invalid nil error")
		}

		testutil.AssertGolden(t,
			"testdata/golden/MergePullRequest-error",
			update("MergePullRequest"),
			err,
		)
	})
}

func TestUpdatePullRequest_Archived(t *testing.T) {
	ctx := context.Background()

	cli, save := newTestV4Client(t, "UpdatePullRequest_Archived")
	defer save()

	// Repository used: sourcegraph-testing/archived
	//
	// This test can be updated at any time with `-update`, provided
	// `sourcegraph-testing/archived` is still archived.
	//
	// You can update just this test with `-update UpdatePullRequest_Archived`.
	input := &UpdatePullRequestInput{
		PullRequestID: "PR_kwDOHpFg8M47NV9e",
		Body:          "This PR should never have its body changed.",
	}

	pr, err := cli.UpdatePullRequest(ctx, input)
	assert.Nil(t, pr)
	assert.Error(t, err)
	assert.True(t, errcode.IsArchived(err))

	testutil.AssertGolden(t,
		"testdata/golden/UpdatePullRequest_Archived",
		update("UpdatePullRequest_Archived"),
		pr,
	)
}

func TestEstimateGraphQLCost(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query string
		want  int
	}{
		{
			name: "Canonical example",
			query: `query {
  viewer {
    login
    repositories(first: 100) {
      edges {
        node {
          id

          issues(first: 50) {
            edges {
              node {
                id
                labels(first: 60) {
                  edges {
                    node {
                      id
                      name
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`,
			want: 51,
		},
		{
			name: "simple query",
			query: `
query {
  viewer {
    repositories(first: 50) {
      edges {
        repository:node {
          name
          issues(first: 10) {
            totalCount
            edges {
              node {
                title
                bodyHTML
              }
            }
          }
        }
      }
    }
  }
}
`,
			want: 1,
		},
		{
			name: "complex query",
			query: `query {
  viewer {
    repositories(first: 50) {
      edges {
        repository:node {
          name

          pullRequests(first: 20) {
            edges {
              pullRequest:node {
                title

                comments(first: 10) {
                  edges {
                    comment:node {
                      bodyHTML
                    }
                  }
                }
              }
            }
          }

          issues(first: 20) {
            totalCount
            edges {
              issue:node {
                title
                bodyHTML

                comments(first: 10) {
                  edges {
                    comment:node {
                      bodyHTML
                    }
                  }
                }
              }
            }
          }
        }
      }
    }

    followers(first: 10) {
      edges {
        follower:node {
          login
        }
      }
    }
  }
}`,
			want: 21,
		},
		{
			name: "Multiple top level queries",
			query: `query {
  thing
}
query{
  thing
}
`,
			want: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			have, err := estimateGraphQLCost(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if have != tc.want {
				t.Fatalf("have %d, want %d", have, tc.want)
			}
		})
	}
}

func TestRecentCommitters(t *testing.T) {
	cli, save := newTestV4Client(t, "RecentCommitters")
	t.Cleanup(save)

	recentCommitters, err := cli.RecentCommitters(context.Background(), &RecentCommittersParams{
		Owner: "sourcegraph-testing",
		Name:  "etcd",
	})
	if err != nil {
		t.Fatal(err)
	}

	testutil.AssertGolden(t,
		"testdata/golden/RecentCommitters",
		update("RecentCommitters"),
		recentCommitters,
	)
}

func TestV4Client_SearchRepos_Enterprise(t *testing.T) {
	cli, save := newEnterpriseV4Client(t, "SearchRepos-Enterprise")
	t.Cleanup(save)

	testCases := []struct {
		name   string
		ctx    context.Context
		params SearchReposParams
		err    string
	}{
		{
			name: "narrow-query-enterprise",
			params: SearchReposParams{
				Query: "repo:admiring-austin-120/fluffy-enigma",
				First: 1,
			},
		},
	}

	for _, tc := range testCases {
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				ExperimentalFeatures: &schema.ExperimentalFeatures{
					EnableGithubInternalRepoVisibility: true,
				},
			},
		})

		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			if tc.err == "" {
				tc.err = "<nil>"
			}

			results, err := cli.SearchRepos(tc.ctx, tc.params)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			testutil.AssertGolden(t,
				fmt.Sprintf("testdata/golden/SearchRepos-Enterprise-%s", tc.name),
				update("SearchRepos-Enterprise"),
				results,
			)
		})
	}
}

func TestV4Client_WithAuthenticator(t *testing.T) {
	uri, err := url.Parse("https://github.com")
	if err != nil {
		t.Fatal(err)
	}

	oldClient := &V4Client{
		apiURL: uri,
		auth:   &auth.OAuthBearerToken{Token: "old_token"},
	}

	newToken := &auth.OAuthBearerToken{Token: "new_token"}
	newClient := oldClient.WithAuthenticator(newToken)
	if oldClient == newClient {
		t.Fatal("both clients have the same address")
	}

	if newClient.auth != newToken {
		t.Fatalf("token: want %p but got %p", newToken, newClient.auth)
	}
}

func newTestV4Client(t testing.TB, name string) (*V4Client, func()) {
	t.Helper()

	cf, save := httptestutil.NewGitHubRecorderFactory(t, update(name), name)
	uri, err := url.Parse("https://github.com")
	if err != nil {
		t.Fatal(err)
	}

	cli, err := NewV4Client("Test", uri, vcrToken, cf)
	require.NoError(t, err)
	cli.internalRateLimiter = ratelimit.NewInstrumentedLimiter("githubv4", rate.NewLimiter(100, 10))

	return cli, save
}

func newEnterpriseV4Client(t testing.TB, name string) (*V4Client, func()) {
	t.Helper()

	cf, save := httptestutil.NewGitHubRecorderFactory(t, update(name), name)
	uri, err := url.Parse("https://ghe.sgdev.org/")
	if err != nil {
		t.Fatal(err)
	}
	uri, _ = APIRoot(uri)

	cli, err := NewV4Client("Test", uri, gheToken, cf)
	require.NoError(t, err)
	cli.internalRateLimiter = ratelimit.NewInstrumentedLimiter("githubv4", rate.NewLimiter(100, 10))
	return cli, save
}

func TestClient_GetReposByNameWithOwner(t *testing.T) {
	namesWithOwners := []string{
		"sourcegraph/grapher-tutorial",
		"sourcegraph/clojure-grapher",
	}

	grapherTutorialRepo := &Repository{
		ID:               "MDEwOlJlcG9zaXRvcnkxNDYwMTc5OA==",
		DatabaseID:       14601798,
		NameWithOwner:    "sourcegraph/grapher-tutorial",
		Description:      "monkey language",
		URL:              "https://github.com/sourcegraph/grapher-tutorial",
		IsPrivate:        true,
		IsFork:           false,
		IsArchived:       true,
		IsLocked:         true,
		ViewerPermission: "ADMIN",
		Visibility:       "internal",
		RepositoryTopics: RepositoryTopics{Nodes: []RepositoryTopic{
			{Topic: Topic{Name: "topic1"}},
			{Topic: Topic{Name: "topic2"}},
		}},
	}

	clojureGrapherRepo := &Repository{
		ID:               "MDEwOlJlcG9zaXRvcnkxNTc1NjkwOA==",
		DatabaseID:       15756908,
		NameWithOwner:    "sourcegraph/clojure-grapher",
		Description:      "clojure grapher",
		URL:              "https://github.com/sourcegraph/clojure-grapher",
		IsPrivate:        true,
		IsFork:           false,
		IsArchived:       true,
		IsDisabled:       true,
		ViewerPermission: "ADMIN",
		Visibility:       "private",
	}

	testCases := []struct {
		name             string
		mockResponseBody string
		wantRepos        []*Repository
		err              string
	}{
		{
			name: "found",
			mockResponseBody: `
{
  "data": {
    "repo_sourcegraph_grapher_tutorial": {
      "id": "MDEwOlJlcG9zaXRvcnkxNDYwMTc5OA==",
      "databaseId": 14601798,
      "nameWithOwner": "sourcegraph/grapher-tutorial",
      "description": "monkey language",
      "url": "https://github.com/sourcegraph/grapher-tutorial",
      "isPrivate": true,
      "isFork": false,
      "isArchived": true,
      "isLocked": true,
      "viewerPermission": "ADMIN",
      "visibility": "internal",
	  "repositoryTopics": {
		"nodes": [
		  {
		    "topic": {
			  "name": "topic1"
			}
		  },
		  {
			"topic": {
			  "name": "topic2"
			}
		  }
	    ]
	  }
    },
    "repo_sourcegraph_clojure_grapher": {
      "id": "MDEwOlJlcG9zaXRvcnkxNTc1NjkwOA==",
	  "databaseId": 15756908,
      "nameWithOwner": "sourcegraph/clojure-grapher",
      "description": "clojure grapher",
      "url": "https://github.com/sourcegraph/clojure-grapher",
      "isPrivate": true,
      "isFork": false,
      "isArchived": true,
      "isDisabled": true,
      "viewerPermission": "ADMIN",
      "visibility": "private"
    }
  }
}
`,
			wantRepos: []*Repository{grapherTutorialRepo, clojureGrapherRepo},
		},
		{
			name: "not found",
			mockResponseBody: `
{
  "data": {
    "repo_sourcegraph_grapher_tutorial": {
      "id": "MDEwOlJlcG9zaXRvcnkxNDYwMTc5OA==",
      "databaseId": 14601798,
      "nameWithOwner": "sourcegraph/grapher-tutorial",
      "description": "monkey language",
      "url": "https://github.com/sourcegraph/grapher-tutorial",
      "isPrivate": true,
      "isFork": false,
      "isArchived": true,
      "isLocked": true,
      "viewerPermission": "ADMIN",
      "visibility": "internal",
	  "repositoryTopics": {
		  "nodes": [
			  {
				  "topic": {
					  "name": "topic1"
				  }
			  },
			  {
				  "topic": {
					  "name": "topic2"
				  }
			  }
		  ]
	  }
    },
    "repo_sourcegraph_clojure_grapher": null
  },
  "errors": [
    {
      "type": "NOT_FOUND",
      "path": [
        "repo_sourcegraph_clojure_grapher"
      ],
      "locations": [
        {
          "line": 13,
          "column": 3
        }
      ],
      "message": "Could not resolve to a Repository with the name 'clojure-grapher'."
    }
  ]
}
`,
			wantRepos: []*Repository{grapherTutorialRepo},
		},
		{
			name: "error",
			mockResponseBody: `
{
  "errors": [
    {
      "path": [
        "fragment RepositoryFields",
        "foobar"
      ],
      "extensions": {
        "code": "undefinedField",
        "typeName": "Repository",
        "fieldName": "foobar"
      },
      "locations": [
        {
          "line": 10,
          "column": 3
        }
      ],
      "message": "Field 'foobar' doesn't exist on type 'Repository'"
    }
  ]
}
`,
			wantRepos: []*Repository{},
			err:       "error in GraphQL response: Field 'foobar' doesn't exist on type 'Repository'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			apiURL := &url.URL{Scheme: "https", Host: "example.com", Path: "/"}
			c, err := NewV4Client("Test", apiURL, nil, httpcli.NewFactory(nil, func(c *http.Client) error {
				c.Transport = httpcli.WrapTransport(&mockTransport{do: func(r *http.Request) (*http.Response, error) {
					return &http.Response{
						Request:    r,
						StatusCode: 200,
						Body:       io.NopCloser(strings.NewReader(tc.mockResponseBody)),
					}, nil
				}}, http.DefaultTransport)
				return nil
			}))
			require.NoError(t, err)
			c.internalRateLimiter = ratelimit.NewInstrumentedLimiter("githubv4", rate.NewLimiter(100, 10))

			repos, err := c.GetReposByNameWithOwner(context.Background(), namesWithOwners...)
			if have, want := fmt.Sprint(err), fmt.Sprint(tc.err); tc.err != "" && have != want {
				t.Errorf("error:\nhave: %v\nwant: %v", have, want)
			}

			if want, have := len(tc.wantRepos), len(repos); want != have {
				t.Errorf("wrong number of repos. want=%d, have=%d", want, have)
			}

			newSortFunc := func(s []*Repository) func(int, int) bool {
				return func(i, j int) bool { return s[i].ID < s[j].ID }
			}

			sort.Slice(tc.wantRepos, newSortFunc(tc.wantRepos))
			sort.Slice(repos, newSortFunc(repos))

			if diff := cmp.Diff(repos, tc.wantRepos, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("got repositories:\n%s", diff)
			}
		})
	}
}

func TestClient_buildGetRepositoriesBatchQuery(t *testing.T) {
	repos := []string{
		"sourcegraph/grapher-tutorial",
		"sourcegraph/clojure-grapher",
		"sourcegraph/programming-challenge",
		"sourcegraph/annotate",
		"sourcegraph/sourcegraph-sublime-old",
		"sourcegraph/makex",
		"sourcegraph/pydep",
		"sourcegraph/vcsstore",
		"sourcegraph/contains.dot",
	}

	wantIncluded := `
repo0: repository(owner: "sourcegraph", name: "grapher-tutorial") { ... on Repository { ...RepositoryFields parent { nameWithOwner, isFork } } }
repo1: repository(owner: "sourcegraph", name: "clojure-grapher") { ... on Repository { ...RepositoryFields parent { nameWithOwner, isFork } } }
repo2: repository(owner: "sourcegraph", name: "programming-challenge") { ... on Repository { ...RepositoryFields parent { nameWithOwner, isFork } } }
repo3: repository(owner: "sourcegraph", name: "annotate") { ... on Repository { ...RepositoryFields parent { nameWithOwner, isFork } } }
repo4: repository(owner: "sourcegraph", name: "sourcegraph-sublime-old") { ... on Repository { ...RepositoryFields parent { nameWithOwner, isFork } } }
repo5: repository(owner: "sourcegraph", name: "makex") { ... on Repository { ...RepositoryFields parent { nameWithOwner, isFork } } }
repo6: repository(owner: "sourcegraph", name: "pydep") { ... on Repository { ...RepositoryFields parent { nameWithOwner, isFork } } }
repo7: repository(owner: "sourcegraph", name: "vcsstore") { ... on Repository { ...RepositoryFields parent { nameWithOwner, isFork } } }
repo8: repository(owner: "sourcegraph", name: "contains.dot") { ... on Repository { ...RepositoryFields parent { nameWithOwner, isFork } } }`

	apiURL := &url.URL{Scheme: "https", Host: "example.com", Path: "/"}
	c, err := NewV4Client("Test", apiURL, nil, httpcli.NewFactory(nil, func(c *http.Client) error {
		c.Transport = httpcli.WrapTransport(&mockTransport{do: func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				Request:    r,
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}}, http.DefaultTransport)
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	query, err := c.buildGetReposBatchQuery(context.Background(), repos)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(query, wantIncluded) {
		t.Fatalf("query does not contain repository query. query=%q, want=%q", query, wantIncluded)
	}
}

func TestClient_Releases(t *testing.T) {
	cli, save := newTestV4Client(t, "Releases")
	t.Cleanup(save)

	releases, err := cli.Releases(context.Background(), &ReleasesParams{
		Name:  "src-cli",
		Owner: "sourcegraph",
	})
	assert.NoError(t, err)

	testutil.AssertGolden(t,
		"testdata/golden/Releases",
		update("Releases"),
		releases,
	)
}
