package query

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-enry/go-enry/v2"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/search/filter"
)

// IsBasic returns whether a query is a basic query. A basic query is one which
// does not have a DNF-expansion. I.e., there is only one disjunct. A basic
// query implies that it has no subexpressions that we need to evaluate. IsBasic
// is used in our codebase where legacy code has not been updated to handle
// queries with multiple expressions (like alerts), and assume only one
// evaluatable query.
func IsBasic(nodes []Node) bool {
	return len(Dnf(nodes)) == 1
}

// exists traverses every node in nodes and returns early as soon as fn is satisfied.
func exists(nodes []Node, fn func(node Node) bool) bool {
	found := false
	for _, node := range nodes {
		if fn(node) {
			return true
		}
		if operator, ok := node.(Operator); ok {
			return exists(operator.Operands, fn)
		}
	}
	return found
}

// forAll traverses every node in nodes and returns whether all nodes satisfy fn.
func forAll(nodes []Node, fn func(node Node) bool) bool {
	sat := true
	for _, node := range nodes {
		if !fn(node) {
			return false
		}
		if operator, ok := node.(Operator); ok {
			return forAll(operator.Operands, fn)
		}
	}
	return sat
}

// isPatternExpression returns true if every leaf node in nodes is a search
// pattern expression.
func isPatternExpression(nodes []Node) bool {
	return !exists(nodes, func(node Node) bool {
		// Any non-pattern leaf, i.e., Parameter, falsifies the condition.
		_, ok := node.(Parameter)
		return ok
	})
}

// containsPattern returns true if any descendent of nodes is a search pattern.
func containsPattern(node Node) bool {
	return exists([]Node{node}, func(node Node) bool {
		_, ok := node.(Pattern)
		return ok
	})
}

// ContainsRegexpMetasyntax returns true if a string is a valid regular
// expression and contains regex metasyntax (i.e., it is not a literal).
func ContainsRegexpMetasyntax(input string) bool {
	_, err := regexp.Compile(input)
	if err == nil {
		// It is a regexp. But does it contain metasyntax, or is it literal?
		if len(regexp.QuoteMeta(input)) != len(input) {
			return true
		}
	}
	return false
}

// processTopLevel processes the top level of a query. It validates that we can
// process the query with respect to and/or expressions on file content, but not
// otherwise for nested parameters.
func processTopLevel(nodes []Node) ([]Node, error) {
	if term, ok := nodes[0].(Operator); ok {
		if term.Kind == And && isPatternExpression([]Node{term}) {
			return nodes, nil
		} else if term.Kind == Or && isPatternExpression([]Node{term}) {
			return nodes, nil
		} else if term.Kind == And {
			return term.Operands, nil
		} else if term.Kind == Concat {
			return nodes, nil
		} else {
			return nil, &UnsupportedError{Msg: "cannot evaluate: unable to partition pure search pattern"}
		}
	}
	return nodes, nil
}

// PartitionSearchPattern partitions an and/or query into (1) a single search
// pattern expression and (2) other parameters that scope the evaluation of
// search patterns (e.g., to repos, files, etc.). It validates that a query
// contains at most one search pattern expression and that scope parameters do
// not contain nested expressions.
func PartitionSearchPattern(nodes []Node) (parameters []Node, pattern Node, err error) {
	if len(nodes) == 1 {
		nodes, err = processTopLevel(nodes)
		if err != nil {
			return nil, nil, err
		}
	}

	var patterns []Node
	for _, node := range nodes {
		if isPatternExpression([]Node{node}) {
			patterns = append(patterns, node)
		} else if term, ok := node.(Parameter); ok {
			parameters = append(parameters, term)
		} else {
			return nil, nil, &UnsupportedError{Msg: "cannot evaluate: unable to partition pure search pattern"}
		}
	}
	if len(patterns) > 1 {
		pattern = Operator{Kind: And, Operands: patterns}
	} else if len(patterns) == 1 {
		pattern = patterns[0]
	}

	return parameters, pattern, nil
}

// parseBool is like strconv.ParseBool except that it also accepts y, Y, yes,
// YES, Yes, n, N, no, NO, No.
func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		b, err := strconv.ParseBool(s)
		if err != nil {
			err = fmt.Errorf("invalid boolean %q", s)
		}
		return b, err
	}
}

func validateField(field, value string, negated bool, seen map[string]struct{}) error {
	isNotNegated := func() error {
		if negated {
			return fmt.Errorf("field %q does not support negation", field)
		}
		return nil
	}

	isSingular := func() error {
		if _, notSingular := seen[field]; notSingular {
			return fmt.Errorf("field %q may not be used more than once", field)
		}
		return nil
	}

	isValidRegexp := func() error {
		if _, err := regexp.Compile(value); err != nil {
			return err
		}
		return nil
	}

	isBoolean := func() error {
		if _, err := parseBool(value); err != nil {
			return err
		}
		return nil
	}

	isNumber := func() error {
		count, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			if err.(*strconv.NumError).Err == strconv.ErrRange {
				return fmt.Errorf("field %s has a value that is out of range, try making it smaller", field)
			}
			return fmt.Errorf("field %s has value %[2]s, %[2]s is not a number", field, value)
		}
		if count <= 0 {
			return fmt.Errorf("field %s requires a positive number", field)
		}
		return nil
	}

	isDuration := func() error {
		_, err := time.ParseDuration(value)
		if err != nil {
			return errors.New(`invalid value for field 'timeout' (examples: "timeout:2s", "timeout:200ms")`)
		}
		return nil
	}

	isLanguage := func() error {
		_, ok := enry.GetLanguageByAlias(value)
		if !ok {
			return fmt.Errorf("unknown language: %q", value)
		}
		return nil
	}

	isYesNoOnly := func() error {
		v := ParseYesNoOnly(value)
		if v == Invalid {
			return fmt.Errorf("invalid value %q for field %q. Valid values are: yes, only, no", value, field)
		}
		return nil
	}

	isUnrecognizedField := func() error {
		return fmt.Errorf("unrecognized field %q", field)
	}

	isValidSelect := func() error {
		_, err := filter.SelectPathFromString(value)
		return err
	}

	satisfies := func(fns ...func() error) error {
		for _, fn := range fns {
			if err := fn(); err != nil {
				return err
			}
		}
		return nil
	}

	switch field {
	case
		FieldDefault:
		// Search patterns are not validated here, as it depends on the search type.
	case
		FieldCase:
		return satisfies(isSingular, isBoolean, isNotNegated)
	case
		FieldRepo:
		return satisfies(isValidRegexp)
	case
		FieldRepoGroup,
		FieldContext:
		return satisfies(isSingular, isNotNegated)
	case
		FieldFile:
		return satisfies(isValidRegexp)
	case
		FieldLang:
		return satisfies(isLanguage)
	case
		FieldType:
		return satisfies(isNotNegated)
	case
		FieldPatternType,
		FieldContent,
		FieldVisibility:
		return satisfies(isSingular, isNotNegated)
	case
		FieldRepoHasFile:
		return satisfies(isValidRegexp)
	case
		FieldRepoHasCommitAfter:
		return satisfies(isSingular, isNotNegated)
	case
		FieldBefore,
		FieldAfter:
		return satisfies(isNotNegated)
	case
		FieldAuthor,
		FieldCommitter,
		FieldMessage:
		return satisfies(isValidRegexp)
	case
		FieldIndex,
		FieldFork,
		FieldArchived:
		return satisfies(isSingular, isNotNegated, isYesNoOnly)
	case
		FieldCount:
		return satisfies(isSingular, isNumber, isNotNegated)
	case
		FieldStable:
		return satisfies(isSingular, isBoolean, isNotNegated)
	case
		FieldCombyRule:
		return satisfies(isSingular, isNotNegated)
	case
		FieldTimeout:
		return satisfies(isSingular, isNotNegated, isDuration)
	case
		FieldRev:
		return satisfies(isSingular, isNotNegated)
	case
		FieldSelect:
		return satisfies(isSingular, isNotNegated, isValidSelect)
	default:
		return isUnrecognizedField()
	}
	return nil
}

// A query is invalid if it contains a rev: filter and a repo is specified with @.
func validateRepoRevPair(nodes []Node) error {
	var seenRepoWithCommit bool
	VisitField(nodes, FieldRepo, func(value string, negated bool, _ Annotation) {
		if !negated && strings.ContainsRune(value, '@') {
			seenRepoWithCommit = true
		}
	})
	revSpecified := exists(nodes, func(node Node) bool {
		n, ok := node.(Parameter)
		if ok && n.Field == FieldRev {
			return true
		}
		return false
	})
	if seenRepoWithCommit && revSpecified {
		return errors.New("invalid syntax. You specified both @ and rev: for a" +
			" repo: filter and I don't know how to interpret this. Remove either @ or rev: and try again")
	}
	return nil
}

// Queries containing commit parameters without type:diff or type:commit are not
// valid. cf. https://docs.sourcegraph.com/code_search/reference/language#commit-parameter
func validateCommitParameters(nodes []Node) error {
	var seenCommitParam string
	var typeCommitExists bool
	VisitParameter(nodes, func(field, value string, _ bool, _ Annotation) {
		if field == FieldAuthor || field == FieldBefore || field == FieldAfter || field == FieldMessage {
			seenCommitParam = field
		}
		if field == FieldType && (value == "commit" || value == "diff") {
			typeCommitExists = true
		}
	})
	if seenCommitParam != "" && !typeCommitExists {
		return fmt.Errorf(`your query contains the field '%s', which requires type:commit or type:diff in the query`, seenCommitParam)
	}
	return nil
}

// validateRepoHasFile validates that the repohasfile parameter can be executed.
// A query like `repohasfile:foo type:symbol patter-to-match-symbols` is
// currently not supported.
func validateRepoHasFile(nodes []Node) error {
	var seenRepoHasFile, seenTypeSymbol bool
	VisitParameter(nodes, func(field, value string, _ bool, _ Annotation) {
		if field == FieldRepoHasFile {
			seenRepoHasFile = true
		}
		if field == FieldType && strings.EqualFold(value, "symbol") {
			seenTypeSymbol = true
		}
	})
	if seenRepoHasFile && seenTypeSymbol {
		return errors.New("repohasfile is not compatible for type:symbol. Subscribe to https://github.com/sourcegraph/sourcegraph/issues/4610 for updates")
	}
	return nil
}

// validatePureLiteralPattern checks that no pattern expression contains and/or
// operators nested inside concat. It may happen that we interpret a query this
// way due to ambiguity. If this happens, return an error message.
func validatePureLiteralPattern(nodes []Node, balanced bool) error {
	impure := exists(nodes, func(node Node) bool {
		if operator, ok := node.(Operator); ok && operator.Kind == Concat {
			for _, node := range operator.Operands {
				if op, ok := node.(Operator); ok && (op.Kind == Or || op.Kind == And) {
					return true
				}
			}
		}
		return false
	})
	if impure {
		if !balanced {
			return errors.New("this literal search query contains unbalanced parentheses. I tried to guess what you meant, but wasn't able to. Maybe you missed a parenthesis? Otherwise, try using the content: filter if the pattern is unbalanced")
		}
		return errors.New("i'm having trouble understanding that query. The combination of parentheses is the problem. Try using the content: filter to quote patterns that contain parentheses")
	}
	return nil
}

func validateParameters(nodes []Node) error {
	var err error
	seen := map[string]struct{}{}
	VisitParameter(nodes, func(field, value string, negated bool, _ Annotation) {
		if err != nil {
			return
		}
		err = validateField(field, value, negated, seen)
		seen[field] = struct{}{}
	})
	return err
}

func validatePattern(nodes []Node) error {
	var err error
	VisitPattern(nodes, func(value string, negated bool, annotation Annotation) {
		if annotation.Labels.isSet(Regexp) {
			if err != nil {
				return
			}
			_, err = regexp.Compile(value)
		}
		if annotation.Labels.isSet(Structural) && negated {
			if err != nil {
				return
			}
			err = errors.New("the query contains a negated search pattern. Structural search does not support negated search patterns at the moment")
		}
	})
	return err
}

func validate(nodes []Node) error {
	succeeds := func(fns ...func([]Node) error) error {
		for _, fn := range fns {
			if err := fn(nodes); err != nil {
				return err
			}
		}
		return nil
	}

	return succeeds(
		validateParameters,
		validatePattern,
		validateRepoRevPair,
		validateRepoHasFile,
		validateCommitParameters,
	)
}

type YesNoOnly string

const (
	Yes     YesNoOnly = "yes"
	No      YesNoOnly = "no"
	Only    YesNoOnly = "only"
	Invalid YesNoOnly = "invalid"
)

func ParseYesNoOnly(s string) YesNoOnly {
	switch s {
	case "y", "Y", "yes", "YES", "Yes":
		return Yes
	case "n", "N", "no", "NO", "No":
		return No
	case "o", "only", "ONLY", "Only":
		return Only
	default:
		if b, err := strconv.ParseBool(s); err == nil {
			if b {
				return Yes
			}
			return No
		}
		return Invalid
	}
}

func ContainsRefGlobs(q Q) bool {
	containsRefGlobs := false
	if repoFilterValues, _ := q.Repositories(); len(repoFilterValues) > 0 {
		for _, v := range repoFilterValues {
			repoRev := strings.SplitN(v, "@", 2)
			if len(repoRev) == 1 { // no revision
				continue
			}
			if ContainsNoGlobSyntax(repoRev[1]) {
				continue
			}
			containsRefGlobs = true
			break
		}
	}
	return containsRefGlobs
}

func HasTypeRepo(q Q) bool {
	found := false
	VisitField(q, "type", func(value string, _ bool, _ Annotation) {
		if value == "repo" {
			found = true
		}
	})
	return found
}
