import type { GraphQLResult } from '@sourcegraph/http-client'
import type { FlatExtensionHostAPI } from '@sourcegraph/shared/src/api/contract'
import type { ProxySubscribable } from '@sourcegraph/shared/src/api/extension/api/common'
import type { ViewerData, ViewerId } from '@sourcegraph/shared/src/api/viewerTypes'
import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import type { EventSource } from '@sourcegraph/shared/src/graphql-operations'
import type { SearchMatch, StreamSearchOptions } from '@sourcegraph/shared/src/search/stream'
import type { SettingsCascadeOrError } from '@sourcegraph/shared/src/settings/settings'

import type { Event } from './graphql-operations'
import type { VSCEQueryState, VSCEState, VSCEStateMachine } from './state'

export interface ExtensionCoreAPI {
    /** For search panel webview to signal that it is ready for messages. */
    panelInitialized: (panelId: string) => void

    requestGraphQL: (
        request: string,
        variables: any,
        overrideAccessToken?: string,
        overrideSourcegraphURL?: string
    ) => Promise<GraphQLResult<any>>
    observeSourcegraphSettings: () => ProxySubscribable<SettingsCascadeOrError>
    getAuthenticatedUser: () => ProxySubscribable<AuthenticatedUser | null>
    /** Endpoint settings */
    getInstanceURL: () => ProxySubscribable<string>
    getAccessToken: Promise<string | undefined>
    removeAccessToken: () => Promise<void>
    setEndpointUri: (accessToken: string, uri: string) => Promise<void>
    /**
     * Observe search box query state.
     * Used to send current query from panel to sidebar.
     *
     * v1 Debt: Transient query state isn't stored in state machine for performance
     * as it would lead to re-rendering the whole search panel on each keystroke.
     * Implement selector system w/ key path for state machine. Alternatively,
     * aggressively memoize top-level "View" components (i.e. don't just take whole state as prop).
     */
    observePanelQueryState: () => ProxySubscribable<VSCEQueryState>
    /** State Management*/
    observeState: () => ProxySubscribable<VSCEState>
    emit: VSCEStateMachine['emit']
    /** Opens a remote file given a serialized SourcegraphUri */
    openSourcegraphFile: (uri: string) => Promise<void>
    openLink: (uri: string) => Promise<void>
    copyLink: (uri: string) => Promise<void>
    reloadWindow: () => void
    focusSearchPanel: () => void
    /** Cancels previous search when called. */
    streamSearch: (query: string, options: StreamSearchOptions) => void
    fetchStreamSuggestions: (query: string, sourcegraphURL: string) => ProxySubscribable<SearchMatch[]>
    setSelectedSearchContextSpec: (spec: string) => void
    /** Used to send current query from panel to sidebar. */
    setSidebarQueryState: (queryState: VSCEQueryState) => void
    /** Local Storage Item */
    getLocalStorageItem: (key: string) => string
    setLocalStorageItem: (key: string, value: string) => Promise<boolean>
    /** For Telemetry Service / logging */
    logEvents: (variables: Event) => void
    /** Get EventSource Type to use based on instance version */
    getEventSource: EventSource
    /** Get EventSource Type to use based on instance version */
    getEditorTheme: string
}

export interface SearchPanelAPI {
    ping: () => ProxySubscribable<'pong'>

    focusSearchBox: () => void
}

export interface SearchSidebarAPI
    extends Pick<FlatExtensionHostAPI, 'addTextDocumentIfNotExists' | 'getDefinition' | 'getHover' | 'getReferences'> {
    ping: () => ProxySubscribable<'pong'>

    addViewerIfNotExists: (viewer: ViewerData) => Promise<ViewerId>
}

export interface HelpSidebarAPI {}
