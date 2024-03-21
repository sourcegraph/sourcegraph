import '../../platform/polyfills'

import React, { useMemo, useState } from 'react'

import { VSCodeProgressRing } from '@vscode/webview-ui-toolkit/react'
import * as Comlink from 'comlink'
import { createRoot } from 'react-dom/client'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { useDeepCompareEffectNoCheck } from 'use-deep-compare-effect'

import { wrapRemoteObservable } from '@sourcegraph/shared/src/api/client/api/common'
import { ShortcutProvider } from '@sourcegraph/shared/src/react-shortcuts'
import type { Filter } from '@sourcegraph/shared/src/search/stream'
import { AnchorLink, setLinkComponent, useObservable, WildcardThemeContext } from '@sourcegraph/wildcard'

import type { ExtensionCoreAPI } from '../../../contract'
import type { VsCodeApi } from '../../../vsCodeApi'
import { createEndpointsForWebToNode } from '../../comlink/webviewEndpoint'
import { createPlatformContext, type WebviewPageProps } from '../../platform/context'
import { adaptSourcegraphThemeToEditorTheme } from '../../theming/sourcegraphTheme'
import { AuthSidebarCta, AuthSidebarView } from '../auth/AuthSidebarView'
import { HistoryHomeSidebar } from '../history/HistorySidebarView'

import { createSearchSidebarAPI } from './api'
import { ContextInvalidatedSidebarView } from './ContextInvalidatedSidebarView'
import { SearchSidebarView } from './SearchSidebarView'

declare const acquireVsCodeApi: () => VsCodeApi

const vsCodeApi = acquireVsCodeApi()

const { proxy, expose } = createEndpointsForWebToNode(vsCodeApi)

export const extensionCoreAPI: Comlink.Remote<ExtensionCoreAPI> = Comlink.wrap(proxy)

const platformContext = createPlatformContext(extensionCoreAPI)

const searchSidebarAPI = createSearchSidebarAPI({
    platformContext,
    instanceURL: document.documentElement.dataset.instanceUrl!,
})
Comlink.expose(searchSidebarAPI, expose)

setLinkComponent(AnchorLink)

const themes = adaptSourcegraphThemeToEditorTheme()

const Main: React.FC<React.PropsWithChildren<unknown>> = () => {
    // Debt: make sure we only rerender on necessary changes
    const state = useObservable(useMemo(() => wrapRemoteObservable(extensionCoreAPI.observeState()), []))

    const [filters, setFilters] = useState<Filter[] | undefined>(undefined)
    useDeepCompareEffectNoCheck(() => {
        setFilters(state?.context.searchResults?.filters)
    }, [state?.context.searchResults?.filters])

    const authenticatedUser = useObservable(
        useMemo(() => wrapRemoteObservable(extensionCoreAPI.getAuthenticatedUser()), [])
    )

    const instanceURL = useObservable(useMemo(() => wrapRemoteObservable(extensionCoreAPI.getInstanceURL()), []))

    const theme = useObservable(themes)

    const settingsCascade = useObservable(
        useMemo(() => wrapRemoteObservable(extensionCoreAPI.observeSourcegraphSettings()), [])
    )
    // Do not block rendering on settings unless we observe UI jitter

    // Debt: If init is taking too long, show a message.
    // Also check if anything has errored out.

    // If any of the remote values have yet to load.
    const initialized =
        state !== undefined &&
        authenticatedUser !== undefined &&
        instanceURL !== undefined &&
        theme !== undefined &&
        settingsCascade !== undefined
    if (!initialized) {
        return <VSCodeProgressRing />
    }

    const webviewPageProps: WebviewPageProps = {
        extensionCoreAPI,
        platformContext,
        authenticatedUser,
        settingsCascade,
        instanceURL,
    }

    if (state.status === 'context-invalidated') {
        return <ContextInvalidatedSidebarView {...webviewPageProps} />
    }

    // If a search hasn't been performed yet
    if (state.status === 'search-home' || !state.context.submittedSearchQueryState) {
        // TODO: should we hide the access token form permanently if an unauthenticated user
        // has performed a search before? Or just for this session?
        if (!authenticatedUser) {
            return (
                <>
                    <AuthSidebarCta platformContext={platformContext} />
                    <AuthSidebarView {...webviewPageProps} />
                </>
            )
        }
        return <HistoryHomeSidebar {...webviewPageProps} authenticatedUser={authenticatedUser} />
    }

    // <SearchSidebarView> is wrapped w/ React.memo so pass only necessary props.
    return (
        <>
            <SearchSidebarView
                platformContext={platformContext}
                extensionCoreAPI={extensionCoreAPI}
                settingsCascade={settingsCascade}
                filters={filters}
            />
        </>
    )
}

const root = createRoot(document.querySelector('#root')!)

const routes = [
    {
        path: '/*',
        element: <Main />,
    },
]
const router = createMemoryRouter(routes, {
    initialEntries: ['/'],
})

root.render(
    <ShortcutProvider>
        <WildcardThemeContext.Provider value={{ isBranded: true }}>
            <RouterProvider router={router} />
        </WildcardThemeContext.Provider>
    </ShortcutProvider>
)
