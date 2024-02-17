package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tomnomnom/linkheader"

	"github.com/sourcegraph/sourcegraph/internal/httpcli"
)

func TestEnforceAuthViaGitLab(t *testing.T) {
	type testCase struct {
		description        string
		query              url.Values
		repoName           string
		expectedStatusCode int
		expectedErr        error
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		makeLinkHeader := func(cursor string) string {
			urlWithCursor := fmt.Sprintf("%s?cursor=%s", gitlabURL, cursor)
			link := linkheader.Link{URL: urlWithCursor, Rel: "next"}
			return link.String()
		}

		switch r.URL.Query().Get("cursor") {
		case "":
			w.Header().Add("Link", makeLinkHeader("c1"))
			w.Write([]byte(`[{"id": 34949794, "path_with_namespace": "efritz/test"}]`))
		case "c1":
			w.Header().Add("Link", makeLinkHeader("c2"))
			w.Write([]byte(`[{"id": 34949798, "path_with_namespace": "efritz/test2"}]`))
		case "c2":
			w.Write([]byte(`[]`))
		}
	}))
	defer ts.Close()
	gitlabURL, _ = url.Parse(ts.URL)

	testCases := []testCase{
		{
			description: "authorized",
			query:       url.Values{"gitlab_token": []string{"hunter2"}},
			repoName:    "gitlab.com/efritz/test",
			expectedErr: nil,
		},
		{
			description: "authorized (second page)",
			query:       url.Values{"gitlab_token": []string{"hunter2"}},
			repoName:    "gitlab.com/efritz/test2",
			expectedErr: nil,
		},
		{
			description:        "unauthorized (no token supplied)",
			query:              nil,
			repoName:           "gitlab.com/efritz/test",
			expectedStatusCode: http.StatusUnauthorized,
			expectedErr:        ErrGitLabMissingToken,
		},
		{
			description:        "unauthorized (repo not in result set)",
			query:              url.Values{"gitlab_token": []string{"hunter2"}},
			repoName:           "gitlab.com/efritz/test3",
			expectedStatusCode: http.StatusUnauthorized,
			expectedErr:        ErrGitLabUnauthorized,
		},
	}

	cli, err := httpcli.NewFactory(nil).Doer()
	require.NoError(t, err)

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			statusCode, err := enforceAuthViaGitLab(context.Background(), cli, testCase.query, testCase.repoName)
			if statusCode != testCase.expectedStatusCode {
				t.Errorf("unexpected status code. want=%d have=%d", testCase.expectedStatusCode, statusCode)
			}
			if ((err == nil) != (testCase.expectedErr == nil)) || (err != nil && testCase.expectedErr != nil && err.Error() != testCase.expectedErr.Error()) {
				t.Errorf("unexpected error. want=%s have=%s", testCase.expectedErr, err)
			}
		})
	}
}
