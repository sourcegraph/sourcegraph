package conversion

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/datastructures"
)

func TestPrune(t *testing.T) {
	gitContentsOracle := map[string][]string{
		"root":     {"root/sub/", "root/foo.go", "root/bar.go"},
		"root/sub": {"root/sub/baz.go"},
	}

	getChildren := func(ctx context.Context, dirnames []string) (map[string][]string, error) {
		out := map[string][]string{}
		for _, dirname := range dirnames {
			out[dirname] = gitContentsOracle[dirname]
		}

		return out, nil
	}

	state := &State{
		DocumentData: map[int]string{
			1001: "foo.go",
			1002: "bar.go",
			1003: "sub/baz.go",
			1004: "foo.generated.go",
			1005: "foo.generated.go",
		},
		DefinitionData: map[int]*datastructures.DefaultIDSetMap{
			2001: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1001: datastructures.NewIDSet(),
				1004: datastructures.NewIDSet(),
			}),
			2002: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1002: datastructures.NewIDSet(),
			}),
		},
		ReferenceData: map[int]*datastructures.DefaultIDSetMap{
			2003: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1002: datastructures.NewIDSet(),
			}),
			2004: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1002: datastructures.NewIDSet(),
				1005: datastructures.NewIDSet(),
			}),
		},
	}

	if err := prune(context.Background(), state, "root", getChildren); err != nil {
		t.Fatalf("unexpected error pruning state: %s", err)
	}

	expectedState := &State{
		DocumentData: map[int]string{
			1001: "foo.go",
			1002: "bar.go",
			1003: "sub/baz.go",
		},
		DefinitionData: map[int]*datastructures.DefaultIDSetMap{
			2001: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1001: datastructures.NewIDSet(),
			}),
			2002: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1002: datastructures.NewIDSet(),
			}),
		},
		ReferenceData: map[int]*datastructures.DefaultIDSetMap{
			2003: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1002: datastructures.NewIDSet(),
			}),
			2004: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1002: datastructures.NewIDSet(),
			}),
		},
	}
	if diff := cmp.Diff(expectedState, state, datastructures.Comparers...); diff != "" {
		t.Errorf("unexpected state (-want +got):\n%s", diff)
	}
}
