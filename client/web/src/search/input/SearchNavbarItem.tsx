import React, { useCallback, useRef, useEffect } from 'react'

import { useLocation, useNavigate } from 'react-router-dom'
import shallow from 'zustand/shallow'

import { SearchBox } from '@sourcegraph/branded'
import { Toggles } from '@sourcegraph/branded/src/search-ui/input/toggles/Toggles'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { SearchContextInputProps, SubmitSearchParameters } from '@sourcegraph/shared/src/search'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Form } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import { useNavbarQueryState, setSearchCaseSensitivity } from '../../stores'
import { type NavbarQueryState, setSearchMode, setSearchPatternType } from '../../stores/navbarSearchQueryState'
import { useV2QueryInput } from '../useV2QueryInput'

import { LazyV2SearchInput } from './LazyV2SearchInput'
import { useRecentSearches } from './useRecentSearches'

interface Props
    extends SettingsCascadeProps,
        SearchContextInputProps,
        TelemetryProps,
        TelemetryV2Props,
        PlatformContextProps<'requestGraphQL'> {
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
    isSearchAutoFocusRequired?: boolean
    isRepositoryRelatedPage?: boolean
}

const selectQueryState = ({
    queryState,
    setQueryState,
    submitSearch,
    searchCaseSensitivity,
    searchPatternType,
    defaultPatternType,
    searchMode,
}: NavbarQueryState): Pick<
    NavbarQueryState,
    | 'queryState'
    | 'setQueryState'
    | 'submitSearch'
    | 'searchCaseSensitivity'
    | 'searchPatternType'
    | 'defaultPatternType'
    | 'searchMode'
> => ({
    queryState,
    setQueryState,
    submitSearch,
    searchCaseSensitivity,
    searchPatternType,
    defaultPatternType,
    searchMode,
})

/**
 * The search item in the navbar
 */
export const SearchNavbarItem: React.FunctionComponent<React.PropsWithChildren<Props>> = (props: Props) => {
    const navigate = useNavigate()
    const location = useLocation()

    const {
        queryState,
        setQueryState,
        submitSearch,
        searchCaseSensitivity,
        searchPatternType,
        defaultPatternType,
        searchMode,
    } = useNavbarQueryState(selectQueryState, shallow)

    const [v2QueryInput] = useV2QueryInput()

    const { recentSearches } = useRecentSearches()

    const submitSearchOnChange = useCallback(
        (parameters: Partial<SubmitSearchParameters> = {}) => {
            submitSearch({
                historyOrNavigate: navigate,
                location,
                source: 'nav',
                selectedSearchContextSpec: props.selectedSearchContextSpec,
                telemetryRecorder: props.telemetryRecorder,
                ...parameters,
            })
        },
        [submitSearch, navigate, location, props.selectedSearchContextSpec, props.telemetryRecorder]
    )
    const submitSearchOnChangeRef = useRef(submitSearchOnChange)
    useEffect(() => {
        submitSearchOnChangeRef.current = submitSearchOnChange
    }, [submitSearchOnChange])

    const onSubmit = useCallback((event?: React.FormEvent): void => {
        event?.preventDefault()
        submitSearchOnChangeRef.current()
    }, [])

    // TODO (#48103): Remove/simplify when new search input is released
    if (v2QueryInput) {
        return (
            <Form
                className="search--navbar-item d-flex align-items-flex-start flex-grow-1 flex-shrink-past-contents"
                onSubmit={onSubmit}
            >
                <LazyV2SearchInput
                    visualMode="compact"
                    telemetryService={props.telemetryService}
                    telemetryRecorder={props.telemetryRecorder}
                    patternType={searchPatternType}
                    interpretComments={false}
                    queryState={queryState}
                    onChange={setQueryState}
                    onSubmit={onSubmit}
                    authenticatedUser={props.authenticatedUser}
                    isSourcegraphDotCom={props.isSourcegraphDotCom}
                    submitSearch={submitSearchOnChange}
                    selectedSearchContextSpec={props.selectedSearchContextSpec}
                    className="flex-grow-1"
                >
                    <Toggles
                        patternType={searchPatternType}
                        defaultPatternType={defaultPatternType}
                        caseSensitive={searchCaseSensitivity}
                        setPatternType={setSearchPatternType}
                        setCaseSensitivity={setSearchCaseSensitivity}
                        searchMode={searchMode}
                        setSearchMode={setSearchMode}
                        navbarSearchQuery={queryState.query}
                        submitSearch={submitSearchOnChange}
                        structuralSearchDisabled={window.context?.experimentalFeatures?.structuralSearch !== 'enabled'}
                        telemetryService={props.telemetryService}
                        telemetryRecorder={props.telemetryRecorder}
                    />
                </LazyV2SearchInput>
            </Form>
        )
    }

    return (
        <Form
            className="search--navbar-item d-flex align-items-flex-start flex-grow-1 flex-shrink-past-contents"
            onSubmit={onSubmit}
        >
            <SearchBox
                {...props}
                autoFocus={false}
                showSearchContext={props.searchContextsEnabled}
                showSearchContextManagement={true}
                caseSensitive={searchCaseSensitivity}
                setCaseSensitivity={setSearchCaseSensitivity}
                patternType={searchPatternType}
                defaultPatternType={defaultPatternType}
                setPatternType={setSearchPatternType}
                searchMode={searchMode}
                setSearchMode={setSearchMode}
                queryState={queryState}
                onChange={setQueryState}
                onSubmit={onSubmit}
                submitSearchOnToggle={submitSearchOnChange}
                submitSearchOnSearchContextChange={submitSearchOnChange}
                structuralSearchDisabled={window.context?.experimentalFeatures?.structuralSearch !== 'enabled'}
                hideHelpButton={false}
                showSearchHistory={true}
                recentSearches={recentSearches}
            />
        </Form>
    )
}
