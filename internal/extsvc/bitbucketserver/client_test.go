package bitbucketserver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/schema"
)

var update = flag.Bool("update", false, "update testdata")

func TestParseQueryStrings(t *testing.T) {
	for _, tc := range []struct {
		name string
		qs   []string
		vals url.Values
		err  string
	}{
		{
			name: "ignores query separator",
			qs:   []string{"?foo=bar&baz=boo"},
			vals: url.Values{"foo": {"bar"}, "baz": {"boo"}},
		},
		{
			name: "ignores query separator by itself",
			qs:   []string{"?"},
			vals: url.Values{},
		},
		{
			name: "perserves multiple values",
			qs:   []string{"?foo=bar&foo=baz", "foo=boo"},
			vals: url.Values{"foo": {"bar", "baz", "boo"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == "" {
				tc.err = "<nil>"
			}

			vals, err := parseQueryStrings(tc.qs...)

			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if have, want := vals, tc.vals; !reflect.DeepEqual(have, want) {
				t.Error(cmp.Diff(have, want))
			}
		})
	}
}

func TestClientKeepsBaseURLPath(t *testing.T) {
	ctx := context.Background()

	succeeded := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/testpath") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		succeeded = true
	}))
	defer srv.Close()

	srvURL, err := url.JoinPath(srv.URL, "/testpath")
	require.NoError(t, err)
	bbConf := &schema.BitbucketServerConnection{Url: srvURL}
	client, err := NewClient("test", bbConf, httpcli.TestExternalDoer)
	require.NoError(t, err)
	client.rateLimit = ratelimit.NewInstrumentedLimiter("bitbucket", rate.NewLimiter(100, 10))

	_, _ = client.AuthenticatedUsername(ctx)
	assert.Equal(t, true, succeeded)
}

func TestUserFilters(t *testing.T) {
	for _, tc := range []struct {
		name string
		fs   UserFilters
		qry  url.Values
	}{
		{
			name: "last one wins",
			fs: UserFilters{
				{Filter: "admin"},
				{Filter: "tomas"}, // Last one wins
			},
			qry: url.Values{"filter": []string{"tomas"}},
		},
		{
			name: "filters can be combined",
			fs: UserFilters{
				{Filter: "admin"},
				{Group: "admins"},
			},
			qry: url.Values{
				"filter": []string{"admin"},
				"group":  []string{"admins"},
			},
		},
		{
			name: "permissions",
			fs: UserFilters{
				{
					Permission: PermissionFilter{
						Root:       PermProjectAdmin,
						ProjectKey: "ORG",
					},
				},
				{
					Permission: PermissionFilter{
						Root:           PermRepoWrite,
						ProjectKey:     "ORG",
						RepositorySlug: "foo",
					},
				},
			},
			qry: url.Values{
				"permission.1":                []string{"PROJECT_ADMIN"},
				"permission.1.projectKey":     []string{"ORG"},
				"permission.2":                []string{"REPO_WRITE"},
				"permission.2.projectKey":     []string{"ORG"},
				"permission.2.repositorySlug": []string{"foo"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			have := make(url.Values)
			tc.fs.EncodeTo(have)
			if want := tc.qry; !reflect.DeepEqual(have, want) {
				t.Error(cmp.Diff(have, want))
			}
		})
	}
}

func TestClient_Users(t *testing.T) {
	cli := NewTestClient(t, "Users", *update)

	timeout, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	users := map[string]*User{
		"admin": {
			Name:         "admin",
			EmailAddress: "tomas@sourcegraph.com",
			ID:           1,
			DisplayName:  "admin",
			Active:       true,
			Slug:         "admin",
			Type:         "NORMAL",
		},
		"john": {
			Name:         "john",
			EmailAddress: "john@mycorp.org",
			ID:           52,
			DisplayName:  "John Doe",
			Active:       true,
			Slug:         "john",
			Type:         "NORMAL",
		},
	}

	for _, tc := range []struct {
		name    string
		ctx     context.Context
		page    *PageToken
		filters []UserFilter
		users   []*User
		next    *PageToken
		err     string
	}{
		{
			name: "timeout",
			ctx:  timeout,
			err:  "context deadline exceeded",
		},
		{
			name:  "pagination: first page",
			page:  &PageToken{Limit: 1},
			users: []*User{users["admin"]},
			next: &PageToken{
				Size:          1,
				Limit:         1,
				NextPageStart: 1,
			},
		},
		{
			name: "pagination: last page",
			page: &PageToken{
				Size:          1,
				Limit:         1,
				NextPageStart: 1,
			},
			users: []*User{users["john"]},
			next: &PageToken{
				Size:       1,
				Start:      1,
				Limit:      1,
				IsLastPage: true,
			},
		},
		{
			name:    "filter by substring match in username, name and email address",
			page:    &PageToken{Limit: 1000},
			filters: []UserFilter{{Filter: "Doe"}}, // matches "John Doe" in name
			users:   []*User{users["john"]},
			next: &PageToken{
				Size:       1,
				Limit:      1000,
				IsLastPage: true,
			},
		},
		{
			name:    "filter by group",
			page:    &PageToken{Limit: 1000},
			filters: []UserFilter{{Group: "admins"}},
			users:   []*User{users["admin"]},
			next: &PageToken{
				Size:       1,
				Limit:      1000,
				IsLastPage: true,
			},
		},
		{
			name: "filter by multiple ANDed permissions",
			page: &PageToken{Limit: 1000},
			filters: []UserFilter{
				{
					Permission: PermissionFilter{
						Root: PermSysAdmin,
					},
				},
				{
					Permission: PermissionFilter{
						Root:           PermRepoRead,
						ProjectKey:     "ORG",
						RepositorySlug: "foo",
					},
				},
			},
			users: []*User{users["admin"]},
			next: &PageToken{
				Size:       1,
				Limit:      1000,
				IsLastPage: true,
			},
		},
		{
			name: "multiple filters are ANDed",
			page: &PageToken{Limit: 1000},
			filters: []UserFilter{
				{
					Filter: "admin",
				},
				{
					Permission: PermissionFilter{
						Root:           PermRepoRead,
						ProjectKey:     "ORG",
						RepositorySlug: "foo",
					},
				},
			},
			users: []*User{users["admin"]},
			next: &PageToken{
				Size:       1,
				Limit:      1000,
				IsLastPage: true,
			},
		},
		{
			name: "maximum 50 permission filters",
			page: &PageToken{Limit: 1000},
			filters: func() (fs UserFilters) {
				for range 51 {
					fs = append(fs, UserFilter{
						Permission: PermissionFilter{
							Root: PermSysAdmin,
						},
					})
				}
				return fs
			}(),
			err: ErrUserFiltersLimit.Error(),
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

			users, next, err := cli.Users(tc.ctx, tc.page, tc.filters...)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if have, want := next, tc.next; !reflect.DeepEqual(have, want) {
				t.Error(cmp.Diff(have, want))
			}

			if have, want := users, tc.users; !reflect.DeepEqual(have, want) {
				t.Error(cmp.Diff(have, want))
			}
		})
	}
}

func TestClient_LabeledRepos(t *testing.T) {
	cli := NewTestClient(t, "LabeledRepos", *update)

	// We have archived label on bitbucket.sgdev.org with a repo in it.
	repos, _, err := cli.LabeledRepos(context.Background(), nil, "archived")
	if err != nil {
		t.Fatal("archived label should not fail on bitbucket.sgdev.org", err)
	}
	checkGolden(t, "LabeledRepos-archived", repos)

	// This label shouldn't exist. Check we get back the correct error
	_, _, err = cli.LabeledRepos(context.Background(), nil, "doesnotexist")
	if err == nil {
		t.Fatal("expected doesnotexist label to fail")
	}
	if !IsNoSuchLabel(err) {
		t.Fatalf("expected NoSuchLabel error, got %v", err)
	}
}

func TestClient_LoadPullRequest(t *testing.T) {
	instanceURL := os.Getenv("BITBUCKET_SERVER_URL")
	if instanceURL == "" {
		instanceURL = "https://bitbucket.sgdev.org"
	}

	timeout, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	pr := &PullRequest{ID: 2}
	pr.ToRef.Repository.Slug = "vegeta"
	pr.ToRef.Repository.Project.Key = "SOUR"

	for _, tc := range []struct {
		name string
		ctx  context.Context
		pr   func() *PullRequest
		err  string
	}{
		{
			name: "timeout",
			pr:   func() *PullRequest { return pr },
			ctx:  timeout,
			err:  "context deadline exceeded",
		},
		{
			name: "repo not set",
			pr:   func() *PullRequest { return &PullRequest{ID: 2} },
			err:  "repository slug empty",
		},
		{
			name: "project not set",
			pr: func() *PullRequest {
				pr := &PullRequest{ID: 2}
				pr.ToRef.Repository.Slug = "vegeta"
				return pr
			},
			err: "project key empty",
		},
		{
			name: "non existing pr",
			pr: func() *PullRequest {
				pr := &PullRequest{ID: 9999}
				pr.ToRef.Repository.Slug = "vegeta"
				pr.ToRef.Repository.Project.Key = "SOUR"
				return pr
			},
			err: "pull request not found",
		},
		{
			name: "non existing repo",
			pr: func() *PullRequest {
				pr := &PullRequest{ID: 9999}
				pr.ToRef.Repository.Slug = "invalidslug"
				pr.ToRef.Repository.Project.Key = "SOUR"
				return pr
			},
			err: "Bitbucket API HTTP error: code=404 url=\"${INSTANCEURL}/rest/api/1.0/projects/SOUR/repos/invalidslug/pull-requests/9999\" body=\"{\\\"errors\\\":[{\\\"context\\\":null,\\\"message\\\":\\\"Repository SOUR/invalidslug does not exist.\\\",\\\"exceptionName\\\":\\\"com.atlassian.bitbucket.repository.NoSuchRepositoryException\\\"}]}\"",
		},
		{
			name: "success",
			pr:   func() *PullRequest { return pr },
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			name := "PullRequests-" + strings.ReplaceAll(tc.name, " ", "-")
			cli := NewTestClient(t, name, *update)

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			if tc.err == "" {
				tc.err = "<nil>"
			}
			tc.err = strings.ReplaceAll(tc.err, "${INSTANCEURL}", instanceURL)

			pr := tc.pr()
			err := cli.LoadPullRequest(tc.ctx, pr)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Fatalf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil || tc.err != "<nil>" {
				return
			}

			checkGolden(t, "LoadPullRequest-"+strings.ReplaceAll(tc.name, " ", "-"), pr)
		})
	}
}

func TestClient_CreatePullRequest(t *testing.T) {
	instanceURL := os.Getenv("BITBUCKET_SERVER_URL")
	if instanceURL == "" {
		instanceURL = "https://bitbucket.sgdev.org"
	}

	timeout, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	pr := &PullRequest{}
	pr.Title = "This is a test PR"
	pr.Description = "This is a test PR. Feel free to ignore."
	pr.ToRef.Repository.ID = 10070
	pr.ToRef.Repository.Slug = "automation-testing"
	pr.ToRef.Repository.Project.Key = "SOUR"
	pr.ToRef.ID = "refs/heads/master"
	pr.FromRef.Repository.ID = 10070
	pr.FromRef.Repository.Slug = "automation-testing"
	pr.FromRef.Repository.Project.Key = "SOUR"
	pr.FromRef.ID = "refs/heads/test-pr-bbs-1"

	for _, tc := range []struct {
		name string
		ctx  context.Context
		pr   func() *PullRequest
		err  string
	}{
		{
			name: "timeout",
			pr:   func() *PullRequest { return pr },
			ctx:  timeout,
			err:  "context deadline exceeded",
		},
		{
			name: "ToRef repo not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.ToRef.Repository.Slug = ""
				return &pr
			},
			err: "ToRef repository slug empty",
		},
		{
			name: "ToRef project not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.ToRef.Repository.Project.Key = ""
				return &pr
			},
			err: "ToRef project key empty",
		},
		{
			name: "ToRef ID not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.ToRef.ID = ""
				return &pr
			},
			err: "ToRef id empty",
		},
		{
			name: "FromRef repo not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.FromRef.Repository.Slug = ""
				return &pr
			},
			err: "FromRef repository slug empty",
		},
		{
			name: "FromRef project not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.FromRef.Repository.Project.Key = ""
				return &pr
			},
			err: "FromRef project key empty",
		},
		{
			name: "FromRef ID not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.FromRef.ID = ""
				return &pr
			},
			err: "FromRef id empty",
		},
		{
			name: "success",
			pr: func() *PullRequest {
				pr := *pr
				pr.FromRef.ID = "refs/heads/test-pr-bbs-3"
				return &pr
			},
		},
		{
			name: "pull request already exists",
			pr: func() *PullRequest {
				pr := *pr
				pr.FromRef.ID = "refs/heads/always-open-pr-bbs"
				return &pr
			},
			err: ErrAlreadyExists{}.Error(),
		},
		{
			name: "description includes GFM tasklist items",
			pr: func() *PullRequest {
				pr := *pr
				pr.FromRef.ID = "refs/heads/test-pr-bbs-17"
				pr.Description = "- [ ] One\n- [ ] Two\n"
				return &pr
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			name := "CreatePullRequest-" + strings.ReplaceAll(tc.name, " ", "-")
			cli := NewTestClient(t, name, *update)

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			if tc.err == "" {
				tc.err = "<nil>"
			}
			tc.err = strings.ReplaceAll(tc.err, "${INSTANCEURL}", instanceURL)

			pr := tc.pr()
			err := cli.CreatePullRequest(tc.ctx, pr)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Fatalf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil || tc.err != "<nil>" {
				return
			}

			checkGolden(t, name, pr)
		})
	}
}

func TestClient_FetchDefaultReviewers(t *testing.T) {
	instanceURL := os.Getenv("BITBUCKET_SERVER_URL")
	if instanceURL == "" {
		instanceURL = "https://bitbucket.sgdev.org"
	}

	timeout, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	pr := &PullRequest{}
	pr.Title = "This is a test PR"
	pr.Description = "This is a test PR. Feel free to ignore."
	pr.ToRef.Repository.ID = 10070
	pr.ToRef.Repository.Slug = "automation-testing"
	pr.ToRef.Repository.Project.Key = "SOUR"
	pr.ToRef.ID = "refs/heads/master"
	pr.FromRef.Repository.ID = 10070
	pr.FromRef.Repository.Slug = "automation-testing"
	pr.FromRef.Repository.Project.Key = "SOUR"
	pr.FromRef.ID = "refs/heads/test-pr-bbs-1"

	for _, tc := range []struct {
		name string
		ctx  context.Context
		pr   func() *PullRequest
		err  string
	}{
		{
			name: "timeout",
			pr:   func() *PullRequest { return pr },
			ctx:  timeout,
			err:  "context deadline exceeded",
		},
		{
			name: "ToRef repo id not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.ToRef.Repository.ID = 0
				return &pr
			},
			err: "ToRef repository id empty",
		},
		{
			name: "ToRef repo slug not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.ToRef.Repository.Slug = ""
				return &pr
			},
			err: "ToRef repository slug empty",
		},
		{
			name: "ToRef project not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.ToRef.Repository.Project.Key = ""
				return &pr
			},
			err: "ToRef project key empty",
		},
		{
			name: "ToRef ID not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.ToRef.ID = ""
				return &pr
			},
			err: "ToRef id empty",
		},
		{
			name: "FromRef repo id not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.FromRef.Repository.ID = 0
				return &pr
			},
			err: "FromRef repository id empty",
		},
		{
			name: "FromRef repo slug not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.FromRef.Repository.Slug = ""
				return &pr
			},
			err: "FromRef repository slug empty",
		},
		{
			name: "FromRef project not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.FromRef.Repository.Project.Key = ""
				return &pr
			},
			err: "FromRef project key empty",
		},
		{
			name: "FromRef ID not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.FromRef.ID = ""
				return &pr
			},
			err: "FromRef id empty",
		},
		{
			name: "success",
			pr: func() *PullRequest {
				pr := *pr
				pr.FromRef.ID = "refs/heads/test-pr-bbs-3"
				return &pr
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			name := "FetchDefaultReviewers-" + strings.ReplaceAll(tc.name, " ", "-")
			cli := NewTestClient(t, name, *update)

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			if tc.err == "" {
				tc.err = "<nil>"
			}
			tc.err = strings.ReplaceAll(tc.err, "${INSTANCEURL}", instanceURL)

			pr := tc.pr()
			reviewers, err := cli.FetchDefaultReviewers(tc.ctx, pr)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Fatalf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil || tc.err != "<nil>" {
				return
			}

			checkGolden(t, name, reviewers)
		})
	}
}

func TestClient_DeclinePullRequest(t *testing.T) {
	instanceURL := os.Getenv("BITBUCKET_SERVER_URL")
	if instanceURL == "" {
		instanceURL = "https://bitbucket.sgdev.org"
	}

	timeout, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	pr := &PullRequest{}
	pr.ToRef.Repository.Slug = "automation-testing"
	pr.ToRef.Repository.Project.Key = "SOUR"

	for _, tc := range []struct {
		name string
		ctx  context.Context
		pr   func() *PullRequest
		err  string
	}{
		{
			name: "timeout",
			pr:   func() *PullRequest { return pr },
			ctx:  timeout,
			err:  "context deadline exceeded",
		},
		{
			name: "ToRef repo not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.ToRef.Repository.Slug = ""
				return &pr
			},
			err: "repository slug empty",
		},
		{
			name: "ToRef project not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.ToRef.Repository.Project.Key = ""
				return &pr
			},
			err: "project key empty",
		},
		{
			name: "success",
			pr: func() *PullRequest {
				pr := *pr
				pr.ID = 63
				pr.Version = 2
				return &pr
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			name := "DeclinePullRequest-" + strings.ReplaceAll(tc.name, " ", "-")
			cli := NewTestClient(t, name, *update)

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			if tc.err == "" {
				tc.err = "<nil>"
			}
			tc.err = strings.ReplaceAll(tc.err, "${INSTANCEURL}", instanceURL)

			pr := tc.pr()
			err := cli.DeclinePullRequest(tc.ctx, pr)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Fatalf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil || tc.err != "<nil>" {
				return
			}

			checkGolden(t, name, pr)
		})
	}
}

func TestClient_LoadPullRequestActivities(t *testing.T) {
	instanceURL := os.Getenv("BITBUCKET_SERVER_URL")
	if instanceURL == "" {
		instanceURL = "https://bitbucket.sgdev.org"
	}

	cli := NewTestClient(t, "PullRequestActivities", *update)

	timeout, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	pr := &PullRequest{ID: 2}
	pr.ToRef.Repository.Slug = "vegeta"
	pr.ToRef.Repository.Project.Key = "SOUR"

	for _, tc := range []struct {
		name string
		ctx  context.Context
		pr   func() *PullRequest
		err  string
	}{
		{
			name: "timeout",
			pr:   func() *PullRequest { return pr },
			ctx:  timeout,
			err:  "context deadline exceeded",
		},
		{
			name: "repo not set",
			pr:   func() *PullRequest { return &PullRequest{ID: 2} },
			err:  "repository slug empty",
		},
		{
			name: "project not set",
			pr: func() *PullRequest {
				pr := &PullRequest{ID: 2}
				pr.ToRef.Repository.Slug = "vegeta"
				return pr
			},
			err: "project key empty",
		},
		{
			name: "success",
			pr:   func() *PullRequest { return pr },
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
			tc.err = strings.ReplaceAll(tc.err, "${INSTANCEURL}", instanceURL)

			pr := tc.pr()
			err := cli.LoadPullRequestActivities(tc.ctx, pr)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Fatalf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil || tc.err != "<nil>" {
				return
			}

			checkGolden(t, "LoadPullRequestActivities-"+strings.ReplaceAll(tc.name, " ", "-"), pr)
		})
	}
}

func TestClient_CreatePullRequestComment(t *testing.T) {
	instanceURL := os.Getenv("BITBUCKET_SERVER_URL")
	if instanceURL == "" {
		instanceURL = "https://bitbucket.sgdev.org"
	}

	timeout, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	pr := &PullRequest{}
	pr.ToRef.Repository.Slug = "automation-testing"
	pr.ToRef.Repository.Project.Key = "SOUR"

	for _, tc := range []struct {
		name string
		ctx  context.Context
		pr   func() *PullRequest
		err  string
	}{
		{
			name: "timeout",
			pr:   func() *PullRequest { return pr },
			ctx:  timeout,
			err:  "context deadline exceeded",
		},
		{
			name: "ToRef repo not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.ToRef.Repository.Slug = ""
				return &pr
			},
			err: "repository slug empty",
		},
		{
			name: "ToRef project not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.ToRef.Repository.Project.Key = ""
				return &pr
			},
			err: "project key empty",
		},
		{
			name: "success",
			pr: func() *PullRequest {
				pr := *pr
				pr.ID = 63
				pr.Version = 2
				return &pr
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			name := "CreatePullRequestComment-" + strings.ReplaceAll(tc.name, " ", "-")
			cli := NewTestClient(t, name, *update)

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			if tc.err == "" {
				tc.err = "<nil>"
			}
			tc.err = strings.ReplaceAll(tc.err, "${INSTANCEURL}", instanceURL)

			pr := tc.pr()
			err := cli.CreatePullRequestComment(tc.ctx, pr, "test_comment")
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Fatalf("error:\nhave: %q\nwant: %q", have, want)
			}
		})
	}
}

func TestClient_MergePullRequest(t *testing.T) {
	instanceURL := os.Getenv("BITBUCKET_SERVER_URL")
	if instanceURL == "" {
		instanceURL = "https://bitbucket.sgdev.org"
	}

	timeout, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	pr := &PullRequest{}
	pr.ToRef.Repository.Slug = "automation-testing"
	pr.ToRef.Repository.Project.Key = "SOUR"

	for _, tc := range []struct {
		name string
		ctx  context.Context
		pr   func() *PullRequest
		err  string
	}{
		{
			name: "timeout",
			pr:   func() *PullRequest { return pr },
			ctx:  timeout,
			err:  "context deadline exceeded",
		},
		{
			name: "ToRef repo not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.ToRef.Repository.Slug = ""
				return &pr
			},
			err: "repository slug empty",
		},
		{
			name: "ToRef project not set",
			pr: func() *PullRequest {
				pr := *pr
				pr.ToRef.Repository.Project.Key = ""
				return &pr
			},
			err: "project key empty",
		},
		{
			name: "success",
			pr: func() *PullRequest {
				pr := *pr
				pr.ID = 146
				pr.Version = 0
				return &pr
			},
		},
		{
			name: "not mergeable",
			pr: func() *PullRequest {
				pr := *pr
				pr.ID = 154
				pr.Version = 16
				return &pr
			},
			err: "com.atlassian.bitbucket.pull.PullRequestMergeVetoedException",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			name := "MergePullRequest-" + strings.ReplaceAll(tc.name, " ", "-")

			cli := NewTestClient(t, name, *update)

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			if tc.err == "" {
				tc.err = "<nil>"
			}
			tc.err = strings.ReplaceAll(tc.err, "${INSTANCEURL}", instanceURL)

			pr := tc.pr()
			err := cli.MergePullRequest(tc.ctx, pr)
			if have, want := fmt.Sprint(err), tc.err; !strings.Contains(have, want) {
				t.Fatalf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil || tc.err != "<nil>" {
				return
			}

			checkGolden(t, name, pr)
		})
	}
}

// NOTE: This test validates that correct repository IDs are returned from the
// roaring bitmap permissions endpoint. Therefore, the expected results are
// dependent on the user token supplied. The current golden files are generated
// from using the account zoom@sourcegraph.com on bitbucket.sgdev.org.
func TestClient_RepoIDs(t *testing.T) {
	cli := NewTestClient(t, "RepoIDs", *update)

	ids, err := cli.RepoIDs(context.Background(), "READ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checkGolden(t, "RepoIDs", ids)
}

func checkGolden(t *testing.T, name string, got any) {
	t.Helper()

	data, err := json.MarshalIndent(got, " ", " ")
	if err != nil {
		t.Fatal(err)
	}

	path := "testdata/golden/" + name
	if *update {
		if err = os.WriteFile(path, data, 0o640); err != nil {
			t.Fatalf("failed to update golden file %q: %s", path, err)
		}
	}

	golden, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read golden file %q: %s", path, err)
	}

	if have, want := string(data), string(golden); have != want {
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(have, want, false)
		t.Error(dmp.DiffPrettyText(diffs))
	}
}

func TestAuth(t *testing.T) {
	t.Run("auth from config", func(t *testing.T) {
		// Ensure that the different configuration types create the right
		// implicit Authenticator.
		t.Run("bearer token", func(t *testing.T) {
			client, err := NewClient("urn", &schema.BitbucketServerConnection{
				Url:   "http://example.com/",
				Token: "foo",
			}, nil)
			if err != nil {
				t.Fatal(err)
			}

			want := &auth.OAuthBearerToken{Token: "foo"}
			if have, ok := client.Auth.(*auth.OAuthBearerToken); !ok {
				t.Errorf("unexpected Authenticator: have=%T want=%T", client.Auth, want)
			} else if diff := cmp.Diff(have, want); diff != "" {
				t.Errorf("unexpected token:\n%s", diff)
			}
		})

		t.Run("basic auth", func(t *testing.T) {
			client, err := NewClient("urn", &schema.BitbucketServerConnection{
				Url:      "http://example.com/",
				Username: "foo",
				Password: "bar",
			}, nil)
			if err != nil {
				t.Fatal(err)
			}

			want := &auth.BasicAuth{Username: "foo", Password: "bar"}
			if have, ok := client.Auth.(*auth.BasicAuth); !ok {
				t.Errorf("unexpected Authenticator: have=%T want=%T", client.Auth, want)
			} else if diff := cmp.Diff(have, want); diff != "" {
				t.Errorf("unexpected token:\n%s", diff)
			}
		})

		t.Run("OAuth 1 error", func(t *testing.T) {
			if _, err := NewClient("urn", &schema.BitbucketServerConnection{
				Url: "http://example.com/",
				Authorization: &schema.BitbucketServerAuthorization{
					Oauth: schema.BitbucketServerOAuth{
						ConsumerKey: "foo",
						SigningKey:  "this is an invalid key",
					},
				},
			}, nil); err == nil {
				t.Error("unexpected nil error")
			}
		})

		t.Run("OAuth 1", func(t *testing.T) {
			// Generate a plausible enough key with as little entropy as
			// possible just to get through the SetOAuth validation.
			key, err := rsa.GenerateKey(rand.Reader, 64)
			if err != nil {
				t.Fatal(err)
			}
			block := x509.MarshalPKCS1PrivateKey(key)
			pemKey := pem.EncodeToMemory(&pem.Block{Bytes: block})
			signingKey := base64.StdEncoding.EncodeToString(pemKey)

			client, err := NewClient("urn", &schema.BitbucketServerConnection{
				Url: "http://example.com/",
				Authorization: &schema.BitbucketServerAuthorization{
					Oauth: schema.BitbucketServerOAuth{
						ConsumerKey: "foo",
						SigningKey:  signingKey,
					},
				},
			}, nil)
			if err != nil {
				t.Fatal(err)
			}

			if have, ok := client.Auth.(*SudoableOAuthClient); !ok {
				t.Errorf("unexpected Authenticator: have=%T want=%T", client.Auth, &SudoableOAuthClient{})
			} else if have.Client.Client.Credentials.Token != "foo" {
				t.Errorf("unexpected token: have=%q want=%q", have.Client.Client.Credentials.Token, "foo")
			} else if !key.Equal(have.Client.Client.PrivateKey) {
				t.Errorf("unexpected key: have=%v want=%v", have.Client.Client.PrivateKey, key)
			}
		})
	})

	t.Run("Username", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			for name, tc := range map[string]struct {
				a    auth.Authenticator
				want string
			}{
				"OAuth 1 without Sudo": {
					a:    &SudoableOAuthClient{},
					want: "",
				},
				"OAuth 1 with Sudo": {
					a:    &SudoableOAuthClient{Username: "foo"},
					want: "foo",
				},
				"BasicAuth": {
					a:    &auth.BasicAuth{Username: "foo"},
					want: "foo",
				},
			} {
				t.Run(name, func(t *testing.T) {
					client := &Client{Auth: tc.a}
					have, err := client.Username()
					if err != nil {
						t.Errorf("unexpected non-nil error: %v", err)
					}
					if have != tc.want {
						t.Errorf("unexpected username: have=%q want=%q", have, tc.want)
					}
				})
			}
		})

		t.Run("errors", func(t *testing.T) {
			for name, a := range map[string]auth.Authenticator{
				"OAuth 2 token": &auth.OAuthBearerToken{Token: "abcdef"},
				"nil":           nil,
			} {
				t.Run(name, func(t *testing.T) {
					client := &Client{Auth: a}
					if _, err := client.Username(); err == nil {
						t.Errorf("unexpected nil error: %v", err)
					}
				})
			}
		})
	})
}

func TestClient_WithAuthenticator(t *testing.T) {
	uri, err := url.Parse("https://bbs.example.com")
	if err != nil {
		t.Fatal(err)
	}

	old := &Client{
		URL:       uri,
		rateLimit: &ratelimit.InstrumentedLimiter{Limiter: rate.NewLimiter(10, 10)},
		Auth:      &auth.BasicAuth{Username: "johnsson", Password: "mothersmaidenname"},
	}

	newToken := &auth.OAuthBearerToken{Token: "new_token"}
	newClient := old.WithAuthenticator(newToken)
	if old == newClient {
		t.Fatal("both clients have the same address")
	}

	if newClient.Auth != newToken {
		t.Fatalf("auth: want %p but got %p", newToken, newClient.Auth)
	}

	if newClient.URL != old.URL {
		t.Fatalf("url: want %q but got %q", old.URL, newClient.URL)
	}

	if newClient.rateLimit != old.rateLimit {
		t.Fatalf("RateLimit: want %#v but got %#v", old.rateLimit, newClient.rateLimit)
	}
}

func TestClient_GetVersion(t *testing.T) {
	fixture := "GetVersion"
	cli := NewTestClient(t, fixture, *update)

	have, err := cli.GetVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if want := "7.11.2"; have != want {
		t.Fatalf("wrong version. want=%s, have=%s", want, have)
	}
}

func TestClient_CreateFork(t *testing.T) {
	ctx := context.Background()

	fixture := "CreateFork"
	cli := NewTestClient(t, fixture, *update)

	have, err := cli.Fork(ctx, "SGDEMO", "go", CreateForkInput{})
	assert.Nil(t, err)
	assert.NotNil(t, have)
	assert.Equal(t, "go", have.Slug)
	assert.NotEqual(t, "SGDEMO", have.Project.Key)

	checkGolden(t, fixture, have)
}

func TestClient_ProjectRepos(t *testing.T) {
	cli := NewTestClient(t, "ProjectRepos", *update)

	// Empty project key should cause an error
	_, err := cli.ProjectRepos(context.Background(), "")
	if err == nil {
		t.Fatal("Empty projectKey should cause an error", err)
	}

	repos, err := cli.ProjectRepos(context.Background(), "SGDEMO")
	if err != nil {
		t.Fatal("Error during getting SGDEMO project repos", err)
	}

	checkGolden(t, "ProjectRepos", repos)
}

func TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Verbose() {
		log15.Root().SetHandler(log15.LvlFilterHandler(log15.LvlError, log15.Root().GetHandler()))
	}
	os.Exit(m.Run())
}
