import { SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'
import { FilterType } from '@sourcegraph/shared/src/search/query/filters'
import { FilterKind, findFilter } from '@sourcegraph/shared/src/search/query/query'

/**
 * Returns pattern type filter value if it's represented in the query string,
 * otherwise returns default value for patternType filter - literal
 */
export function getQueryPatternTypeFilter(query: string, defaultPatternType: SearchPatternType): SearchPatternType {
    const patternType = findFilter(query, FilterType.patterntype, FilterKind.Global)

    if (patternType?.value) {
        switch (patternType.value.value) {
            case SearchPatternType.regexp: {
                return SearchPatternType.regexp
            }
            case SearchPatternType.structural: {
                return SearchPatternType.structural
            }
            case SearchPatternType.literal: {
                return SearchPatternType.literal
            }
            case SearchPatternType.standard: {
                return SearchPatternType.standard
            }
            case SearchPatternType.keyword: {
                return SearchPatternType.keyword
            }
            default: {
                return defaultPatternType
            }
        }
    }

    return defaultPatternType
}
