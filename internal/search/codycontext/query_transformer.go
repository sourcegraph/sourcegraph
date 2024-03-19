package codycontext

import (
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const maxTransformedPatterns = 10

type keywordQuery struct {
	query    query.Basic
	patterns []string
}

func concatNodeToPatterns(concat query.Operator) []string {
	patterns := make([]string, 0, len(concat.Operands))
	for _, operand := range concat.Operands {
		pattern, ok := operand.(query.Pattern)
		if ok {
			patterns = append(patterns, pattern.Value)
		}
	}
	return patterns
}

func nodeToPatternsAndParameters(rootNode query.Node) ([]string, []query.Parameter) {
	operator, ok := rootNode.(query.Operator)
	if !ok {
		return nil, nil
	}

	patterns := []string{}
	parameters := []query.Parameter{
		// Only search file content
		{Field: query.FieldType, Value: "file"},
	}

	switch operator.Kind {
	case query.And:
		for _, operand := range operator.Operands {
			switch op := operand.(type) {
			case query.Operator:
				if op.Kind == query.Concat {
					patterns = append(patterns, concatNodeToPatterns(op)...)
				}
			case query.Parameter:
				if op.Field == query.FieldContent {
					// Split any content field on white space into a set of patterns
					patterns = append(patterns, strings.Fields(op.Value)...)
				} else if op.Field != query.FieldCase && op.Field != query.FieldType {
					parameters = append(parameters, op)
				}
			case query.Pattern:
				patterns = append(patterns, op.Value)
			}
		}
	case query.Concat:
		patterns = concatNodeToPatterns(operator)
	}

	return patterns, parameters
}

// transformPatterns applies stops words and stemming. The returned slice
// contains the lowercased patterns and their stems minus the stop words.
func transformPatterns(patterns []string) []string {
	var transformedPatterns []string
	transformedPatternsSet := stringSet{}

	// To eliminate a possible source of non-determinism of search results, we
	// want transformPatterns to be a pure function. Hence we maintain a slice
	// of transformed patterns (transformedPatterns) in addition to
	// transformedPatternsSet.
	add := func(pattern string) {
		if transformedPatternsSet.Has(pattern) {
			return
		}
		transformedPatternsSet.Add(pattern)
		transformedPatterns = append(transformedPatterns, pattern)
	}

	for _, pattern := range patterns {
		pattern = strings.ToLower(pattern)
		pattern = removePunctuation(pattern)
		if len(pattern) < 3 || isCommonTerm(pattern) {
			continue
		}

		pattern = stemTerm(pattern)
		add(pattern)
	}

	// To maintain decent latency, limit the number of patterns we search.
	if len(transformedPatterns) > maxTransformedPatterns {
		transformedPatterns = transformedPatterns[:maxTransformedPatterns]
	}

	return transformedPatterns
}

func queryStringToKeywordQuery(queryString string) (*keywordQuery, error) {
	rawParseTree, err := query.Parse(queryString, query.SearchTypeStandard)
	if err != nil {
		return nil, err
	}

	if len(rawParseTree) != 1 {
		return nil, errors.New("The 'codycontext' patterntype does not support multiple clauses")
	}

	patterns, parameters := nodeToPatternsAndParameters(rawParseTree[0])

	transformedPatterns := transformPatterns(patterns)
	var nodes []query.Node
	for _, p := range parameters {
		nodes = append(nodes, p)
	}

	patternNodes := make([]query.Node, 0, len(transformedPatterns))
	for _, p := range transformedPatterns {
		node := query.Pattern{Value: p}
		node.Annotation.Labels.Set(query.Literal)
		patternNodes = append(patternNodes, node)
	}

	if len(patternNodes) > 0 {
		nodes = append(nodes, query.NewOperator(patternNodes, query.Or)...)
	}

	newNodes, err := query.Sequence(query.For(query.SearchTypeStandard))(nodes)
	if err != nil {
		return nil, err
	}

	newBasic, err := query.ToBasicQuery(newNodes)
	if err != nil {
		return nil, err
	}

	return &keywordQuery{newBasic, transformedPatterns}, nil
}
