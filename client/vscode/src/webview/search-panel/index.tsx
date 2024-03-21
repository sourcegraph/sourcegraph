import '../platform/polyfills'

import React, { useMemo } from 'react'

import { VSCodeProgressRing } from '@vscode/webview-ui-toolkit/react'
import * as Comlink from 'comlink'
import { createRoot } from 'react-dom/client'
import { MemoryRouter } from 'react-router-dom'

import { wrapRemoteObservable } from '@sourcegraph/shared/src/api/client/api/common'
import { ShortcutProvider } from '@sourcegraph/shared/src/react-shortcuts'
import { ThemeSetting, ThemeContext } from '@sourcegraph/shared/src/theme'
import { AnchorLink, setLinkComponent, useObservable, WildcardThemeContext } from '@sourcegraph/wildcard'

import type { ExtensionCoreAPI } from '../../contract'
import type { VsCodeApi } from '../../vsCodeApi'
import { createEndpointsForWebToNode } from '../comlink/webviewEndpoint'
import { createPlatformContext, WebviewPageContext, type WebviewPageProps } from '../platform/context'
import { adaptSourcegraphThemeToEditorTheme } from '../theming/sourcegraphTheme'

import { searchPanelAPI } from './api'
import { SearchHomeView } from './SearchHomeView'
import { SearchResultsView } from './SearchResultsView'

import './index.module.scss'

declare const acquireVsCodeApi: () => VsCodeApi

const vsCodeApi = acquireVsCodeApi()

const { proxy, expose } = createEndpointsForWebToNode(vsCodeApi)

Comlink.expose(searchPanelAPI, expose)

export const extensionCoreAPI: Comlink.Remote<ExtensionCoreAPI> = Comlink.wrap(proxy)

const themes = adaptSourcegraphThemeToEditorTheme()

extensionCoreAPI.panelInitialized(document.documentElement.dataset.panelId!).catch(() => {
    // noop (TODO?)
})

const platformContext = createPlatformContext(extensionCoreAPI)

setLinkComponent(AnchorLink)

const Main: React.FC<React.PropsWithChildren<unknown>> = () => {
    const theme = useObservable(themes)
    const state = useObservable(useMemo(() => wrapRemoteObservable(extensionCoreAPI.observeState()), []))
    const instanceURL = useObservable(useMemo(() => wrapRemoteObservable(extensionCoreAPI.getInstanceURL()), []))
    const authenticatedUser = useObservable(
        useMemo(() => wrapRemoteObservable(extensionCoreAPI.getAuthenticatedUser()), [])
    )
    const settingsCascade = useObservable(
        useMemo(() => wrapRemoteObservable(extensionCoreAPI.observeSourcegraphSettings()), [])
    )
    // Do not block rendering on settings unless we observe UI jitter

    // TODO: If init is taking too long, show a message.
    // Also check if anything has errored out.

    // If any of the remote values have yet to load.
    const initialized =
        state !== undefined &&
        authenticatedUser !== undefined &&
        instanceURL !== undefined &&
        theme !== undefined &&
        settingsCascade !== undefined

    const themeSetting = useMemo(
        () => ({ themeSetting: theme === 'theme-light' ? ThemeSetting.Light : ThemeSetting.Dark }),
        [theme]
    )

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

    if (state?.status === 'context-invalidated') {
        // TODO context-invalidated state
        return null
    }

    // Render SearchHomeView until the user submits a search.
    if (state.context.submittedSearchQueryState === null) {
        return (
            <WebviewPageContext.Provider value={webviewPageProps}>
                <ThemeContext.Provider value={themeSetting}>
                    <SearchHomeView {...webviewPageProps} context={state.context} />
                </ThemeContext.Provider>
            </WebviewPageContext.Provider>
        )
    }

    return (
        <WebviewPageContext.Provider value={webviewPageProps}>
            <ThemeContext.Provider value={themeSetting}>
                <SearchResultsView
                    {...webviewPageProps}
                    context={{
                        ...state.context,
                        submittedSearchQueryState: state.context.submittedSearchQueryState,
                    }}
                />
            </ThemeContext.Provider>
        </WebviewPageContext.Provider>
    )
}

const root = createRoot(document.querySelector('#root')!)

root.render(
    <ShortcutProvider>
        <WildcardThemeContext.Provider value={{ isBranded: true }}>
            {/* Required for shared components that depend on `location`. */}
            <MemoryRouter>
                <Main />
            </MemoryRouter>
        </WildcardThemeContext.Provider>
    </ShortcutProvider>
)
