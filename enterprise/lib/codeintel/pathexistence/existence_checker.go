package pathexistence

import (
	"context"
	"path/filepath"
)

type ExistenceChecker struct {
	root              string
	directoryContents map[string]StringSet
}

// NewExistenceChecker constructs a map of directory contents from the given set of paths and the given
// getChildren function pointer that determines which of the given paths exist in the git clone at the
// target commit.
func NewExistenceChecker(ctx context.Context, root string, paths []string, getChildren GetChildrenFunc) (*ExistenceChecker, error) {
	directoryContents, err := directoryContents(ctx, root, paths, getChildren)
	if err != nil {
		return nil, err
	}

	return &ExistenceChecker{root, directoryContents}, nil
}

// Exists determines if the given path exists in the git clone at the target commit.
func (ec *ExistenceChecker) Exists(path string) bool {
	path = filepath.Join(ec.root, path)

	if children, ok := ec.directoryContents[dirWithoutDot(path)]; ok {
		if _, ok := children[path]; ok {
			return true
		}
	}

	return false
}
