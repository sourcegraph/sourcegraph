import { proxy, type Remote } from 'comlink'
import { noop, sortBy } from 'lodash'
import { BehaviorSubject, EMPTY, type Unsubscribable } from 'rxjs'
import { map } from 'rxjs/operators'
import type * as sourcegraph from 'sourcegraph'

import { logger } from '@sourcegraph/common'
import { Location, MarkupKind, Position, Range, Selection } from '@sourcegraph/extension-api-classes'

import type { ClientAPI } from '../client/api/api'
import { syncRemoteSubscription } from '../util'

import { DocumentHighlightKind } from './api/documentHighlights'
import { type InitData, updateContext } from './extensionHost'
import type { ExtensionHostState } from './extensionHostState'
import { addWithRollback } from './util'

export interface InitResult {
    configuration: sourcegraph.ConfigurationService
    workspace: typeof sourcegraph['workspace']
    commands: typeof sourcegraph['commands']
    languages: typeof sourcegraph['languages'] & {
        // Backcompat definitions that were removed from sourcegraph.d.ts but are still defined (as
        // noops with a log message), to avoid completely breaking extensions that use them.
        registerTypeDefinitionProvider: any
        registerImplementationProvider: any
    }
    graphQL: typeof sourcegraph['graphQL']
    app: typeof sourcegraph['app']
}

/**
 * Creates a factory function to create extension API objects. This factory function should be called
 * when an extension is activated and the resulting extension API object should be passed to `replaceAPIRequire`.
 *
 * Methods shared between all API instances are implemented when the factory function is created.
 * Methods scoped to an API instance (e.g. `sourcegraph.app.log`) are implemented when the factory function is called.
 */
export function createExtensionAPIFactory(
    state: ExtensionHostState,
    clientAPI: Remote<ClientAPI>,
    initData: Pick<InitData, 'clientApplication' | 'sourcegraphURL'>
): (extensionID: string) => typeof sourcegraph & {
    // Backcompat definitions that were removed from sourcegraph.d.ts but are still defined (as
    // noops with a log message), to avoid completely breaking extensions that use them.
    languages: {
        registerTypeDefinitionProvider: any
        registerImplementationProvider: any
    }
} {
    // Configuration
    const getConfiguration = <C extends object>(): sourcegraph.Configuration<C> => {
        const snapshot = state.settings.value.final as Readonly<C>

        const configuration: sourcegraph.Configuration<C> & { toJSON: any } = {
            value: snapshot,
            get: key => snapshot[key],
            update: (key, value) => clientAPI.applySettingsEdit({ path: [key as string | number], value }),
            toJSON: () => snapshot,
        }
        return configuration
    }
    const configuration: typeof sourcegraph['configuration'] = Object.assign(
        state.settings.pipe(map(() => undefined)),
        {
            get: getConfiguration,
        }
    )

    // Workspace
    const workspace: typeof sourcegraph['workspace'] = {
        get textDocuments() {
            return [...state.textDocuments.values()]
        },
        get roots() {
            return state.roots.value
        },
        get versionContext() {
            return undefined
        },
        get searchContext() {
            return state.searchContext
        },
        onDidOpenTextDocument: state.openedTextDocuments.asObservable(),
        openedTextDocuments: state.openedTextDocuments.asObservable(),
        onDidChangeRoots: state.roots.pipe(map(() => undefined)),
        rootChanges: state.rootChanges.asObservable(),
        versionContextChanges: EMPTY,
        searchContextChanges: state.searchContextChanges.asObservable(),
    }

    // App
    const window: sourcegraph.Window = {
        get visibleViewComponents(): sourcegraph.ViewComponent[] {
            const entries = [...state.viewComponents.entries()]
            return sortBy(entries, 0).map(([, viewComponent]) => viewComponent)
        },
        get activeViewComponent(): sourcegraph.ViewComponent | undefined {
            return state.activeViewComponentChanges.value
        },
        activeViewComponentChanges: state.activeViewComponentChanges.asObservable(),
    }

    const app: typeof sourcegraph['app'] = {
        get activeWindow() {
            return window
        },
        activeWindowChanges: new BehaviorSubject(window).asObservable(),
        get windows() {
            return [window]
        },
        // `log` is implemented on extension activation
        log: noop,
    }

    // Commands
    const commands: typeof sourcegraph['commands'] = {
        executeCommand: (command, ...args) => clientAPI.executeCommand(command, args),
        registerCommand: (command, callback) =>
            syncRemoteSubscription(clientAPI.registerCommand(command, proxy(callback))),
    }

    const languages: InitResult['languages'] = {
        registerHoverProvider: (
            selector: sourcegraph.DocumentSelector,
            provider: sourcegraph.HoverProvider
        ): Unsubscribable => addWithRollback(state.hoverProviders, { selector, provider }),
        registerDocumentHighlightProvider: (
            selector: sourcegraph.DocumentSelector,
            provider: sourcegraph.DocumentHighlightProvider
        ): Unsubscribable => addWithRollback(state.documentHighlightProviders, { selector, provider }),
        registerDefinitionProvider: (
            selector: sourcegraph.DocumentSelector,
            provider: sourcegraph.DefinitionProvider
        ): Unsubscribable => addWithRollback(state.definitionProviders, { selector, provider }),
        registerReferenceProvider: (
            selector: sourcegraph.DocumentSelector,
            provider: sourcegraph.ReferenceProvider
        ): Unsubscribable => addWithRollback(state.referenceProviders, { selector, provider }),
        registerLocationProvider: (
            id: string,
            selector: sourcegraph.DocumentSelector,
            provider: sourcegraph.LocationProvider
        ): Unsubscribable => addWithRollback(state.locationProviders, { selector, provider: { id, provider } }),

        // These were removed, but keep them here so that calls from old extensions do not throw
        // an exception and completely break.
        registerTypeDefinitionProvider: () => {
            logger.warn(
                'sourcegraph.languages.registerTypeDefinitionProvider was removed. Use sourcegraph.languages.registerLocationProvider instead.'
            )
            return { unsubscribe: () => undefined }
        },
        registerImplementationProvider: () => {
            logger.warn(
                'sourcegraph.languages.registerImplementationProvider was removed. Use sourcegraph.languages.registerLocationProvider instead.'
            )
            return { unsubscribe: () => undefined }
        },
        registerCompletionItemProvider: (): Unsubscribable => {
            logger.warn('sourcegraph.languages.registerCompletionItemProvider was removed.')
            return { unsubscribe: () => undefined }
        },
    }

    // GraphQL
    const graphQL: typeof sourcegraph['graphQL'] = {
        execute: ((query: any, variables: any) => clientAPI.requestGraphQL(query, variables)) as any,
    }

    // For debugging/tests.
    const sync = async (): Promise<void> => {
        await clientAPI.ping()
    }

    return function extensionAPIFactory(extensionID) {
        return {
            URI: URL,
            Position,
            Range,
            Selection,
            Location,
            MarkupKind,
            DocumentHighlightKind,
            app: {
                ...app,
                log: (...data) => {
                    if (state.activeLoggers.has(extensionID)) {
                        // Use a light gray background to differentiate extension ID from the message
                        clientAPI
                            .logExtensionMessage(`🧩 %c${extensionID}`, 'background-color: lightgrey;', ...data)
                            .catch(error => {
                                logger.error('Error sending extension message to main thread:', error)
                            })
                    }
                },
            },

            workspace,
            configuration,

            languages,

            commands,
            graphQL,

            internal: {
                sync: () => sync(),
                updateContext: (updates: sourcegraph.ContextValues) => updateContext(updates, state),
                sourcegraphURL: new URL(initData.sourcegraphURL),
                clientApplication: initData.clientApplication,
            },
        }
    }
}
