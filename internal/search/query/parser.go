package query

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/grafana/regexp"
	"github.com/sourcegraph/sourcegraph/lib/errors"
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

type OperatorKind int

const (
	Or OperatorKind = iota
	And
	Concat
)

// Operator is a nonterminal node of kind Kind with child nodes Operands.
type Operator struct {
	Kind       OperatorKind
	Operands   []Node
	Annotation Annotation
}

func (node Pattern) String() string {
	if node.Negated {
		return fmt.Sprintf("(not %s)", strconv.Quote(node.Value))
	}
	return strconv.Quote(node.Value)
}

// IsRegExp returns true if the Pattern.Value should be interpreted as a Regex
// otherwise returns false for a Literal.
//
// Note: This checks that the relevant annotation is set, which occasionally
// we regress on setting. In such situations it will return that the Pattern
// is Regex. Use this method so we have consistent behaviour across our
// backends rather than directly checking annotations when building queries
// for your backend.
func (node Pattern) IsRegExp() bool {
	// NOTE: Structural tech debt. We want the patterns to be treated like
	// literals and not passed down as regex to searcher.
	return !node.Annotation.Labels.IsSet(Literal | Structural)
}

// RegExpPattern returns the pattern value as a regex string. If node.IsRegExp
// this is just node.Value, otherwise we escape the literal.
func (node Pattern) RegExpPattern() string {
	if node.IsRegExp() {
		return node.Value
	}
	return regexp.QuoteMeta(node.Value)
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

// isSpace returns true if the buffer only contains UTF-8 encoded whitespace as
// defined by unicode.IsSpace.
func isSpace(buf []byte) bool {
	if len(buf) == 0 {
		return false
	}
	for len(buf) > 0 {
		r, n := utf8.DecodeRune(buf)
		if !unicode.IsSpace(r) {
			return false
		}
		buf = buf[n:]
	}
	return true
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
	// literal search patterns instead. Example: (a b) -> "(a b)"
	parensAsPatterns heuristics = 1 << iota
	// If set, all parentheses, whether balanced or unbalanced, are parsed
	// as literal search patterns (i.e., interpreting parentheses as
	// expression groups is completely disabled).
	allowDanglingParens
	// If set, implies that at least one expression was disambiguated by
	// explicit parentheses.
	disambiguated
	// If set, the parser will parse patterns containing balanced parentheses as
	// literal patterns. Example: func(a int, b int) -> "func(a int, b int)"
	balancedPattern
	// If set, the parser will parse empty parenthesis, "()", as a literal pattern
	// instead of as pattern group.
	emptyParens
)

func isSet(h, heuristic heuristics) bool { return h&heuristic != 0 }

type parser struct {
	buf        []byte
	heuristics heuristics
	pos        int
	balanced   int
	leafParser SearchType

	seenRightParen bool
	seenOr         bool
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
	for range n {
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

// matchKeyword is like match but checks whether the keyword has a valid prefix
// and suffix.
func (p *parser) matchKeyword(keyword keyword) bool {
	if p.pos == 0 {
		return false
	}
	if isSpace(p.buf[:p.pos]) {
		return false
	}
	if !(isSpace(p.buf[p.pos-1:p.pos]) || p.buf[p.pos-1] == ')') {
		return false
	}
	v, err := p.peek(len(string(keyword)))
	if err != nil {
		return false
	}
	after := p.pos + len(string(keyword))
	if after >= len(p.buf) || !(isSpace(p.buf[after:after+1]) || p.buf[after] == '(') {
		return false
	}
	return strings.EqualFold(v, string(keyword))
}

// matchUnaryKeyword is like match but expects the keyword to be followed by whitespace.
func (p *parser) matchUnaryKeyword(keyword keyword) bool {
	if p.pos != 0 && !(isSpace(p.buf[p.pos-1:p.pos]) || p.buf[p.pos-1] == ')' || p.buf[p.pos-1] == '(') {
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

// ScanPredicate scans for a predicate that exists in the predicate
// registry. It takes the current field as context.
func ScanPredicate(field string, buf []byte, lookup PredicateRegistry) (string, int, bool) {
	fieldRegistry, ok := lookup[resolveFieldAlias(field)]
	if !ok {
		// This field has no registered predicates
		return "", 0, false
	}

	predicateName, nameAdvance, ok := ScanPredicateName(buf, fieldRegistry)
	if !ok {
		return "", 0, false
	}
	buf = buf[nameAdvance:]

	// If the predicate name isn't followed by a parenthesis, this
	// isn't a predicate
	if len(buf) == 0 || buf[0] != '(' {
		return "", 0, false
	}

	params, paramsAdvance, ok := ScanBalancedParens(buf)
	if !ok {
		return "", 0, false
	}

	return predicateName + params, nameAdvance + paramsAdvance, true
}

// ScanPredicateName scans whether buf contains a well-known name in the predicate lookup table.
func ScanPredicateName(buf []byte, lookup PredicateTable) (string, int, bool) {
	var predicateName string
	var advance int
	for {
		r, i := utf8.DecodeRune(buf[advance:])
		if r == utf8.RuneError {
			break
		}

		if !(unicode.IsLetter(r) || r == '.') {
			predicateName = string(buf[:advance])
			break
		}
		advance += i
	}

	if _, ok := lookup[predicateName]; !ok {
		// The string is not a predicate
		return "", 0, false
	}

	return predicateName, advance, true
}

// ScanBalancedParens will return the full string including
// and inside the parantheses that start with the first character.
// This is different from ScanBalancedPattern because that attempts
// to take into account whether the content looks like other filters.
// In the case of predicates, we offload the job of parsing parameters
// onto the predicates themselves, so we just want the full content
// of the parameters, whatever it contains.
func ScanBalancedParens(buf []byte) (string, int, bool) {
	var r rune
	var count int
	var result []rune

	next := func() rune {
		r, advance := utf8.DecodeRune(buf)
		count += advance
		buf = buf[advance:]
		result = append(result, r)
		return r
	}

	r = next()
	if r != '(' {
		panic(fmt.Sprintf("ScanBalancedParens expects the input buffer to start with delimiter (, but it starts with %s.", string(r)))
	}
	balance := 1

	for {
		r = next()
		if r == utf8.RuneError {
			return "", 0, false
		}
		switch r {
		case '(':
			balance++
		case ')':
			balance--
		case '\\':
			// Consume the next escaped value since an escaped paren
			// won't ever affect the balance
			_ = next()
		}
		if balance == 0 {
			break
		}
	}

	return string(result), count, true
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

// Delimit inverts the process of ScanDelimiter, escaping any special
// characters or delimiters in s.
//
// NOTE: this does not provide a clean roundtrip with ScanDelimited because
// ScanDelimited is lossy. We cannot know whether a backslash was passed
// through because it was escaped or because its successor rune was not
// escapable.
func Delimit(s string, delimiter rune) string {
	ds := string(delimiter)
	delimitReplacer := strings.NewReplacer(
		"\n", "\\n",
		"\r", "\\r",
		"\t", "\\t",
		"\\", "\\\\",
		ds, "\\"+ds,
	)
	return ds + delimitReplacer.Replace(s) + ds
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

func (p *parser) parseQuoted(delimiter rune) (string, bool) {
	start := p.pos
	value, advance, err := ScanDelimited(p.buf[p.pos:], false, delimiter)
	if err != nil {
		return "", false
	}
	p.pos += advance
	if !p.done() {
		if r, _ := utf8.DecodeRune([]byte{p.buf[p.pos]}); !unicode.IsSpace(r) && !p.match(RPAREN) {
			p.pos = start // backtrack
			// delimited value should be followed by terminal (whitespace or closing paren).
			return "", false
		}
	}
	return value, true
}

// parseStringQuotes parses "..." or '...' syntax and returns a Patter node.
// Returns whether parsing succeeds.
func (p *parser) parseStringQuotes() (Pattern, bool) {
	start := p.pos

	if p.match(DQUOTE) {
		if v, ok := p.parseQuoted('"'); ok {
			return newPattern(v, Literal|Quoted, newRange(start, p.pos)), true
		}
	}

	if p.match(SQUOTE) {
		if v, ok := p.parseQuoted('\''); ok {
			return newPattern(v, Literal|Quoted, newRange(start, p.pos)), true
		}
	}

	return Pattern{}, false
}

// parseRegexpQuotes parses "/.../" syntax and returns a Pattern node. Returns
// whether parsing succeeds.
func (p *parser) parseRegexpQuotes() (Pattern, bool) {
	if !p.match(SLASH) {
		return Pattern{}, false
	}

	start := p.pos
	v, ok := p.parseQuoted('/')
	if !ok {
		return Pattern{}, false
	}

	labels := Regexp
	if v == "" {
		// This is an empty `//` delimited pattern: treat this
		// heuristically as a literal // pattern instead, since an empty
		// regex pattern offers lower utility.
		v = "//"
		labels = Literal
	}
	return newPattern(v, labels, newRange(start, p.pos)), true
}

// ParseFieldValue parses a value after a field like "repo:". It returns the
// parsed value and any labels to annotate this value with. If the value starts
// with a recognized quoting delimiter but does not close it, an error is
// returned.
func (p *parser) ParseFieldValue(field string) (string, labels, error) {
	delimited := func(delimiter rune) (string, labels, error) {
		value, advance, err := ScanDelimited(p.buf[p.pos:], true, delimiter)
		if err != nil {
			return "", None, err
		}
		p.pos += advance
		return value, Quoted, nil
	}
	if p.match(SQUOTE) {
		return delimited('\'')
	}
	if p.match(DQUOTE) {
		return delimited('"')
	}

	value, advance, ok := ScanPredicate(field, p.buf[p.pos:], DefaultPredicateRegistry)
	if ok {
		p.pos += advance
		return value, IsPredicate, nil
	}

	// First try scan a field value for cases like (a b repo:foo), where a
	// trailing ) may be closing a group, and not part of the value.
	value, advance, ok = ScanBalancedPattern(p.buf[p.pos:])
	if ok {
		p.pos += advance
		return value, None, nil

	}

	// The above failed, so attempt a best effort.
	value, advance = ScanValue(p.buf[p.pos:], false)
	p.pos += advance
	return value, None, nil
}

func (p *parser) TryScanBalancedPattern(label labels) (Pattern, bool) {
	if value, advance, ok := ScanBalancedPattern(p.buf[p.pos:]); ok {
		pattern := newPattern(value, label, newRange(p.pos, p.pos+advance))
		p.pos += advance
		return pattern, true
	}
	return Pattern{}, false
}

func newPattern(value string, labels labels, range_ Range) Pattern {
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
	if label.IsSet(Standard | Regexp) {
		if pattern, ok := p.parseRegexpQuotes(); ok {
			return pattern
		}
	}

	if label.IsSet(Regexp | QuotesAsLiterals) {
		if pattern, ok := p.parseStringQuotes(); ok {
			return pattern
		}
	}

	if isSet(p.heuristics, balancedPattern) {
		if pattern, ok := p.TryScanBalancedPattern(label); ok {
			return pattern
		}
	}

	start := p.pos
	var value string
	var advance int
	if label.IsSet(Regexp) {
		value, advance = ScanValue(p.buf[p.pos:], isSet(p.heuristics, allowDanglingParens))
	} else {
		value, advance = ScanAnyPattern(p.buf[p.pos:])
	}
	if isSet(p.heuristics, allowDanglingParens) {
		label.Set(HeuristicDanglingParens)
	}
	p.pos += advance
	return newPattern(value, label, newRange(start, p.pos))
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
	value, labels, err := p.ParseFieldValue(field)
	if err != nil {
		return Parameter{}, false, err
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
		orderedPatterns := NewOperator(patterns, Concat)
		return NewOperator(append(unorderedParams, orderedPatterns...), And)
	}
	return NewOperator(append(unorderedParams, patterns...), And)
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
					if label.IsSet(Literal) {
						label.Set(HeuristicParensAsPatterns)
					}
					pattern := newPattern(value, label, newRange(p.pos, p.pos+advance))
					p.pos += advance
					nodes = append(nodes, pattern)
					continue
				}
			}
			// If the above failed, we treat this paren
			// group as part of an and/or expression.
			_ = p.expect(LPAREN) // Guaranteed to succeed.
			p.balanced++
			if p.seenOr {
				p.heuristics |= disambiguated
			}
			result, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, result...)
		case p.match(RPAREN) && !isSet(p.heuristics, allowDanglingParens):
			// Caller advances.
			if p.balanced <= 0 {
				if label.IsSet(QuotesAsLiterals) {
					return nil, errors.New("unsupported expression. The combination of parentheses in the query has an unclear meaning. Use \"...\" to quote patterns that contain parentheses")
				}
				return nil, errors.New("unsupported expression. The combination of parentheses in the query have an unclear meaning. Try using the content: filter to quote patterns that contain parentheses")
			}
			p.balanced--
			p.seenRightParen = true
			if len(nodes) == 0 {
				// We parsed "()".
				if isSet(p.heuristics, emptyParens) {
					// Interpret literally.
					nodes = []Node{newPattern("()", Literal|HeuristicParensAsPatterns, newRange(start, p.pos))}
				} else {
					// Interpret as a group: return an empty non-nil node.
					nodes = []Node{Parameter{}}
				}
			}
			break loop
		case p.matchKeyword(AND):
			// Caller advances.
			break loop
		case p.matchKeyword(OR):
			// Caller advances.
			p.seenOr = true
			if p.seenRightParen {
				p.heuristics |= disambiguated
			}
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
					return nil, errors.Errorf("unexpected NOT before \"-%s:%s\". Remove NOT and try again",
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
func reduce(left, right []Node, kind OperatorKind) ([]Node, bool) {
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
		reduced, changed := reduce([]Node{right[0]}, right[1:], kind)
		if changed {
			return append(left, reduced...), true
		}
	}
	return append(left, right...), false
}

// NewOperator constructs a new node of kind operatorKind with operands nodes,
// reducing nodes as needed.
func NewOperator(nodes []Node, kind OperatorKind) []Node {
	if len(nodes) == 0 {
		return nil
	} else if len(nodes) == 1 {
		return nodes
	}

	reduced, changed := reduce([]Node{nodes[0]}, nodes[1:], kind)
	if changed {
		return NewOperator(reduced, kind)
	}
	return []Node{Operator{Kind: kind, Operands: reduced}}
}

// parseAnd parses and-expressions.
func (p *parser) parseAnd() ([]Node, error) {
	var left []Node
	var err error
	switch p.leafParser {
	case SearchTypeRegex:
		left, err = p.parseLeaves(Regexp)
	case SearchTypeLiteral, SearchTypeStructural:
		left, err = p.parseLeaves(Literal)
	case SearchTypeStandard, SearchTypeLucky:
		left, err = p.parseLeaves(Literal | Standard)
	case SearchTypeKeyword:
		left, err = p.parseLeaves(Literal | Standard | QuotesAsLiterals)
	default:
		left, err = p.parseLeaves(Literal | Standard)
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
	return NewOperator(append(left, right...), And), nil
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

	// parseAnd might have parsed an expression, in which case parser.pos is
	// currently pointing to a RPAREN.
	_ = p.expect(RPAREN)

	if !p.expect(OR) {
		return left, nil
	}
	right, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	return NewOperator(append(left, right...), Or), nil
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
		return NewOperator(hoistedNodes, And), nil
	}
	return NewOperator(nodes, And), nil
}

// Parse parses a raw input string into a parse tree comprising Nodes.
func Parse(in string, searchType SearchType) ([]Node, error) {
	if strings.TrimSpace(in) == "" {
		return nil, nil
	}

	parser := &parser{
		buf:        []byte(in),
		leafParser: searchType,
	}

	switch searchType {
	case SearchTypeKeyword:
		parser.heuristics = balancedPattern | emptyParens
	default:
		parser.heuristics = balancedPattern | emptyParens | parensAsPatterns
	}

	nodes, err := parser.parseOr()
	if err != nil {
		if errors.HasType(err, &ExpectedOperand{}) {
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

	// Note that we don't include keyword search in this check because we want to
	// support mixing of implicit and explicit operators
	if searchType == SearchTypeLiteral || searchType == SearchTypeStandard {
		err = validatePureLiteralPattern(nodes, parser.balanced == 0)
		if err != nil {
			return nil, err
		}
	}
	return NewOperator(nodes, And), nil
}

func ParseSearchType(in string, searchType SearchType) (Q, error) {
	return Run(Init(in, searchType))
}

func ParseStandard(in string) (Q, error) {
	return Run(Init(in, SearchTypeStandard))
}

func ParseLiteral(in string) (Q, error) {
	return Run(Init(in, SearchTypeLiteral))
}

func ParseRegexp(in string) (Q, error) {
	return Run(Init(in, SearchTypeRegex))
}
