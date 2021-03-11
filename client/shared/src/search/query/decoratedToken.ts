import * as Monaco from 'monaco-editor'
import { Token, Pattern, Literal, PatternKind, CharacterRange, createLiteral } from './token'
import { RegExpParser, visitRegExpAST } from 'regexpp'
import {
    Alternative,
    Assertion,
    CapturingGroup,
    Character,
    CharacterClass,
    CharacterClassRange,
    CharacterSet,
    Group,
    Quantifier,
} from 'regexpp/ast'

/* eslint-disable unicorn/better-regex */

/**
 * A DecoratedToken is a type of token used for syntax highlighting, hovers, and diagnostics. All
 * standard Token types are compatible where DecoratedTokens are used. A DecoratedToken extends
 * the definition of standard tokens to language-specific metasyntax tokens, like .* in regexp,
 * or :[holes] in structural search.
 */
export type DecoratedToken = Token | MetaToken

/**
 * A MetaToken defines a token that is associated with some language-specific metasyntax.
 */
export type MetaToken =
    | MetaRegexp
    | MetaStructural
    | MetaField
    | MetaRepoRevisionSeparator
    | MetaRevision
    | MetaContextPrefix
    | MetaSelector
    | MetaPath

/**
 * Defines common properties for meta tokens.
 */
export interface BaseMetaToken {
    type: MetaToken['type']
    range: CharacterRange
    value: string
}

/**
 * A token that is labeled and interpreted as regular expression syntax.
 */
export interface MetaRegexp extends BaseMetaToken {
    type: 'metaRegexp'
    groupRange?: CharacterRange
    kind: MetaRegexpKind
}

/**
 * Classifications of the kinds of regexp metasyntax.
 */
export enum MetaRegexpKind {
    Assertion = 'Assertion', // like ^ or \b
    Alternative = 'Alternative', // like |
    Delimited = 'Delimited', // like ( or )
    EscapedCharacter = 'EscapedCharacter', // like \(
    CharacterSet = 'CharacterSet', // like \s
    CharacterClass = 'CharacterClass', // like [a-z]
    CharacterClassRange = 'CharacterClassRange', // the a-z part in [a-z]
    CharacterClassRangeHyphen = 'CharacterClassRangeHyphen', // the - part in [a-z]
    CharacterClassMember = 'CharacterClassMember', // a character inside a charcter class like [abcd]
    LazyQuantifier = 'LazyQuantifier', // the ? after a range quantifier
    RangeQuantifier = 'RangeQuantifier', // like +
}

/**
 * A token that is labeled and interpreted as structural search syntax.
 */
export interface MetaStructural extends BaseMetaToken {
    type: 'metaStructural'
    groupRange?: CharacterRange
    kind: MetaStructuralKind
}

/**
 * Classifications of the kinds of structural metasyntax.
 */
export enum MetaStructuralKind {
    Hole = 'Hole',
    RegexpHole = 'RegexpHole',
    Variable = 'Variable',
    RegexpSeparator = 'RegexpSeparator',
}

/**
 * A token that is labeled and interpreted as a field, like "repo:" in the Sourcegraph language syntax.
 */
export interface MetaField extends BaseMetaToken {
    type: 'field'
}

/**
 * A token that is labeled and interpreted as repository revision syntax in Sourcegraph. Note: there
 * are syntactic differences from pure Git ref syntax.
 * See https://docs.sourcegraph.com/code_search/reference/queries#repository-revisions.
 */
export interface MetaRevision extends BaseMetaToken {
    type: 'metaRevision'
    kind: MetaRevisionKind
}

export type MetaRevisionKind = MetaGitRevision | MetaSourcegraphRevision

/**
 * A custom revision syntax that is only valid in Sourcegraph search queries.
 */
export enum MetaSourcegraphRevision {
    Separator = 'Separator', // a ':' that separates revision patterns
    IncludeGlobMarker = 'IncludeGlobMarker', // a '*' at the beginning of a revision pattern to mark it as an "include" glob pattern.
    ExcludeGlobMarker = 'ExcludeGlobMarker', // a '*!' at the beginning of a revision pattern to mark it as an "exclude" glob pattern.
}

/**
 * Revision syntax that correspond to git glob pattern syntax, git refs (e.g., branches), or git objects (e.g., commits, tags).
 */
export enum MetaGitRevision {
    CommitHash = 'CommitHash', // a commit hash
    Label = 'Label', // a catch-all string that refers to a git object or ref, like a branch name or tag
    ReferencePath = 'ReferencePath', // the part of a revision that refers to a git reference path, like refs/heads/ part in refs/heads/*
    Wildcard = 'Wildcard', // a '*' in glob syntax
}

/**
 * A token that denotes a revision separator in the Sourcegraph query language. I.e., the '@' in
 * "repo:^foo$@revision" syntax.
 */
export interface MetaRepoRevisionSeparator extends BaseMetaToken {
    type: 'metaRepoRevisionSeparator'
}

/**
 * A token that denotes the context prefix in a search context value (the '@' in "context:@user").
 */
export interface MetaContextPrefix extends BaseMetaToken {
    type: 'metaContextPrefix'
}

export interface MetaSelector extends BaseMetaToken {
    type: 'metaSelector'
    kind: MetaSelectorKind
}

export enum MetaSelectorKind {
    Repo = 'repo',
    File = 'file',
    Content = 'content',
    Symbol = 'symbol',
    Commit = 'commit',
}

export enum MetaPathKind {
    Separator = 'Separator',
}

/**
 * Tokens that are meaningful in path patterns, like
 * path separators / or wildcards *.
 */
export interface MetaPath {
    type: 'metaPath'
    range: CharacterRange
    kind: MetaPathKind
    value: string
}

/**
 * Coalesces consecutive pattern tokens. Used, for example, when parsing
 * literal characters like 'f', 'o', 'o' in regular expressions, which are
 * coalesced to 'foo' for hovers.
 */
const coalescePatterns = (tokens: DecoratedToken[]): DecoratedToken[] => {
    let previous: Pattern | undefined
    const newTokens: DecoratedToken[] = []
    for (const token of tokens) {
        if (token.type === 'pattern') {
            if (previous === undefined) {
                previous = token
                continue
            }
            // Merge with previous
            previous.value = previous.value + token.value
            previous.range = { start: previous.range.start, end: token.range.end }
            continue
        }
        if (previous) {
            newTokens.push(previous)
            previous = undefined
        }
        newTokens.push(token)
    }
    if (previous) {
        newTokens.push(previous)
    }
    return newTokens
}

const mapRegexpMeta = (pattern: Pattern): DecoratedToken[] | undefined => {
    const tokens: DecoratedToken[] = []
    try {
        const ast = new RegExpParser().parsePattern(pattern.value)
        const offset = pattern.range.start
        visitRegExpAST(ast, {
            onAlternativeEnter(node: Alternative) {
                // regexpp doesn't tell us where a '|' operator is. We infer it by visiting any
                // pattern of an Alternative node, and for a '|' directly after it. Based on
                // regexpp's implementation, we know this is a true '|' operator, and _not_ an
                // escaped \| or part of a character class like [abcd|].
                if (pattern.value[node.end] && pattern.value[node.end] === '|') {
                    tokens.push({
                        type: 'metaRegexp',
                        range: { start: offset + node.end, end: offset + node.end + 1 },
                        value: '|',
                        kind: MetaRegexpKind.Alternative,
                    })
                }
            },
            onAssertionEnter(node: Assertion) {
                tokens.push({
                    type: 'metaRegexp',
                    range: { start: offset + node.start, end: offset + node.end },
                    value: node.raw,
                    kind: MetaRegexpKind.Assertion,
                })
            },
            onGroupEnter(node: Group) {
                // Push the leading '('
                tokens.push({
                    type: 'metaRegexp',
                    range: { start: offset + node.start, end: offset + node.start + 1 },
                    value: '(',
                    kind: MetaRegexpKind.Delimited,
                })
                // Push the trailing ')'
                tokens.push({
                    type: 'metaRegexp',
                    range: { start: offset + node.end - 1, end: offset + node.end },
                    value: ')',
                    kind: MetaRegexpKind.Delimited,
                })
            },
            onCapturingGroupEnter(node: CapturingGroup) {
                // Push the leading '('
                tokens.push({
                    type: 'metaRegexp',
                    range: { start: offset + node.start, end: offset + node.start + 1 },
                    groupRange: { start: offset + node.start, end: offset + node.end },
                    value: '(',
                    kind: MetaRegexpKind.Delimited,
                })
                // Push the trailing ')'
                tokens.push({
                    type: 'metaRegexp',
                    range: { start: offset + node.end - 1, end: offset + node.end },
                    groupRange: { start: offset + node.start, end: offset + node.end },
                    value: ')',
                    kind: MetaRegexpKind.Delimited,
                })
            },
            onCharacterSetEnter(node: CharacterSet) {
                tokens.push({
                    type: 'metaRegexp',
                    range: { start: offset + node.start, end: offset + node.end },
                    value: node.raw,
                    kind: MetaRegexpKind.CharacterSet,
                })
            },
            onCharacterClassEnter(node: CharacterClass) {
                const negatedOffset = node.negate ? 1 : 0
                // Push the leading '['
                tokens.push({
                    type: 'metaRegexp',
                    range: { start: offset + node.start, end: offset + node.start + 1 + negatedOffset },
                    groupRange: { start: offset + node.start, end: offset + node.end },
                    value: node.negate ? '[^' : '[',
                    kind: MetaRegexpKind.CharacterClass,
                })
                // Push the trailing ']'
                tokens.push({
                    type: 'metaRegexp',
                    range: { start: offset + node.end - 1, end: offset + node.end },
                    groupRange: { start: offset + node.start, end: offset + node.end },
                    value: ']',
                    kind: MetaRegexpKind.CharacterClass,
                })
            },
            onCharacterClassRangeEnter(node: CharacterClassRange) {
                // Push the min and max characters of the range to associate them with the
                // same groupRange for hovers.
                tokens.push(
                    {
                        type: 'metaRegexp',
                        range: { start: offset + node.min.start, end: offset + node.min.end },
                        groupRange: { start: offset + node.start, end: offset + node.end },
                        value: node.raw,
                        kind: MetaRegexpKind.CharacterClassRange,
                    },
                    // Highlight the '-' in [a-z]. Take care to use node.min.end, because we
                    // don't want to highlight the first '-' in [--z], nor an escaped '-' with a
                    // two-character offset as in [\--z].
                    {
                        type: 'metaRegexp',
                        range: { start: offset + node.min.end, end: offset + node.min.end + 1 },
                        groupRange: { start: offset + node.start, end: offset + node.end },
                        value: node.raw,
                        kind: MetaRegexpKind.CharacterClassRangeHyphen,
                    },
                    {
                        type: 'metaRegexp',
                        range: { start: offset + node.max.start, end: offset + node.max.end },
                        groupRange: { start: offset + node.start, end: offset + node.end },
                        value: node.raw,
                        kind: MetaRegexpKind.CharacterClassRange,
                    }
                )
            },
            onQuantifierEnter(node: Quantifier) {
                // the lazy quantifier ? adds one
                const lazyQuantifierOffset = node.greedy ? 0 : 1
                if (!node.greedy) {
                    tokens.push({
                        type: 'metaRegexp',
                        range: { start: offset + node.end - 1, end: offset + node.end },
                        value: '?',
                        kind: MetaRegexpKind.LazyQuantifier,
                    })
                }

                const quantifier = node.raw[node.raw.length - lazyQuantifierOffset - 1]
                if (quantifier === '+' || quantifier === '*' || quantifier === '?') {
                    tokens.push({
                        type: 'metaRegexp',
                        range: {
                            start: offset + node.end - 1 - lazyQuantifierOffset,
                            end: offset + node.end - lazyQuantifierOffset,
                        },
                        value: quantifier,
                        kind: MetaRegexpKind.RangeQuantifier,
                    })
                } else {
                    // regexpp provides no easy way to tell whether the quantifier is a range '{number, number}',
                    // nor the offsets of this range.
                    // At this point we know it is none of +, *, or ?, so it is a ranged quantifier.
                    // We need to then find the opening brace of {number, number}, and go backwards from the end
                    // of this quantifier to avoid dealing with other leading braces that are not part of it.
                    let openBrace = node.end - 1 - lazyQuantifierOffset
                    while (pattern.value[openBrace] && pattern.value[openBrace] !== '{') {
                        openBrace = openBrace - 1
                    }
                    tokens.push({
                        type: 'metaRegexp',
                        range: { start: offset + openBrace, end: offset + node.end - lazyQuantifierOffset },
                        value: pattern.value.slice(openBrace, node.end - lazyQuantifierOffset),
                        kind: MetaRegexpKind.RangeQuantifier,
                    })
                }
            },
            onCharacterEnter(node: Character) {
                if (node.end - node.start > 1 && node.raw.startsWith('\\')) {
                    // This is an escaped value like `\.`, `\u0065`, `\x65`.
                    // If this escaped value is part of a range, like [a-\\],
                    // set the group range to associate it with hovers.
                    const groupRange =
                        node.parent.type === 'CharacterClassRange'
                            ? { start: offset + node.parent.start, end: offset + node.parent.end }
                            : undefined
                    tokens.push({
                        type: 'metaRegexp',
                        range: { start: offset + node.start, end: offset + node.end },
                        groupRange,
                        value: node.raw,
                        kind: MetaRegexpKind.EscapedCharacter,
                    })
                    return
                }
                if (node.parent.type === 'CharacterClassRange') {
                    return // This unescaped character is handled by onCharacterClassRangeEnter.
                }
                if (node.parent.type === 'CharacterClass') {
                    // This character is inside a character class like [abcd] and is contextually special for hover tooltips.
                    tokens.push({
                        type: 'metaRegexp',
                        range: { start: offset + node.start, end: offset + node.end },
                        value: node.raw,
                        kind: MetaRegexpKind.CharacterClassMember,
                    })
                    return
                }
                tokens.push({
                    type: 'pattern',
                    range: { start: offset + node.start, end: offset + node.end },
                    value: node.raw,
                    kind: PatternKind.Regexp,
                })
            },
        })
    } catch {
        return undefined
    }
    // The AST is not necessarily traversed in increasing range. We need
    // to sort by increasing range because the ordering is significant to Monaco.
    tokens.sort((left, right) => {
        if (left.range.start < right.range.start) {
            return -1
        }
        return 0
    })
    return coalescePatterns(tokens)
}

/**
 * Returns true for filter values that have path-like values, i.e., values that typically
 * contain path separators like `/`.
 */
export const hasPathLikeValue = (field: string): boolean => {
    const fieldName = field.startsWith('-') ? field.slice(1) : field
    switch (fieldName.toLocaleLowerCase()) {
        case 'repo':
        case 'r':
        case 'file':
        case 'f':
        case 'repohasfile':
            return true
        default:
            return false
    }
}

// Tokenize a literal value like "^foo/bar/baz$" by a path separator '/'.
const mapPathMeta = (token: Literal): DecoratedToken[] => {
    const tokens: DecoratedToken[] = []
    const offset = token.range.start
    let start = 0
    let current = 0
    while (token.value[current]) {
        if (token.value[current] === '\\') {
            current = current + 2 // Continue past escaped value.
            continue
        }
        if (token.value[current] === '/') {
            tokens.push(
                createLiteral(token.value.slice(start, current), { start: offset + start, end: offset + current - 1 })
            )
            tokens.push({
                type: 'metaPath',
                range: { start: offset + current, end: offset + current + 1 },
                kind: MetaPathKind.Separator,
                value: '/',
            })
            current = current + 1
            start = current
            continue
        }
        current = current + 1
    }
    // Push last token.
    tokens.push(createLiteral(token.value.slice(start, current), { start: offset + start, end: offset + current }))
    return tokens
}

/**
 * Tries to parse a pattern into decorated regexp tokens.
 * It always succeeds, even if regexp fails to parse.
 */
const mapRegexpMetaSucceed = (token: Pattern): DecoratedToken[] => mapRegexpMeta(token) || [token]

const toPattern = (token: Literal): Pattern => ({
    type: 'pattern',
    kind: PatternKind.Regexp,
    value: token.value,
    range: token.range,
})

/**
 * Tries to convert all literal tokens in a list of tokens to regular expression tokens.
 * If any tokens fail to parse, the result is undefined.
 */
const tryMapLiteralsToRegexp = (tokens: DecoratedToken[]): DecoratedToken[] | undefined => {
    const decorated: DecoratedToken[] = []
    for (const token of tokens) {
        if (token.type === 'literal') {
            const parsedRegexp = mapRegexpMeta(toPattern(token))
            if (!parsedRegexp) {
                return undefined
            }
            decorated.push(...parsedRegexp)
            continue
        }
        decorated.push(token)
    }
    return decorated
}

/**
 * A helper function for converting path-like regexp values like ^github.com/foo$ to
 * tokens that highlight regexp metasyntax and path separator syntax. It always succeeds,
 * even if regexp fails to parse.
 */
const mapPathMetaForRegexp = (token: Literal): DecoratedToken[] => {
    const patterns = tryMapLiteralsToRegexp(mapPathMeta(token))
    if (!patterns) {
        return mapRegexpMetaSucceed(toPattern(token))
    }
    return patterns
}

const mapRevisionMeta = (token: Literal): DecoratedToken[] => {
    const offset = token.range.start

    const decorated: DecoratedToken[] = []
    let current = ''
    let start = 0
    let accumulator: string[] = []

    const nextChar = (): string => {
        current = token.value[start]
        start = start + 1
        return current
    }

    // Appends a decorated token to the list of tokens, and resets the current accumulator to be empty.
    const appendDecoratedToken = (endIndex: number, kind?: MetaRevisionKind): void => {
        const value = accumulator.join('')
        if (kind === undefined) {
            if (value.includes('refs/')) {
                kind = MetaGitRevision.ReferencePath
            } else if (value.match(/^[\dA-Fa-f]+$/) && value.length > 6) {
                kind = MetaGitRevision.CommitHash
            } else {
                kind = MetaGitRevision.Label
            }
        }
        const range = { start: offset + endIndex - value.length, end: offset + endIndex }
        decorated.push({ type: 'metaRevision', kind, value, range })
        accumulator = []
    }

    // Return true when we're at the beginning of a revision: at the beginning of the string,
    // or when the preceding character is a ':' revision separator.
    const atRevision = (index: number): boolean =>
        index === 0 || (token.value[index - 1] !== null && token.value[index - 1] === ':')

    while (token.value[start] !== undefined) {
        current = nextChar()
        switch (current) {
            case '*': {
                appendDecoratedToken(start - 1) // Push the running revision string up to this special syntax.
                if (atRevision(start - 1) && token.value[start] !== '!') {
                    // Demarcates that this is an "include" glob pattern that follows.
                    accumulator.push(current)
                    appendDecoratedToken(start, MetaSourcegraphRevision.IncludeGlobMarker)
                } else if (atRevision(start - 1) && token.value[start] === '!') {
                    // Demarcates that this is an "exclude" glob pattern that follows.
                    current = nextChar() // Consume the '!'
                    accumulator.push('*!')
                    appendDecoratedToken(start, MetaSourcegraphRevision.ExcludeGlobMarker)
                } else {
                    accumulator.push(current)
                    appendDecoratedToken(start, MetaGitRevision.Wildcard)
                }
                break
            }
            case ':':
                appendDecoratedToken(start - 1)
                accumulator.push(current)
                appendDecoratedToken(start, MetaSourcegraphRevision.Separator)
                break
            default:
                accumulator.push(current)
        }
    }
    appendDecoratedToken(start)
    return decorated
}

const mapStructuralMeta = (pattern: Pattern): DecoratedToken[] => {
    const offset = pattern.range.start

    const decorated: DecoratedToken[] = []
    let current = ''
    let start = 0
    let token: string[] = []

    // Track context of whether we are inside an opening hole, e.g., after
    // ':['. Value is greater than 1 when inside.
    let open = 0
    // Track whether we are balanced inside a regular expression character
    // set like '[a]' inside an open hole, e.g., :[foo~[a]]. Value is greater
    // than 1 when inside.
    let inside = 0

    const nextChar = (): string => {
        current = pattern.value[start]
        start = start + 1
        return current
    }

    // Appends a decorated token to the list of tokens, and resets the current token to be empty.
    const appendDecoratedToken = (endIndex: number, kind: PatternKind.Literal | MetaStructuralKind.Hole): void => {
        const value = token.join('')
        const range = { start: offset + endIndex - value.length, end: offset + endIndex }
        if (kind === PatternKind.Literal) {
            decorated.push({ type: 'pattern', kind, value, range })
        } else if (value.match(/^:\[(\w*)~(.*)\]$/)) {
            // Handle regexp hole.
            const [, variable, pattern] = value.match(/^:\[(\w*)~(.*)\]$/)!
            const variableStart = range.start + 2 /* :[ */
            const variableRange = { start: variableStart, end: variableStart + variable.length }
            const patternStart = variableRange.end + 1 /* ~ */
            const patternRange = { start: patternStart, end: patternStart + pattern.length }
            decorated.push(
                ...([
                    {
                        type: 'metaStructural',
                        kind: MetaStructuralKind.RegexpHole,
                        range: { start: range.start, end: variableStart },
                        groupRange: range,
                        value: ':[',
                    },
                    {
                        type: 'metaStructural',
                        kind: MetaStructuralKind.Variable,
                        range: variableRange,
                        value: variable,
                    },
                    {
                        type: 'metaStructural',
                        kind: MetaStructuralKind.RegexpSeparator,
                        range: { start: variableRange.end, end: variableRange.end + 1 },
                        value: '~',
                    },
                    ...mapRegexpMetaSucceed({
                        type: 'pattern',
                        kind: PatternKind.Regexp,
                        range: patternRange,
                        value: pattern,
                    }),
                    {
                        type: 'metaStructural',
                        kind: MetaStructuralKind.RegexpHole,
                        range: { start: patternRange.end, end: patternRange.end + 1 },
                        groupRange: range,
                        value: ']',
                    },
                ] as DecoratedToken[])
            )
        } else {
            decorated.push({ type: 'metaStructural', kind, value, range })
        }
        token = []
    }

    while (pattern.value[start] !== undefined) {
        current = nextChar()
        switch (current) {
            case '.':
                // Look ahead and see if this is a ... hole alias.
                if (pattern.value.slice(start, start + 2) === '..') {
                    // It is a ... hole.
                    if (token.length > 0) {
                        // Append the value before this '...'.
                        appendDecoratedToken(start - 1, PatternKind.Literal)
                    }
                    start = start + 2
                    // Append the value of '...' after advancing.
                    token.push('...')
                    appendDecoratedToken(start, MetaStructuralKind.Hole)
                    continue
                }
                token.push('.')
                break
            case ':':
                if (open > 0) {
                    // ':' inside a hole, likely part of a regexp pattern.
                    token.push(':')
                    continue
                }
                if (pattern.value[start] !== undefined) {
                    // Look ahead and see if this is the start of a hole.
                    if (pattern.value[start] === '[') {
                        // It is the start of a hole, consume the '['.
                        current = nextChar()
                        open = open + 1
                        // Persist the literal token scanned up to this point.
                        appendDecoratedToken(start - 2, PatternKind.Literal)
                        token.push(':[')
                        continue
                    }
                    // Something else, push the ':' we saw and continue.
                    token.push(':')
                    continue
                }
                // Trailing ':'.
                token.push(current)
                break
            case '\\':
                if (pattern.value[start] !== undefined && open > 0) {
                    // Assume this is an escape sequence inside a regexp hole.
                    current = nextChar()
                    token.push('\\', current)
                    continue
                }
                token.push('\\')
                break
            case '[':
                if (open > 0) {
                    // Assume this is a character set inside a regexp hole.
                    inside = inside + 1
                    token.push('[')
                    continue
                }
                token.push('[')
                break
            case ']':
                if (open > 0 && inside > 0) {
                    // This ']' closes a regular expression inside a hole.
                    inside = inside - 1
                    token.push(current)
                    continue
                }
                if (open > 0) {
                    // This ']' closes a hole.
                    open = open - 1
                    token.push(']')
                    appendDecoratedToken(start, MetaStructuralKind.Hole)
                    continue
                }
                token.push(current)
                break
            default:
                token.push(current)
        }
    }
    if (token.length > 0) {
        // Append any left over literal at the end.
        appendDecoratedToken(start, PatternKind.Literal)
    }
    return decorated
}

/**
 * Returns true for filter values that have regexp values, e.g., repo, file.
 * Excludes FilterType.content because that depends on the pattern kind.
 */
export const hasRegexpValue = (field: string): boolean => {
    const fieldName = field.startsWith('-') ? field.slice(1) : field
    switch (fieldName.toLocaleLowerCase()) {
        case 'repo':
        case 'r':
        case 'file':
        case 'f':
        case 'repohasfile':
        case 'message':
        case 'msg':
        case 'm':
        case 'commiter':
        case 'author':
            return true
        default:
            return false
    }
}

const specifiesRevision = (value: string): boolean => value.match(/@/) !== null

const decorateRepoRevision = (token: Literal): DecoratedToken[] => {
    const [repo, revision] = token.value.split('@', 2)
    const offset = token.range.start

    return [
        ...mapPathMetaForRegexp(createLiteral(repo, { start: offset, end: offset + repo.length })),
        {
            type: 'metaRepoRevisionSeparator',
            value: '@',
            range: {
                start: offset + repo.length,
                end: offset + repo.length + 1,
            },
        },
        ...mapRevisionMeta(
            createLiteral(revision, {
                start: token.range.start + repo.length + 1,
                end: token.range.start + repo.length + 1 + revision.length,
            })
        ),
    ]
}

const decorateContext = (token: Literal): DecoratedToken[] => {
    if (!token.value.startsWith('@')) {
        return [token]
    }

    const { start, end } = token.range
    return [
        { type: 'metaContextPrefix', range: { start, end: start + 1 }, value: '@' },
        createLiteral(token.value.slice(1), { start: start + 1, end }),
    ]
}

const decorateSelector = (token: Literal): DecoratedToken[] => {
    const kind = token.value as MetaSelectorKind
    if (!kind) {
        return [token]
    }
    return [{ type: 'metaSelector', range: token.range, value: token.value, kind }]
}

export const decorate = (token: Token): DecoratedToken[] => {
    const decorated: DecoratedToken[] = []
    switch (token.type) {
        case 'pattern':
            switch (token.kind) {
                case PatternKind.Regexp:
                    decorated.push(...mapRegexpMetaSucceed(token))
                    break
                case PatternKind.Structural:
                    decorated.push(...mapStructuralMeta(token))
                    break
                case PatternKind.Literal:
                    decorated.push(token)
                    break
            }
            break
        case 'filter': {
            decorated.push({
                type: 'field',
                range: token.field.range,
                value: token.field.value,
            })
            if (
                token.value &&
                token.field.value.toLowerCase().match(/^-?(repo|r)$/i) &&
                token.value.type === 'literal' &&
                specifiesRevision(token.value.value)
            ) {
                decorated.push(...decorateRepoRevision(token.value))
            } else if (
                token.value &&
                token.field.value.toLowerCase().match(/rev|revision/i) &&
                token.value.type === 'literal'
            ) {
                decorated.push(...mapRevisionMeta(createLiteral(token.value.value, token.value.range)))
            } else if (token.value && token.value.type === 'literal' && hasRegexpValue(token.field.value)) {
                // Highlight fields with regexp values.
                if (hasPathLikeValue(token.field.value) && token.value?.type === 'literal') {
                    decorated.push(...mapPathMetaForRegexp(token.value))
                } else {
                    decorated.push(...mapRegexpMetaSucceed(toPattern(token.value)))
                }
            } else if (token.field.value === 'context' && token.value?.type === 'literal') {
                decorated.push(...decorateContext(token.value))
            } else if (token.field.value === 'select' && token.value?.type === 'literal') {
                decorated.push(...decorateSelector(token.value))
            } else if (token.value) {
                decorated.push(token.value)
            }
            break
        }
        default:
            decorated.push(token)
    }
    return decorated
}

const decoratedToMonaco = (token: DecoratedToken): Monaco.languages.IToken => {
    switch (token.type) {
        case 'field':
        case 'whitespace':
        case 'keyword':
        case 'comment':
        case 'openingParen':
        case 'closingParen':
        case 'metaRepoRevisionSeparator':
        case 'metaContextPrefix':
            return {
                startIndex: token.range.start,
                scopes: token.type,
            }
        case 'metaPath':
        case 'metaRevision':
        case 'metaRegexp':
        case 'metaStructural':
            // The scopes value is derived from the token type and its kind.
            // E.g., regexpMetaDelimited derives from {@link RegexpMeta} and {@link RegexpMetaKind}.
            return {
                startIndex: token.range.start,
                scopes: `${token.type}${token.kind}`,
            }

        default:
            return {
                startIndex: token.range.start,
                scopes: 'identifier',
            }
    }
}

const toMonaco = (token: Token): Monaco.languages.IToken[] => {
    switch (token.type) {
        case 'filter': {
            const monacoTokens: Monaco.languages.IToken[] = []
            monacoTokens.push({
                startIndex: token.field.range.start,
                scopes: 'field',
            })
            if (token.value) {
                monacoTokens.push({
                    startIndex: token.value.range.start,
                    scopes: 'identifier',
                })
            }
            return monacoTokens
        }
        case 'whitespace':
        case 'keyword':
        case 'comment':
            return [
                {
                    startIndex: token.range.start,
                    scopes: token.type,
                },
            ]
        default:
            return [
                {
                    startIndex: token.range.start,
                    scopes: 'identifier',
                },
            ]
    }
}

/**
 * Returns the tokens in a scanned search query displayed in the Monaco query input. If the experimental
 * decorate flag is true, a list of {@link DecoratedToken} provides more contextual highlighting for patterns.
 */
export const getMonacoTokens = (tokens: Token[], toDecorate = false): Monaco.languages.IToken[] =>
    toDecorate ? tokens.flatMap(token => decorate(token).map(decoratedToMonaco)) : tokens.flatMap(toMonaco)

/**
 * Converts a zero-indexed, single-line {@link CharacterRange} to a Monaco {@link IRange}.
 */
export const toMonacoRange = ({ start, end }: CharacterRange): Monaco.IRange => ({
    startLineNumber: 1,
    endLineNumber: 1,
    startColumn: start + 1,
    endColumn: end + 1,
})
