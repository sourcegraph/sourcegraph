import { SettingsCascade } from '../../settings/settings'
import { Remote, proxy, ProxyMarked } from 'comlink'
import * as sourcegraph from 'sourcegraph'
import {
    BehaviorSubject,
    Subject,
    of,
    Observable,
    from,
    concat,
    EMPTY,
    ReplaySubject,
    combineLatest,
    Subscribable,
    Subscription,
} from 'rxjs'
import {
    FlatExtensionHostAPI,
    MainThreadAPI,
    NotificationType,
    PlainNotification,
    ProgressNotification,
    ViewProviderResult,
} from '../contract'
import { syncRemoteSubscription, tryCatchPromise } from '../util'
import {
    switchMap,
    mergeMap,
    map,
    defaultIfEmpty,
    catchError,
    distinctUntilChanged,
    mapTo,
    tap,
    withLatestFrom,
    debounceTime,
    concatMap,
} from 'rxjs/operators'
import { proxySubscribable, providerResultToObservable } from './api/common'
import { TextDocumentIdentifier, match } from '../client/types/textDocument'
import { getModeFromPath } from '../../languages'
import { parseRepoURI } from '../../util/url'
import { fromLocation, toPosition } from './api/types'
import { ContributableViewContainer, Contributions, TextDocumentPositionParameters } from '../protocol'
import { LOADING, MaybeLoadingResult } from '@sourcegraph/codeintellify'
import { combineLatestOrDefault } from '../../util/rxjs/combineLatestOrDefault'
import { castArray, groupBy, identity, isEqual, isMatch, sortBy } from 'lodash'
import { fromHoverMerged } from '../client/types/hover'
import { isNot, isExactly, isDefined, property } from '../../util/types'
import { createDecorationType, validateFileDecoration } from './api/decorations'
import { InitData } from './extensionHost'
import { ExtensionDocument } from './api/textDocument'
import { ReferenceCounter } from '../../util/ReferenceCounter'
import { ExtensionCodeEditor } from './api/codeEditor'
import { ExtensionViewer, ViewerWithPartialModel, ViewerId, ViewerUpdate } from '../viewerTypes'
import { ExtensionDirectoryViewer } from './api/directoryViewer'
import { ExtensionWorkspaceRoot } from './api/workspaceRoot'
import { asError, createAggregateError, ErrorLike, isErrorLike } from '../../util/errors'
import { computeContext } from '../client/context/context'
import {
    filterContributions,
    evaluateContributions,
    mergeContributions,
    parseContributionExpressions,
} from '../client/services/contribution'
import { wrapRemoteObservable } from '../client/api/common'
import {
    ConfiguredRegistryExtension,
    toConfiguredRegistryExtension,
    ConfiguredExtension,
    getScriptURLFromExtensionManifest,
} from '../../extensions/extension'
import { gql } from '../../graphql/graphql'
import * as GQL from '../../graphql/schema'
import { memoizeObservable } from '../../util/memoizeObservable'
import { areExtensionsSame } from '../../extensions/extensions'
import { WorkspaceRoot } from '@sourcegraph/extension-api-types'

/**
 * Holds the entire state exposed to the extension host
 * as a single object
 */
export interface ExtensionHostState {
    haveInitialExtensionsLoaded: BehaviorSubject<boolean>
    settings: BehaviorSubject<Readonly<SettingsCascade>>

    // Workspace
    roots: BehaviorSubject<readonly ExtensionWorkspaceRoot[]>
    rootChanges: Subject<void>
    versionContext: BehaviorSubject<string | undefined>

    // Search
    queryTransformers: BehaviorSubject<readonly sourcegraph.QueryTransformer[]>

    // Language features
    hoverProviders: BehaviorSubject<readonly RegisteredProvider<sourcegraph.HoverProvider>[]>
    documentHighlightProviders: BehaviorSubject<readonly RegisteredProvider<sourcegraph.DocumentHighlightProvider>[]>
    definitionProviders: BehaviorSubject<readonly RegisteredProvider<sourcegraph.DefinitionProvider>[]>
    referenceProviders: BehaviorSubject<readonly RegisteredProvider<sourcegraph.ReferenceProvider>[]>
    locationProviders: BehaviorSubject<
        readonly RegisteredProvider<{ id: string; provider: sourcegraph.LocationProvider }>[]
    >

    // Decorations
    fileDecorationProviders: BehaviorSubject<readonly sourcegraph.FileDecorationProvider[]>

    // Context + Contributions
    context: BehaviorSubject<Context>
    /** All contributions, including those that are not enabled in the current context. */
    contributions: BehaviorSubject<readonly Contributions[]>

    // Viewer + Text documents
    lastViewerId: number
    openedTextDocuments: Subject<ExtensionDocument>
    activeLanguages: BehaviorSubject<ReadonlySet<string>>
    modelReferences: ReferenceCounter<string>
    languageReferences: ReferenceCounter<string>
    /** Mutable map of URIs to text documents */
    textDocuments: Map<string, ExtensionDocument>

    /** Mutable map of viewer ID to viewer. */
    viewComponents: Map<string, ExtensionViewer>
    activeViewComponentChanges: BehaviorSubject<ExtensionViewer | undefined>
    viewerUpdates: Subject<ViewerUpdate>

    plainNotifications: ReplaySubject<PlainNotification>
    progressNotifications: ReplaySubject<ProgressNotification & ProxyMarked>

    // Views
    panelViews: BehaviorSubject<readonly Observable<PanelViewData>[]>
    insightsPageViewProviders: BehaviorSubject<readonly RegisteredViewProvider<'insightsPage'>[]>
    homepageViewProviders: BehaviorSubject<readonly RegisteredViewProvider<'homepage'>[]>
    globalPageViewProviders: BehaviorSubject<readonly RegisteredViewProvider<'global/page'>[]>
    directoryViewProviders: BehaviorSubject<readonly RegisteredViewProvider<'directory'>[]>

    // Content
    linkPreviewProviders: BehaviorSubject<
        readonly { urlMatchPattern: string; provider: sourcegraph.LinkPreviewProvider }[]
    >
}

interface RegisteredViewProvider<W extends ContributableViewContainer> {
    id: string
    viewProvider: {
        provideView: (context: ViewContexts[W]) => sourcegraph.ProviderResult<sourcegraph.View>
    }
}

export interface InitResult {
    configuration: sourcegraph.ConfigurationService
    workspace: typeof sourcegraph['workspace']
    exposedToMain: FlatExtensionHostAPI
    // todo this is needed as a temp solution for getter problem
    state: Readonly<ExtensionHostState>
    commands: typeof sourcegraph['commands']
    search: typeof sourcegraph['search']
    languages: typeof sourcegraph['languages'] & {
        // Backcompat definitions that were removed from sourcegraph.d.ts but are still defined (as
        // noops with a log message), to avoid completely breaking extensions that use them.
        registerTypeDefinitionProvider: any
        registerImplementationProvider: any
    }
    graphQL: typeof sourcegraph['graphQL']
    content: typeof sourcegraph['content']
    internal: Pick<typeof sourcegraph['internal'], 'updateContext'>
    app: typeof sourcegraph['app']
}

/**
 * Holds internally ExtState and manages communication with the Client
 * Returns the initialized public extension API pieces ready for consumption and the internal extension host API ready to be exposed to the main thread
 * NOTE that this function will slowly merge with the one in extensionHost.ts
 *
 * @param mainAPI
 */
export const initNewExtensionAPI = (
    mainAPI: Remote<MainThreadAPI>,
    { initialSettings, clientApplication }: Pick<InitData, 'initialSettings' | 'clientApplication'>
): InitResult => {
    const state: ExtensionHostState = {
        haveInitialExtensionsLoaded: new BehaviorSubject<boolean>(false),

        roots: new BehaviorSubject<readonly ExtensionWorkspaceRoot[]>([]),
        rootChanges: new Subject<void>(),
        versionContext: new BehaviorSubject<string | undefined>(undefined),

        // Most extensions never call `configuration.get()` synchronously in `activate()` to get
        // the initial settings data, and instead only subscribe to configuration changes.
        // In order for these extensions to be able to access settings, make sure `configuration` emits on subscription.
        settings: new BehaviorSubject<Readonly<SettingsCascade<object>>>(initialSettings),

        queryTransformers: new BehaviorSubject<readonly sourcegraph.QueryTransformer[]>([]),

        hoverProviders: new BehaviorSubject<readonly RegisteredProvider<sourcegraph.HoverProvider>[]>([]),
        documentHighlightProviders: new BehaviorSubject<
            readonly RegisteredProvider<sourcegraph.DocumentHighlightProvider>[]
        >([]),
        definitionProviders: new BehaviorSubject<readonly RegisteredProvider<sourcegraph.DefinitionProvider>[]>([]),
        referenceProviders: new BehaviorSubject<readonly RegisteredProvider<sourcegraph.ReferenceProvider>[]>([]),
        locationProviders: new BehaviorSubject<
            readonly RegisteredProvider<{ id: string; provider: sourcegraph.LocationProvider }>[]
        >([]),

        fileDecorationProviders: new BehaviorSubject<readonly sourcegraph.FileDecorationProvider[]>([]),

        context: new BehaviorSubject<Context>({
            'clientApplication.isSourcegraph': clientApplication === 'sourcegraph',

            // Arbitrary, undocumented versioning for extensions that need different behavior for different
            // Sourcegraph versions.
            //
            // TODO: Make this more advanced if many extensions need this (although we should try to avoid
            // extensions needing this).
            'clientApplication.extensionAPIVersion.major': 3,
        }),
        contributions: new BehaviorSubject<readonly Contributions[]>([]),

        lastViewerId: 0,
        textDocuments: new Map<string, ExtensionDocument>(),
        openedTextDocuments: new Subject<ExtensionDocument>(),
        viewComponents: new Map<string, ExtensionCodeEditor>(),

        activeLanguages: new BehaviorSubject<ReadonlySet<string>>(new Set()),
        languageReferences: new ReferenceCounter<string>(),
        modelReferences: new ReferenceCounter<string>(),

        activeViewComponentChanges: new BehaviorSubject<ExtensionViewer | undefined>(undefined),
        viewerUpdates: new Subject<ViewerUpdate>(),

        // Use ReplaySubject so we don't lose notifications in case the client application subscribes
        // to notification streams after extensions have already sent notifications.
        // There should be no issue re: stale notifications, since client applications should only
        // create one "notification manager" instance.
        plainNotifications: new ReplaySubject<PlainNotification>(3),
        progressNotifications: new ReplaySubject<ProgressNotification & ProxyMarked>(3),

        panelViews: new BehaviorSubject<readonly Observable<PanelViewData>[]>([]),
        insightsPageViewProviders: new BehaviorSubject<readonly RegisteredViewProvider<'insightsPage'>[]>([]),
        homepageViewProviders: new BehaviorSubject<readonly RegisteredViewProvider<'homepage'>[]>([]),
        globalPageViewProviders: new BehaviorSubject<readonly RegisteredViewProvider<'global/page'>[]>([]),
        directoryViewProviders: new BehaviorSubject<readonly RegisteredViewProvider<'directory'>[]>([]),

        linkPreviewProviders: new BehaviorSubject<
            readonly { urlMatchPattern: string; provider: sourcegraph.LinkPreviewProvider }[]
        >([]),
    }

    const getTextDocument = (uri: string): ExtensionDocument => {
        const textDocument = state.textDocuments.get(uri)
        if (!textDocument) {
            throw new Error(`Text document does not exist with URI ${uri}`)
        }
        return textDocument
    }

    /**
     * Removes a model.
     *
     * @param uri The URI of the model to remove.
     */
    const removeTextDocument = (uri: string): void => {
        const model = getTextDocument(uri)
        state.textDocuments.delete(uri)
        if (state.languageReferences.decrement(model.languageId)) {
            state.activeLanguages.next(new Set<string>(state.languageReferences.keys()))
        }
    }

    /**
     * Returns the Viewer with the given viewerId.
     * Throws if no viewer exists with the given viewerId.
     */
    const getViewer = (viewerId: ViewerId['viewerId']): ExtensionViewer => {
        const viewer = state.viewComponents.get(viewerId)
        if (!viewer) {
            throw new ViewerNotFoundError(viewerId)
        }
        return viewer
    }

    const enabledExtensions = wrapRemoteObservable(mainAPI.getEnabledExtensions())
    const activatedExtensionIDs = new Set<string>()

    const activeExtensions: Observable<(ConfiguredExtension | ExecutableExtension)[]> = combineLatest([
        state.activeLanguages,
        enabledExtensions,
    ]).pipe(
        tap(([activeLanguages, enabledExtensions]) => {
            const activeExtensions = extensionsWithMatchedActivationEvent(enabledExtensions, activeLanguages)
            for (const extension of activeExtensions) {
                if (!activatedExtensionIDs.has(extension.id)) {
                    activatedExtensionIDs.add(extension.id)
                }
            }
        }),
        map(([, extensions]) =>
            extensions ? extensions.filter(extension => activatedExtensionIDs.has(extension.id)) : []
        ),
        distinctUntilChanged((a, b) => areExtensionsSame(a, b))
    )

    const exposedToMain: FlatExtensionHostAPI = {
        haveInitialExtensionsLoaded: () => proxySubscribable(state.haveInitialExtensionsLoaded.asObservable()),

        // Configuration
        syncSettingsData: settings => state.settings.next(Object.freeze(settings)),

        // Workspace
        getWorkspaceRoots: () => state.roots.value.map(({ uri, inputRevision }) => ({ uri: uri.href, inputRevision })),
        addWorkspaceRoot: root => {
            state.roots.next(Object.freeze([...state.roots.value, new ExtensionWorkspaceRoot(root)]))
            state.rootChanges.next()
        },
        removeWorkspaceRoot: uri => {
            state.roots.next(Object.freeze(state.roots.value.filter(workspace => workspace.uri.href !== uri)))
            state.rootChanges.next()
        },
        setVersionContext: context => state.versionContext.next(context),

        // Search
        transformSearchQuery: query =>
            // TODO (simon) I don't enjoy the dark arts below
            // we return observable because of potential deferred addition of transformers
            // in this case we need to reissue the transformation and emit the resulting value
            // we probably won't need an Observable if we somehow coordinate with extensions activation
            proxySubscribable(
                state.queryTransformers.pipe(
                    switchMap(transformers =>
                        transformers.reduce(
                            (currentQuery: Observable<string>, transformer) =>
                                currentQuery.pipe(
                                    mergeMap(query => {
                                        const result = transformer.transformQuery(query)
                                        return result instanceof Promise ? from(result) : of(result)
                                    })
                                ),
                            of(query)
                        )
                    )
                )
            ),

        // Language
        getHover: (textParameters: TextDocumentPositionParameters) => {
            const document = getTextDocument(textParameters.textDocument.uri)
            const position = toPosition(textParameters.position)

            return proxySubscribable(
                callProviders(
                    state.hoverProviders,
                    providers => providersForDocument(document, providers, ({ selector }) => selector),
                    ({ provider }) => provider.provideHover(document, position),
                    results => fromHoverMerged(mergeProviderResults(results))
                )
            )
        },
        getDocumentHighlights: (textParameters: TextDocumentPositionParameters) => {
            const document = getTextDocument(textParameters.textDocument.uri)
            const position = toPosition(textParameters.position)

            return proxySubscribable(
                callProviders(
                    state.documentHighlightProviders,
                    providers => providersForDocument(document, providers, ({ selector }) => selector),
                    ({ provider }) => provider.provideDocumentHighlights(document, position),
                    mergeProviderResults
                ).pipe(map(result => (result.isLoading ? [] : result.result)))
            )
        },
        getDefinition: (textParameters: TextDocumentPositionParameters) => {
            const document = getTextDocument(textParameters.textDocument.uri)
            const position = toPosition(textParameters.position)

            return proxySubscribable(
                callProviders(
                    state.definitionProviders,
                    providers => providersForDocument(document, providers, ({ selector }) => selector),
                    ({ provider }) => provider.provideDefinition(document, position),
                    results => mergeProviderResults(results).map(fromLocation)
                )
            )
        },
        getReferences: (textParameters: TextDocumentPositionParameters, context: sourcegraph.ReferenceContext) => {
            const document = getTextDocument(textParameters.textDocument.uri)
            const position = toPosition(textParameters.position)

            return proxySubscribable(
                callProviders(
                    state.referenceProviders,
                    providers => providersForDocument(document, providers, ({ selector }) => selector),
                    ({ provider }) => provider.provideReferences(document, position, context),
                    results => mergeProviderResults(results).map(fromLocation)
                )
            )
        },
        hasReferenceProvidersForDocument: (textParameters: TextDocumentPositionParameters) => {
            const document = getTextDocument(textParameters.textDocument.uri)

            return proxySubscribable(
                state.referenceProviders.pipe(
                    map(providers => providersForDocument(document, providers, ({ selector }) => selector).length !== 0)
                )
            )
        },

        getLocations: (id: string, textParameters: TextDocumentPositionParameters) => {
            const document = getTextDocument(textParameters.textDocument.uri)
            const position = toPosition(textParameters.position)

            return proxySubscribable(
                callProviders(
                    state.locationProviders,
                    providers =>
                        providersForDocument(
                            document,
                            providers.filter(({ provider }) => id === provider.id),
                            ({ selector }) => selector
                        ),
                    ({ provider }) => provider.provider.provideLocations(document, position),
                    results => mergeProviderResults(results).map(fromLocation)
                )
            )
        },

        // Decorations
        getFileDecorations: (parameters: sourcegraph.FileDecorationContext) =>
            proxySubscribable(
                parameters.files.length === 0
                    ? EMPTY // Don't call providers when there are no files in the directory
                    : callProviders(
                          state.fileDecorationProviders,
                          identity,
                          // No need to filter
                          provider => provider.provideFileDecorations(parameters),
                          mergeProviderResults
                      ).pipe(
                          map(({ result }) =>
                              groupBy(
                                  result.filter(validateFileDecoration),
                                  // Get path from uri to key by path.
                                  // Path should always exist, but fall back to uri just in case
                                  ({ uri }) => parseRepoURI(uri).filePath || uri
                              )
                          )
                      )
            ),

        // MODELS

        //  TODO(tj): if not exists? doesn't seem that we can guarantee that just based on uri
        addViewerIfNotExists: viewerData => {
            const viewerId = `viewer#${state.lastViewerId++}`
            if (viewerData.type === 'CodeEditor') {
                state.modelReferences.increment(viewerData.resource)
            }
            let viewComponent: ExtensionViewer
            switch (viewerData.type) {
                case 'CodeEditor': {
                    const textDocument = getTextDocument(viewerData.resource)
                    viewComponent = new ExtensionCodeEditor({ ...viewerData, viewerId }, textDocument)
                    break
                }

                case 'DirectoryViewer': {
                    viewComponent = new ExtensionDirectoryViewer({ ...viewerData, viewerId })
                    break
                }
            }

            state.viewComponents.set(viewerId, viewComponent)
            if (viewerData.isActive) {
                state.activeViewComponentChanges.next(viewComponent)
            }
            state.viewerUpdates.next({ viewerId, action: 'addition', type: viewerData.type })
            return { viewerId }
        },

        removeViewer: ({ viewerId }) => {
            const viewer = getViewer(viewerId)
            state.viewComponents.delete(viewerId)
            // Check if this was the active viewer
            if (state.activeViewComponentChanges.value?.viewerId === viewerId) {
                state.activeViewComponentChanges.next(undefined)
            }
            state.viewerUpdates.next({ viewerId, action: 'removal', type: viewer.type })
            if (viewer.type === 'CodeEditor' && state.modelReferences.decrement(viewer.resource)) {
                removeTextDocument(viewer.resource)
            }
        },

        viewerUpdates: () => proxySubscribable(state.viewerUpdates.asObservable()),

        setEditorSelections: ({ viewerId }, selections) => {
            const viewer = getViewer(viewerId)
            assertViewerType(viewer, 'CodeEditor')
            viewer.update({ selections })
        },
        getTextDecorations: ({ viewerId }) => {
            const viewer = getViewer(viewerId)
            assertViewerType(viewer, 'CodeEditor')
            return proxySubscribable(viewer.mergedDecorations)
        },

        addTextDocumentIfNotExists: textDocumentData => {
            if (state.textDocuments.has(textDocumentData.uri)) {
                return
            }
            const textDocument = new ExtensionDocument(textDocumentData)
            state.textDocuments.set(textDocumentData.uri, textDocument)
            state.openedTextDocuments.next(textDocument)
            // Update activeLanguages if no other existing model has the same language.
            if (state.languageReferences.increment(textDocumentData.languageId)) {
                state.activeLanguages.next(new Set<string>(state.languageReferences.keys()))
            }
        },

        // For panel view location provider arguments
        getActiveCodeEditorPosition: () =>
            proxySubscribable(
                state.activeViewComponentChanges.pipe(
                    map(activeViewer => {
                        if (activeViewer?.type !== 'CodeEditor') {
                            return null
                        }
                        const sel = activeViewer.selections[0]
                        if (!sel) {
                            return null
                        }
                        // TODO(sqs): Return null for empty selections (but currently all selected tokens are treated as an empty
                        // selection at the beginning of the token, so this would break a lot of things, so we only do this for empty
                        // selections when the start character is -1). HACK(sqs): Character === -1 means that the whole line is
                        // selected (this is a bug in the caller, but it is useful here).
                        const isEmpty =
                            sel.start.line === sel.end.line &&
                            sel.start.character === sel.end.character &&
                            sel.start.character === -1
                        if (isEmpty) {
                            return null
                        }
                        return {
                            textDocument: { uri: activeViewer.resource },
                            position: { line: sel.start.line, character: sel.start.character },
                        }
                    })
                )
            ),

        // Context data + Contributions
        updateContext,
        registerContributions: rawContributions => {
            const parsedContributions = parseContributionExpressions(rawContributions)

            return proxy(addWithRollback(state.contributions, parsedContributions))
        },
        getContributions: (scope, extraContext) =>
            // TODO(tj): memoize access from mainthread (maybe by scope and extraContext (shallow))
            proxySubscribable(
                combineLatest([
                    state.contributions,
                    state.activeViewComponentChanges.pipe(
                        map((activeEditor): ViewerWithPartialModel | undefined => {
                            if (!activeEditor) {
                                return undefined
                            }

                            switch (activeEditor.type) {
                                case 'CodeEditor': {
                                    const { languageId } = getTextDocument(activeEditor.resource)
                                    return Object.assign(activeEditor, { model: { languageId } })
                                }
                                case 'DirectoryViewer':
                                    return activeEditor
                            }
                        })
                    ),
                    state.settings,
                    state.context as Subscribable<Context<unknown>>,
                ]).pipe(
                    map(([multiContributions, activeEditor, settings, context]) => {
                        // Merge in extra context.
                        if (extraContext) {
                            context = { ...context, ...extraContext }
                        }

                        // TODO(sqs): Observe context so that we update immediately upon changes.
                        const computedContext = computeContext(activeEditor, settings, context, scope)

                        return multiContributions.map(contributions => {
                            try {
                                return filterContributions(evaluateContributions(computedContext, contributions))
                            } catch (error) {
                                // An error during evaluation causes all of the contributions in the same entry to be
                                // discarded.
                                console.log('Discarding contributions: evaluating expressions or templates failed.', {
                                    contributions,
                                    error,
                                })
                                return {}
                            }
                        })
                    }),
                    map(mergeContributions),
                    distinctUntilChanged(isEqual)
                )
            ),

        // Notifications
        getPlainNotifications: () => proxySubscribable(state.plainNotifications.asObservable()),
        getProgressNotifications: () => proxySubscribable(state.progressNotifications.asObservable()),

        // Views
        getPanelViews: () =>
            // Don't need `combineLatestOrDefault` here since each panel view
            // is a BehaviorSubject, and therefore guaranteed to emit
            proxySubscribable(
                state.panelViews.pipe(
                    switchMap(panelViews => combineLatest([...panelViews])),
                    debounceTime(0)
                )
            ),

        getInsightsViews: context => proxySubscribable(callViewProviders(context, state.insightsPageViewProviders)),
        getHomepageViews: context => proxySubscribable(callViewProviders(context, state.homepageViewProviders)),
        getGlobalPageViews: context => proxySubscribable(callViewProviders(context, state.globalPageViewProviders)),
        getDirectoryViews: context =>
            proxySubscribable(
                callViewProviders(
                    {
                        viewer: {
                            ...context.viewer,
                            directory: {
                                ...context.viewer.directory,
                                uri: new URL(context.viewer.directory.uri),
                            },
                        },
                        workspace: { uri: new URL(context.workspace.uri) },
                    },
                    state.directoryViewProviders
                )
            ),

        // Content
        getLinkPreviews: (url: string) =>
            proxySubscribable(
                callProviders(
                    state.linkPreviewProviders,
                    entries => entries.filter(entry => url.startsWith(entry.urlMatchPattern)),
                    ({ provider }) => provider.provideLinkPreview(new URL(url)),
                    stuffs => mergeLinkPreviews(stuffs)
                ).pipe(map(result => (result.isLoading ? null : result.result)))
            ),

        getActiveExtensions: () => proxySubscribable(activeExtensions),
    }

    // Configuration
    const getConfiguration = <C extends object>(): sourcegraph.Configuration<C> => {
        const snapshot = state.settings.value.final as Readonly<C>

        const configuration: sourcegraph.Configuration<C> & { toJSON: any } = {
            value: snapshot,
            get: key => snapshot[key],
            update: (key, value) => mainAPI.applySettingsEdit({ path: [key as string | number], value }),
            toJSON: () => snapshot,
        }
        return configuration
    }

    // Workspace
    const workspace: typeof sourcegraph['workspace'] = {
        get textDocuments() {
            return [...state.textDocuments.values()]
        },
        get roots() {
            return state.roots.value
        },
        get versionContext() {
            return state.versionContext.value
        },
        onDidOpenTextDocument: state.openedTextDocuments.asObservable(),
        openedTextDocuments: state.openedTextDocuments.asObservable(),
        onDidChangeRoots: state.roots.pipe(mapTo(undefined)),
        rootChanges: state.rootChanges.asObservable(),
        versionContextChanges: state.versionContext.pipe(mapTo(undefined)),
    }

    const createProgressReporter = async (
        options: sourcegraph.ProgressOptions
        // `showProgress` returned a promise when progress reporters were created
        // in the main thread. continue to return promise for backward compatibility
        // eslint-disable-next-line @typescript-eslint/require-await
    ): Promise<sourcegraph.ProgressReporter> => {
        // There's no guarantee that UI consumers have subscribed to the progress observable
        // by the time that an extension reports progress, so replay the latest report on subscription.
        const progressSubject = new ReplaySubject<sourcegraph.Progress>(1)

        // progress notifications have to be proxied since the observable
        // `progress` property cannot be cloned
        state.progressNotifications.next(
            proxy({
                baseNotification: {
                    message: options.title,
                    type: NotificationType.Log,
                },
                progress: proxySubscribable(progressSubject.asObservable()),
            })
        )

        // return ProgressReporter, which exposes a subset of Subject methods to extensions
        return {
            next: (progress: sourcegraph.Progress) => {
                progressSubject.next(progress)
            },
            error: (value: any) => {
                const error = asError(value)
                progressSubject.error({
                    message: error.message,
                    name: error.name,
                    stack: error.stack,
                })
            },
            complete: () => {
                progressSubject.complete()
            },
        }
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
        showNotification: (message, type) => {
            state.plainNotifications.next({ message, type })
        },
        withProgress: async (options, task) => {
            const reporter = await createProgressReporter(options)
            try {
                const result = task(reporter)
                reporter.complete()
                return await result
            } catch (error) {
                reporter.error(error)
                throw error
            }
        },

        showProgress: options => createProgressReporter(options),
        showMessage: message => mainAPI.showMessage(message),
        showInputBox: options => mainAPI.showInputBox(options),
    }

    const app: typeof sourcegraph['app'] = {
        get activeWindow() {
            return window
        },
        activeWindowChanges: new BehaviorSubject(window).asObservable(),
        get windows() {
            return [window]
        },
        registerFileDecorationProvider: (provider: sourcegraph.FileDecorationProvider): sourcegraph.Unsubscribable =>
            addWithRollback(state.fileDecorationProviders, provider),
        createPanelView: id => {
            const panelViewData = new BehaviorSubject<PanelViewData>({
                id,
                title: '',
                content: '',
                component: null,
                priority: 0,
            })

            const panelView: sourcegraph.PanelView = {
                get title() {
                    return panelViewData.value.title
                },
                set title(title: string) {
                    panelViewData.next({ ...panelViewData.value, title })
                },
                get content() {
                    return panelViewData.value.content
                },
                set content(content: string) {
                    panelViewData.next({ ...panelViewData.value, content })
                },
                get component() {
                    return panelViewData.value.component
                },
                set component(component: { locationProvider: string } | null) {
                    panelViewData.next({ ...panelViewData.value, component })
                },
                get priority() {
                    return panelViewData.value.priority
                },
                set priority(priority: number) {
                    panelViewData.next({ ...panelViewData.value, priority })
                },
                unsubscribe: () => {
                    subscription.unsubscribe()
                },
            }

            // batch updates from same tick
            const subscription = addWithRollback(state.panelViews, panelViewData.pipe(debounceTime(0)))

            return panelView
        },
        registerViewProvider: (id, provider) => {
            switch (provider.where) {
                case 'insightsPage':
                    return addWithRollback(state.insightsPageViewProviders, { id, viewProvider: provider })

                case 'directory':
                    return addWithRollback(state.directoryViewProviders, { id, viewProvider: provider })

                case 'global/page':
                    return addWithRollback(state.globalPageViewProviders, { id, viewProvider: provider })

                case 'homepage':
                    return addWithRollback(state.homepageViewProviders, { id, viewProvider: provider })
            }
        },
        createDecorationType,
    }

    // Commands
    const commands: typeof sourcegraph['commands'] = {
        executeCommand: (command, ...args) => mainAPI.executeCommand(command, args),
        registerCommand: (command, callback) =>
            syncRemoteSubscription(mainAPI.registerCommand(command, proxy(callback))),
    }

    // Search
    const search: typeof sourcegraph['search'] = {
        registerQueryTransformer: transformer => addWithRollback(state.queryTransformers, transformer),
    }

    const languages: InitResult['languages'] = {
        registerHoverProvider: (
            selector: sourcegraph.DocumentSelector,
            provider: sourcegraph.HoverProvider
        ): sourcegraph.Unsubscribable => addWithRollback(state.hoverProviders, { selector, provider }),
        registerDocumentHighlightProvider: (
            selector: sourcegraph.DocumentSelector,
            provider: sourcegraph.DocumentHighlightProvider
        ): sourcegraph.Unsubscribable => addWithRollback(state.documentHighlightProviders, { selector, provider }),
        registerDefinitionProvider: (
            selector: sourcegraph.DocumentSelector,
            provider: sourcegraph.DefinitionProvider
        ): sourcegraph.Unsubscribable => addWithRollback(state.definitionProviders, { selector, provider }),
        registerReferenceProvider: (
            selector: sourcegraph.DocumentSelector,
            provider: sourcegraph.ReferenceProvider
        ): sourcegraph.Unsubscribable => addWithRollback(state.referenceProviders, { selector, provider }),
        registerLocationProvider: (
            id: string,
            selector: sourcegraph.DocumentSelector,
            provider: sourcegraph.LocationProvider
        ): sourcegraph.Unsubscribable =>
            addWithRollback(state.locationProviders, { selector, provider: { id, provider } }),

        // These were removed, but keep them here so that calls from old extensions do not throw
        // an exception and completely break.
        registerTypeDefinitionProvider: () => {
            console.warn(
                'sourcegraph.languages.registerTypeDefinitionProvider was removed. Use sourcegraph.languages.registerLocationProvider instead.'
            )
            return { unsubscribe: () => undefined }
        },
        registerImplementationProvider: () => {
            console.warn(
                'sourcegraph.languages.registerImplementationProvider was removed. Use sourcegraph.languages.registerLocationProvider instead.'
            )
            return { unsubscribe: () => undefined }
        },
        registerCompletionItemProvider: (): sourcegraph.Unsubscribable => {
            console.warn('sourcegraph.languages.registerCompletionItemProvider was removed.')
            return { unsubscribe: () => undefined }
        },
    }

    // GraphQL
    const graphQL: typeof sourcegraph['graphQL'] = {
        execute: (query, variables) => mainAPI.requestGraphQL(query, variables),
    }

    // Content
    const content: typeof sourcegraph['content'] = {
        registerLinkPreviewProvider: (urlMatchPattern: string, provider: sourcegraph.LinkPreviewProvider) =>
            addWithRollback(state.linkPreviewProviders, { urlMatchPattern, provider }),
    }

    // Context + Contributions
    // Same implementation is exposed to main and extensions
    function updateContext(update: { [k: string]: unknown }): void {
        if (isMatch(state.context.value, update)) {
            return
        }
        const result: any = {}
        for (const [key, oldValue] of Object.entries(state.context.value)) {
            if (update[key] !== null) {
                result[key] = oldValue
            }
        }
        for (const [key, value] of Object.entries(update)) {
            if (value !== null) {
                result[key] = value
            }
        }
        state.context.next(result)
    }

    const getScriptURLs = memoizeObservable(
        () =>
            from(mainAPI.getScriptURLForExtension()).pipe(
                map(getScriptURL => {
                    function getBundleURLs(urls: string[]): Promise<(string | ErrorLike)[]> {
                        return getScriptURL ? getScriptURL(urls) : Promise.resolve(urls)
                    }

                    return getBundleURLs
                })
            ),
        () => 'getScriptURL'
    )

    const previouslyActivatedExtensions = new Set<string>()
    const extensionContributions = new Map<string, Contributions>()
    const subscription = activeExtensions
        .pipe(
            withLatestFrom(getScriptURLs(null)),
            concatMap(([activeExtensions, getScriptURLs]) => {
                const toDeactivate = new Set<string>()
                const toActivate = new Map<string, ConfiguredExtension | ExecutableExtension>()
                const activeExtensionIDs = new Set<string>()

                for (const extension of activeExtensions) {
                    // Populate set of currently active extension IDs
                    activeExtensionIDs.add(extension.id)
                    // Populate map of extensions to activate
                    if (!previouslyActivatedExtensions.has(extension.id)) {
                        toActivate.set(extension.id, extension)
                    }
                }

                for (const id of previouslyActivatedExtensions) {
                    // Populate map of extensions to deactivate
                    if (!activeExtensionIDs.has(id)) {
                        toDeactivate.add(id)
                    }
                }

                return from(
                    getScriptURLs(
                        [...toActivate.values()].map(extension => {
                            if ('scriptURL' in extension) {
                                // This is already an executable extension (inline extension)
                                return extension.scriptURL
                            }

                            return getScriptURLFromExtensionManifest(extension)
                        })
                    ).then(scriptURLs => {
                        // TODO: (not urgent) add scriptURL cache

                        const executableExtensionsToActivate: ExecutableExtension[] = [...toActivate.values()]
                            .map((extension, index) => ({
                                id: extension.id,
                                manifest: extension.manifest,
                                scriptURL: scriptURLs[index],
                            }))
                            .filter(
                                (extension): extension is ExecutableExtension => typeof extension.scriptURL === 'string'
                            )

                        return { toActivate: executableExtensionsToActivate, toDeactivate }
                    })
                ).pipe(
                    tap(({ toActivate }) => {
                        // Register extension contributions before loading extensions
                        // so that contributed UI elements are visible ASAP
                        for (const extension of toActivate) {
                            if (
                                extension.manifest &&
                                !isErrorLike(extension.manifest) &&
                                extension.manifest.contributes
                            ) {
                                const parsedContributions = parseContributionExpressions(extension.manifest.contributes)
                                extensionContributions.set(extension.id, parsedContributions)
                                state.contributions.next([...state.contributions.value, parsedContributions])
                            }
                        }
                    }),
                    map(({ toActivate, toDeactivate }) =>
                        from(
                            Promise.all([
                                toActivate.map(({ id, scriptURL }) =>
                                    activateExtension(id, scriptURL).catch(error =>
                                        console.error(`Error activating extension ${id}:`, asError(error))
                                    )
                                ),
                                [...toDeactivate].map(id =>
                                    deactivateExtension(id).catch(error =>
                                        console.error(`Error deactivating extension ${id}:`, asError(error))
                                    )
                                ),
                            ])
                        )
                    ),
                    map(() => ({ activated: toActivate, deactivated: toDeactivate })),
                    catchError(error => {
                        console.error(`Uncaught error during extension activation: ${error}`)
                        return []
                    })
                )
            })
        )
        .subscribe(({ activated, deactivated }) => {
            const contributionsToRemove = [...deactivated].map(id => extensionContributions.get(id)).filter(Boolean)

            for (const id of deactivated) {
                previouslyActivatedExtensions.delete(id)
                extensionContributions.delete(id)
            }

            for (const [id] of activated) {
                previouslyActivatedExtensions.add(id)
            }

            if (contributionsToRemove.length > 0) {
                state.contributions.next(
                    state.contributions.value.filter(contributions => !contributionsToRemove.includes(contributions))
                )
            }

            if (state.haveInitialExtensionsLoaded.value === false) {
                state.haveInitialExtensionsLoaded.next(true)
            }
        })

    return {
        configuration: Object.assign(state.settings.pipe(mapTo(undefined)), {
            get: getConfiguration,
        }),
        exposedToMain,
        workspace,
        state,
        commands,
        search,
        languages,
        app,
        graphQL,
        content,
        internal: {
            updateContext,
        },
    }
}

// Providers
export interface RegisteredProvider<T> {
    selector: sourcegraph.DocumentSelector
    provider: T
}

// TODO (loic, felix) it might make sense to port tests with the rest of provider registries.
/**
 * Filt ers a list of Providers (P type) based on their selectors and a document
 *
 * @param document to use for filtering
 * @param entries array of providers (P[])
 * @param selector a way to get a selector from a Provider
 * @returns a filtered array of providers
 */
export function providersForDocument<P>(
    document: TextDocumentIdentifier,
    entries: readonly P[],
    selector: (p: P) => sourcegraph.DocumentSelector
): P[] {
    return entries.filter(provider =>
        match(selector(provider), {
            uri: document.uri,
            languageId: getModeFromPath(parseRepoURI(document.uri).filePath || ''),
        })
    )
}

/**
 * calls next() on behaviorSubject with a immutably added element ([...old, value])
 *
 * @param behaviorSubject subject that holds a collection
 * @param value to add to a collection
 * @returns Unsubscribable that will remove that element from the behaviorSubject.value and call next() again
 */
function addWithRollback<T>(behaviorSubject: BehaviorSubject<readonly T[]>, value: T): sourcegraph.Unsubscribable {
    behaviorSubject.next([...behaviorSubject.value, value])
    return {
        unsubscribe: () => behaviorSubject.next(behaviorSubject.value.filter(item => item !== value)),
    }
}

/**
 * Helper function to abstract common logic of invoking language providers.
 *
 * 1. filters providers
 * 2. invokes filtered providers via invokeProvider function
 * 3. adds [LOADING] state for each provider result stream
 * 4. omits errors from provider results with potential logging
 * 5. aggregates latests results from providers based on mergeResult function
 *
 * @param providersObservable observable of provider collection (expected to emit if a provider was added or removed)
 * @param filterProviders specifies which providers should be invoked
 * @param invokeProvider specifies how to get results from a provider (usually a closure over provider arguments)
 * @param mergeResult specifies how providers results should be aggregated
 * @param logErrors if console.error should be used for reporting errors from providers
 * @returns observable of aggregated results from all providers based on mergeProviderResults function
 */
export function callProviders<TRegisteredProvider, TProviderResult, TMergedResult>(
    providersObservable: Observable<readonly TRegisteredProvider[]>,
    filterProviders: (providers: readonly TRegisteredProvider[]) => TRegisteredProvider[],
    invokeProvider: (provider: TRegisteredProvider) => sourcegraph.ProviderResult<TProviderResult>,
    mergeResult: (providerResults: readonly (TProviderResult | 'loading' | null | undefined)[]) => TMergedResult,
    logErrors: boolean = true
): Observable<MaybeLoadingResult<TMergedResult>> {
    return providersObservable
        .pipe(
            map(providers => filterProviders(providers)),

            switchMap(providers =>
                combineLatestOrDefault(
                    providers.map(provider =>
                        concat(
                            [LOADING],
                            providerResultToObservable(invokeProvider(provider)).pipe(
                                defaultIfEmpty<typeof LOADING | TProviderResult | null | undefined>(null),
                                catchError(error => {
                                    if (logErrors) {
                                        console.error('Provider errored:', error)
                                    }
                                    return [null]
                                })
                            )
                        )
                    )
                )
            )
        )
        .pipe(
            defaultIfEmpty<(typeof LOADING | TProviderResult | null | undefined)[]>([]),
            map(results => ({
                isLoading: results.some(hover => hover === LOADING),
                result: mergeResult(results),
            })),
            distinctUntilChanged((a, b) => isEqual(a, b))
        )
}

/**
 * Merges provider results
 *
 * @param results latest results from providers
 * @template TProviderResultElement Type of an element of the provider result array
 */
export function mergeProviderResults<TProviderResultElement>(
    results: readonly (typeof LOADING | TProviderResultElement | TProviderResultElement[] | null | undefined)[]
): TProviderResultElement[] {
    return results
        .filter(isNot(isExactly(LOADING)))
        .flatMap(castArray)
        .filter(isDefined)
}

/** Object of array of file decorations keyed by path relative to repo root uri */
export type FileDecorationsByPath = Record<string, sourcegraph.FileDecoration[] | undefined>

// Viewers + documents

const VIEWER_NOT_FOUND_ERROR_NAME = 'ViewerNotFoundError'
class ViewerNotFoundError extends Error {
    public readonly name = VIEWER_NOT_FOUND_ERROR_NAME
    constructor(viewerId: string) {
        super(`Viewer not found: ${viewerId}`)
    }
}

function assertViewerType<T extends ExtensionViewer['type']>(
    viewer: ExtensionViewer,
    type: T
): asserts viewer is ExtensionViewer & { type: T } {
    if (viewer.type !== type) {
        throw new Error(`Viewer ID ${viewer.viewerId} is type ${viewer.type}, expected ${type}`)
    }
}

// Context + contributions

/**
 * Context is an arbitrary, immutable set of key-value pairs. Its value can be any JSON object.
 *
 * @template T If you have a value with a property of type T that is not one of the primitive types listed below
 * (or Context), you can use Context<T> to hold that value. T must be a value that can be represented by a JSON
 * object.
 */
export interface Context<T = never>
    extends Record<
        string,
        string | number | boolean | null | Context<T> | T | (string | number | boolean | null | Context<T> | T)[]
    > {}

/** A registered set of contributions from an extension in the registry. */
export interface ContributionsEntry {
    /**
     * The contributions, either as a value or an observable.
     *
     * If an observable is used, it should be a cold Observable and emit (e.g., its current value) upon
     * subscription. The {@link ContributionRegistry#contributions} observable blocks until all observables have
     * emitted.
     */
    contributions: Contributions | Observable<Contributions | Contributions[]>
}

/** @internal */
export interface PanelViewData extends Omit<sourcegraph.PanelView, 'unsubscribe'> {
    id: string
}

// Activation

/** The WebWorker's global scope */
declare const self: any

/**
 * The information about an extension necessary to execute and activate it.
 */
export interface ExecutableExtension extends Pick<ConfiguredExtension, 'id' | 'manifest'> {
    /** The URL to the JavaScript bundle of the extension. */
    scriptURL: string
}

/**
 * Query the GraphQL API for registry metadata about the extensions given in {@link extensionIDs}.
 *
 * @returns An observable that emits once with the results.
 */
export function queryConfiguredRegistryExtensions(
    // TODO(tj): can copy this over to extension host, just replace platformContext.requestGraphQL
    // with mainThreadAPI.requestGraphQL
    graphQL: typeof sourcegraph['graphQL'],
    extensionIDs: string[]
): Observable<ConfiguredRegistryExtension[]> {
    if (extensionIDs.length === 0) {
        return of([])
    }
    const variables: GQL.IExtensionsOnExtensionRegistryArguments = {
        first: extensionIDs.length,
        prioritizeExtensionIDs: extensionIDs,
    }
    return from(
        graphQL.execute<GQL.IQuery, GQL.IExtensionsOnExtensionRegistryArguments>(
            gql`
                query Extensions($first: Int!, $prioritizeExtensionIDs: [String!]!) {
                    extensionRegistry {
                        extensions(first: $first, prioritizeExtensionIDs: $prioritizeExtensionIDs) {
                            nodes {
                                id
                                extensionID
                                url
                                manifest {
                                    raw
                                }
                                viewerCanAdminister
                            }
                        }
                    }
                }
            `,
            variables
        )
    ).pipe(
        map(({ data, errors }) => {
            if (!data?.extensionRegistry?.extensions?.nodes) {
                throw createAggregateError(errors)
            }
            return data.extensionRegistry.extensions.nodes.map(
                ({ id, extensionID, url, manifest, viewerCanAdminister }) => ({
                    id,
                    extensionID,
                    url,
                    manifest: manifest ? { raw: manifest.raw } : null,
                    viewerCanAdminister,
                })
            )
        }),
        map(registryExtensions => {
            const configuredExtensions: ConfiguredRegistryExtension[] = []
            for (const extensionID of extensionIDs) {
                const registryExtension = registryExtensions.find(extension => extension.extensionID === extensionID)
                configuredExtensions.push(
                    registryExtension
                        ? toConfiguredRegistryExtension(registryExtension)
                        : { id: extensionID, manifest: null, rawManifest: null, registryExtension: undefined }
                )
            }
            return configuredExtensions
        })
    )
}

function extensionsWithMatchedActivationEvent(
    enabledExtensions: (ConfiguredExtension | ExecutableExtension)[],
    visibleTextDocumentLanguages: ReadonlySet<string>
): (ConfiguredExtension | ExecutableExtension)[] {
    const languageActivationEvents = new Set(
        [...visibleTextDocumentLanguages].map(language => `onLanguage:${language}`)
    )
    return enabledExtensions.filter(extension => {
        try {
            if (!extension.manifest) {
                const match = /^sourcegraph\/lang-(.*)$/.exec(extension.id)
                if (match) {
                    console.warn(
                        `Extension ${extension.id} has been renamed to sourcegraph/${match[1]}. It's safe to remove ${extension.id} from your settings.`
                    )
                } else {
                    console.warn(
                        `Extension ${extension.id} was not found. Remove it from settings to suppress this warning.`
                    )
                }
                return false
            }
            if (isErrorLike(extension.manifest)) {
                console.warn(extension.manifest)
                return false
            }
            if (!extension.manifest.activationEvents) {
                console.warn(`Extension ${extension.id} has no activation events, so it will never be activated.`)
                return false
            }
            return extension.manifest.activationEvents.some(
                event => event === '*' || languageActivationEvents.has(event)
            )
        } catch (error) {
            console.error(error)
        }
        return false
    })
}

// TODO(tj): move to `activate.ts`

/** Extensions' deactivate functions. */
const extensionDeactivates = new Map<string, () => void | Promise<void>>()

async function activateExtension(extensionID: string, bundleURL: string): Promise<void> {
    // Load the extension bundle and retrieve the extension entrypoint module's exports on
    // the global `module` property.
    try {
        const exports = {}
        self.exports = exports
        self.module = { exports }
        self.importScripts(bundleURL)
    } catch (error) {
        throw new Error(
            `error thrown while executing extension ${JSON.stringify(
                extensionID
            )} bundle (in importScripts of ${bundleURL}): ${String(error)}`
        )
    }
    const extensionExports = self.module.exports
    delete self.exports
    delete self.module

    if (!('activate' in extensionExports)) {
        throw new Error(
            `extension bundle for ${JSON.stringify(extensionID)} has no exported activate function (in ${bundleURL})`
        )
    }

    // During extension deactivation, we first call the extension's `deactivate` function and then unsubscribe
    // the Subscription passed to the `activate` function.
    const extensionSubscriptions = new Subscription()
    const extensionDeactivate = typeof extensionExports.deactivate === 'function' ? extensionExports.deactivate : null
    extensionDeactivates.set(extensionID, async () => {
        try {
            if (extensionDeactivate) {
                await Promise.resolve(extensionDeactivate())
            }
        } finally {
            extensionSubscriptions.unsubscribe()
        }
    })

    // The behavior should be consistent for both sync and async activate functions that throw
    // errors or reject. Both cases should yield a rejected promise.
    //
    // TODO(sqs): Add timeouts to prevent long-running activate or deactivate functions from
    // significantly delaying other extensions.
    await tryCatchPromise<void>(() => extensionExports.activate({ subscriptions: extensionSubscriptions })).catch(
        error => {
            error = asError(error)
            throw Object.assign(
                new Error(
                    `error during extension ${JSON.stringify(extensionID)} activate function: ${String(
                        error.stack || error
                    )}`
                ),
                { error }
            )
        }
    )
}

async function deactivateExtension(extensionID: string): Promise<void> {
    const deactivate = extensionDeactivates.get(extensionID)
    if (deactivate) {
        extensionDeactivates.delete(extensionID)
        await Promise.resolve(deactivate())
    }
}

// Content

interface MarkupContentPlainTextOnly extends Pick<sourcegraph.MarkupContent, 'value'> {
    kind: sourcegraph.MarkupKind.PlainText
}

/**
 * Represents one or more {@link sourcegraph.LinkPreview} values merged together.
 */
export interface LinkPreviewMerged {
    /** The content of the merged {@link sourcegraph.LinkPreview} values. */
    content: sourcegraph.MarkupContent[]

    /** The hover content of the merged {@link sourcegraph.LinkPreview} values. */
    hover: MarkupContentPlainTextOnly[]
}

function mergeLinkPreviews(
    values: readonly (sourcegraph.LinkPreview | 'loading' | null | undefined)[]
): LinkPreviewMerged | null {
    const nonemptyValues = values
        .filter(isDefined)
        .filter((value): value is Exclude<sourcegraph.LinkPreview | 'loading', 'loading'> => value !== 'loading')
    const contentValues = nonemptyValues.filter(property('content', isDefined))
    const hoverValues = nonemptyValues.filter(property('hover', isDefined))
    if (hoverValues.length === 0 && contentValues.length === 0) {
        return null
    }
    return { content: contentValues.map(({ content }) => content), hover: hoverValues.map(({ hover }) => hover) }
}

// Views

/**
 * A map from type of container names to the internal type of the context parameter provided by the container.
 */
export interface ViewContexts {
    [ContributableViewContainer.Panel]: never
    [ContributableViewContainer.Homepage]: {}
    [ContributableViewContainer.InsightsPage]: {}
    [ContributableViewContainer.GlobalPage]: Record<string, string>
    [ContributableViewContainer.Directory]: sourcegraph.DirectoryViewContext
}

function callViewProviders<W extends ContributableViewContainer>(
    context: ViewContexts[W],
    providers: Observable<readonly RegisteredViewProvider<W>[]>
): Observable<ViewProviderResult[]> {
    return providers.pipe(
        debounceTime(0),
        switchMap(providers =>
            combineLatest([
                of(null),
                ...providers.map(({ viewProvider, id }) =>
                    concat(
                        [undefined],
                        providerResultToObservable(viewProvider.provideView(context)).pipe(
                            catchError((error): [ErrorLike] => {
                                console.error('View provider errored:', error)
                                return [asError(error)]
                            })
                        )
                    ).pipe(map(view => ({ id, view })))
                ),
            ])
        ),
        map(views => views.filter(isDefined))
    )
}

/**
 * A workspace root with additional metadata that is not exposed to extensions.
 */
export interface WorkspaceRootWithMetadata extends WorkspaceRoot {
    /**
     * The original input Git revision that the user requested. The {@link WorkspaceRoot#uri} value will contain
     * the Git commit SHA resolved from the input revision, but it is useful to also know the original revision
     * (e.g., to construct URLs for the user that don't result in them navigating from a branch view to a commit
     * SHA view).
     *
     * For example, if the user is viewing the web page https://github.com/alice/myrepo/blob/master/foo.js (note
     * that the URL contains a Git revision "master"), the input revision is "master".
     *
     * The empty string is a valid value (meaning that the default should be used, such as "HEAD" in Git) and is
     * distinct from undefined. If undefined, the Git commit SHA from {@link WorkspaceRoot#uri} should be used.
     */
    inputRevision?: string
}
