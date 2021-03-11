package graph

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const rootPackage = "github.com/sourcegraph/sourcegraph"

// parseImports returns a map from package paths to the set of (internal) packages that
// package imports.
func parseImports(root string, packageMap map[string]struct{}) (map[string][]string, error) {
	imports := map[string][]string{}
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

			imports, err := extractImports(filepath.Join(root, pkg, info.Name()))
			if err != nil {
				return nil, err
			}
			for pkg := range imports {
				importMap[pkg] = struct{}{}
			}
		}

		flattened := make([]string, 0, len(importMap))
		for pkg := range importMap {
			if strings.HasPrefix(pkg, rootPackage) {
				// internal packages only; omit leading root package prefix
				flattened = append(flattened, strings.TrimPrefix(strings.TrimPrefix(pkg, rootPackage), "/"))
			}
		}
		sort.Strings(flattened)

		imports[pkg] = flattened
	}

	return imports, nil
}

var (
	importPattern           = regexp.MustCompile(`(?:\w+ )?"([^"]+)"`)
	singleImportPattern     = regexp.MustCompile(fmt.Sprintf(`^import %s`, importPattern))
	importGroupStartPattern = regexp.MustCompile(`^import \($`)
	groupedImportPattern    = regexp.MustCompile(fmt.Sprintf(`^\t%s`, importPattern))
	importGroupEndPattern   = regexp.MustCompile(`^\)$`)
)

// extractionControlMap is a map from parse state to the regular expressions that
// are useful in relation to the text within that parse state. The parse state
// distinguishes whether or not the current line of Go code is inside of an import
// group (i.e. `import ( /* this */ )`).
//
// Outside of an import group, we are looking for un-grouped/single-line imports as
// well as the start of a new import group. Inside of an import group, we are looking
// for package paths as well as the end of the current import group.
var extractionControlMap = map[bool]struct {
	stateChangePattern *regexp.Regexp // the line content that flips the parse state
	capturePattern     *regexp.Regexp // the line content that is useful within the current parse state
}{
	true:  {stateChangePattern: importGroupEndPattern, capturePattern: groupedImportPattern},
	false: {stateChangePattern: importGroupStartPattern, capturePattern: singleImportPattern},
}

// extractImports returns a set of package paths that are imported by this file.
func extractImports(path string) (map[string]struct{}, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	inImportGroup := false
	importMap := map[string]struct{}{}
	lines := bytes.Split(contents, []byte{'\n'})

	for _, line := range lines {
		// See if we need to flip parse states
		if extractionControlMap[inImportGroup].stateChangePattern.Match(line) {
			inImportGroup = !inImportGroup
			continue
		}

		// See if we can capture any useful data from this line
		if match := extractionControlMap[inImportGroup].capturePattern.FindSubmatch(line); len(match) > 0 {
			importMap[string(match[1])] = struct{}{}
		}
	}

	return importMap, nil
}
