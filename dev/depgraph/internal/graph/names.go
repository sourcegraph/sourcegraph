package graph

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// parseNames returns a map from package paths to the name declared by that package.
func parseNames(root string, packageMap map[string]struct{}) (map[string][]string, error) {
	names := map[string][]string{}
	for pkg := range packageMap {
		fileInfos, err := os.ReadDir(filepath.Join(root, pkg))
		if err != nil {
			return nil, err
		}

		importMap := map[string]struct{}{}
		for _, info := range fileInfos {
			if info.IsDir() || filepath.Ext(info.Name()) != ".go" {
				continue
			}

			imports, err := extractPackageName(filepath.Join(root, pkg, info.Name()))
			if err != nil {
				return nil, err
			}
			importMap[imports] = struct{}{}
		}

		flattened := make([]string, 0, len(importMap))
		for name := range importMap {
			flattened = append(flattened, name)
		}
		sort.Strings(flattened)

		if len(flattened) == 1 || (len(flattened) == 2 && flattened[0]+"_test" == flattened[1]) {
			names[pkg] = []string{flattened[0]}
			continue
		} else if len(flattened) > 1 {
			names[pkg] = flattened
		}
	}

	return names, nil
}

var packagePattern = regexp.MustCompile(`^package (\w+)`)

// extractPackageName returns the package name declared by this file.
func extractPackageName(path string) (string, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	for _, line := range bytes.Split(contents, []byte{'\n'}) {
		if matches := packagePattern.FindSubmatch(line); len(matches) > 0 {
			return string(matches[1]), nil
		}
	}

	return "", nil
}
