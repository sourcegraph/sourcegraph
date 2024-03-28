import React, { type FC, useCallback, useEffect, useRef } from 'react'

import { useLocation, useNavigate } from 'react-router-dom'
import type { NavbarQueryState } from 'src/stores/navbarSearchQueryState'
import shallow from 'zustand/shallow'

import { SearchBox, LegacyToggles } from '@sourcegraph/branded'
import { Toggles } from '@sourcegraph/branded/src/search-ui/input/toggles/Toggles'
import { TraceSpanProvider } from '@sourcegraph/observability-client'
import {
    type CaseSensitivityProps,
    type SearchPatternTypeProps,
    type SubmitSearchParameters,
    canSubmitSearch,
    type QueryState,
    type SearchModeProps,
    getUserSearchContextNamespaces,
} from '@sourcegraph/shared/src/search'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { Form } from '@sourcegraph/wildcard'

import { Notices } from '../../../global/Notices'
import { useLegacyContext_onlyInStormRoutes } from '../../../LegacyRouteContext'
import { submitSearch } from '../../../search/helpers'
import { LazyV2SearchInput } from '../../../search/input/LazyV2SearchInput'
import { useRecentSearches } from '../../../search/input/useRecentSearches'
import { useV2QueryInput } from '../../../search/useV2QueryInput'
import { useNavbarQueryState, setSearchCaseSensitivity, setSearchPatternType, setSearchMode } from '../../../stores'

import { SimpleSearch } from './SimpleSearch'

import styles from './SearchPageInput.module.scss'

// We want to prevent autofocus by default on devices with touch as their only input method.
// Touch only devices result in the onscreen keyboard not showing until the input loses focus and
// gets focused again by the user. The logic is not fool proof, but should rule out majority of cases
// where a touch enabled device has a physical keyboard by relying on detection of a fine pointer with hover ability.
const isTouchOnlyDevice =
    !window.matchMedia('(any-pointer:fine)').matches && window.matchMedia('(any-hover:none)').matches

const queryStateSelector = (
    state: NavbarQueryState
): Pick<CaseSensitivityProps, 'caseSensitive'> & SearchPatternTypeProps & Pick<SearchModeProps, 'searchMode'> => ({
    caseSensitive: state.searchCaseSensitivity,
    patternType: state.searchPatternType,
    searchMode: state.searchMode,
})

interface SearchPageInputProps {
    queryState: QueryState
    setQueryState: (newState: QueryState) => void
    hardCodedSearchContextSpec?: string
    simpleSearch: boolean
    showKeywordSearchToggle?: boolean
}

export const SearchPageInput: FC<SearchPageInputProps> = props => {
    const { queryState, setQueryState, hardCodedSearchContextSpec, simpleSearch } = props

    const {
        authenticatedUser,
        isSourcegraphDotCom,
        telemetryService,
        platformContext,
        searchContextsEnabled,
        selectedSearchContextSpec: dynamicSearchContextSpec,
        fetchSearchContexts,
        setSelectedSearchContextSpec,
    } = useLegacyContext_onlyInStormRoutes()

    const selectedSearchContextSpec = hardCodedSearchContextSpec || dynamicSearchContextSpec

    const location = useLocation()
    const navigate = useNavigate()

    const { caseSensitive, patternType, searchMode } = useNavbarQueryState(queryStateSelector, shallow)
    const [v2QueryInput] = useV2QueryInput()

    const { recentSearches } = useRecentSearches()

    const submitSearchOnChange = useCallback(
        (parameters: Partial<SubmitSearchParameters> = {}) => {
            const query = parameters.query ?? queryState.query

            if (canSubmitSearch(query, selectedSearchContextSpec)) {
                submitSearch({
                    source: 'home',
                    query,
                    historyOrNavigate: navigate,
                    location,
                    patternType,
                    caseSensitive,
                    searchMode,
                    // In the new query input, context is either omitted (-> global)
                    // or explicitly specified.
                    selectedSearchContextSpec: v2QueryInput ? undefined : selectedSearchContextSpec,
                    ...parameters,
                })
            }
        },
        [
            queryState.query,
            selectedSearchContextSpec,
            navigate,
            location,
            patternType,
            caseSensitive,
            searchMode,
            v2QueryInput,
        ]
    )
    const submitSearchOnChangeRef = useRef(submitSearchOnChange)
    useEffect(() => {
        submitSearchOnChangeRef.current = submitSearchOnChange
    }, [submitSearchOnChange])

    const onSubmit = useCallback(
        (event?: React.FormEvent): void => {
            event?.preventDefault()
            submitSearchOnChangeRef.current()
        },
        [submitSearchOnChangeRef]
    )

    const onSimpleSearchUpdate = useCallback(
        (val: string) => {
            setQueryState({ query: val })
        },
        [setQueryState]
    )

    // TODO (#48103): Remove/simplify when new search input is released
    const input = v2QueryInput ? (
        <LazyV2SearchInput
            autoFocus={!isTouchOnlyDevice}
            telemetryService={telemetryService}
            patternType={patternType}
            interpretComments={false}
            queryState={queryState}
            onChange={setQueryState}
            onSubmit={onSubmit}
            authenticatedUser={authenticatedUser}
            isSourcegraphDotCom={isSourcegraphDotCom}
            submitSearch={submitSearchOnChange}
            selectedSearchContextSpec={selectedSearchContextSpec}
            className="flex-grow-1"
        >
            {props.showKeywordSearchToggle ? (
                <Toggles
                    patternType={patternType}
                    caseSensitive={caseSensitive}
                    setPatternType={setSearchPatternType}
                    setCaseSensitivity={setSearchCaseSensitivity}
                    searchMode={searchMode}
                    setSearchMode={setSearchMode}
                    navbarSearchQuery={queryState.query}
                    submitSearch={submitSearchOnChange}
                    structuralSearchDisabled={window.context?.experimentalFeatures?.structuralSearch !== 'enabled'}
                    telemetryService={telemetryService}
                />
            ) : (
                <LegacyToggles
                    patternType={patternType}
                    caseSensitive={caseSensitive}
                    setPatternType={setSearchPatternType}
                    setCaseSensitivity={setSearchCaseSensitivity}
                    searchMode={searchMode}
                    setSearchMode={setSearchMode}
                    navbarSearchQuery={queryState.query}
                    submitSearch={submitSearchOnChange}
                    showSmartSearchButton={false}
                    structuralSearchDisabled={window.context?.experimentalFeatures?.structuralSearch !== 'enabled'}
                />
            )}
        </LazyV2SearchInput>
    ) : (
        <SearchBox
            platformContext={platformContext}
            getUserSearchContextNamespaces={getUserSearchContextNamespaces}
            fetchSearchContexts={fetchSearchContexts}
            selectedSearchContextSpec={selectedSearchContextSpec}
            setSelectedSearchContextSpec={setSelectedSearchContextSpec}
            telemetryService={telemetryService}
            authenticatedUser={authenticatedUser}
            isSourcegraphDotCom={isSourcegraphDotCom}
            searchContextsEnabled={searchContextsEnabled}
            showSearchContext={searchContextsEnabled}
            showSearchContextManagement={true}
            caseSensitive={caseSensitive}
            patternType={patternType}
            setPatternType={setSearchPatternType}
            setCaseSensitivity={setSearchCaseSensitivity}
            searchMode={searchMode}
            setSearchMode={setSearchMode}
            queryState={queryState}
            onChange={setQueryState}
            onSubmit={onSubmit}
            autoFocus={!isTouchOnlyDevice}
            structuralSearchDisabled={window.context?.experimentalFeatures?.structuralSearch !== 'enabled'}
            showSearchHistory={true}
            recentSearches={recentSearches}
            showKeywordSearchToggle={props.showKeywordSearchToggle}
        />
    )
    return (
        <div>
            <div className="d-flex flex-row flex-shrink-past-contents">
                <Form className="flex-grow-1 flex-shrink-past-contents" onSubmit={onSubmit}>
                    <div data-search-page-input-container={true} className={styles.inputContainer}>
                        <TraceSpanProvider name="SearchBox">
                            <div className="d-flex flex-grow-1 w-100">{input}</div>
                        </TraceSpanProvider>
                    </div>
                    <Notices className="my-3 text-center" location="home" telemetryRecorder={noOpTelemetryRecorder} />
                </Form>
            </div>
            {simpleSearch && (
                <div>
                    <hr className="mt-4 mb-4" />
                    <SimpleSearch
                        telemetryService={telemetryService}
                        onSubmit={onSubmit}
                        onSimpleSearchUpdate={onSimpleSearchUpdate}
                    />
                </div>
            )}
        </div>
    )
}
