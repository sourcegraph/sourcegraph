package reposource

import (
	"net/url"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/schema"
)

type Other struct {
	*schema.OtherExternalServiceConnection
}

var _ RepoSource = Other{}

const DefaultRepositoryPathPattern = "{base}/{repo}"

func (c Other) CloneURLToRepoURI(cloneURL string) (string, error) {
	return cloneURLToRepoName(cloneURL, c.Url, DefaultRepositoryPathPattern)
}

func (c Other) CloneURLToRepoName(cloneURL string) (api.RepoName, error) {
	repoName, err := cloneURLToRepoName(cloneURL, c.Url, c.RepositoryPathPattern)
	return api.RepoName(repoName), err
}

func cloneURLToRepoName(cloneURL, baseURL, repositoryPathPattern string) (string, error) {
	parsedCloneURL, parsedBaseURL, match, err := parseURLs(cloneURL, baseURL)
	if err != nil {
		return "", err
	}
	if !match {
		return "", nil
	}

	// For SCP-style clone URLs, the path may not start with a slash
	// e.g. both of the following are valid clone URLs
	// - git@codehost.com:a/b
	// - git@codehost.com:/a/b
	standardizedPath := parsedCloneURL.Path
	if strings.HasPrefix(parsedBaseURL.Path, "/") && !strings.HasPrefix(standardizedPath, "/") {
		standardizedPath = "/" + standardizedPath
	}
	basePrefix := parsedBaseURL.Path
	if !strings.HasSuffix(basePrefix, "/") {
		basePrefix += "/"
	}
	if standardizedPath != parsedBaseURL.Path && !strings.HasPrefix(standardizedPath, basePrefix) {
		return "", nil
	}
	relativeRepoPath := strings.TrimPrefix(standardizedPath, basePrefix)
	base := url.URL{
		Host: parsedBaseURL.Host,
		Path: parsedBaseURL.Path,
	}
	return OtherRepoName(repositoryPathPattern, base.String(), relativeRepoPath), nil
}

var otherRepoNameReplacer = strings.NewReplacer(":", "-", "@", "-", "//", "")

func OtherRepoName(repositoryPathPattern, base, relativeRepoPath string) string {
	if repositoryPathPattern == "" {
		repositoryPathPattern = DefaultRepositoryPathPattern
	}
	return strings.NewReplacer(
		"{base}", otherRepoNameReplacer.Replace(strings.TrimSuffix(base, "/")),
		"{repo}", otherRepoNameReplacer.Replace(strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(relativeRepoPath, "/"), ".git"), "/")),
	).Replace(repositoryPathPattern)
}
