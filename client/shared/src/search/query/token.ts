/**
 * Represents a zero-indexed character range in a single-line search query.
 */
export interface CharacterRange {
    /** Zero-based character on the line */
    start: number
    /** Zero-based character on the line */
    end: number
}

/**
 * Defines common properties for tokens.
 */
export interface BaseToken {
    type: Token['type']
    range: CharacterRange
}

/**
 * All recognized tokens.
 */
export type Token = Whitespace | OpeningParen | ClosingParen | Keyword | Comment | Literal | Pattern | Filter

/**
 * A label associated with a pattern token. We don't use SearchPatternType because
 * that is used as a global quantifier for all patterns in a query. PatternKind
 * allows to qualify multiple pattern tokens differently within a single query.
 */
export enum PatternKind {
    Literal = 1,
    Regexp,
    Structural,
}

/**
 * A value interpreted as a pattern of kind {@link PatternKind}.
 */
export interface Pattern extends BaseToken {
    type: 'pattern'
    kind: PatternKind
    value: string
}

/**
 * Represents a value in a search query. E.g., either a quoted or unquoted pattern or field value.
 *
 * Example: `Conn`.
 */
export interface Literal extends BaseToken {
    type: 'literal'
    value: string
    quoted: boolean
}

/**
 * Represents a filter in a search query.
 *
 * Example: `repo:^github\.com\/sourcegraph\/sourcegraph$`.
 */
export interface Filter extends BaseToken {
    type: 'filter'
    field: Literal
    value: Literal | undefined
    negated: boolean
}

export enum KeywordKind {
    Or = 'or',
    And = 'and',
    Not = 'not',
}

/**
 * Represents a keyword in a search query.
 *
 * Current keywords are: AND, and, OR, or, NOT, not.
 */
export interface Keyword extends BaseToken {
    type: 'keyword'
    value: string
    kind: KeywordKind
}

/**
 * Represents a C-style comment, terminated by a newline.
 *
 * Example: `// Oh hai`
 */
export interface Comment extends BaseToken {
    type: 'comment'
    value: string
}

export interface Whitespace extends BaseToken {
    type: 'whitespace'
}

export interface OpeningParen extends BaseToken {
    type: 'openingParen'
}

export interface ClosingParen extends BaseToken {
    type: 'closingParen'
}

export const createLiteral = (value: string, range: CharacterRange, quoted = false): Literal => ({
    type: 'literal',
    value,
    range,
    quoted,
})
