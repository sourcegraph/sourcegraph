package pathexistence

import (
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMakeTree(t *testing.T) {
	root := makeTree("projects/lsif-go", []string{
		"go.mod",
		"go.sum",
		"LICENSE",
		"README.md",
		"cmd/lsif-go/main.go",
		"internal/gomod/module.go",
		"internal/index/astutil.go",
		"internal/index/helper.go",
		"internal/index/indexer.go",
		"internal/index/types.go",
		"protocol/protocol.go",
		"protocol/writer.go",
	})
	canonicalizeDirTreeNode(root)

	expected := DirTreeNode{
		"", []DirTreeNode{
			{
				"projects", []DirTreeNode{
					{
						"lsif-go", []DirTreeNode{
							{
								"cmd", []DirTreeNode{
									{"lsif-go", nil},
								},
							},
							{
								"internal", []DirTreeNode{
									{"gomod", nil},
									{"index", nil},
								},
							},
							{"protocol", nil},
						},
					},
				},
			},
		},
	}

	if diff := cmp.Diff(expected, root); diff != "" {
		t.Errorf("unexpected file tree (-want +got):\n%s", diff)
	}
}

// canonicalizeDirTreeNode sorts children of each node in the given
// subtree by the directory name. This is necessary for tests as the
// order of slices matters in comparison, but not in dirtree usage.
func canonicalizeDirTreeNode(n DirTreeNode) {
	for _, child := range n.Children {
		canonicalizeDirTreeNode(child)
	}

	sort.Slice(n.Children, func(i, j int) bool {
		return strings.Compare(n.Children[i].Name, n.Children[j].Name) < 0
	})
}
