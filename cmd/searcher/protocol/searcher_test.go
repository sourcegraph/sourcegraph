package protocol_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/searcher/protocol"
)

func TestRequestProtoRoundtrip(t *testing.T) {
	r1 := protocol.Request{
		Repo:   "sourcegraph/zoekt",
		RepoID: 42,
		URL:    "github.com/sourcegraph/zoekt",
		Commit: "abcdef",
		Branch: "HEAD",
		PatternInfo: protocol.PatternInfo{
			Query: &protocol.PatternNode{
				Value:     "pattern",
				IsNegated: true,
			},
			IsStructuralPat:              false,
			IsCaseSensitive:              false,
			ExcludePaths:                 "exclude",
			IncludePaths:                 []string{"include1", "include2"},
			PathPatternsAreCaseSensitive: false,
			Limit:                        0,
			PatternMatchesContent:        false,
			PatternMatchesPath:           false,
			IncludeLangs:                 []string{},
			CombyRule:                    "",
			Select:                       "",
		},
		FetchTimeout:    1000,
		Indexed:         false,
		NumContextLines: 27,
	}

	p1 := r1.ToProto()

	var r2 protocol.Request
	r2.FromProto(p1)
	require.Equal(t, r1, r2)

	p2 := r2.ToProto()
	require.Equal(t, p1, p2)
}

func TestQueryNodeProtoRoundtrip(t *testing.T) {
	err := quick.Check(func(q1 queryGenerator) bool {
		p1 := q1.Query.ToProto()

		var q2 protocol.QueryNode
		q2 = protocol.NodeFromProto(p1)
		require.Equal(t, q1.Query, q2)

		p2 := q2.ToProto()
		require.Equal(t, p1, p2)

		return true
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

// We need to implement this explicitly, because quickcheck cannot generate random
// protocol.QueryNode structs.
type queryGenerator struct {
	Query protocol.QueryNode
}

func (queryGenerator) Generate(rand *rand.Rand, size int) reflect.Value {
	// Set max size to avoid massive trees
	if size > 10 {
		size = 10
	}
	q := generateQuery(rand, size)
	return reflect.ValueOf(queryGenerator{q})
}

// generateQuery generates a random query with configurable depth. Atom,
// And, and Or nodes will occur with a 1:1:1 ratio on average.
func generateQuery(rand *rand.Rand, depth int) protocol.QueryNode {
	if depth == 0 {
		return generateRegexpNode(rand)
	}

	switch rand.Int() % 3 {
	case 0:
		children := make([]protocol.QueryNode, 0)
		for range rand.Int() % 4 {
			children = append(children, generateQuery(rand, depth-1))
		}
		return &protocol.AndNode{Children: children}
	case 1:
		children := make([]protocol.QueryNode, 0)
		for range rand.Int() % 4 {
			children = append(children, generateQuery(rand, depth-1))
		}
		return &protocol.OrNode{Children: children}
	case 2:
		return generateRegexpNode(rand)
	default:
		panic("unreachable")
	}
}

func generateRegexpNode(rand *rand.Rand) protocol.QueryNode {
	return &protocol.PatternNode{
		Value:     "random.*regex",
		IsNegated: rand.Int()%2 == 0,
		IsRegExp:  rand.Int()%2 == 0,
	}
}

func TestFileMatchProtoRoundTrip(t *testing.T) {
	var errString string
	err := quick.Check(func(original protocol.FileMatch) bool {
		var converted protocol.FileMatch
		converted.FromProto(original.ToProto())

		if diff := cmp.Diff(original, converted); diff != "" {
			errString = fmt.Sprintf("unexpected diff (-want +got):\n%s", diff)
			return false
		}

		return true
	}, nil)
	if err != nil {
		t.Fatal(errString)
	}
}
