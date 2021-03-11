package query

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

type ExpectedOperand struct {
	Msg string
}

func (e *ExpectedOperand) Error() string {
	return e.Msg
}

type UnsupportedError struct {
	Msg string
}

func (e *UnsupportedError) Error() string {
	return e.Msg
}

type SearchType int

const (
	SearchTypeRegex SearchType = iota
	SearchTypeLiteral
	SearchTypeStructural
)

func (s SearchType) String() string {
	switch s {
	case SearchTypeRegex:
		return "regex"
	case SearchTypeLiteral:
		return "literal"
	case SearchTypeStructural:
		return "structural"
	default:
		return fmt.Sprintf("unknown{%d}", s)
	}
}

// QueryInfo is an interface for accessing query values that drive our search logic.
// It will be removed in favor of a cleaner query API to access values.
type QueryInfo interface {
	Count() *int
	Archived() *YesNoOnly
	Fork() *YesNoOnly
	Timeout() *time.Duration
	Repositories() (repos []string, negated []string)
	RegexpPatterns(field string) (values, negatedValues []string)
	StringValues(field string) (values, negatedValues []string)
	StringValue(field string) (value, negatedValue string)
	Values(field string) []*Value
	Fields() map[string][]*Value
	BoolValue(field string) bool
	IsCaseSensitive() bool
}

// A query is a tree of Nodes. We choose the type name Q so that external uses like query.Q do not stutter.
type Q []Node

func (q Q) String() string {
	return toString(q)
}

func (q Q) RegexpPatterns(field string) (values, negatedValues []string) {
	VisitField(q, field, func(visitedValue string, negated bool, _ Annotation) {
		if negated {
			negatedValues = append(negatedValues, visitedValue)
		} else {
			values = append(values, visitedValue)
		}
	})
	return values, negatedValues
}

func (q Q) StringValues(field string) (values, negatedValues []string) {
	VisitField(q, field, func(visitedValue string, negated bool, _ Annotation) {
		if negated {
			negatedValues = append(negatedValues, visitedValue)
		} else {
			values = append(values, visitedValue)
		}
	})
	return values, negatedValues
}

func (q Q) StringValue(field string) (value, negatedValue string) {
	VisitField(q, field, func(visitedValue string, negated bool, _ Annotation) {
		if negated {
			negatedValue = visitedValue
		} else {
			value = visitedValue
		}
	})
	return value, negatedValue
}

func (q Q) Values(field string) []*Value {
	var values []*Value
	if field == "" {
		VisitPattern(q, func(value string, _ bool, annotation Annotation) {
			values = append(values, q.valueToTypedValue(field, value, annotation.Labels)...)
		})
	} else {
		VisitField(q, field, func(value string, _ bool, _ Annotation) {
			values = append(values, q.valueToTypedValue(field, value, None)...)
		})
	}
	return values
}

func (q Q) Fields() map[string][]*Value {
	fields := make(map[string][]*Value)
	VisitPattern(q, func(value string, _ bool, _ Annotation) {
		fields[""] = q.Values("")
	})
	VisitParameter(q, func(field, _ string, _ bool, _ Annotation) {
		fields[field] = q.Values(field)
	})
	return fields
}

func (q Q) BoolValue(field string) bool {
	result := false
	VisitField(q, field, func(value string, _ bool, _ Annotation) {
		result, _ = parseBool(value) // err was checked during parsing and validation.
	})
	return result
}

func (q Q) Count() *int {
	var count *int
	VisitField(q, FieldCount, func(value string, _ bool, _ Annotation) {
		c, err := strconv.Atoi(value)
		if err != nil {
			panic(fmt.Sprintf("Value %q for count cannot be parsed as an int: %s", value, err))
		}
		count = &c
	})
	return count
}

func (q Q) Archived() *YesNoOnly {
	return q.yesNoOnlyValue(FieldArchived)
}

func (q Q) Fork() *YesNoOnly {
	return q.yesNoOnlyValue(FieldFork)
}

func (q Q) yesNoOnlyValue(field string) *YesNoOnly {
	var res *YesNoOnly
	VisitField(q, field, func(value string, _ bool, _ Annotation) {
		yno := ParseYesNoOnly(value)
		if yno == Invalid {
			panic(fmt.Sprintf("Invalid value %q for field %q", value, field))
		}
		res = &yno
	})
	return res
}

func (q Q) Timeout() *time.Duration {
	var timeout *time.Duration
	VisitField(q, FieldTimeout, func(value string, _ bool, _ Annotation) {
		t, err := time.ParseDuration(value)
		if err != nil {
			panic(fmt.Sprintf("Value %q for timeout cannot be parsed as an duration: %s", value, err))
		}
		timeout = &t
	})
	return timeout
}

func (q Q) IsCaseSensitive() bool {
	return q.BoolValue("case")
}

func (q Q) Repositories() (repos []string, negatedRepos []string) {
	VisitField(q, FieldRepo, func(value string, negated bool, _ Annotation) {
		if negated {
			negatedRepos = append(negatedRepos, value)
			return
		}
		repos = append(repos, value)
	})
	return repos, negatedRepos
}

func parseRegexpOrPanic(field, value string) *regexp.Regexp {
	r, err := regexp.Compile(value)
	if err != nil {
		panic(fmt.Sprintf("Value %s for field %s invalid regex: %s", field, value, err.Error()))
	}
	return r
}

// valueToTypedValue approximately preserves the field validation of our
// previous query processing. It does not check the validity of field negation
// or if the same field is specified more than once. This role is now performed
// by validate.go.
func (q Q) valueToTypedValue(field, value string, label labels) []*Value {
	switch field {
	case
		FieldDefault:
		if label.isSet(Literal) {
			return []*Value{{String: &value}}
		}
		if label.isSet(Regexp) {
			regexp, err := regexp.Compile(value)
			if err != nil {
				panic(fmt.Sprintf("Invariant broken: value must have been checked to be valid regexp. Error: %s", err))
			}
			return []*Value{{Regexp: regexp}}
		}
		// All patterns should have a label after parsing, but if not, treat the pattern as a string literal.
		return []*Value{{String: &value}}

	case
		FieldCase:
		b, _ := parseBool(value)
		return []*Value{{Bool: &b}}

	case
		FieldRepo, "r":
		return []*Value{{Regexp: parseRegexpOrPanic(field, value)}}

	case
		FieldRepoGroup, "g",
		FieldContext:
		return []*Value{{String: &value}}

	case
		FieldFile, "f":
		return []*Value{{Regexp: parseRegexpOrPanic(field, value)}}

	case
		FieldFork,
		FieldArchived,
		FieldLang, "l", "language",
		FieldType,
		FieldPatternType,
		FieldContent:
		return []*Value{{String: &value}}

	case FieldRepoHasFile:
		return []*Value{{Regexp: parseRegexpOrPanic(field, value)}}

	case
		FieldRepoHasCommitAfter,
		FieldBefore, "until",
		FieldAfter, "since":
		return []*Value{{String: &value}}

	case
		FieldAuthor,
		FieldCommitter,
		FieldMessage, "m", "msg":
		return []*Value{{Regexp: parseRegexpOrPanic(field, value)}}

	case
		FieldIndex,
		FieldCount,
		FieldTimeout,
		FieldCombyRule:
		return []*Value{{String: &value}}
	}
	return []*Value{{String: &value}}
}
