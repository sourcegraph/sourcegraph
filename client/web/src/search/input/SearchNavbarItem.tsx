import React, { useCallback, useRef, useEffect } from 'react'

import { useLocation, useNavigate } from 'react-router-dom'
import shallow from 'zustand/shallow'

import { SearchBox, LegacyToggles } from '@sourcegraph/branded'
import { Toggles } from '@sourcegraph/branded/src/search-ui/input/toggles/Toggles'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { SearchContextInputProps, SubmitSearchParameters } from '@sourcegraph/shared/src/search'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
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
        PlatformContextProps<'requestGraphQL'> {
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
    isSearchAutoFocusRequired?: boolean
    isRepositoryRelatedPage?: boolean
    showKeywordSearchToggle?: boolean
}

const selectQueryState = ({
    queryState,
    setQueryState,
    submitSearch,
    searchCaseSensitivity,
    searchPatternType,
    searchMode,
}: NavbarQueryState): Pick<
    NavbarQueryState,
    'queryState' | 'setQueryState' | 'submitSearch' | 'searchCaseSensitivity' | 'searchPatternType' | 'searchMode'
> => ({ queryState, setQueryState, submitSearch, searchCaseSensitivity, searchPatternType, searchMode })

/**
 * The search item in the navbar
 */
export const SearchNavbarItem: React.FunctionComponent<React.PropsWithChildren<Props>> = (props: Props) => {
    const navigate = useNavigate()
    const location = useLocation()

    const { queryState, setQueryState, submitSearch, searchCaseSensitivity, searchPatternType, searchMode } =
        useNavbarQueryState(selectQueryState, shallow)

    const [v2QueryInput] = useV2QueryInput()

    const { recentSearches } = useRecentSearches()

    const submitSearchOnChange = useCallback(
        (parameters: Partial<SubmitSearchParameters> = {}) => {
            submitSearch({
                historyOrNavigate: navigate,
                location,
                source: 'nav',
                selectedSearchContextSpec: props.selectedSearchContextSpec,
                ...parameters,
            })
        },
        [submitSearch, navigate, location, props.selectedSearchContextSpec]
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
                    {props.showKeywordSearchToggle ? (
                        <Toggles
                            patternType={searchPatternType}
                            caseSensitive={searchCaseSensitivity}
                            setPatternType={setSearchPatternType}
                            setCaseSensitivity={setSearchCaseSensitivity}
                            searchMode={searchMode}
                            setSearchMode={setSearchMode}
                            navbarSearchQuery={queryState.query}
                            submitSearch={submitSearchOnChange}
                            structuralSearchDisabled={
                                window.context?.experimentalFeatures?.structuralSearch !== 'enabled'
                            }
                            telemetryService={props.telemetryService}
                        />
                    ) : (
                        <LegacyToggles
                            patternType={searchPatternType}
                            caseSensitive={searchCaseSensitivity}
                            setPatternType={setSearchPatternType}
                            setCaseSensitivity={setSearchCaseSensitivity}
                            searchMode={searchMode}
                            setSearchMode={setSearchMode}
                            navbarSearchQuery={queryState.query}
                            submitSearch={submitSearchOnChange}
                            structuralSearchDisabled={
                                window.context?.experimentalFeatures?.structuralSearch !== 'enabled'
                            }
                        />
                    )}
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
                showKeywordSearchToggle={props.showKeywordSearchToggle}
            />
        </Form>
    )
}
