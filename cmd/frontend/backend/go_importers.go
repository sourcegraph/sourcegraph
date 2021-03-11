package backend

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context/ctxhttp"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

var MockCountGoImporters func(ctx context.Context, repo api.RepoName) (int, error)

var (
	goImportersCountCache = rcache.NewWithTTL("go-importers-count", 14400) // 4 hours

	countGoImportersHTTPClient *http.Client // mockable in tests
)

// CountGoImporters returns the number of Go importers for the repository's Go subpackages. This is
// a special case used only on Sourcegraph.com for repository badges.
//
// TODO: The import path is not always the same as the repository name.
func CountGoImporters(ctx context.Context, repo api.RepoName) (count int, err error) {
	if MockCountGoImporters != nil {
		return MockCountGoImporters(ctx, repo)
	}

	if !envvar.SourcegraphDotComMode() {
		// Avoid confusing users by exposing this on self-hosted instances, because it relies on the
		// public godoc.org API.
		return 0, errors.New("counting Go importers is not supported on self-hosted instances")
	}

	cacheKey := string(repo)
	b, ok := goImportersCountCache.Get(cacheKey)
	if ok {
		count, err = strconv.Atoi(string(b))
		if err == nil {
			return count, nil // cache hit
		}
		goImportersCountCache.Delete(cacheKey) // remove unexpectedly invalid cache value
	}

	defer func() {
		if err == nil {
			// Store in cache.
			goImportersCountCache.Set(cacheKey, []byte(strconv.Itoa(count)))
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second) // avoid tying up resources unduly
	defer cancel()

	// Find all (possible) Go packages in the repository.
	goPackages, err := listGoPackagesInRepoImprecise(ctx, repo)
	if err != nil {
		return 0, err
	}
	const maxSubpackages = 50 // arbitrary limit to avoid overloading api.godoc.org
	if len(goPackages) > maxSubpackages {
		goPackages = goPackages[:maxSubpackages]
	}

	// Count importers for each of the repository's Go packages.
	//
	// TODO: The count sums together the user counts of all of the repository's subpackages. This
	// overcounts the number of users, because if another project uses multiple subpackages in this
	// repository, it is counted multiple times. This limitation is now documented and will be
	// addressed in the future. See https://github.com/sourcegraph/sourcegraph/issues/2663.
	for _, pkg := range goPackages {
		// Assumes the import path is the same as the repo name - not always true!
		response, err := ctxhttp.Get(ctx, countGoImportersHTTPClient, "https://api.godoc.org/importers/"+pkg)
		if err != nil {
			return 0, err
		}
		defer response.Body.Close()

		var result struct {
			Results []struct {
				Path string
			}
		}
		bytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return 0, err
		}
		err = json.Unmarshal(bytes, &result)
		if err != nil {
			return 0, err
		}
		count += len(result.Results)
	}

	return count, nil
}

// listGoPackagesInRepoImprecise returns a list of import paths for all (probable) Go packages in
// the repository. It computes the list based solely on the repository name (as a prefix) and
// filenames in the repository; it does not parse or build the Go files to determine the list
// precisely.
func listGoPackagesInRepoImprecise(ctx context.Context, repoName api.RepoName) ([]string, error) {
	if !envvar.SourcegraphDotComMode() {
		// 🚨 SECURITY: Avoid leaking information about private repositories that the viewer is not
		// allowed to access.
		return nil, errors.New("listGoPackagesInRepoImprecise is only supported on Sourcegraph.com for public repositories")
	}

	repo, err := Repos.GetByName(ctx, repoName)
	if err != nil {
		return nil, err
	}

	commitID, err := git.ResolveRevision(ctx, repo.Name, "HEAD", git.ResolveRevisionOptions{})
	if err != nil {
		return nil, err
	}
	fis, err := git.ReadDir(ctx, repo.Name, commitID, "", true)
	if err != nil {
		return nil, err
	}

	subpaths := map[string]struct{}{} // all non-vendor/internal/hidden dirs containing *.go files
	for _, fi := range fis {
		if name := fi.Name(); filepath.Ext(name) == ".go" {
			dir := path.Dir(name)
			if isPossibleExternallyImportableGoPackageDir(dir) {
				subpaths[dir] = struct{}{}
			}
		}
	}

	importPaths := make([]string, 0, len(subpaths))
	for subpath := range subpaths {
		importPaths = append(importPaths, path.Join(string(repo.Name), subpath))
	}
	sort.Strings(importPaths)
	return importPaths, nil
}

func isPossibleExternallyImportableGoPackageDir(dirPath string) bool {
	components := strings.Split(dirPath, "/")
	for _, c := range components {
		if (strings.HasPrefix(c, ".") && len(c) > 1) || strings.HasPrefix(c, "_") || c == "vendor" || c == "internal" || c == "testdata" {
			return false
		}
	}
	return true
}
