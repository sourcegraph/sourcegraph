package vfsutil

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
)

// NewGitHubRepoVFS creates a new VFS backed by a GitHub downloadable
// repository archive.
func NewGitHubRepoVFS(repo, rev string) (*ArchiveFS, error) {
	if !githubRepoRx.MatchString(repo) {
		return nil, fmt.Errorf(`invalid GitHub repo %q: must be "github.com/user/repo"`, repo)
	}

	url := fmt.Sprintf("https://codeload.%s/zip/%s", repo, rev)
	return NewZipVFS(url, ghFetch.Inc, ghFetchFailed.Inc, false)
}

var githubRepoRx = lazyregexp.New(`^github\.com/[\w.-]{1,100}/[\w.-]{1,100}$`)

var ghFetch = promauto.NewCounter(prometheus.CounterOpts{
	Name: "vfsutil_vfs_github_fetch_total",
	Help: "Total number of fetches by GitHubRepoVFS.",
})

var ghFetchFailed = promauto.NewCounter(prometheus.CounterOpts{
	Name: "vfsutil_vfs_github_fetch_failed_total",
	Help: "Total number of fetches by GitHubRepoVFS that failed.",
})
