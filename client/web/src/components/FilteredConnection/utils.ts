import type { GraphQLError } from 'graphql'
import type { Location } from 'history'

import { hasProperty } from '@sourcegraph/common'
import type { GraphQLResult } from '@sourcegraph/http-client'
import type { Scalars } from '@sourcegraph/shared/src/graphql-operations'

import type { Connection } from './ConnectionType'
import { QUERY_KEY } from './constants'
import type { FilteredConnectionFilter, FilteredConnectionFilterValue } from './FilterControl'

/** Checks if the passed value satisfies the GraphQL Node interface */
export const hasID = (value: unknown): value is { id: Scalars['ID'] } =>
    typeof value === 'object' && value !== null && hasProperty('id')(value) && typeof value.id === 'string'

export const hasDisplayName = (value: unknown): value is { displayName: Scalars['String'] } =>
    typeof value === 'object' &&
    value !== null &&
    hasProperty('displayName')(value) &&
    typeof value.displayName === 'string'

export const getFilterFromURL = (
    searchParameters: URLSearchParams,
    filters: FilteredConnectionFilter[] | undefined
): Record<string, FilteredConnectionFilterValue['value'] | null> => {
    const values: Record<string, FilteredConnectionFilterValue['value'] | null> = {}
    if (filters === undefined) {
        return values
    }
    for (const filter of filters) {
        const urlValue = searchParameters.get(filter.id)
        if (urlValue !== null) {
            const value = filter.values.find(value => value.value === urlValue)
            if (value !== undefined) {
                values[filter.id] = value.value
                continue
            }
        }
        // couldn't find a value, add default
        values[filter.id] = filter.values[0].value
    }
    return values
}

export const parseQueryInt = (searchParameters: URLSearchParams, name: string): number | null => {
    const valueString = searchParameters.get(name)
    if (valueString === null) {
        return null
    }
    const valueNumber = parseInt(valueString, 10)
    if (valueNumber > 0) {
        return valueNumber
    }
    return null
}

/**
 * Determine if a connection has a next page.
 * Provides fallback logic to support queries where `hasNextPage` is undefined.
 */
export const hasNextPage = (connection: Connection<unknown>): boolean =>
    connection.pageInfo
        ? connection.pageInfo.hasNextPage
        : typeof connection.totalCount === 'number' && connection.nodes.length < connection.totalCount

export interface GetUrlQueryParameters {
    first?: {
        actual: number
        default: number
    }
    query?: string
    filterValues?: Record<string, FilteredConnectionFilterValue['value'] | null>
    filters?: FilteredConnectionFilter[]
    visibleResultCount?: number
    search: Location['search']
}

/**
 * Determines the URL search parameters for a connection.
 */
export const getUrlQuery = ({
    first,
    query,
    filterValues,
    visibleResultCount,
    filters,
    search,
}: GetUrlQueryParameters): string => {
    const searchParameters = new URLSearchParams(search)

    if (query) {
        searchParameters.set(QUERY_KEY, query)
    }

    if (!!first && first.actual !== first.default) {
        searchParameters.set('first', String(first.actual))
    }

    if (filterValues && filters) {
        for (const filter of filters) {
            const value = filterValues[filter.id]
            if (value === undefined || value === null) {
                continue
            }
            if (value !== filter.values[0].value) {
                searchParameters.set(filter.id, value)
            } else {
                searchParameters.delete(filter.id)
            }
        }
    }

    if (visibleResultCount && visibleResultCount !== 0 && visibleResultCount !== first?.actual) {
        searchParameters.set('visible', String(visibleResultCount))
    }

    return searchParameters.toString()
}

interface AsGraphQLResultParameters<TResult> {
    data?: TResult
    errors: readonly GraphQLError[]
}

/**
 * Map non-conforming GraphQL responses to a GraphQLResult.
 */
export const asGraphQLResult = <T>({ data, errors }: AsGraphQLResultParameters<T>): GraphQLResult<T> => {
    if (!data) {
        return { data: null, errors }
    }
    return {
        data,
        errors: undefined,
    }
}
