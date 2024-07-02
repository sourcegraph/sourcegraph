// NOTE (@fkling): The use of 'zustand' in this codebase should be considered as
// experimental until we had more time to evaluate this library. General
// application of this library is not recommended at this point.
// It is used here because it solves a very real performance issue
// (see https://github.com/sourcegraph/sourcegraph/issues/21200).
import create from 'zustand'

import {
    type BuildSearchQueryURLParameters,
    canSubmitSearch,
    InitialParametersSource,
    SearchMode,
    type SearchQueryState,
    updateQuery,
} from '@sourcegraph/shared/src/search'
import { FilterType } from '@sourcegraph/shared/src/search/query/filters'
import type { Settings, SettingsCascadeOrError } from '@sourcegraph/shared/src/settings/settings'
import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'

import { SearchPatternType } from '../graphql-operations'
import type { ParsedSearchURL } from '../search'
import { submitSearch } from '../search/helpers'
import {
    defaultCaseSensitiveFromSettings,
    defaultPatternTypeFromSettings,
    defaultSearchModeFromSettings,
} from '../util/settings'

export interface NavbarQueryState extends SearchQueryState {}

export const useNavbarQueryState = create<NavbarQueryState>((set, get) => ({
    parametersSource: InitialParametersSource.DEFAULT,
    queryState: { query: '' },
    searchCaseSensitivity: false,
    searchPatternType: SearchPatternType.keyword,
    defaultPatternType: SearchPatternType.keyword,
    searchMode: SearchMode.Precise,
    searchQueryFromURL: '',

    setQueryState: queryStateUpdate => {
        if (typeof queryStateUpdate === 'function') {
            set({ queryState: queryStateUpdate(get().queryState) })
        } else {
            set({ queryState: queryStateUpdate })
        }
    },

    submitSearch: (parameters, updates = []) => {
        const {
            queryState,
            searchCaseSensitivity: caseSensitive,
            searchPatternType: patternType,
            searchMode: searchMode,
        } = get()

        const query = parameters.query ?? queryState.query
        const updatedQuery = updateQuery(query, updates)

        if (canSubmitSearch(query, parameters.selectedSearchContextSpec)) {
            submitSearch({
                ...parameters,
                query: updatedQuery,
                caseSensitive,
                patternType,
                searchMode,
                telemetryRecorder: parameters.telemetryRecorder,
            })
        }
    },
}))

export function setSearchPatternType(searchPatternType: SearchPatternType): void {
    // When changing the patterntype, we also need to reset the query to strip out any potential patterntype: filter
    const state = useNavbarQueryState.getState()
    const query = state.searchQueryFromURL ?? state.queryState.query
    useNavbarQueryState.setState({ searchPatternType, queryState: { query } })
}

export function setSearchCaseSensitivity(searchCaseSensitivity: boolean): void {
    useNavbarQueryState.setState({ searchCaseSensitivity })
}

export function setSearchMode(searchMode: SearchMode): void {
    useNavbarQueryState.setState({ searchMode })
}

/**
 * Update or initialize query state related data from URL search parameters.
 *
 * @param parsedSearchURL contains the information extracted from a URL
 * @param query can be used to specify the query to use when it differs from
 * the one contained in the URL (e.g. when the context:... filter got removed)
 */
export function setQueryStateFromURL(parsedSearchURL: ParsedSearchURL, query = parsedSearchURL.query ?? ''): void {
    const currentState = useNavbarQueryState.getState()
    if (currentState.parametersSource > InitialParametersSource.URL) {
        return
    }

    // This will be updated with the default in settings when the web app mounts.
    const newState: Partial<
        Pick<
            NavbarQueryState,
            | 'queryState'
            | 'searchPatternType'
            | 'searchCaseSensitivity'
            | 'searchQueryFromURL'
            | 'parametersSource'
            | 'searchMode'
        >
    > = {}

    if (parsedSearchURL.query) {
        // Only update flags if the URL contains a search query.
        newState.parametersSource = InitialParametersSource.URL
        newState.searchCaseSensitivity = parsedSearchURL.caseSensitive

        const parsedPatternType = parsedSearchURL.patternType
        if (parsedPatternType !== undefined) {
            newState.searchPatternType = parsedPatternType
            if (showPatternTypeInQuery(parsedPatternType, currentState.defaultPatternType)) {
                query = `${query} ${FilterType.patterntype}:${parsedPatternType}`
            }
        }
        newState.queryState = { query }
        newState.searchQueryFromURL = parsedSearchURL.query
        newState.searchMode = parsedSearchURL.searchMode
    }

    // The way Zustand is designed makes it difficult to build up a partial new
    // state object, hence the cast to any here.
    useNavbarQueryState.setState(newState as any)
}

// The only pattern types explicitly represented in the UI are the default one, plus regexp and structural. For
// other pattern types, we make sure to surface them in the query input itself.
export function showPatternTypeInQuery(
    patternType: SearchPatternType,
    defaultPatternType?: SearchPatternType
): boolean {
    return patternType !== defaultPatternType && !explicitPatternTypes.has(patternType)
}

const explicitPatternTypes = new Set([
    SearchPatternType.regexp,
    SearchPatternType.structural,
    SearchPatternType.keyword,
])

/**
 * Update or initialize query state related data from settings
 */
export function setQueryStateFromSettings(settings: SettingsCascadeOrError<Settings>): void {
    if (useNavbarQueryState.getState().parametersSource > InitialParametersSource.USER_SETTINGS) {
        return
    }

    const newState: Partial<
        Pick<
            NavbarQueryState,
            'searchPatternType' | 'defaultPatternType' | 'searchCaseSensitivity' | 'parametersSource' | 'searchMode'
        >
    > = {
        parametersSource: InitialParametersSource.USER_SETTINGS,
    }

    const caseSensitive = defaultCaseSensitiveFromSettings(settings)
    if (caseSensitive !== undefined) {
        newState.searchCaseSensitivity = caseSensitive
    }

    const searchMode = defaultSearchModeFromSettings(settings)
    if (searchMode !== undefined) {
        newState.searchMode = searchMode
    }

    const searchPatternType = defaultPatternTypeFromSettings(settings)
    if (searchPatternType) {
        newState.searchPatternType = searchPatternType
        newState.defaultPatternType = searchPatternType
    }

    // The way Zustand is designed makes it difficult to build up a partial new
    // state object, hence the cast to any here.
    useNavbarQueryState.setState(newState as any)
}

/**
 * This function wraps 'buildSearchURLQuery' and uses values from the global
 * query state for parameters that have not been supplied. This provides a
 * simple way for components to compose new queries with global query state
 * without needing access to that state themselves.
 */
export function buildSearchURLQueryFromQueryState(parameters: BuildSearchQueryURLParameters): string {
    const currentState = useNavbarQueryState.getState()

    return buildSearchURLQuery(
        parameters.query,
        parameters.patternType ?? currentState.searchPatternType,
        parameters.caseSensitive ?? currentState.searchCaseSensitivity,
        parameters.searchContextSpec,
        parameters.searchMode ?? currentState.searchMode
    )
}
