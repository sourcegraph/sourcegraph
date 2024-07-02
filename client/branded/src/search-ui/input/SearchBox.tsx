import { type FC, useCallback, useMemo, useRef } from 'react'

import classNames from 'classnames'

import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import {
    type SearchContextInputProps,
    type QueryState,
    type SubmitSearchProps,
    EditorHint,
} from '@sourcegraph/shared/src/search'
import { getGlobalSearchContextFilter } from '@sourcegraph/shared/src/search/query/query'
import { omitFilter } from '@sourcegraph/shared/src/search/query/transformer'
import type { fetchStreamSuggestions as defaultFetchStreamSuggestions } from '@sourcegraph/shared/src/search/suggestions'
import type { RecentSearch } from '@sourcegraph/shared/src/settings/temporary/recentSearches'
import { useTemporarySetting } from '@sourcegraph/shared/src/settings/temporary/useTemporarySetting'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'

import { type IEditor, LazyQueryInput, type LazyQueryInputProps } from './LazyQueryInput'
import { SearchButton } from './SearchButton'
import { SearchContextDropdown } from './SearchContextDropdown'
import { SearchHelpDropdownButton } from './SearchHelpDropdownButton'
import { SearchHistoryDropdown } from './SearchHistoryDropdown'
import { Toggles, type TogglesProps } from './toggles/Toggles'

import styles from './SearchBox.module.scss'

export interface SearchBoxProps
    extends Omit<TogglesProps, 'navbarSearchQuery' | 'submitSearch'>,
        SearchContextInputProps,
        TelemetryProps,
        TelemetryV2Props,
        PlatformContextProps<'requestGraphQL'>,
        Pick<LazyQueryInputProps, 'autoFocus' | 'onFocus' | 'onSubmit' | 'interpretComments' | 'onChange'> {
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean // significant for query suggestions
    showSearchContext: boolean
    showSearchContextManagement: boolean
    queryState: QueryState
    submitSearchOnSearchContextChange?: SubmitSearchProps['submitSearch']
    submitSearchOnToggle?: SubmitSearchProps['submitSearch']
    fetchStreamSuggestions?: typeof defaultFetchStreamSuggestions // Alternate implementation is used in the VS Code extension.
    className?: string
    containerClassName?: string

    /** Don't show search help button */
    hideHelpButton?: boolean

    /** Called with the underlying editor instance on creation. */
    onEditorCreated?: (editor: IEditor) => void

    /**
     * Whether or not to show the search history button. Also disables the
     * search button. Does not affect history in the search input itself (via
     * arrow up/down)
     */
    showSearchHistory?: boolean

    recentSearches?: RecentSearch[]
}

export const SearchBox: FC<SearchBoxProps> = props => {
    const {
        queryState,
        onEditorCreated: onEditorCreatedCallback,
        showSearchHistory,
        hideHelpButton,
        onChange,
        selectedSearchContextSpec,
        recentSearches,
    } = props

    const [usedInlineHistory, setUsedInlineHistory] = useTemporarySetting('search.input.usedInlineHistory', false)
    const usedInlineHistoryRef = useRef(usedInlineHistory)

    const onInlineSearchHistorySelect = useCallback(() => {
        if (usedInlineHistoryRef.current !== true) {
            setUsedInlineHistory(true)
        }
    }, [setUsedInlineHistory, usedInlineHistoryRef])

    const onEditorCreated = useCallback(
        (editor: IEditor) => {
            onEditorCreatedCallback?.(editor)
        },
        [onEditorCreatedCallback]
    )

    const onSearchHistorySelect = useCallback(
        (search: RecentSearch) => {
            const searchContext = getGlobalSearchContextFilter(search.query)
            const query =
                searchContext && searchContext.spec === selectedSearchContextSpec
                    ? omitFilter(search.query, searchContext?.filter)
                    : search.query
            onChange({ query, hint: EditorHint.Focus })
        },
        [onChange, selectedSearchContextSpec]
    )

    // Simplify history entries by removing the context filter if it is the same
    // as the currently selected search context.
    const recentSearchesWithoutSearchContext = useMemo(() => {
        if (!recentSearches || !selectedSearchContextSpec) {
            return undefined
        }

        return recentSearches.map(search => {
            const searchContext = getGlobalSearchContextFilter(search.query)
            if (searchContext && searchContext.spec === selectedSearchContextSpec) {
                return {
                    ...search,
                    query: omitFilter(search.query, searchContext?.filter),
                }
            }
            return search
        })
    }, [recentSearches, selectedSearchContextSpec])

    return (
        <div
            className={classNames(
                styles.searchBox,
                props.containerClassName,
                props.hideHelpButton ? styles.searchBoxShadow : null
            )}
            data-testid="searchbox"
        >
            <div
                className={classNames(
                    styles.searchBoxBackgroundContainer,
                    props.className,
                    'flex-shrink-past-contents'
                )}
            >
                {showSearchHistory && (
                    <>
                        <SearchHistoryDropdown
                            className={styles.searchBoxAlignMiddle}
                            recentSearches={props.recentSearches ?? []}
                            onSelect={onSearchHistorySelect}
                            telemetryService={props.telemetryService}
                            telemetryRecorder={props.telemetryRecorder}
                        />
                        <div className={styles.searchBoxSeparator} />
                    </>
                )}
                {props.searchContextsEnabled && props.showSearchContext && (
                    <>
                        <SearchContextDropdown
                            {...props}
                            query={queryState.query}
                            submitSearch={props.submitSearchOnSearchContextChange}
                            className={styles.searchBoxAlignMiddle}
                        />
                        <div className={styles.searchBoxSeparator} />
                    </>
                )}
                {/*
                    To fix Rule: "region" (All page content should be contained by landmarks)
                    Added role attribute to the following element to satisfy the rule.
                */}
                <div className={classNames(styles.searchBoxFocusContainer, 'flex-shrink-past-contents')} role="search">
                    <LazyQueryInput
                        className={styles.searchBoxInput}
                        onEditorCreated={onEditorCreated}
                        placeholder={
                            recentSearchesWithoutSearchContext &&
                            recentSearchesWithoutSearchContext.length > 0 &&
                            usedInlineHistory === false
                                ? 'Tip: Use ↑ for previous searches'
                                : 'Search for code or files'
                        }
                        preventNewLine={true}
                        autoFocus={props.autoFocus}
                        caseSensitive={props.caseSensitive}
                        fetchStreamSuggestions={props.fetchStreamSuggestions}
                        interpretComments={props.interpretComments}
                        isSourcegraphDotCom={props.isSourcegraphDotCom}
                        onChange={props.onChange}
                        onFocus={props.onFocus}
                        onSubmit={props.onSubmit}
                        patternType={props.patternType}
                        queryState={queryState}
                        selectedSearchContextSpec={props.selectedSearchContextSpec}
                        searchHistory={recentSearchesWithoutSearchContext}
                        onSelectSearchFromHistory={onInlineSearchHistorySelect}
                        enableJumpToSuggestion={true}
                    />
                    <Toggles
                        patternType={props.patternType}
                        defaultPatternType={props.defaultPatternType}
                        setPatternType={props.setPatternType}
                        caseSensitive={props.caseSensitive}
                        setCaseSensitivity={props.setCaseSensitivity}
                        searchMode={props.searchMode}
                        setSearchMode={props.setSearchMode}
                        submitSearch={props.submitSearchOnToggle}
                        navbarSearchQuery={queryState.query}
                        className={styles.searchBoxToggles}
                        structuralSearchDisabled={props.structuralSearchDisabled}
                        telemetryService={props.telemetryService}
                        telemetryRecorder={props.telemetryRecorder}
                    />
                </div>
            </div>
            <div className={styles.searchBoxButton}>
                <SearchButton />
                {!hideHelpButton && (
                    <SearchHelpDropdownButton
                        isSourcegraphDotCom={props.isSourcegraphDotCom}
                        telemetryService={props.telemetryService}
                        telemetryRecorder={props.telemetryRecorder}
                    />
                )}
            </div>
        </div>
    )
}
