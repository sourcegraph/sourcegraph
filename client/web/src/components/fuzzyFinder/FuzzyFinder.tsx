import React, { useEffect, type Dispatch, type SetStateAction, useCallback, useRef } from 'react'

import type * as H from 'history'

import { Shortcut } from '@sourcegraph/shared/src/react-shortcuts'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'

import { FuzzyModal } from './FuzzyModal'
import { useFuzzyShortcuts } from './FuzzyShortcuts'
import { fuzzyIsActive, type FuzzyTabsProps, type FuzzyState, useFuzzyState, type FuzzyTabKey } from './FuzzyTabs'

const DEFAULT_MAX_RESULTS = 50

export interface FuzzyFinderContainerProps
    extends TelemetryProps,
        TelemetryV2Props,
        Pick<FuzzyFinderProps, 'location'>,
        SettingsCascadeProps,
        FuzzyTabsProps {
    isVisible: boolean
    setIsVisible: React.Dispatch<SetStateAction<boolean>>
}

/**
 * This components registers a global keyboard shortcut to render the fuzzy
 * finder and renders the fuzzy finder.
 */
export const FuzzyFinderContainer: React.FunctionComponent<FuzzyFinderContainerProps> = props => {
    const { isVisible, setIsVisible } = props
    const isVisibleRef = useRef(isVisible)
    isVisibleRef.current = isVisible
    const state = useFuzzyState(props)
    const { tabs, setQuery, activeTab, setActiveTab, repoRevision, scope, isScopeToggleDisabled, toggleScope } = state
    const isScopeToggleDisabledRef = useRef(isScopeToggleDisabled)
    isScopeToggleDisabledRef.current = isScopeToggleDisabled

    // We need useRef to access the latest state inside `openFuzzyFinder` below.
    // The keyboard shortcut does not pick up changes to the callback even if we
    // declare them as dependencies of `openFuzzyFinder`.
    const tabsRef = useRef(tabs)
    tabsRef.current = tabs
    const repositoryName = useRef('')
    repositoryName.current = repoRevision.repositoryName
    const activeTabRef = useRef(activeTab)
    activeTabRef.current = activeTab

    const openFuzzyFinder = useCallback(
        (tab: FuzzyTabKey): void => {
            if (tabsRef.current.isOnlyFilesEnabled() && !repositoryName.current) {
                return // Legacy mode: only activate inside a repository
            }
            const activeTab = activeTabRef.current
            const isVisible = isVisibleRef.current
            if (!isVisible) {
                if (activeTabRef.current !== tab) {
                    // Reset the query when the user activates a different tab.
                    // For example, if the user had "Repos" open, opens a repo,
                    // and then triggers Cmd+P to activate the "Files" tab then
                    // we discard the previous query from the "Repos" tab.
                    setQuery('')
                }
                setIsVisible(true)
            }
            if (!isScopeToggleDisabledRef.current && isVisible && tab === activeTab) {
                switch (tab) {
                    case 'files':
                    case 'symbols':
                    case 'all': {
                        toggleScope()
                    }
                }
            } else {
                const newTab = tabsRef.current.focusNamedTab(tab)
                if (newTab) {
                    setActiveTab(newTab)
                }
            }
        },
        [setActiveTab, setIsVisible, toggleScope, setQuery]
    )

    const shortcuts = useFuzzyShortcuts()

    useEffect(() => {
        if (isVisible) {
            props.telemetryService.log('FuzzyFinderViewed', { action: 'shortcut open' })
            props.telemetryRecorder.recordEvent('fuzzyFinder', 'view')
        }
    }, [props.telemetryService, props.telemetryRecorder, isVisible])

    const handleResultClick = useCallback(() => {
        props.telemetryService.log('FuzzyFinderResultClicked', { activeTab, scope }, { activeTab, scope })
        props.telemetryRecorder.recordEvent('fuzzyFinder.result', 'click')
        setIsVisible(false)
    }, [props.telemetryService, props.telemetryRecorder, setIsVisible, activeTab, scope])

    const handleGoToResultsPageClick = useCallback(() => {
        props.telemetryService.log('FuzzyFinderGoToResultsPageClicked', { activeTab, scope }, { activeTab, scope })
        props.telemetryRecorder.recordEvent('fuzzyFinder.goToResultsPage', 'click')
        setIsVisible(false)
    }, [props.telemetryService, props.telemetryRecorder, setIsVisible, activeTab, scope])

    if (tabs.isAllDisabled()) {
        return null
    }

    // Disable the fuzzy finder if only the 'files' tab is enabled and we're not
    // in a repository-related page.
    if (tabs.isOnlyFilesEnabled() && !fuzzyIsActive(activeTab, 'files')) {
        return null
    }

    return (
        <>
            {shortcuts
                .filter(shortcut => shortcut.isEnabled)
                .flatMap(shortcut =>
                    shortcut.shortcut?.keybindings.map(keybinding => (
                        <Shortcut
                            {...keybinding}
                            key={`fuzzy-shortcut-${shortcut.name}-${JSON.stringify(keybinding)}`}
                            onMatch={() => openFuzzyFinder(shortcut.name)}
                            ignoreInput={true}
                        />
                    ))
                )}
            {isVisible && (
                <FuzzyFinder
                    {...state}
                    setIsVisible={setIsVisible}
                    location={props.location}
                    onClickResult={handleResultClick}
                    onClickGoToResultsPage={handleGoToResultsPageClick}
                />
            )}
        </>
    )
}

interface FuzzyFinderProps extends FuzzyState {
    setIsVisible: Dispatch<SetStateAction<boolean>>

    /**
     * Search result click handler.
     */
    onClickResult: () => void

    /**
     * Search results page click handler.
     */
    onClickGoToResultsPage: () => void

    location: H.Location

    /**
     * The maximum number of files a repo can have to use case-insensitive fuzzy finding.
     *
     * Case-insensitive fuzzy finding is more expensive to compute compared to
     * word-sensitive fuzzy finding.  The fuzzy modal will use case-insensitive
     * fuzzy finding when the repo has fewer files than this number, and
     * word-sensitive fuzzy finding otherwise.
     */
    caseInsensitiveFileCountThreshold?: number
}

const FuzzyFinder: React.FunctionComponent<React.PropsWithChildren<FuzzyFinderProps>> = props => {
    const { setIsVisible } = props
    const onClose = useCallback(() => setIsVisible(false), [setIsVisible])

    return <FuzzyModal {...props} initialMaxResults={DEFAULT_MAX_RESULTS} initialQuery="" onClose={onClose} />
}
