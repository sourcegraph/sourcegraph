package pathexistence

import (
	"path/filepath"
	"strings"
)

// DirTree node represents a single directory in a file tree.
type DirTreeNode struct {
	// Name is the path segment indicating the name of this directory. It does
	// not contain any separator characters.
	Name string

	// Children represent the directory subtrees directly nested under this
	// directory. Ordering of this slice is arbitrary.
	Children []DirTreeNode
}

// makeTree creates a file tree (excluding files, directories only) from the given
// set of file paths. The given root is prepended to every path in the list.
func makeTree(root string, paths []string) DirTreeNode {
	directorySet := map[string]struct{}{}
	for _, path := range paths {
		if dir := dirWithoutDot(filepath.Join(root, path)); !strings.HasPrefix(dir, "..") {
			directorySet[dir] = struct{}{}
		}
	}

	tree := DirTreeNode{}
	for dir := range directorySet {
		tree = insertPathSegmentsIntoNode(tree, strings.Split(dir, "/"))
	}

	return tree
}

// insertPathSegmentsIntoNode inserts a directory into a subtree.
func insertPathSegmentsIntoNode(n DirTreeNode, pathSegments []string) DirTreeNode {
	if len(pathSegments) == 0 {
		return n
	}

	for i, c := range n.Children {
		if c.Name == pathSegments[0] {
			// Child matches, insert remainder of path segments in subtree
			n.Children[i] = insertPathSegmentsIntoNode(c, pathSegments[1:])
			return n
		}
	}

	// Unknown directory, create a new subtree
	newChild := DirTreeNode{pathSegments[0], nil}
	// Insert remainder of path segments into new subtree
	n.Children = append(n.Children, insertPathSegmentsIntoNode(newChild, pathSegments[1:]))
	return n
}
