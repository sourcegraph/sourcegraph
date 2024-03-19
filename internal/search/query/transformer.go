package query

import (
	"strconv"
	"strings"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// SubstituteAliases substitutes field name aliases for their canonical names,
// and substitutes `content:` for pattern nodes.
func SubstituteAliases(searchType SearchType) func(nodes []Node) []Node {
	mapper := func(nodes []Node) []Node {
		return MapParameter(nodes, func(field, value string, negated bool, annotation Annotation) Node {
			if field == "content" {
				if searchType == SearchTypeRegex && !annotation.Labels.IsSet(Quoted) {
					annotation.Labels.Set(Regexp)
				} else {
					annotation.Labels.Set(Literal)
				}
				annotation.Labels.Set(IsContent)
				return Pattern{Value: value, Negated: negated, Annotation: annotation}
			}
			if canonical, ok := aliases[field]; ok {
				annotation.Labels.Set(IsAlias)
				field = canonical
			}
			return Parameter{Field: field, Value: value, Negated: negated, Annotation: annotation}
		})
	}
	return mapper
}

// LowercaseFieldNames performs strings.ToLower on every field name.
func LowercaseFieldNames(nodes []Node) []Node {
	return MapParameter(nodes, func(field, value string, negated bool, annotation Annotation) Node {
		return Parameter{Field: strings.ToLower(field), Value: value, Negated: negated, Annotation: annotation}
	})
}

const CountAllLimit = 99999999

var countAllLimitStr = strconv.Itoa(CountAllLimit)

// SubstituteCountAll replaces count:all with count:99999999.
func SubstituteCountAll(nodes []Node) []Node {
	return MapParameter(nodes, func(field, value string, negated bool, annotation Annotation) Node {
		if field == FieldCount && strings.ToLower(value) == "all" {
			c := countAllLimitStr
			return Parameter{Field: field, Value: c, Negated: negated, Annotation: annotation}
		}
		return Parameter{Field: field, Value: value, Negated: negated, Annotation: annotation}
	})
}

func toNodes(parameters []Parameter) []Node {
	nodes := make([]Node, 0, len(parameters))
	for _, p := range parameters {
		nodes = append(nodes, p)
	}
	return nodes
}

// Converts a flat list of nodes to parameters. Invariant: nodes are parameters.
// This function is intended for internal use only, which assumes the invariant.
func toParameters(nodes []Node) []Parameter {
	var parameters []Parameter
	for _, n := range nodes {
		parameters = append(parameters, n.(Parameter))
	}
	return parameters
}

// naturallyOrdered returns true if, reading the query from left to right,
// patterns only appear after parameters. When reverse is true it returns true,
// if, reading from right to left, patterns only appear after parameters.
func naturallyOrdered(node Node, reverse bool) bool {
	// This function looks at the position of the rightmost Parameter and
	// leftmost Pattern range to check ordering (reverse respectively
	// reverses the position tracking). This because term order in the tree
	// structure is not guaranteed at all, even under a consistent traversal
	// (like post-order DFS).
	rightmostParameterPos := 0
	rightmostPatternPos := 0
	leftmostParameterPos := 1 << 30
	leftmostPatternPos := 1 << 30
	v := &Visitor{
		Parameter: func(_, _ string, _ bool, a Annotation) {
			if a.Range.Start.Column > rightmostParameterPos {
				rightmostParameterPos = a.Range.Start.Column
			}
			if a.Range.Start.Column < leftmostParameterPos {
				leftmostParameterPos = a.Range.Start.Column
			}
		},
		Pattern: func(_ string, _ bool, a Annotation) {
			if a.Range.Start.Column > rightmostPatternPos {
				rightmostPatternPos = a.Range.Start.Column
			}
			if a.Range.Start.Column < leftmostPatternPos {
				leftmostPatternPos = a.Range.Start.Column
			}
		},
	}
	v.Visit(node)
	if reverse {
		return leftmostParameterPos > rightmostPatternPos
	}
	return rightmostParameterPos < leftmostPatternPos
}

// Hoist is a heuristic that rewrites simple but possibly ambiguous queries. It
// changes certain expressions in a way that some consider to be more natural.
// For example, the following query without parentheses is interpreted as
// follows in the grammar:
//
// repo:foo a or b and c => (repo:foo a) or ((b) and (c))
//
// This function rewrites the above expression as follows:
//
// repo:foo a or b and c => repo:foo (a or b and c)
//
// For this heuristic to apply, reading the query from left to right, a query
// must start with a contiguous sequence of parameters, followed by contiguous
// sequence of pattern expressions, followed by a contiquous sequence of
// parameters. When this shape holds, the pattern expressions are hoisted out.
//
// Valid example and interpretation:
//
// - repo:foo file:bar a or b and c => repo:foo file:bar (a or b and c)
// - repo:foo a or b file:bar => repo:foo (a or b) file:bar
// - a or b file:bar => file:bar (a or b)
//
// Invalid examples:
//
// - a or repo:foo b => Reading left to right, a parameter is interpolated between patterns
// - a repo:foo or b => As above.
// - repo:foo a or file:bar b => As above.
//
// In invalid cases, we want preserve the default interpretation, which
// corresponds to groupings around `or` expressions, i.e.,
//
// repo:foo a or b or repo:bar c => (repo:foo a) or (b) or (repo:bar c)
func Hoist(nodes []Node) ([]Node, error) {
	if len(nodes) != 1 {
		return nil, errors.Errorf("heuristic requires one top-level expression")
	}

	expression, ok := nodes[0].(Operator)
	if !ok || expression.Kind == Concat {
		return nil, errors.Errorf("heuristic requires top-level and- or or-expression")
	}

	n := len(expression.Operands)
	var pattern []Node
	var scopeParameters []Parameter
	for i, node := range expression.Operands {
		if i == 0 {
			// Handles the case "(and parameter expression)"
			if parameter, ok := node.(Parameter); ok && expression.Kind == And {
				hoisted, err := Hoist(NewOperator(expression.Operands[1:], expression.Kind))
				if err != nil {
					return nil, err
				}
				return append([]Node{parameter}, hoisted...), nil
			}

			scopePart, patternPart, err := PartitionSearchPattern([]Node{node})
			if err != nil || patternPart == nil {
				return nil, errors.New("could not partition first expression")
			}
			if !naturallyOrdered(node, false) {
				return nil, errors.New("unnatural order: patterns not followed by parameter")
			}
			pattern = append(pattern, patternPart)
			scopeParameters = append(scopeParameters, scopePart...)
			continue
		}
		if i == n-1 {
			scopePart, patternPart, err := PartitionSearchPattern([]Node{node})
			if err != nil || patternPart == nil {
				return nil, errors.New("could not partition first expression")
			}
			if !naturallyOrdered(node, true) {
				return nil, errors.New("unnatural order: patterns not followed by parameter")
			}
			pattern = append(pattern, patternPart)
			scopeParameters = append(scopeParameters, scopePart...)
			continue
		}
		if !isPatternExpression([]Node{node}) {
			return nil, errors.Errorf("inner expression %s is not a pure pattern expression", node.String())
		}
		pattern = append(pattern, node)
	}
	pattern = MapPattern(pattern, func(value string, negated bool, annotation Annotation) Node {
		annotation.Labels |= HeuristicHoisted
		return Pattern{Value: value, Negated: negated, Annotation: annotation}
	})
	return append(toNodes(scopeParameters), NewOperator(pattern, expression.Kind)...), nil
}

// distribute applies the distributed property to the parameters of basic
// queries. See the BuildPlan function for context. Its first argument takes
// the current set of prefixes to prepend to each term in an or-expression.
// Importantly, unlike a full DNF, this function does not distribute `or`
// expressions in the pattern.
func distribute(prefixes []Basic, nodes []Node) []Basic {
	for _, node := range nodes {
		switch v := node.(type) {
		case Operator:
			// If the node is all pattern expressions,
			// we can add it to the existing patterns as-is.
			if isPatternExpression(v.Operands) {
				prefixes = product(prefixes, Basic{Pattern: v})
				continue
			}

			switch v.Kind {
			case Or:
				result := make([]Basic, 0, len(prefixes)*len(v.Operands))
				for _, o := range v.Operands {
					newBasics := distribute([]Basic{}, []Node{o})
					for _, newBasic := range newBasics {
						result = append(result, product(prefixes, newBasic)...)
					}
				}
				prefixes = result
			case And, Concat:
				prefixes = distribute(prefixes, v.Operands)
			}
		case Parameter:
			prefixes = product(prefixes, Basic{Parameters: []Parameter{v}})
		case Pattern:
			prefixes = product(prefixes, Basic{Pattern: v})
		}
	}
	return prefixes
}

// product computes a conjunction between toMerge and each of the
// input Basic queries.
func product(basics []Basic, toMerge Basic) []Basic {
	if len(basics) == 0 {
		return []Basic{toMerge}
	}
	result := make([]Basic, len(basics))
	for i, basic := range basics {
		result[i] = conjunction(basic, toMerge)
	}
	return result
}

// conjunction returns a new Basic query that is equivalent to the
// conjunction of the two inputs. The equivalent of combining
// `(repo:a b) and (repo:c d)` into `repo:a repo:c b and d`
func conjunction(left, right Basic) Basic {
	var pattern Node
	if left.Pattern == nil {
		pattern = right.Pattern
	} else if right.Pattern == nil {
		pattern = left.Pattern
	} else if left.Pattern != nil && right.Pattern != nil {
		pattern = NewOperator([]Node{left.Pattern, right.Pattern}, And)[0]
	}
	return Basic{
		// Deep copy parameters to avoid appending multiple times to the same backing array.
		Parameters: append(append([]Parameter{}, left.Parameters...), right.Parameters...),
		Pattern:    pattern,
	}
}

// BuildPlan converts a raw query tree into a set of disjunct basic queries
// (Plan). Note that a basic query can still have a tree structure within its
// pattern node, just not in any of the parameters.
//
// For example, the query
//
//	repo:a (file:b OR file:c)
//
// is transformed to
//
//	(repo:a file:b) OR (repo:a file:c)
//
// but the query
//
//	(repo:a OR repo:b) (b OR c)
//
// is transformed to
//
//	(repo:a (b OR c)) OR (repo:b (b OR c))
func BuildPlan(query []Node) Plan {
	return distribute([]Basic{}, query)
}

// fuzzyRegexp interpolates patterns with .*? regular expressions and
// concatenates them. Invariant: len(patterns) > 0.
func fuzzyRegexp(patterns []Pattern) []Node {
	if len(patterns) == 1 {
		return []Node{patterns[0]}
	}
	var values []string
	for _, p := range patterns {
		values = append(values, p.RegExpPattern())
	}
	return []Node{
		Pattern{
			Annotation: Annotation{Labels: Regexp},
			Value:      "(?:" + strings.Join(values, ").*?(?:") + ")",
		},
	}
}

// standard reduces a sequence of Patterns such that:
//
// - adjacent literal patterns are concatenated with space. I.e., contiguous
// literal patterns are joined on space to create one literal pattern.
//
// - any patterns adjacent to regular expression patterns are AND-ed.
//
// Here are concrete examples of input strings and equivalent transformation.
// I'm using the `content` field for literal patterns to explicitly delineate
// how those are processed.
//
// `/foo/ /bar/ baz` -> (/foo/ AND /bar/ AND content:"baz")
// `/foo/ bar baz` -> (/foo/ AND content:"bar baz")
// `/foo/ bar /baz/` -> (/foo/ AND content:"bar" AND /baz/)
func standard(patterns []Pattern) []Node {
	if len(patterns) == 1 {
		return []Node{patterns[0]}
	}

	var literals []Pattern
	var result []Node
	for _, p := range patterns {
		if p.Annotation.Labels.IsSet(Regexp) {
			// Push any sequence of literal patterns accumulated.
			// Then push this regexp pattern.
			if len(literals) > 0 {
				// Use existing `space` concatenator on literal
				// patterns. Correct and safe cast under
				// invariant len(literals) > 0.
				result = append(result, space(literals)[0].(Pattern))
			}

			result = append(result, p)
			literals = []Pattern{}
			continue
		}
		// Not Regexp => assume literal pattern and accumulate.
		literals = append(literals, p)
	}

	if len(literals) > 0 {
		result = append(result, space(literals)[0].(Pattern))
	}

	return result
}

// fuzzyRegexp interpolates patterns with spaces and concatenates them.
// Invariant: len(patterns) > 0.
func space(patterns []Pattern) []Node {
	if len(patterns) == 1 {
		return []Node{patterns[0]}
	}
	var values []string
	for _, p := range patterns {
		values = append(values, p.Value)
	}

	return []Node{
		Pattern{
			// Preserve labels based on first pattern. Required to
			// distinguish quoted, literal, structural pattern labels.
			Annotation: patterns[0].Annotation,
			Value:      strings.Join(values, " "),
		},
	}
}

// and concatenates patterns with AND.
func and(patterns []Node) []Node {
	return NewOperator(patterns, And)
}

// substituteConcat returns a function that concatenates all contiguous patterns
// in the tree, rooted by a concat operator. Concat operators containing negated
// patterns are lifted out: (concat "a" (not "b")) -> ("a" (not "b"))
//
// The callback parameter defines how the function concatenates patterns. The
// return value of callback is substituted in-place in the tree.
func substituteConcat(callback func([]Pattern) []Node) func([]Node) []Node {
	isPattern := func(node Node) bool {
		if pattern, ok := node.(Pattern); ok && !pattern.Negated {
			return true
		}
		return false
	}

	// define a recursive function to close over callback and isPattern.
	var substituteNodes func(nodes []Node) []Node
	substituteNodes = func(nodes []Node) []Node {
		newNode := []Node{}
		for _, node := range nodes {
			switch v := node.(type) {
			case Parameter, Pattern:
				newNode = append(newNode, node)
			case Operator:
				if v.Kind == Concat {
					// Merge consecutive patterns.
					ps := []Pattern{}
					previous := v.Operands[0]
					if p, ok := previous.(Pattern); ok {
						ps = append(ps, p)
					}
					for _, node := range v.Operands[1:] {
						if isPattern(node) && isPattern(previous) {
							p := node.(Pattern)
							ps = append(ps, p)
							previous = node
							continue
						}
						if len(ps) > 0 {
							newNode = append(newNode, callback(ps)...)
							ps = []Pattern{}
						}
						newNode = append(newNode, substituteNodes([]Node{node})...)
					}
					if len(ps) > 0 {
						newNode = append(newNode, callback(ps)...)
					}
				} else {
					newNode = append(newNode, NewOperator(substituteNodes(v.Operands), v.Kind)...)
				}
			}
		}
		return newNode
	}
	return substituteNodes
}

// substituteConcatForKeyword returns a function that replaces concat with the
// result of callback. Unlike substituteConcat, this function allows "OR" and
// "AND" operators to be nested inside "CONCAT".
func substituteConcatForKeyword(callback func([]Node) []Node) func([]Node) []Node {
	isPattern := func(node Node) bool {
		if pattern, ok := node.(Pattern); ok && !pattern.Negated {
			return true
		}
		return false
	}

	// define a recursive function to close over callback and isPattern.
	var substituteNodes func(nodes []Node) []Node
	substituteNodes = func(nodes []Node) []Node {
		var newNode []Node
		for _, node := range nodes {
			switch v := node.(type) {
			case Parameter, Pattern:
				newNode = append(newNode, node)
			case Operator:
				if v.Kind == Concat {
					// Merge consecutive patterns.
					var ps []Node
					for _, node := range v.Operands {
						if isPattern(node) {
							ps = append(ps, node)
							continue
						} else {
							ps = append(ps, substituteNodes([]Node{node})...)
						}
					}
					if len(ps) > 0 {
						newNode = append(newNode, callback(ps)...)
					}
				} else {
					newNode = append(newNode, NewOperator(substituteNodes(v.Operands), v.Kind)...)
				}
			}
		}
		return newNode
	}
	return substituteNodes
}

// escapeParens is a heuristic used in the context of regular expression search.
// It escapes two kinds of patterns:
//
// 1. Any occurrence of () is converted to \(\).
// In regex () implies the empty string, which is meaningless as a search
// query and probably not what the user intended.
//
// 2. If the pattern ends with a trailing and unescaped (, it is escaped.
// Normally, a pattern like foo.*bar( would be an invalid regexp, and we would
// show no results. But, it is a common and convenient syntax to search for, so
// we convert thsi pattern to interpret a trailing parenthesis literally.
//
// Any other forms are ignored, for example, foo.*(bar is unchanged. In the
// parser pipeline, such unchanged and invalid patterns are rejected by the
// validate function.
func escapeParens(s string) string {
	var i int
	for i := range len(s) {
		if s[i] == '(' || s[i] == '\\' {
			break
		}
	}

	// No special characters found, so return original string.
	if i >= len(s) {
		return s
	}

	var result []byte
	for i < len(s) {
		switch s[i] {
		case '\\':
			if i+1 < len(s) {
				result = append(result, '\\', s[i+1])
				i += 2 // Next char.
				continue
			}
			i++
			result = append(result, '\\')
		case '(':
			if i+1 == len(s) {
				// Escape a trailing and unescaped ( => \(.
				result = append(result, '\\', '(')
				i++
				continue
			}
			if i+1 < len(s) && s[i+1] == ')' {
				// Escape () => \(\).
				result = append(result, '\\', '(', '\\', ')')
				i += 2 // Next char.
				continue
			}
			result = append(result, s[i])
			i++
		default:
			result = append(result, s[i])
			i++
		}
	}
	return string(result)
}

// escapeParensHeuristic escapes certain parentheses in search patterns (see escapeParens).
func escapeParensHeuristic(nodes []Node) []Node {
	return MapPattern(nodes, func(value string, negated bool, annotation Annotation) Node {
		if !annotation.Labels.IsSet(Quoted) {
			value = escapeParens(value)
		}
		return Pattern{
			Value:      value,
			Negated:    negated,
			Annotation: annotation,
		}
	})
}

// Map pipes query through one or more query transformer functions.
func Map(query []Node, fns ...func([]Node) []Node) []Node {
	for _, fn := range fns {
		query = fn(query)
	}
	return query
}

// ConcatRevFilters removes rev: filters from parameters and attaches their value as @rev to the repo: filters.
// Invariant: Guaranteed to succeed on a validated Basic query.
func ConcatRevFilters(b Basic) Basic {
	var revision string
	nodes := MapField(toNodes(b.Parameters), FieldRev, func(value string, _ bool, _ Annotation) Node {
		revision = value
		return nil // remove this node
	})
	if revision == "" {
		return b
	}
	modified := MapField(nodes, FieldRepo, func(value string, negated bool, ann Annotation) Node {
		if !negated && !ann.Labels.IsSet(IsPredicate) {
			return Parameter{Value: value + "@" + revision, Field: FieldRepo, Negated: negated, Annotation: ann}
		}
		return Parameter{Value: value, Field: FieldRepo, Negated: negated, Annotation: ann}
	})
	return Basic{Parameters: toParameters(modified), Pattern: b.Pattern}
}

// labelStructural converts Literal labels to Structural labels. Structural
// queries are parsed the same as literal queries, we just convert the labels as
// a postprocessing step to keep the parser lean.
func labelStructural(nodes []Node) []Node {
	return MapPattern(nodes, func(value string, negated bool, annotation Annotation) Node {
		annotation.Labels.Unset(Literal)
		annotation.Labels.Set(Structural)
		return Pattern{
			Value:      value,
			Negated:    negated,
			Annotation: annotation,
		}
	})
}

// ellipsesForHoles substitutes ellipses ... for :[_] holes in structural search queries.
func ellipsesForHoles(nodes []Node) []Node {
	return MapPattern(nodes, func(value string, negated bool, annotation Annotation) Node {
		return Pattern{
			Value:      strings.ReplaceAll(value, "...", ":[_]"),
			Negated:    negated,
			Annotation: annotation,
		}
	})
}

// OmitField removes all fields `field` from a query. The `field` string
// should be the canonical name and not an alias ("repo", not "r").
func OmitField(q Q, field string) string {
	return StringHuman(MapField(q, field, func(_ string, _ bool, _ Annotation) Node {
		return nil
	}))
}

// AddRegexpField adds a new expr to the query with the given field and pattern
// value. The non-negated field is assumed to correspond to a regexp value. The
// pattern value is assumed to be unquoted.
//
// It tries to remove redundancy in the result. For example, given
// a query like "x:foo", if given a field "x" with pattern "foobar" to add,
// it will return a query "x:foobar" instead of "x:foo x:foobar". It is not
// guaranteed to always return the simplest query.
func AddRegexpField(q Q, field, pattern string) string {
	var modified bool
	q = MapParameter(q, func(gotField, value string, negated bool, annotation Annotation) Node {
		if field == gotField && strings.Contains(pattern, value) {
			value = pattern
			modified = true
		}
		return Parameter{
			Field:      gotField,
			Value:      value,
			Negated:    negated,
			Annotation: annotation,
		}
	})

	if !modified {
		// use newOperator to reduce And nodes when adding a parameter to the query toplevel.
		q = NewOperator(append(q, Parameter{Field: field, Value: pattern}), And)
	}
	return StringHuman(q)
}

// ToBasicQuery converts a parse tree to a basic query by attempting to obtain a valid partition.
func ToBasicQuery(nodes []Node) (Basic, error) {
	parameters, pattern, err := PartitionSearchPattern(nodes)
	if err != nil {
		return Basic{}, err
	}
	return Basic{Parameters: parameters, Pattern: pattern}, nil
}

// ExperimentalPhraseBoost is transformation on basic queries that appends a
// phrase query to the original query but only if the original query consists of
// a single top-level AND expression. The purpose is to improve ranking of exact
// matches by adding a phrase query for the entire query string.
//
// Example:
//
//	foo bar bas -> (or (and foo bar bas) ("foo bar bas"))
func ExperimentalPhraseBoost(basic Basic) Basic {
	if basic.Pattern == nil {
		return basic
	}

	if n, ok := basic.Pattern.(Operator); ok && n.Kind == And {
		// Gate on the number of operands. We don't want to add a phrase query for very
		// short queries.
		if len(n.Operands) < 3 {
			return basic
		}

		phrase := ""
		for _, child := range n.Operands {
			c, isPattern := child.(Pattern)
			if !isPattern || c.Negated || c.Annotation.Labels.IsSet(Regexp) {
				return basic
			}

			phrase += c.Value + " "
		}
		phrase = strings.TrimSpace(phrase)

		basic.Pattern = Operator{
			Kind: Or,
			Operands: []Node{
				Pattern{
					Value:      phrase,
					Annotation: Annotation{Labels: Boost | Literal | QuotesAsLiterals | Standard},
				},
				n,
			},
		}
	}

	return basic
}
