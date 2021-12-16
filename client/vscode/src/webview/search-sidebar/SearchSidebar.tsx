import React, { useEffect, useMemo } from 'react'
import create, { UseStore } from 'zustand'

import { SearchSidebar as BrandedSearchSidebar } from '@sourcegraph/branded/src/search/results/sidebar/SearchSidebar'
import { wrapRemoteObservable } from '@sourcegraph/shared/src/api/client/api/common'
import { SearchQueryState, updateQuery } from '@sourcegraph/shared/src/search/searchQueryState'
import { useObservable } from '@sourcegraph/shared/src/util/useObservable'

import { WebviewPageProps } from '../platform/context'

import { OpenSearchPanelCta } from './OpenSearchPanelCta'
import styles from './SearchSidebar.module.scss'
import { SidebarAuthCheck } from './SidebarAuthCheck'

interface SearchSidebarProps extends Pick<WebviewPageProps, 'platformContext' | 'sourcegraphVSCodeExtensionAPI'> {}

export const SearchSidebar: React.FC<SearchSidebarProps> = ({ sourcegraphVSCodeExtensionAPI }) => {
    const useQueryState: UseStore<SearchQueryState> = useMemo(() => {
        const useStore = create<SearchQueryState>((set, get) => ({
            queryState: { query: '' },
            setQueryState: queryStateUpdate => {
                const queryState =
                    typeof queryStateUpdate === 'function' ? queryStateUpdate(get().queryState) : queryStateUpdate
                set({ queryState })
                // TODO error handling

                sourcegraphVSCodeExtensionAPI.setActiveWebviewQueryState(queryState).then(
                    () => {},
                    () => {}
                )
            },
            submitSearch: (_parameters, updates = []) => {
                const updatedQuery = updateQuery(get().queryState.query, updates)

                // TODO error handling
                sourcegraphVSCodeExtensionAPI
                    .submitActiveWebviewSearch({
                        query: updatedQuery,
                    })
                    .then(
                        () => {},
                        () => {}
                    )
            },
        }))
        return useStore
    }, [sourcegraphVSCodeExtensionAPI])

    const activeQueryState = useObservable(
        useMemo(() => wrapRemoteObservable(sourcegraphVSCodeExtensionAPI.observeActiveWebviewQueryState()), [
            sourcegraphVSCodeExtensionAPI,
        ])
    )

    const dynamicFilters =
        useObservable(
            useMemo(() => wrapRemoteObservable(sourcegraphVSCodeExtensionAPI.observeActiveWebviewDynamicFilters()), [
                sourcegraphVSCodeExtensionAPI,
            ])
        ) ?? undefined

    const settingsCascade = useObservable(
        useMemo(() => wrapRemoteObservable(sourcegraphVSCodeExtensionAPI.getSettings()), [
            sourcegraphVSCodeExtensionAPI,
        ])
    ) ?? { final: {}, subjects: [] }

    useEffect(() => {
        // On changes that originate from user input in the search webview panel itself,
        // we don't want to trigger another query state update, which would lead to an infinite loop.
        // That's why we set the state directly, instead of using the `setQueryState` method which
        // updates query state in the search webview panel.
        if (activeQueryState) {
            // useQueryState.getState().setQueryState(activeQueryState)
            useQueryState.setState({ queryState: activeQueryState.queryState })
        }
    }, [activeQueryState, useQueryState])

    if (!activeQueryState) {
        // There's no ACTIVE search panel

        // We need to add API to query all open search panels

        // If no open, show button + CTA to open search panel (links to sign up etc.)

        return (
            <>
                <OpenSearchPanelCta
                    className={styles.sidebarContainer}
                    sourcegraphVSCodeExtensionAPI={sourcegraphVSCodeExtensionAPI}
                />
                <SidebarAuthCheck
                    className={styles.sidebarContainer}
                    sourcegraphVSCodeExtensionAPI={sourcegraphVSCodeExtensionAPI}
                />
            </>
        )
    }

    // For v1: Add recent/saved searches/files panel(s)

    const { caseSensitive, patternType } = activeQueryState

    return (
        <BrandedSearchSidebar
            forceButton={true}
            className={styles.sidebarContainer}
            filters={dynamicFilters}
            useQueryState={useQueryState}
            patternType={patternType}
            caseSensitive={caseSensitive}
            settingsCascade={settingsCascade}
            telemetryService={{
                log: () => {},
                logViewEvent: () => {},
            }}
        />
    )
}
