package protocol

import (
	"path"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/api"
)

func NormalizeRepo(input api.RepoName) api.RepoName {
	repo := string(input)

	// Clean with a "/" so we get out an absolute path
	repo = path.Clean("/" + repo)
	repo = strings.TrimPrefix(repo, "/")

	return api.RepoName(repo)
}
