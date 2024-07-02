import { FilterType, resolveFilter } from '@sourcegraph/shared/src/search/query/filters'
import { scanSearchQuery } from '@sourcegraph/shared/src/search/query/scanner'
import type { Filter, Keyword } from '@sourcegraph/shared/src/search/query/token'

export interface Checks {
    isValidOperator: true | false | undefined
    isValidPatternType: true | false | undefined
    isNotRepo: true | false | undefined
    isNotContext: true | false | undefined
    isNotCommitOrDiff: true | false | undefined
    isNotRev: true | false | undefined
}

export const searchQueryValidator = (value: string | undefined): Checks => {
    if (!value || value.length === 0) {
        return {
            isValidOperator: undefined,
            isValidPatternType: undefined,
            isNotRepo: undefined,
            isNotContext: undefined,
            isNotCommitOrDiff: undefined,
            isNotRev: undefined,
        }
    }

    const tokens = scanSearchQuery(value)

    if (tokens.type === 'success') {
        const filters = tokens.term.filter(token => token.type === 'filter') as Filter[]
        const keywords = tokens.term.filter(token => token.type === 'keyword') as Keyword[]

        const hasAnd = keywords.some(filter => filter.kind === 'and')
        const hasOr = keywords.some(filter => filter.kind === 'or')
        const hasNot = keywords.some(filter => filter.kind === 'not')

        const hasInvalidPatternType = filters.some(
            filter =>
                resolveFilter(filter.field.value)?.type === FilterType.patterntype &&
                (filter.value?.value === 'literal' ||
                    filter.value?.value === 'structural' ||
                    filter.value?.value === 'standard' ||
                    filter.value?.value === 'keyword')
        )

        const hasRepo = filters.some(
            filter => resolveFilter(filter.field.value)?.type === FilterType.repo && filter.value
        )

        const hasRev = filters.some(
            filter => resolveFilter(filter.field.value)?.type === FilterType.rev && filter.value
        )

        const hasContext = filters.some(
            filter => resolveFilter(filter.field.value)?.type === FilterType.context && filter.value
        )

        const hasCommit = filters.some(
            filter => resolveFilter(filter.field.value)?.type === FilterType.type && filter.value?.value === 'commit'
        )

        const hasDiff = filters.some(
            filter => resolveFilter(filter.field.value)?.type === FilterType.type && filter.value?.value === 'diff'
        )

        return {
            isValidOperator: !hasAnd && !hasOr && !hasNot,
            isValidPatternType: !hasInvalidPatternType,
            isNotRepo: !hasRepo,
            isNotContext: !hasContext,
            isNotCommitOrDiff: !hasCommit && !hasDiff,
            isNotRev: !hasRev,
        }
    }

    return {
        isValidOperator: false,
        isValidPatternType: false,
        isNotRepo: false,
        isNotContext: false,
        isNotCommitOrDiff: false,
        isNotRev: false,
    }
}
