import * as comlink from 'comlink'
import { BehaviorSubject, Observable, of, ReplaySubject, Subject } from 'rxjs'
import * as sourcegraph from 'sourcegraph'

import { ConfiguredExtension } from '../../extensions/extension'
import { SettingsCascade } from '../../settings/settings'
import { MainThreadAPI } from '../contract'
import { ExtensionViewer, ViewerUpdate } from '../viewerTypes'

import { ExecutableExtension, observeActiveExtensions } from './activation'
import { ExtensionCodeEditor } from './api/codeEditor'
import { ExtensionDocument } from './api/textDocument'
import { ExtensionWorkspaceRoot } from './api/workspaceRoot'
import { InitData } from './extensionHost'
import { RegisteredProvider, PlainNotification, ProgressNotification } from './extensionHostApi'
import { ReferenceCounter } from './utils/ReferenceCounter'

export function createExtensionHostState(
    initData: Pick<InitData, 'initialSettings' | 'clientApplication'>,
    mainAPI: comlink.Remote<MainThreadAPI> | null,
    mainThreadAPIInitializations: Observable<boolean> | null
): ExtensionHostState {
    // We make the mainAPI nullable in which case no extension will ever be activated. This is
    // used only for the noop controller.
    let activeLanguages = new BehaviorSubject<ReadonlySet<string>>(new Set())
    let activeExtensions: Observable<(ConfiguredExtension | ExecutableExtension)[]> = of([])
    if (mainAPI !== null && mainThreadAPIInitializations !== null) {
        ;({ activeLanguages, activeExtensions } = observeActiveExtensions(mainAPI, mainThreadAPIInitializations))
    }

    return {
        haveInitialExtensionsLoaded: new BehaviorSubject<boolean>(false),

        roots: new BehaviorSubject<readonly ExtensionWorkspaceRoot[]>([]),
        rootChanges: new Subject<void>(),
        searchContextChanges: new Subject<string | undefined>(),
        searchContext: undefined,

        // Most extensions never call `configuration.get()` synchronously in `activate()` to get
        // the initial settings data, and instead only subscribe to configuration changes.
        // In order for these extensions to be able to access settings, make sure `configuration` emits on subscription.
        settings: new BehaviorSubject<Readonly<SettingsCascade>>(initData.initialSettings),

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

        lastViewerId: 0,
        textDocuments: new Map<string, ExtensionDocument>(),
        openedTextDocuments: new Subject<ExtensionDocument>(),
        viewComponents: new Map<string, ExtensionCodeEditor>(),

        activeLanguages,
        languageReferences: new ReferenceCounter<string>(),
        modelReferences: new ReferenceCounter<string>(),

        activeViewComponentChanges: new BehaviorSubject<ExtensionViewer | undefined>(undefined),
        viewerUpdates: new Subject<ViewerUpdate>(),

        // Use ReplaySubject so we don't lose notifications in case the client application subscribes
        // to notification streams after extensions have already sent notifications.
        // There should be no issue re: stale notifications, since client applications should only
        // create one "notification manager" instance.
        plainNotifications: new ReplaySubject<PlainNotification>(3),
        progressNotifications: new ReplaySubject<ProgressNotification & comlink.ProxyMarked>(3),

        activeExtensions,
        activeLoggers: new Set<string>(),
    }
}

export interface ExtensionHostState {
    haveInitialExtensionsLoaded: BehaviorSubject<boolean>
    settings: BehaviorSubject<Readonly<SettingsCascade>>

    // Workspace
    roots: BehaviorSubject<readonly ExtensionWorkspaceRoot[]>
    rootChanges: Subject<void>
    searchContextChanges: Subject<string | undefined>
    searchContext: string | undefined

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
    progressNotifications: ReplaySubject<ProgressNotification & comlink.ProxyMarked>

    // Extensions
    activeExtensions: Observable<(ConfiguredExtension | ExecutableExtension)[]>

    /** Set of names of active loggers determined by user settings */
    activeLoggers: Set<string>
}
