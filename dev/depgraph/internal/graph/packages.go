package graph

import (
	"io/fs"
	"path/filepath"
)

// listPackages returns a set of directories relative to the root of the sg/sg
// repository which contain go files. This is a very fast approximation of the
// set of (valid) packages that exist in the source tree.
//
// We also find additional value in false positives as we'd like to not have free
// floating go files in a non-standard directory.
func listPackages(root string) (map[string]struct{}, error) {
	packageMap := map[string]struct{}{}
	if err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && shouldSkipDir(relative(path, root)) {
			return filepath.SkipDir
		}

		if filepath.Ext(path) == ".go" {
			packageMap[relative(filepath.Dir(path), root)] = struct{}{}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return packageMap, nil
}

// listPackages lists path segments skipped by listPackages.
var skipDirectories = []string{
	"testdata",
}

// skipExactPaths lists exact paths skipped by listPackages.
var skipExactPaths = []string{
	"client",
	"ui/assets",
	"node_modules",
}

// shouldSkipDir returns true if the given path should be skipped during a source tree
// traversal looking for go packages. This is to remove unfruitful subtrees which are
// guaranteed not ot have interesting Sourcegraph-authored Go code.
func shouldSkipDir(path string) bool {
	for _, skip := range skipExactPaths {
		if path == skip {
			return true
		}
	}

	for _, skip := range skipDirectories {
		if filepath.Base(path) == skip {
			return true
		}
	}

	return false
}
