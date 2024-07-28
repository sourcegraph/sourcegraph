package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/grafana/regexp"
	"github.com/sourcegraph/sourcegraph/internal/cmd/srcgrep/internal/fastwalk"
)

// filterRepos will filter repos to only return repos matching params. It expects all params to be of type
func filterRepos(repos []string, include, exclude []*regexp.Regexp) []string {
	filtered := repos[:0]
Loop:
	for _, name := range repos {
		for _, inc := range include {
			if !inc.MatchString(name) {
				continue Loop
			}
		}
		for _, exc := range exclude {
			if exc.MatchString(name) {
				continue Loop
			}
		}
		filtered = append(filtered, name)
	}
	return filtered
}

// walkRepos will return all repositories under the current working directory.
// It will not search inside of repositories for submodules/etc. It returns
// repositories sorted such that more recently modified repositories are
// earlier in the slice.
func walkRepos() ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Same calculation done by fastwalk
	numWorkers := 4
	if n := runtime.NumCPU(); n > numWorkers {
		numWorkers = n
	}

	type Repo struct {
		Name string

		// HEAD is the modtime of HEAD. Useful indicator of the last time a
		// repository was used. If we failed to find the modtime of HEAD, HEAD
		// will be the zero time instant.
		HEAD time.Time
	}

	c := make(chan Repo, numWorkers*2) // extra buffering to avoid stalling a worker
	var walkErr error
	go func() {
		defer close(c)
		walkErr = fastwalk.Walk(root, func(path string, typ os.FileMode) error {
			if typ != os.ModeDir {
				return nil
			}

			if base := filepath.Base(path); len(base) > 0 && base[0] == '.' {
				return filepath.SkipDir
			}

			if _, err := os.Stat(filepath.Join(path, ".git")); os.IsNotExist(err) {
				return nil
			}

			name, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}

			var mod time.Time
			if info, err := os.Stat(filepath.Join(path, ".git/HEAD")); err == nil {
				mod = info.ModTime()
			}

			c <- Repo{
				Name: name,
				HEAD: mod,
			}
			return filepath.SkipDir
		})
	}()

	var repos []Repo
	for repo := range c {
		repos = append(repos, repo)
	}

	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(repos, func(i, j int) bool {
		if repos[i].HEAD.Equal(repos[j].HEAD) {
			return repos[i].Name > repos[j].Name
		}
		return repos[i].HEAD.After(repos[j].HEAD)
	})

	names := make([]string, 0, len(repos))
	for _, repo := range repos {
		names = append(names, repo.Name)
	}

	return names, nil
}
