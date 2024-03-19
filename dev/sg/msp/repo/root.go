package repo

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var once sync.Once
var repositoryRootValue string
var repositoryRootError error

var ErrNotInsideManagedServices = errors.New("this command must be run in the github.com/sourcegraph/managed-services repository")

// RepositoryRoot caches and returns the value of findRoot.
func repositoryRoot(cwd string) (string, error) {
	once.Do(func() {
		if forcedRoot := os.Getenv("SG_MSP_FORCE_REPO_ROOT"); forcedRoot != "" {
			repositoryRootValue = forcedRoot
		} else {
			repositoryRootValue, repositoryRootError = findRoot(cwd)
		}
	})
	return repositoryRootValue, repositoryRootError
}

// findRoot finds the root path of sourcegraph/managed-services from wd
func findRoot(wd string) (string, error) {
	for {
		contents, err := os.ReadFile(filepath.Join(wd, ".repository"))
		if err == nil {
			for _, line := range strings.Split(string(contents), "\n") {
				if strings.HasPrefix(line, "sourcegraph/managed-services") {
					return wd, nil
				}
			}
		} else if !os.IsNotExist(err) {
			return "", err
		}

		if parent := filepath.Dir(wd); parent != wd {
			wd = parent
			continue
		}

		return "", ErrNotInsideManagedServices
	}
}
