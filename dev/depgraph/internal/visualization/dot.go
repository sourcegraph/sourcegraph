package visualization

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Dotify serializes the given package and edge data into a DOT-formatted graph.
func Dotify(packages []string, dependencyEdges, dependentEdges map[string][]string) string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "digraph deps {\n")

	pathTree := &treeNode{
		children: map[string]*treeNode{
			"": nestPaths("", getAllIntermediatePaths(packages)),
		},
	}
	displayPackageTree(buf, pathTree, packages, 1)

	for k, vs := range dependencyEdges {
		for _, v := range vs {
			fmt.Fprintf(buf, "    %s -> %s [fillcolor=red]\n", normalize(k), normalize(v))
		}
	}
	for k, vs := range dependentEdges {
		for _, v := range vs {
			fmt.Fprintf(buf, "    %s -> %s [fillcolor=blue]\n", normalize(v), normalize(k))
		}
	}

	fmt.Fprintf(buf, "}\n")
	return buf.String()
}

func displayPackageTree(buf *bytes.Buffer, node *treeNode, packages []string, level int) {
	for pkg, children := range node.children {
		if len(children.children) == 0 {
			fmt.Fprintf(buf, "%s%s [label=\"%s\"]\n", indent(level), normalize(pkg), labelize(pkg))
		} else {
			fmt.Fprintf(buf, "%ssubgraph cluster_%s {\n", indent(level), normalize(pkg))
			fmt.Fprintf(buf, "%slabel = \"%s\"\n", indent(level+1), labelize(pkg))

			found := false
			for _, node := range packages {
				if pkg == node {
					found = true
					break
				}
			}
			if found {
				fmt.Fprintf(buf, "%s%s [label=\"%s\"]\n", indent(level+1), normalize(pkg), labelize(pkg))
			}

			displayPackageTree(buf, children, packages, level+1)
			fmt.Fprintf(buf, "%s}\n", indent(level))
		}
	}
}

func indent(level int) string {
	return strings.Repeat(" ", 4*level)
}

// getAllIntermediatePaths calls getIntermediatePaths on the given values, then
// deduplicates and orders the results.
func getAllIntermediatePaths(pkgs []string) []string {
	uniques := map[string]struct{}{}
	for _, pkg := range pkgs {
		for _, pkg := range getIntermediatePaths(pkg) {
			uniques[pkg] = struct{}{}
		}
	}

	flattened := make([]string, 0, len(uniques))
	for key := range uniques {
		flattened = append(flattened, key)
	}
	sort.Strings(flattened)

	return flattened
}

// getIntermediatePaths returns all proper (path) prefixes of the given package.
// For example, a/b/c will return the set containing {a/b/c, a/b, a}.
func getIntermediatePaths(pkg string) []string {
	if dirname := filepath.Dir(pkg); dirname != "." {
		return append([]string{pkg}, getIntermediatePaths(dirname)...)
	}

	return []string{pkg}
}

type treeNode struct {
	children map[string]*treeNode
}

// nestPaths constructs the treeNode forming the subtree rooted at the given prefix.
func nestPaths(prefix string, pkgs []string) *treeNode {
	nodes := map[string]*treeNode{}

outer:
	for _, pkg := range pkgs {
		// Skip self and anything not within the current prefix
		if pkg == prefix || !isParent(pkg, prefix) {
			continue
		}

		// Skip anything already claimed by this level
		for prefix := range nodes {
			if isParent(pkg, prefix) {
				continue outer
			}
		}

		nodes[pkg] = nestPaths(pkg, pkgs)
	}

	return &treeNode{nodes}
}

// isParent returns true if child is a proper (path) suffix of parent.
func isParent(child, parent string) bool {
	return parent == "" || strings.HasPrefix(child, parent+"/")
}

// labelize returns the last segment of the given package path.
func labelize(pkg string) string {
	if pkg == "" {
		pkg = "sg/sg"
	}

	return filepath.Base(pkg)
}

var nonAlphaPattern = regexp.MustCompile(`[^a-z]`)

// normalize makes a package path suitable for a dot node name.
func normalize(pkg string) string {
	if pkg == "" {
		pkg = "sg/sg"
	}

	return nonAlphaPattern.ReplaceAllString(pkg, "_")
}
