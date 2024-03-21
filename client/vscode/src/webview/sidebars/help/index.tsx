import '../../platform/polyfills'

import React, { useMemo } from 'react'

import { VSCodeProgressRing } from '@vscode/webview-ui-toolkit/react'
import * as Comlink from 'comlink'
import { createRoot } from 'react-dom/client'

import { wrapRemoteObservable } from '@sourcegraph/shared/src/api/client/api/common'
import { AnchorLink, setLinkComponent, useObservable } from '@sourcegraph/wildcard'

import type { ExtensionCoreAPI, HelpSidebarAPI } from '../../../contract'
import type { VsCodeApi } from '../../../vsCodeApi'
import { createEndpointsForWebToNode } from '../../comlink/webviewEndpoint'
import { createPlatformContext } from '../../platform/context'

import { HelpSidebarView } from './HelpSidebarView'

declare const acquireVsCodeApi: () => VsCodeApi

const vsCodeApi = acquireVsCodeApi()

const { proxy, expose } = createEndpointsForWebToNode(vsCodeApi)

export const extensionCoreAPI: Comlink.Remote<ExtensionCoreAPI> = Comlink.wrap(proxy)

const helpSidebarAPI: HelpSidebarAPI = {}

const platformContext = createPlatformContext(extensionCoreAPI)

Comlink.expose(helpSidebarAPI, expose)

setLinkComponent(AnchorLink)

const Main: React.FC<React.PropsWithChildren<unknown>> = () => {
    const authenticatedUser = useObservable(
        useMemo(() => wrapRemoteObservable(extensionCoreAPI.getAuthenticatedUser()), [])
    )

    const state = useObservable(useMemo(() => wrapRemoteObservable(extensionCoreAPI.observeState()), []))

    const instanceURL = useObservable(useMemo(() => wrapRemoteObservable(extensionCoreAPI.getInstanceURL()), []))
    if (authenticatedUser === undefined || instanceURL === undefined || state === undefined) {
        return <VSCodeProgressRing />
    }

    return (
        <HelpSidebarView
            platformContext={platformContext}
            extensionCoreAPI={extensionCoreAPI}
            authenticatedUser={authenticatedUser}
            instanceURL={instanceURL}
        />
    )
}

const root = createRoot(document.querySelector('#root')!)

root.render(<Main />)
