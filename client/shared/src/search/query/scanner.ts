import { filterTypeKeysWithAliases } from './filters'
import { SearchPatternType } from '../../graphql-operations'
import {
    Token,
    Whitespace,
    OpeningParen,
    ClosingParen,
    Keyword,
    Comment,
    Literal,
    Pattern,
    Filter,
    KeywordKind,
    PatternKind,
    CharacterRange,
    createLiteral,
} from './token'

/**
 * A scanner produces a term, which is either a token or a list of tokens.
 */
export type Term = Token | Token[]

/**
 * Represents the failed result of running a {@link Scanner} on a search query.
 */
interface ScanError {
    type: 'error'

    /**
     * A string representing the token that would have been expected
     * for successful scanning at {@link ScannerError#at}.
     */
    expected: string

    /**
     * The index in the search query string where scanning failed.
     */
    at: number
}

/**
 * Represents the successful result of running a {@link Scannerer} on a search query.
 */
export interface ScanSuccess<T = Term> {
    type: 'success'

    /**
     * The resulting term.
     */
    term: T
}

/**
 * Represents the result of running a {@link Scanner} on a search query.
 */
export type ScanResult<T = Term> = ScanError | ScanSuccess<T>

type Scanner<T = Term> = (input: string, start: number) => ScanResult<T>

/**
 * Returns a {@link Scanner} that succeeds if zero or more tokens are scanned
 * by the given `scanToken` scanners.
 */
const zeroOrMore = (scanToken: Scanner<Term>): Scanner<Token[]> => (input, start) => {
    const tokens: Token[] = []
    let adjustedStart = start
    let end = start + 1
    while (input[adjustedStart] !== undefined) {
        const result = scanToken(input, adjustedStart)
        if (result.type === 'error') {
            return result
        }
        if (Array.isArray(result.term)) {
            for (const token of result.term) {
                tokens.push(token)
                end = token.range.end
            }
        } else {
            tokens.push(result.term)
            end = result.term.range.end
        }
        adjustedStart = end
    }
    return { type: 'success', term: tokens }
}

/**
 * Returns a {@link Scanner} that succeeds if any of the given scanner succeeds.
 */
const oneOf = <T>(...scanners: Scanner<T>[]): Scanner<T> => (input, start) => {
    const expected: string[] = []
    for (const scanner of scanners) {
        const result = scanner(input, start)
        if (result.type === 'success') {
            return result
        }
        expected.push(result.expected)
    }
    return {
        type: 'error',
        expected: `One of: ${expected.join(', ')}`,
        at: start,
    }
}

/**
 * A {@link Scanner} that will attempt to scan delimited strings for an arbitrary
 * delimiter. `\` is treated as an escape character for the delimited string.
 */
const quoted = (delimiter: string): Scanner<Literal> => (input, start) => {
    if (input[start] !== delimiter) {
        return { type: 'error', expected: delimiter, at: start }
    }
    let end = start + 1
    while (input[end] && (input[end] !== delimiter || input[end - 1] === '\\')) {
        end = end + 1
    }
    if (!input[end]) {
        return { type: 'error', expected: delimiter, at: end }
    }
    return {
        type: 'success',
        // end + 1 as `end` is currently the index of the quote in the string.
        term: createLiteral(input.slice(start + 1, end), { start, end: end + 1 }, true),
    }
}

/**
 * Returns a {@link Scanner} that will attempt to scan tokens matching
 * the given character in a search query.
 */
const character = (character: string): Scanner<Literal> => (input, start) => {
    if (input[start] !== character) {
        return { type: 'error', expected: character, at: start }
    }
    return {
        type: 'success',
        term: createLiteral(character, { start, end: start + 1 }),
    }
}

/**
 * Returns a {@link Scanner} that will attempt to scan
 * tokens matching the given RegExp pattern in a search query.
 */
const scanToken = <T extends Term = Literal>(
    regexp: RegExp,
    output?: T | ((input: string, range: CharacterRange) => T),
    expected?: string
): Scanner<T> => {
    if (!regexp.source.startsWith('^')) {
        regexp = new RegExp(`^${regexp.source}`, regexp.flags)
    }
    return (input, start) => {
        const matchTarget = input.slice(Math.max(0, start))
        if (!matchTarget) {
            return { type: 'error', expected: expected || `/${regexp.source}/`, at: start }
        }
        const match = matchTarget.match(regexp)
        if (!match) {
            return { type: 'error', expected: expected || `/${regexp.source}/`, at: start }
        }
        const range = { start, end: start + match[0].length }
        return {
            type: 'success',
            term: output
                ? typeof output === 'function'
                    ? output(input, range)
                    : output
                : ({ type: 'literal', value: match[0], range } as T),
        }
    }
}

/**
 * ScanBalancedLiteral attempts to scan balanced parentheses as literal strings. This
 * ensures that we interpret patterns containing parentheses _as patterns_ and not
 * groups. For example, it accepts these patterns:
 *
 * ((a|b)|c)              - a regular expression with balanced parentheses for grouping
 * myFunction(arg1, arg2) - a literal string with parens that should be literally interpreted
 * foo(...)               - a structural search pattern
 *
 * If it weren't for this scanner, the above parentheses would have to be
 * interpreted as part of the query language group syntax, like these:
 *
 * (foo or (bar and baz))
 *
 * So, this scanner detects parentheses as patterns without needing the user to
 * explicitly escape them. As such, there are cases where this scanner should
 * not succeed:
 *
 * (foo or (bar and baz)) - a valid query with and/or expression groups in the query langugae
 * (repo:foo bar baz)     - a valid query containing a recognized repo: field. Here parentheses are interpreted as a group, not a pattern.
 */
export const scanBalancedLiteral: Scanner<Literal> = (input, start) => {
    let adjustedStart = start
    let balanced = 0
    let current = ''
    const result: string[] = []

    const nextChar = (): void => {
        current = input[adjustedStart]
        adjustedStart += 1
    }

    if (!keepScanning(input, start)) {
        return {
            type: 'error',
            expected: 'no recognized filter or keyword',
            at: start,
        }
    }

    while (input[adjustedStart] !== undefined) {
        nextChar()
        if (current.match(/\s/) && balanced === 0) {
            // Stop scanning a potential pattern when we see whitespace in a balanced state.
            adjustedStart -= 1 // Backtrack.
            break
        } else if (current === '(') {
            if (!keepScanning(input, adjustedStart)) {
                return {
                    type: 'error',
                    expected: 'no recognized filter or keyword',
                    at: adjustedStart,
                }
            }
            balanced += 1
            result.push(current)
        } else if (current === ')') {
            balanced -= 1
            if (balanced < 0) {
                // This paren is an unmatched closing paren, so we stop treating it as a potential
                // pattern here--it might be closing a group.
                adjustedStart -= 1 // Backtrack.
                balanced = 0 // Pattern is balanced up to this point
                break
            }
            result.push(current)
        } else if (current === ' ') {
            if (!keepScanning(input, adjustedStart)) {
                return {
                    type: 'error',
                    expected: 'no recognized filter or keyword',
                    at: adjustedStart,
                }
            }
            result.push(current)
        } else if (current === '\\') {
            if (input[adjustedStart] !== undefined) {
                nextChar()
                // Accept anything anything escaped. The point is to consume escaped spaces like "\ "
                // so that we don't recognize it as terminating a pattern.
                result.push('\\', current)
                continue
            }
            result.push(current)
        } else {
            result.push(current)
        }
    }

    if (balanced !== 0) {
        return {
            type: 'error',
            expected: 'no unbalanced parentheses',
            at: adjustedStart,
        }
    }

    return {
        type: 'success',
        term: createLiteral(result.join(''), { start, end: adjustedStart }),
    }
}

const whitespace = scanToken(/\s+/, (_input, range) => ({
    type: 'whitespace',
    range,
}))

const literal = scanToken(/[^\s)]+/)

const keywordNot = scanToken(/(not|NOT)/, (input, { start, end }) => ({
    type: 'keyword',
    value: input.slice(start, end),
    range: { start, end },
    kind: KeywordKind.Not,
}))

const keywordAnd = scanToken(/(and|AND)/, (input, { start, end }) => ({
    type: 'keyword',
    value: input.slice(start, end),
    range: { start, end },
    kind: KeywordKind.And,
}))

const keywordOr = scanToken(/(or|OR)/, (input, { start, end }) => ({
    type: 'keyword',
    value: input.slice(start, end),
    range: { start, end },
    kind: KeywordKind.Or,
}))

const keyword = oneOf<Keyword>(keywordAnd, keywordOr, keywordNot)

const comment = scanToken(
    /\/\/.*/,
    (input, { start, end }): Comment => ({ type: 'comment', value: input.slice(start, end), range: { start, end } })
)

const filterKeyword = scanToken(new RegExp(`-?(${filterTypeKeysWithAliases.join('|')})+(?=:)`, 'i'))

const filterDelimiter = character(':')

const filterValue = oneOf<Literal>(quoted('"'), quoted("'"), scanBalancedLiteral, literal)

const openingParen = scanToken(/\(/, (_input, range): OpeningParen => ({ type: 'openingParen', range }))

const closingParen = scanToken(/\)/, (_input, range): ClosingParen => ({ type: 'closingParen', range }))

/**
 * Returns a {@link Scanner} that succeeds if a token scanned by `scanToken`,
 * followed by whitespace or EOF, is found in the search query.
 */
const followedBy = (scanToken: Scanner<Token>, scanNext: Scanner<Token>): Scanner<Token[]> => (input, start) => {
    const tokens: Token[] = []
    const tokenResult = scanToken(input, start)
    if (tokenResult.type === 'error') {
        return tokenResult
    }
    tokens.push(tokenResult.term)
    let { end } = tokenResult.term.range
    if (input[end] !== undefined) {
        const separatorResult = scanNext(input, end)
        if (separatorResult.type === 'error') {
            return separatorResult
        }
        tokens.push(separatorResult.term)
        end = separatorResult.term.range.end
    }
    return {
        type: 'success',
        term: tokens,
    }
}

/**
 * A {@link Scanner} that will attempt to scan {@link Filter} tokens
 * (consisting a of a filter type and a filter value, separated by a colon)
 * in a search query.
 */
const filter: Scanner<Filter> = (input, start) => {
    const scannedKeyword = filterKeyword(input, start)
    if (scannedKeyword.type === 'error') {
        return scannedKeyword
    }
    const scannedDelimiter = filterDelimiter(input, scannedKeyword.term.range.end)
    if (scannedDelimiter.type === 'error') {
        return scannedDelimiter
    }
    const scannedValue =
        input[scannedDelimiter.term.range.end] === undefined
            ? undefined
            : filterValue(input, scannedDelimiter.term.range.end)
    if (scannedValue && scannedValue.type === 'error') {
        return scannedValue
    }
    return {
        type: 'success',
        term: {
            type: 'filter',
            range: { start, end: scannedValue ? scannedValue.term.range.end : scannedDelimiter.term.range.end },
            field: scannedKeyword.term,
            value: scannedValue?.term,
            negated: scannedKeyword.term.value.startsWith('-'),
        },
    }
}

const createPattern = (value: string, range: CharacterRange, kind: PatternKind): ScanSuccess<Pattern> => ({
    type: 'success',
    term: {
        type: 'pattern',
        range,
        kind,
        value,
    },
})

const scanFilterOrKeyword = oneOf<Literal | Token[]>(filterKeyword, followedBy(keyword, whitespace))
const keepScanning = (input: string, start: number): boolean => scanFilterOrKeyword(input, start).type !== 'success'

/**
 * A helper function that maps a {@link Literal} scanner result to a {@link Pattern} scanner.
 *
 * @param scanner The literal scanner.
 * @param kind The {@link PatternKind} label to apply to the resulting pattern scanner.
 */
export const toPatternResult = (scanner: Scanner<Literal>, kind: PatternKind): Scanner<Pattern> => (input, start) => {
    const result = scanner(input, start)
    if (result.type === 'success') {
        return createPattern(result.term.value, result.term.range, kind)
    }
    return result
}

const scanPattern = (kind: PatternKind): Scanner<Pattern> =>
    toPatternResult(oneOf<Literal>(scanBalancedLiteral, literal), kind)

const whitespaceOrClosingParen = oneOf<Whitespace | ClosingParen>(whitespace, closingParen)

/**
 * A {@link Scanner} for a Sourcegraph search query, interpreting patterns for {@link PatternKind}.
 *
 * @param interpretComments Interpets C-style line comments for multiline queries.
 */
const createScanner = (kind: PatternKind, interpretComments?: boolean): Scanner<Token[]> => {
    const baseQuotedScanner = [quoted('"'), quoted("'")]
    const quotedScanner = kind === PatternKind.Regexp ? [quoted('/'), ...baseQuotedScanner] : baseQuotedScanner

    const baseScanner = [keyword, filter, ...quotedScanner, scanPattern(kind)]
    const tokenScanner: Scanner<Token>[] = interpretComments ? [comment, ...baseScanner] : baseScanner

    const baseEarlyPatternScanner = [...quotedScanner, toPatternResult(scanBalancedLiteral, kind)]
    const earlyPatternScanner = interpretComments ? [comment, ...baseEarlyPatternScanner] : baseEarlyPatternScanner

    return zeroOrMore(
        oneOf<Term>(
            whitespace,
            ...earlyPatternScanner.map(token => followedBy(token, whitespaceOrClosingParen)),
            openingParen,
            closingParen,
            ...tokenScanner.map(token => followedBy(token, whitespaceOrClosingParen))
        )
    )
}

/**
 * Scans a search query string.
 */
export const scanSearchQuery = (
    query: string,
    interpretComments?: boolean,
    searchPatternType = SearchPatternType.literal
): ScanResult<Token[]> => {
    let patternKind
    switch (searchPatternType) {
        case SearchPatternType.literal:
            patternKind = PatternKind.Literal
            break
        case SearchPatternType.regexp:
            patternKind = PatternKind.Regexp
            break
        case SearchPatternType.structural:
            patternKind = PatternKind.Structural
            break
    }
    const scanner = createScanner(patternKind, interpretComments)
    return scanner(query, 0)
}
