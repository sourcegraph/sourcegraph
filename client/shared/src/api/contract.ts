import { SettingsCascade } from '../settings/settings'
import { SettingsEdit } from './client/services/settings'
import * as clientType from '@sourcegraph/extension-api-types'
import { Remote, ProxyMarked } from 'comlink'
import * as sourcegraph from 'sourcegraph'
import { ProxySubscribable } from './extension/api/common'
import { Contributions, Evaluated, Raw, TextDocumentPositionParameters } from './protocol'
import { MaybeLoadingResult } from '@sourcegraph/codeintellify'
import { HoverMerged } from './client/types/hover'
import { GraphQLResult } from '../graphql/graphql'
import {
    Context,
    ExecutableExtension,
    FileDecorationsByPath,
    LinkPreviewMerged,
    PanelViewData,
    ViewContexts,
} from './extension/flatExtensionApi'
import { ContributionScope } from './client/context/context'
import { ErrorLike } from '../util/errors'
import { ConfiguredExtension } from '../extensions/extension'
import { DeepReplace } from '../util/types'
import { ViewerData, ViewerId, ViewerUpdate } from './viewerTypes'

// TODO: Move types to extension-api-types

/**
 * A text model is a text document and associated metadata.
 *
 * How does this relate to editors (in {@link ViewerService}? A model is the file, an editor is the
 * window that the file is shown in. Things like the content and language are properties of the
 * model; things like decorations and the selection ranges are properties of the editor.
 */
export interface TextDocumentData extends Pick<sourcegraph.TextDocument, 'uri' | 'languageId' | 'text'> {}

/**
 * A notification message to display to the user.
 */
export type ExtensionNotification = PlainNotification | ProgressNotification

interface BaseNotification {
    /** The message of the notification. */
    message?: string

    /**
     * The type of the message.
     */
    type: sourcegraph.NotificationType

    /** The source of the notification.  */
    source?: string
}

export interface PlainNotification extends BaseNotification {}

export interface ProgressNotification {
    // Put all base notification properties in a nested object because
    // ProgressNotifications are proxied, so it's better to clone this
    // notification object than to wait for all property access promises
    // to resolve
    baseNotification: BaseNotification

    /**
     * Progress updates to show in this notification (progress bar and status messages).
     * If this Observable errors, the notification will be changed to an error type.
     */
    progress: ProxySubscribable<sourcegraph.Progress>
}

export interface ViewProviderResult {
    /** The ID of the view provider. */
    id: string

    /** The result returned by the provider. */
    view: sourcegraph.View | undefined | ErrorLike
}

/**
 * The type of a notification.
 * This is needed because if sourcegraph.NotificationType enum values are referenced,
 * the `sourcegraph` module import at the top of the file is emitted in the generated code.
 */
export const NotificationType: typeof sourcegraph.NotificationType = {
    Error: 1,
    Warning: 2,
    Info: 3,
    Log: 4,
    Success: 5,
}

/**
 * This is exposed from the extension host thread to the main thread
 * e.g. for communicating  direction "main -> ext host"
 * Note this API object lives in the extension host thread
 */
export interface FlatExtensionHostAPI {
    /**
     * Updates the settings exposed to extensions.
     */
    syncSettingsData: (data: Readonly<SettingsCascade<object>>) => void

    // Workspace
    addWorkspaceRoot: (root: clientType.WorkspaceRoot) => void
    getWorkspaceRoots: () => clientType.WorkspaceRoot[]
    removeWorkspaceRoot: (uri: string) => void

    setVersionContext: (versionContext: string | undefined) => void

    // Search
    transformSearchQuery: (query: string) => ProxySubscribable<string>

    // Languages
    getHover: (parameters: TextDocumentPositionParameters) => ProxySubscribable<MaybeLoadingResult<HoverMerged | null>>
    getDocumentHighlights: (
        parameters: TextDocumentPositionParameters
    ) => ProxySubscribable<sourcegraph.DocumentHighlight[]>
    getDefinition: (
        parameters: TextDocumentPositionParameters
    ) => ProxySubscribable<MaybeLoadingResult<clientType.Location[]>>
    getReferences: (
        parameters: TextDocumentPositionParameters,
        context: sourcegraph.ReferenceContext
    ) => ProxySubscribable<MaybeLoadingResult<clientType.Location[]>>
    getLocations: (
        id: string,
        parameters: TextDocumentPositionParameters
    ) => ProxySubscribable<MaybeLoadingResult<clientType.Location[]>>

    hasReferenceProvidersForDocument: (parameters: TextDocumentPositionParameters) => ProxySubscribable<boolean>

    // Tree
    getFileDecorations: (parameters: sourcegraph.FileDecorationContext) => ProxySubscribable<FileDecorationsByPath>

    // CONTEXT + CONTRIBUTIONS

    /**
     * Sets the given context keys and values.
     * If a value is `null`, the context key is removed.
     *
     * @param update Object with context keys as values
     */
    updateContext: (update: { [k: string]: unknown }) => void

    /**
     * Register contributions and return an unsubscribable that deregisters the contributions.
     * Any expressions in the contributions will be parsed in the extension host.
     */
    registerContributions: (rawContributions: Raw<Contributions>) => sourcegraph.Unsubscribable & ProxyMarked

    /**
     * Returns an observable that emits all contributions (merged) evaluated in the current model
     * (with the optional scope). It emits whenever there is any change.
     *
     * @template T Extra allowed property value types for the {@link Context} value. See
     * {@link Context}'s `T` type parameter for more information.
     * @param scope The scope in which contributions are fetched. A scope can be a sub-component of
     * the UI that defines its own context keys, such as the hover, which stores useful loading and
     * definition/reference state in its scoped context keys.
     * @param extraContext Extra context values to use when computing the contributions. Properties
     * in this object shadow (take precedence over) properties in the global context for this
     * computation.
     */
    getContributions: <T>(
        scope?: ContributionScope | undefined,
        extraContext?: Context<T>
    ) => ProxySubscribable<Evaluated<Contributions>>

    // TEXT DOCUMENTS

    /**
     * TODO(tj)
     *
     * @param textDocumentData
     */
    addTextDocumentIfNotExists: (textDocumentData: TextDocumentData) => void

    // VIEWERS
    getActiveCodeEditorPosition: () => ProxySubscribable<TextDocumentPositionParameters | null>

    getTextDecorations: (viewerId: ViewerId) => ProxySubscribable<clientType.TextDocumentDecoration[]>

    /**
     * Add a viewer.
     *
     * @param viewer The description of the viewer to add.
     * @returns The added code viewer (which must be passed as the first argument to other
     * viewer methods to operate on this viewer).
     */
    addViewerIfNotExists(viewer: ViewerData): ViewerId

    /**
     * Emits whenever a viewer is added or removed.
     */
    viewerUpdates: () => ProxySubscribable<ViewerUpdate>

    /**
     * Sets the selections for a CodeEditor.
     *
     * @param codeEditor The editor for which to set the selections.
     * @param selections The new selections to apply.
     * @throws if no editor exists with the given editor ID.
     * @throws if the editor ID is not a CodeEditor.
     */
    setEditorSelections(codeEditor: ViewerId, selections: clientType.Selection[]): void

    /**
     * Removes a viewer.
     * Also removes the corresponding model if no other viewer is referencing it.
     *
     * @param viewer The viewer to remove.
     */
    removeViewer(viewer: ViewerId): void

    // Notifications
    getPlainNotifications: () => ProxySubscribable<PlainNotification>
    getProgressNotifications: () => ProxySubscribable<ProgressNotification & ProxyMarked>

    // Views
    getPanelViews: () => ProxySubscribable<PanelViewData[]>
    getInsightsViews: (context: ViewContexts['insightsPage']) => ProxySubscribable<ViewProviderResult[]>
    getHomepageViews: (context: ViewContexts['homepage']) => ProxySubscribable<ViewProviderResult[]>
    getGlobalPageViews: (context: ViewContexts['global/page']) => ProxySubscribable<ViewProviderResult[]>
    getDirectoryViews: (
        // Construct URL object on host from string provided by main thread
        context: DeepReplace<ViewContexts['directory'], URL, string>
    ) => ProxySubscribable<ViewProviderResult[]>

    // Content
    getLinkPreviews: (url: string) => ProxySubscribable<LinkPreviewMerged | null>

    /**
     * Emits true when the initial batch of extensions have been loaded.
     */
    haveInitialExtensionsLoaded: () => ProxySubscribable<boolean>

    getActiveExtensions: () => ProxySubscribable<ConfiguredExtension[]>
}

/**
 * This is exposed from the main thread to the extension host thread"
 * e.g. for communicating  direction "ext host -> main"
 * Note this API object lives in the main thread
 */
export interface MainThreadAPI {
    /**
     * Applies a settings update from extensions.
     */
    applySettingsEdit: (edit: SettingsEdit) => Promise<void>

    /**
     * GraphQL request API
     */
    requestGraphQL: (request: string, variables: any) => Promise<GraphQLResult<any>>

    // Commands
    executeCommand: (command: string, args: any[]) => Promise<any>
    registerCommand: (
        name: string,
        command: Remote<((...args: any) => any) & ProxyMarked>
    ) => sourcegraph.Unsubscribable & ProxyMarked

    // User interaction methods
    showMessage: (message: string) => Promise<void>
    showInputBox: (options?: sourcegraph.InputBoxOptions) => Promise<string | undefined>

    getSideloadedExtensionURL: () => ProxySubscribable<string | null>
    getScriptURLForExtension: () =>
        | undefined
        | (((bundleURLs: string[]) => Promise<(string | ErrorLike)[]>) & ProxyMarked)

    getEnabledExtensions: () => ProxySubscribable<(ConfiguredExtension | ExecutableExtension)[]>
}
