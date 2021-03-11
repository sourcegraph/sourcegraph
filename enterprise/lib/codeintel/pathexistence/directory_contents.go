package pathexistence

import (
	"context"
	"path/filepath"
	"sort"
)

type StringSet map[string]struct{}

// GetChildrenFunc returns a map of directory contents for a set of directory names.
type GetChildrenFunc func(ctx context.Context, dirnames []string) (map[string][]string, error)

// directoryContents takes in a list of files present in an LSIF index and constructs a mapping from
// directory  names to sets containing that directory's contents. This function calls the given
// GetChildrenFunc a minimal number of times (and with minimal argument lengths) by pruning missing
// subtrees from subsequent request batches. This can save a lot of work for large uncommitted subtrees
// (e.g. node_modules).
func directoryContents(ctx context.Context, root string, paths []string, getChildren GetChildrenFunc) (map[string]StringSet, error) {
	contents := map[string]StringSet{}

	for batch := makeInitialRequestBatch(root, paths); len(batch) > 0; batch = batch.next(contents) {
		batchResults, err := getChildren(ctx, batch.dirnames())
		if err != nil {
			return nil, err
		}

		for directory, children := range batchResults {
			if len(children) > 0 {
				v := StringSet{}
				for _, c := range children {
					v[c] = struct{}{}
				}
				contents[directory] = v
			}
		}
	}

	return contents, nil
}

// RequestBatch is a complete set of directory subtrees whose contents can be requested
// from gitserver onn the next request. Each chunk of directory subtrees are keyed by the
// full path to that subtree in the batch.
type RequestBatch map[string][]DirTreeNode

// makeInitialRequestBatch constructs the first batch to request from gitserver.
func makeInitialRequestBatch(root string, paths []string) RequestBatch {
	node := makeTree(root, paths)
	if root != "" {
		// Skip requesting "" if a root is supplied
		return RequestBatch{"": node.Children}
	}

	return RequestBatch{"": []DirTreeNode{node}}
}

// dirnames returns a sorted set of directories (as full paths) from the batch.
func (batch RequestBatch) dirnames() []string {
	var dirnames []string
	for nodeGroupParentPath, nodes := range batch {
		for _, node := range nodes {
			dirnames = append(dirnames, filepath.Join(nodeGroupParentPath, node.Name))
		}
	}
	sort.Strings(dirnames)
	return dirnames
}

// next creates a new batch of requests from the current batch. The subsequent batch will
// contain all children of the first batch that are known to be visible from processing
// the previous batch. The given directory contents map is used to determine if the new
// batch files are visible.
func (batch RequestBatch) next(contents map[string]StringSet) RequestBatch {
	nextBatch := RequestBatch{}
	for nodeGroupPath, nodes := range batch {
		for _, node := range nodes {
			// Determine new map key
			newNodeGroupPath := filepath.Join(nodeGroupPath, node.Name)

			if len(node.Children) > 0 && len(contents[newNodeGroupPath]) > 0 {
				// Has visible children, include in next batch
				nextBatch[newNodeGroupPath] = node.Children
			}
		}
	}

	return nextBatch
}
