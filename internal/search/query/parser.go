package query

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

/*
Parser implements a parser for the following grammar:

OrTerm     → AndTerm { OR AndTerm }
AndTerm    → Term { AND Term }
Term       → (OrTerm) | Parameters
Parameters → Parameter { " " Parameter }
*/

type Node interface {
	String() string
	node()
}

// All terms that implement Node.
func (Pattern) node()   {}
func (Parameter) node() {}
func (Operator) node()  {}

// An annotation stores information associated with a node.
type Annotation struct {
	Labels labels `json:"labels"`
	Range  Range  `json:"range"`
}

// Pattern is a leaf node of expressions representing a search pattern fragment.
type Pattern struct {
	Value      string     `json:"value"`   // The pattern value.
	Negated    bool       `json:"negated"` // True if this pattern is negated.
	Annotation Annotation `json:"-"`       // An annotation attached to this pattern.
}

// Parameter is a leaf node of expressions representing a parameter of format "repo:foo".
type Parameter struct {
	Field      string     `json:"field"`   // The repo part in repo:sourcegraph.
	Value      string     `json:"value"`   // The sourcegraph part in repo:sourcegraph.
	Negated    bool       `json:"negated"` // True if the - prefix exists, as in -repo:sourcegraph.
	Annotation Annotation `json:"-"`
}

type operatorKind int

const (
	Or operatorKind = iota
	And
	Concat
)

// Operator is a nonterminal node of kind Kind with child nodes Operands.
type Operator struct {
	Kind       operatorKind
	Operands   []Node
	Annotation Annotation
}

func (node Pattern) String() string {
	if node.Negated {
		return fmt.Sprintf("(not %s)", strconv.Quote(node.Value))
	}
	return strconv.Quote(node.Value)
}

func (node Parameter) String() string {
	var v string
	switch {
	case node.Field == "":
		v = node.Value
	case node.Negated:
		v = fmt.Sprintf("-%s:%s", node.Field, node.Value)
	default:
		v = fmt.Sprintf("%s:%s", node.Field, node.Value)
	}
	return strconv.Quote(v)
}

func (node Operator) String() string {
	var result []string
	for _, child := range node.Operands {
		result = append(result, child.String())
	}
	var kind string
	switch node.Kind {
	case Or:
		kind = "or"
	case And:
		kind = "and"
	case Concat:
		kind = "concat"
	}

	return fmt.Sprintf("(%s %s)", kind, strings.Join(result, " "))
}

type keyword string

// Reserved keyword syntax.
const (
	AND    keyword = "and"
	OR     keyword = "or"
	LPAREN keyword = "("
	RPAREN keyword = ")"
	SQUOTE keyword = "'"
	DQUOTE keyword = "\""
	SLASH  keyword = "/"
	NOT    keyword = "not"
)

func isSpace(buf []byte) bool {
	r, _ := utf8.DecodeRune(buf)
	return unicode.IsSpace(r)
}

// skipSpace returns the number of whitespace bytes skipped from the beginning of a buffer buf.
func skipSpace(buf []byte) int {
	count := 0
	for len(buf) > 0 {
		r, advance := utf8.DecodeRune(buf)
		if !unicode.IsSpace(r) {
			break
		}
		count += advance
		buf = buf[advance:]
	}
	return count
}

type heuristics uint8

const (
	// If set, balanced parentheses, which would normally be treated as
	// delimiting expression groups, may in select cases be parsed as
	// literal search patterns instead.
	parensAsPatterns heuristics = 1 << iota
	// If set, all parentheses, whether balanced or unbalanced, are parsed
	// as literal search patterns (i.e., interpreting parentheses as
	// expression groups is completely disabled).
	allowDanglingParens
	// If set, implies that at least one expression was disambiguated by
	// explicit parentheses.
	disambiguated
)

func isSet(h, heuristic heuristics) bool { return h&heuristic != 0 }

type parser struct {
	buf        []byte
	heuristics heuristics
	pos        int
	balanced   int
	leafParser SearchType
}

func (p *parser) done() bool {
	return p.pos >= len(p.buf)
}

func (p *parser) next() rune {
	if p.done() {
		panic("eof")
	}
	r, advance := utf8.DecodeRune(p.buf[p.pos:])
	p.pos += advance
	return r
}

// peek looks ahead n runes in the input and returns a string if it succeeds, or
// an error if the length exceeds what's available in the buffer.
func (p *parser) peek(n int) (string, error) {
	start := p.pos
	defer func() {
		p.pos = start // backtrack
	}()

	var result []rune
	for i := 0; i < n; i++ {
		if p.done() {
			return "", io.ErrShortBuffer
		}
		next := p.next()
		result = append(result, next)
	}
	return string(result), nil
}

// match returns whether it succeeded matching a keyword at the current
// position. It does not advance the position.
func (p *parser) match(keyword keyword) bool {
	v, err := p.peek(len(string(keyword)))
	if err != nil {
		return false
	}
	return strings.EqualFold(v, string(keyword))
}

// expect returns the result of match, and advances the position if it succeeds.
func (p *parser) expect(keyword keyword) bool {
	if !p.match(keyword) {
		return false
	}
	p.pos += len(string(keyword))
	return true
}

// matchKeyword is like match but expects the keyword to be preceded and followed by whitespace.
func (p *parser) matchKeyword(keyword keyword) bool {
	if p.pos == 0 {
		return false
	}
	if !isSpace(p.buf[p.pos-1 : p.pos]) {
		return false
	}
	v, err := p.peek(len(string(keyword)))
	if err != nil {
		return false
	}
	after := p.pos + len(string(keyword))
	if after >= len(p.buf) || !isSpace(p.buf[after:after+1]) {
		return false
	}
	return strings.EqualFold(v, string(keyword))
}

// matchUnaryKeyword is like match but expects the keyword to be followed by whitespace.
func (p *parser) matchUnaryKeyword(keyword keyword) bool {
	if p.pos != 0 && !(isSpace(p.buf[p.pos-1:p.pos]) || p.buf[p.pos-1] == '(') {
		// "not" must be preceded by a space or ( anywhere except the beginning of the string
		return false
	}
	v, err := p.peek(len(string(keyword)))
	if err != nil {
		return false
	}
	after := p.pos + len(string(keyword))
	if after >= len(p.buf) || !isSpace(p.buf[after:after+1]) {
		return false
	}
	return strings.EqualFold(v, string(keyword))
}

// skipSpaces advances the input and places the parser position at the next
// non-space value.
func (p *parser) skipSpaces() error {
	if p.pos > len(p.buf) {
		return io.ErrShortBuffer
	}

	p.pos += skipSpace(p.buf[p.pos:])
	if p.pos > len(p.buf) {
		return io.ErrShortBuffer
	}
	return nil
}

// ScanAnyPattern consumes all characters up to a whitespace character
// and returns the string and how much it consumed.
func ScanAnyPattern(buf []byte) (scanned string, count int) {
	var advance int
	var r rune
	var result []rune

	next := func() rune {
		r, advance = utf8.DecodeRune(buf)
		count += advance
		buf = buf[advance:]
		return r
	}
	for len(buf) > 0 {
		start := count
		r = next()
		if unicode.IsSpace(r) {
			count = start // Backtrack.
			break
		}
		result = append(result, r)
	}
	scanned = string(result)
	return scanned, count
}

// ScanBalancedPattern attempts to scan parentheses as literal patterns. This
// ensures that we interpret patterns containing parentheses _as patterns_ and not
// groups. For example, it accepts these patterns:
//
// ((a|b)|c)              - a regular expression with balanced parentheses for grouping
// myFunction(arg1, arg2) - a literal string with parens that should be literally interpreted
// foo(...)               - a structural search pattern
//
// If it weren't for this scanner, the above parentheses would have to be
// interpreted as part of the query language group syntax, like these:
//
// (foo or (bar and baz))
//
// So, this scanner detects parentheses as patterns without needing the user to
// explicitly escape them. As such, there are cases where this scanner should
// not succeed:
//
// (foo or (bar and baz)) - a valid query with and/or expression groups in the query langugae
// (repo:foo bar baz)     - a valid query containing a recognized repo: field. Here parentheses are interpreted as a group, not a pattern.
func ScanBalancedPattern(buf []byte) (scanned string, count int, ok bool) {
	var advance, balanced int
	var r rune
	var result []rune

	next := func() rune {
		r, advance = utf8.DecodeRune(buf)
		count += advance
		buf = buf[advance:]
		return r
	}

	// looks ahead to see if there are any recognized fields or operators.
	keepScanning := func() bool {
		if field, _, _ := ScanField(buf); field != "" {
			// This "pattern" contains a recognized field, reject it.
			return false
		}
		lookahead := func(v string) bool {
			if len(buf) < len(v) {
				return false
			}
			lookaheadStr := string(buf[:len(v)])
			return strings.EqualFold(lookaheadStr, v)
		}
		if lookahead("and ") ||
			lookahead("or ") ||
			lookahead("not ") {
			// This "pattern" contains a recognized keyword, reject it.
			return false
		}
		return true
	}

	if !keepScanning() {
		return "", 0, false
	}

loop:
	for len(buf) > 0 {
		start := count
		r = next()
		switch {
		case unicode.IsSpace(r) && balanced == 0:
			// Stop scanning a potential pattern when we see
			// whitespace in a balanced state.
			count = start
			break loop
		case r == '(':
			if !keepScanning() {
				return "", 0, false
			}
			balanced++
			result = append(result, r)
		case r == ')':
			balanced--
			if balanced < 0 {
				// This paren is an unmatched closing paren, so
				// we stop treating it as a potential pattern
				// here--it might be closing a group.
				count = start // Backtrack.
				balanced = 0  // Pattern is balanced up to this point.
				break loop
			}
			result = append(result, r)
		case unicode.IsSpace(r):
			if !keepScanning() {
				return "", 0, false
			}

			// We see a space and the pattern is unbalanced, so assume this
			// this space is still part of the pattern.
			result = append(result, r)
		case r == '\\':
			// Handle escape sequence.
			if len(buf) > 0 {
				r = next()
				// Accept anything anything escaped. The point
				// is to consume escaped spaces like "\ " so
				// that we don't recognize it as terminating a
				// pattern.
				result = append(result, '\\', r)
				continue
			}
			result = append(result, r)
		default:
			result = append(result, r)
		}
	}

	return string(result), count, balanced == 0
}

// ScanDelimited takes a delimited (e.g., quoted) value for some arbitrary
// delimiter, returning the undelimited value, and the end position of the
// original delimited value (i.e., including quotes). `\` is treated as an
// escape character for the delimiter and traditional string escape sequences.
// The `strict` input parameter sets whether this delimiter may contain only
// recognized escaped characters (strict), or arbitrary ones.
// The input buffer must start with the chosen delimiter.
func ScanDelimited(buf []byte, strict bool, delimiter rune) (string, int, error) {
	var count, advance int
	var r rune
	var result []rune

	next := func() rune {
		r, advance := utf8.DecodeRune(buf)
		count += advance
		buf = buf[advance:]
		return r
	}

	r = next()
	if r != delimiter {
		panic(fmt.Sprintf("ScanDelimited expects the input buffer to start with delimiter %s, but it starts with %s.", string(delimiter), string(r)))
	}

loop:
	for len(buf) > 0 {
		r = next()
		switch {
		case r == delimiter:
			break loop
		case r == '\\':
			// Handle escape sequence.
			if len(buf[advance:]) > 0 {
				r = next()
				switch r {
				case 'a', 'b', 'f', 'v':
					result = append(result, '\\', r)
				case 'n':
					result = append(result, '\n')
				case 'r':
					result = append(result, '\r')
				case 't':
					result = append(result, '\t')
				case '\\', delimiter:
					result = append(result, r)
				default:
					if strict {
						return "", count, errors.New("unrecognized escape sequence")
					}
					// Accept anything else literally.
					result = append(result, '\\', r)
				}
				if len(buf) == 0 {
					return "", count, errors.New("unterminated literal: expected " + string(delimiter))
				}
			} else {
				return "", count, errors.New("unterminated escape sequence")
			}
		default:
			result = append(result, r)
		}
	}

	if r != delimiter || (r == delimiter && count == 1) {
		return "", count, errors.New("unterminated literal: expected " + string(delimiter))
	}
	return string(result), count, nil
}

// ScanField scans an optional '-' at the beginning of a string, and then scans
// one or more alphabetic characters until it encounters a ':'. The prefix
// string is checked against valid fields. If it is valid, the function returns
// the value before the colon, whether it's negated, and its length. In all
// other cases it returns zero values.
func ScanField(buf []byte) (string, bool, int) {
	var count int
	var r rune
	var result []rune
	allowed := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	next := func() rune {
		r, advance := utf8.DecodeRune(buf)
		count += advance
		buf = buf[advance:]
		return r
	}

	r = next()
	if r != '-' && !strings.ContainsRune(allowed, r) {
		return "", false, 0
	}
	result = append(result, r)

	success := false
	for len(buf) > 0 {
		r = next()
		if strings.ContainsRune(allowed, r) {
			result = append(result, r)
			continue
		}
		if r == ':' {
			// Invariant: len(result) > 0. If len(result) == 1,
			// check that it is not just a '-'. If len(result) > 1, it is valid.
			if result[0] != '-' || len(result) > 1 {
				success = true
			}
		}
		break
	}
	if !success {
		return "", false, 0
	}

	field := string(result)
	negated := field[0] == '-'
	if negated {
		field = field[1:]
	}

	if _, exists := allFields[strings.ToLower(field)]; !exists {
		// Not a recognized parameter field.
		return "", false, 0
	}

	return field, negated, count
}

// ScanValue scans for a value (e.g., of a parameter, or a string corresponding
// to a search pattern). Its main function is to determine when to stop scanning
// a value (e.g., at a parentheses), and which escape sequences to interpret. It
// returns the scanned value, how much was advanced, and whether to allow
// scanning dangling parentheses in patterns like "foo(".
func ScanValue(buf []byte, allowDanglingParens bool) (string, int) {
	var count, advance, balanced int
	var r rune
	var result []rune

	next := func() rune {
		r, advance = utf8.DecodeRune(buf)
		count += advance
		buf = buf[advance:]
		return r
	}

	for len(buf) > 0 {
		start := count
		r = next()
		if unicode.IsSpace(r) {
			count = start // Backtrack.
			break
		}
		if r == '(' || r == ')' {
			if r == '(' {
				balanced++
			}
			if r == ')' {
				balanced--
			}
			if allowDanglingParens {
				result = append(result, r)
				continue
			}
			count = start // Backtrack.
			break
		}
		if r == '\\' {
			// Handle escape sequence.
			if len(buf) > 0 {
				r = next()
				result = append(result, '\\', r)
				continue
			}
		}
		result = append(result, r)
	}
	return string(result), count
}

// TryParseDelimiter tries to parse a delimited string, returning the
// interpreted (i.e., unquoted) value if it succeeds, the delimiter that
// suceeded parsing, and whether it succeeded.
func (p *parser) TryParseDelimiter() (string, rune, bool) {
	delimited := func(delimiter rune) (string, bool) {
		start := p.pos
		value, advance, err := ScanDelimited(p.buf[p.pos:], false, delimiter)
		if err != nil {
			return "", false
		}
		p.pos += advance
		if !p.done() {
			if r, _ := utf8.DecodeRune([]byte{p.buf[p.pos]}); !unicode.IsSpace(r) {
				p.pos = start // backtrack
				// delimited value should be followed by whitespace
				return "", false
			}
		}
		return value, true
	}

	if p.match(SQUOTE) {
		if v, ok := delimited('\''); ok {
			return v, '\'', true
		}
	}
	if p.match(DQUOTE) {
		if v, ok := delimited('"'); ok {
			return v, '"', true
		}
	}
	if p.match(SLASH) {
		if v, ok := delimited('/'); ok {
			return v, '/', true
		}
	}
	return "", 0, false
}

// ParseFieldValue parses a value after a field like "repo:". It returns the
// parsed value and whether it was quoted. If the value starts with a recognized
// quoting delimiter but does not close it, an error is returned.
func (p *parser) ParseFieldValue() (string, bool, error) {
	delimited := func(delimiter rune) (string, bool, error) {
		value, advance, err := ScanDelimited(p.buf[p.pos:], true, delimiter)
		if err != nil {
			return "", false, err
		}
		p.pos += advance
		return value, true, nil
	}
	if p.match(SQUOTE) {
		return delimited('\'')
	}
	if p.match(DQUOTE) {
		return delimited('"')
	}
	// First try scan a field value for cases like (a b repo:foo), where a
	// trailing ) may be closing a group, and not part of the value.
	value, advance, ok := ScanBalancedPattern(p.buf[p.pos:])
	if !ok {
		// The above failed, so attempt a best effort.
		value, advance = ScanValue(p.buf[p.pos:], false)
	}
	p.pos += advance
	return value, false, nil
}

// Try parse a delimited pattern, quoted as "...", '...', or /.../.
func (p *parser) TryParseDelimitedPattern() (Pattern, bool) {
	start := p.pos
	if value, delimiter, ok := p.TryParseDelimiter(); ok {
		var labels labels
		if delimiter == '/' {
			// This is a regex-delimited pattern
			labels = Regexp
		} else {
			labels = Literal | Quoted
		}
		return newPattern(value, false, labels, newRange(start, p.pos)), true
	}
	return Pattern{}, false
}

func (p *parser) TryScanBalancedPattern(label labels) (Pattern, bool) {
	if value, advance, ok := ScanBalancedPattern(p.buf[p.pos:]); ok {
		pattern := newPattern(value, false, label, newRange(p.pos, p.pos+advance))
		p.pos += advance
		return pattern, true
	}
	return Pattern{}, false
}

func newPattern(value string, negated bool, labels labels, range_ Range) Pattern {
	return Pattern{
		Value:   value,
		Negated: false,
		Annotation: Annotation{
			Labels: labels,
			Range:  range_,
		},
	}
}

// ParsePattern parses a leaf node Pattern that corresponds to a search pattern.
// Note that ParsePattern may be called multiple times (a query can have
// multiple Patterns concatenated together).
func (p *parser) ParsePattern(label labels) Pattern {
	if label.isSet(Regexp) {
		// First try parse delimited values for regexp.
		if pattern, ok := p.TryParseDelimitedPattern(); ok {
			return pattern
		}
	}

	if isSet(p.heuristics, parensAsPatterns) {
		if pattern, ok := p.TryScanBalancedPattern(label); ok {
			return pattern
		}
	}

	start := p.pos
	var value string
	var advance int
	if label.isSet(Regexp) {
		value, advance = ScanValue(p.buf[p.pos:], isSet(p.heuristics, allowDanglingParens))
	} else {
		value, advance = ScanAnyPattern(p.buf[p.pos:])
	}
	if isSet(p.heuristics, allowDanglingParens) {
		label.set(HeuristicDanglingParens)
	}
	p.pos += advance
	return newPattern(value, false, label, newRange(start, p.pos))

}

// ParseParameter returns a leaf node corresponding to the syntax
// (-?)field:<string> where : matches the first encountered colon, and field
// must match ^[a-zA-Z]+ and be allowed by allFields. Field may optionally
// be preceded by '-' which means the parameter is negated.
func (p *parser) ParseParameter() (Parameter, bool, error) {
	start := p.pos
	field, negated, advance := ScanField(p.buf[p.pos:])
	if field == "" {
		return Parameter{}, false, nil
	}

	p.pos += advance
	value, quoted, err := p.ParseFieldValue()
	if err != nil {
		return Parameter{}, false, err
	}
	var labels labels
	if quoted {
		labels.set(Quoted)
	}
	return Parameter{
		Field:      field,
		Value:      value,
		Negated:    negated,
		Annotation: Annotation{Range: newRange(start, p.pos), Labels: labels},
	}, true, nil
}

// partitionParameters constructs a parse tree to distinguish terms where
// ordering is insignificant (e.g., "repo:foo file:bar") versus terms where
// ordering may be significant (e.g., search patterns like "foo bar").
//
// The resulting tree defines an ordering relation on nodes in the following cases:
// (1) When more than one search patterns exist at the same operator level, they
// are concatenated in order.
// (2) Any nonterminal node is concatenated (ordered in the tree) if its
// descendents contain one or more search patterns.
func partitionParameters(nodes []Node) []Node {
	var patterns, unorderedParams []Node
	for _, n := range nodes {
		switch n.(type) {
		case Pattern:
			patterns = append(patterns, n)
		case Parameter:
			unorderedParams = append(unorderedParams, n)
		case Operator:
			if containsPattern(n) {
				patterns = append(patterns, n)
			} else {
				unorderedParams = append(unorderedParams, n)
			}
		}
	}
	if len(patterns) > 1 {
		orderedPatterns := newOperator(patterns, Concat)
		return newOperator(append(unorderedParams, orderedPatterns...), And)
	}
	return newOperator(append(unorderedParams, patterns...), And)
}

// parseLeaves scans for consecutive leaf nodes and applies
// label to patterns.
func (p *parser) parseLeaves(label labels) ([]Node, error) {
	var nodes []Node
	start := p.pos
loop:
	for {
		if err := p.skipSpaces(); err != nil {
			return nil, err
		}
		if p.done() {
			break loop
		}
		switch {
		case p.match(LPAREN) && !isSet(p.heuristics, allowDanglingParens):
			if isSet(p.heuristics, parensAsPatterns) {
				if value, advance, ok := ScanBalancedPattern(p.buf[p.pos:]); ok {
					if label.isSet(Literal) {
						label.set(HeuristicParensAsPatterns)
					}
					pattern := newPattern(value, false, label, newRange(p.pos, p.pos+advance))
					p.pos += advance
					nodes = append(nodes, pattern)
					continue
				}
			}
			// If the above failed, we treat this paren
			// group as part of an and/or expression.
			_ = p.expect(LPAREN) // Guaranteed to succeed.
			p.balanced++
			p.heuristics |= disambiguated
			result, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, result...)
		case p.expect(RPAREN) && !isSet(p.heuristics, allowDanglingParens):
			if p.balanced <= 0 {
				return nil, errors.New("unsupported expression. The combination of parentheses in the query have an unclear meaning. Try using the content: filter to quote patterns that contain parentheses")
			}
			p.balanced--
			p.heuristics |= disambiguated
			if len(nodes) == 0 {
				// We parsed "()".
				if isSet(p.heuristics, parensAsPatterns) {
					// Interpret literally.
					nodes = []Node{newPattern("()", false, Literal|HeuristicParensAsPatterns, newRange(start, p.pos))}
				} else {
					// Interpret as a group: return an empty non-nil node.
					nodes = []Node{Parameter{}}
				}
			}
			break loop
		case p.matchKeyword(AND), p.matchKeyword(OR):
			// Caller advances.
			break loop
		case p.matchUnaryKeyword(NOT):
			start := p.pos
			_ = p.expect(NOT)
			err := p.skipSpaces()
			if err != nil {
				return nil, err
			}
			if p.match(LPAREN) {
				return nil, errors.New("it looks like you tried to use an expression after NOT. The NOT operator can only be used with simple search patterns or filters, and is not supported for expressions or subqueries")
			}
			if parameter, ok, _ := p.ParseParameter(); ok {
				// we don't support NOT -field:value
				if parameter.Negated {
					return nil, fmt.Errorf("unexpected NOT before \"-%s:%s\". Remove NOT and try again",
						parameter.Field, parameter.Value)
				}
				parameter.Negated = true
				parameter.Annotation.Range = newRange(start, p.pos)
				nodes = append(nodes, parameter)
				continue
			}
			pattern := p.ParsePattern(label)
			pattern.Negated = true
			pattern.Annotation.Range = newRange(start, p.pos)
			nodes = append(nodes, pattern)
		default:
			parameter, ok, err := p.ParseParameter()
			if err != nil {
				return nil, err
			}
			if ok {
				nodes = append(nodes, parameter)
			} else {
				pattern := p.ParsePattern(label)
				nodes = append(nodes, pattern)
			}
		}
	}
	return partitionParameters(nodes), nil
}

// reduce takes lists of left and right nodes and reduces them if possible. For example,
// (and a (b and c))       => (and a b c)
// (((a and b) or c) or d) => (or (and a b) c d)
func reduce(left, right []Node, kind operatorKind) ([]Node, bool) {
	if param, ok := left[0].(Parameter); ok && param.Value == "" {
		// Remove empty string parameter.
		return right, true
	}

	switch term := right[0].(type) {
	case Operator:
		if kind == term.Kind {
			// Reduce right node.
			left = append(left, term.Operands...)
			if len(right) > 1 {
				left = append(left, right[1:]...)
			}
			return left, true
		}
	case Parameter:
		if term.Value == "" {
			// Remove empty string parameter.
			if len(right) > 1 {
				return append(left, right[1:]...), true
			}
			return left, true
		}
		if operator, ok := left[0].(Operator); ok && operator.Kind == kind {
			// Reduce left node.
			return append(operator.Operands, right...), true
		}
	case Pattern:
		if term.Value == "" {
			// Remove empty string pattern.
			if len(right) > 1 {
				return append(left, right[1:]...), true
			}
			return left, true
		}
		if operator, ok := left[0].(Operator); ok && operator.Kind == kind {
			// Reduce left node.
			return append(operator.Operands, right...), true
		}
	}
	if len(right) > 1 {
		// Reduce right list.
		reduced, changed := reduce(append(left, right[0]), right[1:], kind)
		if changed {
			return reduced, true
		}
	}
	return append(left, right...), false
}

// newOperator constructs a new node of kind operatorKind with operands nodes,
// reducing nodes as needed.
func newOperator(nodes []Node, kind operatorKind) []Node {
	if len(nodes) == 0 {
		return nil
	} else if len(nodes) == 1 {
		return nodes
	}

	reduced, changed := reduce([]Node{nodes[0]}, nodes[1:], kind)
	if changed {
		return newOperator(reduced, kind)
	}
	return []Node{Operator{Kind: kind, Operands: reduced}}
}

// parseAnd parses and-expressions.
func (p *parser) parseAnd() ([]Node, error) {
	var left []Node
	var err error
	if p.leafParser == SearchTypeRegex {
		left, err = p.parseLeaves(Regexp)
	} else {
		left, err = p.parseLeaves(Literal)
	}
	if err != nil {
		return nil, err
	}
	if left == nil {
		return nil, &ExpectedOperand{Msg: fmt.Sprintf("expected operand at %d", p.pos)}
	}
	if !p.expect(AND) {
		return left, nil
	}
	right, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	return newOperator(append(left, right...), And), nil
}

// parseOr parses or-expressions. Or operators have lower precedence than And
// operators, therefore this function calls parseAnd.
func (p *parser) parseOr() ([]Node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	if left == nil {
		return nil, &ExpectedOperand{Msg: fmt.Sprintf("expected operand at %d", p.pos)}
	}
	if !p.expect(OR) {
		return left, nil
	}
	right, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	return newOperator(append(left, right...), Or), nil
}

func (p *parser) tryFallbackParser(in string) ([]Node, error) {
	newParser := &parser{
		buf:        []byte(in),
		heuristics: allowDanglingParens,
		leafParser: p.leafParser,
	}
	nodes, err := newParser.parseOr()
	if err != nil {
		return nil, err
	}
	if hoistedNodes, err := Hoist(nodes); err == nil {
		return newOperator(hoistedNodes, And), nil
	}
	return newOperator(nodes, And), nil
}

// ParseAndOr a raw input string into a parse tree comprising Nodes.
func ParseAndOr(in string, searchType SearchType) ([]Node, error) {
	if strings.TrimSpace(in) == "" {
		return nil, nil
	}

	parser := &parser{
		buf:        []byte(in),
		heuristics: parensAsPatterns,
		leafParser: searchType,
	}

	nodes, err := parser.parseOr()
	if err != nil {
		if _, ok := err.(*ExpectedOperand); ok {
			// The query may be unbalanced or malformed as in "(" or
			// "x or" and expects an operand. Try harder to parse it.
			if nodes, err := parser.tryFallbackParser(in); err == nil {
				return nodes, nil
			}
		}
		// Another kind of error, like a malformed parameter.
		return nil, err
	}
	if parser.balanced != 0 {
		// The query is unbalanced and might be something like "(x" or
		// "x or (x" where patterns start with a leading open
		// parenthesis. Try harder to parse it.
		if nodes, err := parser.tryFallbackParser(in); err == nil {
			return nodes, nil
		}
		return nil, errors.New("unbalanced expression")
	}
	if !isSet(parser.heuristics, disambiguated) {
		// Hoist or expressions if this query is potential ambiguous.
		if hoistedNodes, err := Hoist(nodes); err == nil {
			nodes = hoistedNodes
		}
	}
	if searchType == SearchTypeLiteral {
		err = validatePureLiteralPattern(nodes, parser.balanced == 0)
		if err != nil {
			return nil, err
		}
	}
	return newOperator(nodes, And), nil
}

type ParserOptions struct {
	SearchType SearchType

	// treat repo, file, or repohasfile values as glob syntax if true.
	Globbing bool
}

// Parse parses and validates an and/or query for a given search type.
func Parse(in string, options ParserOptions) (Q, error) {
	var query []Node
	var err error

	query, err = ParseAndOr(in, options.SearchType)
	if err != nil {
		return nil, err
	}
	query = Map(query, LowercaseFieldNames, SubstituteAliases(options.SearchType))

	switch options.SearchType {
	case SearchTypeLiteral:
		query = Map(query, substituteConcat(space))
	case SearchTypeStructural:
		query = Map(query, labelStructural, ellipsesForHoles, substituteConcat(space))
	case SearchTypeRegex:
		query = Map(query, escapeParensHeuristic, substituteConcat(fuzzyRegexp))
	}

	if options.Globbing {
		query, err = mapGlobToRegex(query)
		if err != nil {
			return nil, err
		}
	}

	for _, disjunct := range Dnf(query) {
		err = validate(disjunct)
		if err != nil {
			return nil, err
		}
	}
	return query, nil
}

func ParseLiteral(in string) (Q, error) {
	return Parse(in, ParserOptions{SearchType: SearchTypeLiteral})
}

func ParseRegexp(in string) (Q, error) {
	return Parse(in, ParserOptions{SearchType: SearchTypeRegex})
}
