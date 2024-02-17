package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/tomnomnom/linkheader"

	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	ErrGitLabMissingToken = errors.New("must provide gitlab_token")
	ErrGitLabUnauthorized = errors.New("you do not have write permission to this GitLab project")

	// see https://docs.gitlab.com/ee/api/projects.html#list-all-projects
	gitlabURL = &url.URL{Scheme: "https", Host: "gitlab.com", Path: "/api/v4/projects"}
)

func enforceAuthViaGitLab(ctx context.Context, doer httpcli.Doer, query url.Values, repoName string) (statusCode int, err error) {
	gitlabToken := query.Get("gitlab_token")
	if gitlabToken == "" {
		return http.StatusUnauthorized, ErrGitLabMissingToken
	}

	projectWithNamespace := strings.TrimPrefix(repoName, "gitlab.com/")

	values := url.Values{}
	values.Set("membership", "true")     // Only projects that the current user is a member of
	values.Set("min_access_level", "30") // Only if current user has minimal access level (30=dev, ..., owner=50)
	values.Set("simple", "true")         // Return only limited fields for each project
	values.Set("per_page", "1")          // TODO: for testing only

	// Enable keyset pagination
	// see https://docs.gitlab.com/ee/api/index.html#keyset-based-pagination
	values.Set("pagination", "keyset")
	values.Set("order_by", "id")
	values.Set("sort", "asc")

	// Build url of initial page of results
	urlCopy := *gitlabURL
	urlCopy.RawQuery = values.Encode()
	nextURL := urlCopy.String()

	for nextURL != "" {
		// Get current page of results, and prep the loop for the next iteration. If after
		// this page we haven't found the project with the target name, we'll make a subsequent
		// query.

		var projects []string
		projects, nextURL, err = requestGitlabProjects(ctx, doer, nextURL, gitlabToken)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		for _, name := range projects {
			if name == projectWithNamespace {
				// Authorized
				return 0, nil
			}
		}
	}

	return http.StatusUnauthorized, ErrGitLabUnauthorized
}

var _ AuthValidator = enforceAuthViaGitLab

func requestGitlabProjects(ctx context.Context, doer httpcli.Doer, url, token string) (_ []string, nextPage string, _ error) {
	// Construct request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Add("PRIVATE-TOKEN", token)

	// Perform request
	resp, err := doer.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return nil, "", errors.Wrap(errors.Newf("http status %d: %s", resp.StatusCode, body), "gitlab error")
	}

	var projects []struct {
		Name string `json:"path_with_namespace"`
	}

	// Decode payload
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, "", err
	}

	// Extract project names
	names := make([]string, 0, len(projects))
	for _, project := range projects {
		names = append(names, project.Name)
	}

	// Extract next link header if there are more results
	for _, link := range linkheader.Parse(resp.Header.Get("Link")) {
		if link.Rel == "next" {
			return names, link.URL, nil
		}
	}

	// Return last page of results if no link header matched the target rel
	return names, "", nil
}
