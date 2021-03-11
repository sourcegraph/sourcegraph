import { Filter, Literal } from './token'
import { SearchSuggestion } from '../suggestions'
import { Omit } from 'utility-types'
import { selectorCompletion } from './selectFilter'

export enum FilterType {
    after = 'after',
    archived = 'archived',
    author = 'author',
    before = 'before',
    case = 'case',
    committer = 'committer',
    content = 'content',
    context = 'context',
    count = 'count',
    file = 'file',
    fork = 'fork',
    index = 'index',
    lang = 'lang',
    message = 'message',
    patterntype = 'patterntype',
    repo = 'repo',
    repogroup = 'repogroup',
    repohascommitafter = 'repohascommitafter',
    repohasfile = 'repohasfile',
    // eslint-disable-next-line unicorn/prevent-abbreviations
    rev = 'rev',
    select = 'select',
    stable = 'stable',
    timeout = 'timeout',
    type = 'type',
    visibility = 'visibility',
}

/* eslint-disable unicorn/prevent-abbreviations */
export enum AliasedFilterType {
    f = 'file',
    g = 'repogroup',
    l = 'lang',
    language = 'lang',
    m = 'message',
    msg = 'message',
    r = 'repo',
    revision = 'rev',
    since = 'after',
    until = 'before',
}
/* eslint-enable unicorn/prevent-abbreviations */

export const isFilterType = (filter: string): filter is FilterType => filter in FilterType
export const isAliasedFilterType = (filter: string): boolean => filter in AliasedFilterType

export const filterTypeKeys: FilterType[] = Object.keys(FilterType) as FilterType[]
export const filterTypeKeysWithAliases: (FilterType | AliasedFilterType)[] = [
    ...filterTypeKeys,
    ...Object.keys(AliasedFilterType),
] as (FilterType | AliasedFilterType)[]

export enum NegatedFilters {
    author = '-author',
    committer = '-committer',
    content = '-content',
    f = '-f',
    file = '-file',
    l = '-l',
    lang = '-lang',
    message = '-message',
    r = '-r',
    repo = '-repo',
    repohasfile = '-repohasfile',
}

/** The list of filters that are able to be negated. */
export type NegatableFilter =
    | FilterType.repo
    | FilterType.file
    | FilterType.repohasfile
    | FilterType.lang
    | FilterType.content
    | FilterType.committer
    | FilterType.author
    | FilterType.message

export const isNegatableFilter = (filter: FilterType): filter is NegatableFilter =>
    Object.keys(NegatedFilters).includes(filter)

/** The list of all negated filters. i.e. all valid filters that have `-` as a suffix. */
export const negatedFilters = Object.values(NegatedFilters)

export const isNegatedFilter = (filter: string): filter is NegatedFilters =>
    negatedFilters.includes(filter as NegatedFilters)

const negatedFilterToNegatableFilter: { [key: string]: NegatableFilter } = {
    '-author': FilterType.author,
    '-committer': FilterType.committer,
    '-content': FilterType.content,
    '-f': FilterType.file,
    '-file': FilterType.file,
    '-l': FilterType.lang,
    '-lang': FilterType.lang,
    '-message': FilterType.message,
    '-r': FilterType.repo,
    '-repo': FilterType.repo,
    '-repohasfile': FilterType.repohasfile,
}

export const resolveNegatedFilter = (filter: NegatedFilters): NegatableFilter => negatedFilterToNegatableFilter[filter]

interface BaseFilterDefinition {
    alias?: string
    description: string
    discreteValues?: (value: Literal | undefined) => string[]
    suggestions?: SearchSuggestion['__typename'] | string[]
    default?: string
    /** Whether the filter may only be used 0 or 1 times in a query. */
    singular?: boolean
}

interface NegatableFilterDefinition extends Omit<BaseFilterDefinition, 'description'> {
    negatable: true
    description: (negated: boolean) => string
}

export type FilterDefinition = BaseFilterDefinition | NegatableFilterDefinition

export const LANGUAGES: string[] = [
    'c',
    'cpp',
    'csharp',
    'css',
    'go',
    'graphql',
    'haskell',
    'html',
    'java',
    'javascript',
    'json',
    'lua',
    'markdown',
    'php',
    'powershell',
    'python',
    'r',
    'ruby',
    'rust',
    'sass',
    'swift',
    'typescript',
]

export const FILTERS: Record<NegatableFilter, NegatableFilterDefinition> &
    Record<Exclude<FilterType, NegatableFilter>, BaseFilterDefinition> = {
    [FilterType.after]: {
        description: 'Commits made after a certain date',
    },
    [FilterType.archived]: {
        description: 'Include results from archived repositories.',
        singular: true,
    },
    [FilterType.author]: {
        negatable: true,
        description: negated => `${negated ? 'Exclude' : 'Include only'} commits or diffs authored by a user.`,
    },
    [FilterType.before]: {
        description: 'Commits made before a certain date',
    },
    [FilterType.case]: {
        description: 'Treat the search pattern as case-sensitive.',
        discreteValues: () => ['yes', 'no'],
        default: 'no',
        singular: true,
    },
    [FilterType.committer]: {
        description: (negated: boolean): string =>
            `${negated ? 'Exclude' : 'Include only'} commits and diffs committed by a user.`,
        negatable: true,
        singular: true,
    },
    [FilterType.content]: {
        description: (negated: boolean): string =>
            `${negated ? 'Exclude' : 'Include only'} results from files if their content matches the search pattern.`,
        negatable: true,
        singular: true,
    },
    [FilterType.context]: {
        description: 'Search only repositories within a specified context',
        singular: true,
        suggestions: 'SearchContext',
    },
    [FilterType.count]: {
        description: 'Number of results to fetch (integer)',
        singular: true,
    },
    [FilterType.file]: {
        alias: 'f',
        negatable: true,
        description: negated =>
            `${negated ? 'Exclude' : 'Include only'} results from files matching the given search pattern.`,
        suggestions: 'File',
    },
    [FilterType.fork]: {
        discreteValues: () => ['yes', 'no', 'only'],
        description: 'Include results from forked repositories.',
        singular: true,
    },
    [FilterType.index]: {
        discreteValues: () => ['yes', 'no', 'only'],
        description: 'Include results from indexed repositories',
        singular: true,
    },
    [FilterType.lang]: {
        negatable: true,
        description: negated => `${negated ? 'Exclude' : 'Include only'} results from the given language`,
        suggestions: LANGUAGES,
    },
    [FilterType.message]: {
        negatable: true,
        description: negated =>
            `${negated ? 'Exclude' : 'Include only'} Commits with messages matching a certain string`,
    },
    [FilterType.patterntype]: {
        discreteValues: () => ['regexp', 'literal', 'structural'],
        description: 'The pattern type (regexp, literal, structural) in use',
        singular: true,
    },
    [FilterType.repo]: {
        alias: 'r',
        negatable: true,
        description: negated =>
            `${negated ? 'Exclude' : 'Include only'} results from repositories matching the given search pattern.`,
        suggestions: 'Repository',
    },
    [FilterType.repogroup]: {
        description: 'group-name (include results from the named group)',
        singular: true,
        suggestions: 'RepoGroup',
    },
    [FilterType.repohascommitafter]: {
        description: '"string specifying time frame" (filter out stale repositories without recent commits)',
        singular: true,
    },
    [FilterType.repohasfile]: {
        negatable: true,
        description: negated =>
            `${negated ? 'Exclude' : 'Include only'} results from repos that contain a matching file`,
    },
    [FilterType.rev]: {
        description: 'Search a revision (branch, commit hash, or tag) instead of the default branch.',
        singular: true,
    },
    [FilterType.select]: {
        discreteValues: value => selectorCompletion(value),
        description: 'Selects the kind of result to display.',
        singular: true,
    },
    [FilterType.stable]: {
        discreteValues: () => ['yes', 'no'],
        default: 'no',
        description: 'Forces search to return a stable result ordering (currently limited to file content matches).',
        singular: true,
    },
    [FilterType.timeout]: {
        description: 'Duration before timeout',
        singular: true,
    },
    [FilterType.type]: {
        description: 'Limit results to the specified type.',
        discreteValues: () => ['diff', 'commit', 'symbol', 'repo', 'path', 'file'],
    },
    [FilterType.visibility]: {
        discreteValues: () => ['any', 'private', 'public'],
        description: 'Include results from repositories with the matching visibility (private, public, any).',
        singular: true,
    },
}

export const discreteValueAliases: { [key: string]: string[] } = {
    yes: ['yes', 'y', 'Y', 'YES', 'Yes', '1', 't', 'T', 'true', 'TRUE', 'True'],
    no: ['n', 'N', 'no', 'NO', 'No', '0', 'f', 'F', 'false', 'FALSE', 'False'],
    only: ['o', 'only', 'ONLY', 'Only'],
}

/**
 * Returns the {@link FilterDefinition} for the given filterType if it exists, or `undefined` otherwise.
 */
export const resolveFilter = (
    filterType: string
):
    | { type: NegatableFilter; negated: boolean; definition: NegatableFilterDefinition }
    | { type: Exclude<FilterType, NegatableFilter>; definition: BaseFilterDefinition }
    | undefined => {
    filterType = filterType.toLowerCase()

    if (isAliasedFilterType(filterType)) {
        const aliasKey = filterType as keyof typeof AliasedFilterType
        filterType = AliasedFilterType[aliasKey]
    }

    if (isNegatedFilter(filterType)) {
        const type = resolveNegatedFilter(filterType)
        return {
            type,
            definition: FILTERS[type],
            negated: true,
        }
    }
    if (isFilterType(filterType)) {
        if (isNegatableFilter(filterType)) {
            return {
                type: filterType,
                definition: FILTERS[filterType],
                negated: false,
            }
        }
        if (FILTERS[filterType]) {
            return { type: filterType, definition: FILTERS[filterType] }
        }
    }
    for (const [type, definition] of Object.entries(FILTERS as Record<FilterType, FilterDefinition>)) {
        if (definition.alias && filterType === definition.alias) {
            return {
                type: type as Exclude<FilterType, NegatableFilter>,
                definition: definition as BaseFilterDefinition,
            }
        }
    }
    return undefined
}

/**
 * Checks whether a discrete value is valid for a given filter, accounting for valid aliases.
 */
const isValidDiscreteValue = (
    definition: NegatableFilterDefinition | BaseFilterDefinition,
    input: Literal,
    value: string
): boolean => {
    if (!definition.discreteValues || definition.discreteValues(input).includes(value)) {
        return true
    }

    const validDiscreteValuesForDefinition = Object.keys(discreteValueAliases).filter(
        key => !definition.discreteValues || definition.discreteValues(input).includes(key)
    )

    for (const discreteValue of validDiscreteValuesForDefinition) {
        if (discreteValueAliases[discreteValue].includes(value)) {
            return true
        }
    }
    return false
}

/**
 * Validates a filter given its field and value.
 */
export const validateFilter = (
    field: string,
    value: Filter['value']
): { valid: true } | { valid: false; reason: string } => {
    const typeAndDefinition = resolveFilter(field)
    if (!typeAndDefinition) {
        return { valid: false, reason: 'Invalid filter type.' }
    }
    const { definition } = typeAndDefinition
    if (definition.discreteValues && (!value || !isValidDiscreteValue(definition, value, value.value))) {
        return {
            valid: false,
            reason: `Invalid filter value, expected one of: ${definition.discreteValues(value).join(', ')}.`,
        }
    }
    return { valid: true }
}

/**
 * Prepends a \ to spaces, taking care to skip over existing escape sequences. We apply this to
 * regexp field values like repo: and file:.
 *
 * @param value the value to escape
 */
export const escapeSpaces = (value: string): string => {
    const escaped: string[] = []
    let current = 0
    while (value[current]) {
        switch (value[current]) {
            case '\\': {
                if (value[current + 1]) {
                    escaped.push('\\', value[current + 1])
                    current = current + 2 // Continue past escaped value.
                    continue
                }
                escaped.push('\\')
                current = current + 1
                continue
            }
            case ' ': {
                escaped.push('\\', ' ')
                current = current + 1
                continue
            }
            default:
                escaped.push(value[current])
                current = current + 1
                continue
        }
    }
    return escaped.join('')
}
