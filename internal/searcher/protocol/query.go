package protocol

import (
	"fmt"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/search/query"
	proto "github.com/sourcegraph/sourcegraph/internal/searcher/v1"
)

type QueryNode interface {
	String() string
	ToProto() *proto.QueryNode
}

func FromJobNode(node query.Node) QueryNode {
	// In searcher, nil queries are represented as empty regex patterns
	if node == nil {
		return &PatternNode{
			Value:    "",
			IsRegExp: true,
		}
	}

	switch n := node.(type) {
	case query.Pattern:
		return &PatternNode{
			Value:     n.Value,
			IsNegated: n.Negated,
			IsRegExp:  n.IsRegExp(),
			Boost:     n.Annotation.Labels.IsSet(query.Boost),
		}
	case query.Operator:
		children := make([]QueryNode, 0, len(n.Operands))
		for _, operand := range n.Operands {
			children = append(children, FromJobNode(operand))
		}

		switch n.Kind {
		case query.And:
			return &AndNode{Children: children}
		case query.Or:
			return &OrNode{Children: children}
		default:
			panic(fmt.Sprintf("unexpected operator kind (%d)", n.Kind))
		}
	default:
		// Use a panic since this is used in a struct initializer, and it's really
		// verbose to pass through the error
		panic(fmt.Sprintf("unexpected query node type (%T)", n))
	}
}

type AndNode struct {
	Children []QueryNode
}

func (an *AndNode) String() string {
	cs := make([]string, 0, len(an.Children))
	for _, child := range an.Children {
		cs = append(cs, child.String())
	}
	return "(" + strings.Join(cs, " AND ") + ")"
}

func (an *AndNode) ToProto() *proto.QueryNode {
	children := make([]*proto.QueryNode, 0, len(an.Children))
	for _, child := range an.Children {
		children = append(children, child.ToProto())
	}
	return &proto.QueryNode{
		Value: &proto.QueryNode_And{
			And: &proto.AndNode{Children: children},
		},
	}
}

type OrNode struct {
	Children []QueryNode
}

func (on *OrNode) String() string {
	cs := make([]string, 0, len(on.Children))
	for _, child := range on.Children {
		cs = append(cs, child.String())
	}
	return "(" + strings.Join(cs, " OR ") + ")"
}

func (an *OrNode) ToProto() *proto.QueryNode {
	children := make([]*proto.QueryNode, 0, len(an.Children))
	for _, child := range an.Children {
		children = append(children, child.ToProto())
	}
	return &proto.QueryNode{
		Value: &proto.QueryNode_Or{
			Or: &proto.OrNode{Children: children},
		},
	}
}

type PatternNode struct {
	// Value is the search query. It is a regular expression if IsRegExp
	// is true, otherwise a fixed string. eg "route variable"
	Value string

	// IsNegated if true will invert the matching logic for regexp searches. IsNegated=true is
	// not supported for structural searches.
	IsNegated bool

	// IsRegExp if true will treat the Value as a regular expression.
	IsRegExp bool

	// Boost indicates whether this pattern should have its score boosted in Zoekt ranking
	Boost bool
}

func (rn *PatternNode) String() string {
	var prefix string
	if rn.IsNegated {
		prefix = "NOT "
	}

	if rn.IsRegExp {
		return fmt.Sprintf(`%s/%s/`, prefix, rn.Value)
	} else {
		return fmt.Sprintf(`%s"%s"`, prefix, rn.Value)
	}
}

func (rn *PatternNode) ToProto() *proto.QueryNode {
	return &proto.QueryNode{
		Value: &proto.QueryNode_Pattern{
			Pattern: &proto.PatternNode{
				Value:     rn.Value,
				IsNegated: rn.IsNegated,
				IsRegexp:  rn.IsRegExp,
				Boost:     rn.Boost,
			},
		},
	}
}
