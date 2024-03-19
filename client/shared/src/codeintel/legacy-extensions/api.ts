/* eslint-disable @typescript-eslint/no-explicit-any */
import { lastValueFrom, type Observable, type Unsubscribable } from 'rxjs'

import type { GraphQLResult } from '@sourcegraph/http-client'

import type { PlatformContext } from '../../platform/context'
import type { Settings, SettingsCascade } from '../../settings/settings'

/**
 * Represents a location inside a resource, such as a line
 * inside a text file.
 */
export class Location {
    constructor(public readonly uri: URL, public readonly range?: Range) {}
}

/**
 * A text document, such as a file in a repository.
 */
export interface TextDocument {
    /**
     * The URI of the text document.
     */
    readonly uri: string

    /**
     * The language of the text document.
     */
    readonly languageId: string

    /**
     * The text contents of the text document.
     *
     * When using the [Sourcegraph browser
     * extension](https://sourcegraph.com/docs/integration/browser_extension), the value is
     * `undefined` because determining the text contents (in general) is not possible without
     * additional access to the code host API. In the future, this limitation may be removed.
     */
    readonly text: string | undefined
}

/**
 * A document filter denotes a document by different properties like the
 * [language](#TextDocument.languageId), the scheme of its resource, or a glob-pattern that is
 * applied to the [path](#TextDocument.fileName).
 * A document filter matches if all the provided properties (those of `language`, `scheme` and `pattern` that are not `undefined`) match.
 * If all properties are `undefined`, the document filter matches all documents.
 *
 * Examples:
 * ```ts
 * // A language filter that applies to typescript files on disk
 * { language: 'typescript', scheme: 'file' }
 *
 * // A language filter that applies to all package.json paths
 * { language: 'json', pattern: '**package.json' }
 * ```
 */
export interface DocumentFilter {
    /** A language id, such as `typescript` or `*`. */
    language?: string

    /** A URI scheme, such as `file` or `untitled`. */
    scheme?: string

    /** A glob pattern, such as `*.{ts,js}`. */
    pattern?: string

    /** A base URI (e.g. root URI of a workspace folder) that the document must be within. */
    baseUri?: URL | string
}

/**
 * A document selector is the combination of one or many document filters.
 * A document matches the selector if any of the given filters matches.
 * If the filter is a string and not a {@link DocumentFilter}, it will be treated as a language id.
 *
 * @example let sel: DocumentSelector = [{ language: 'typescript' }, { language: 'json', pattern: '**∕tsconfig.json' }];
 */
export type DocumentSelector = (string | DocumentFilter)[]

export interface Directory {
    /**
     * The URI of the directory.
     *
     * @todo The format of this URI will be changed in the future. It must not be relied on.
     */
    readonly uri: URL
}

/**
 * A workspace root is a directory that has been added to a workspace. A workspace can have zero or more roots.
 * Often, each root is the root directory of a repository.
 */
export interface WorkspaceRoot {
    /**
     * The URI of the root.
     *
     * @todo The format of this URI will be changed in the future. It must not be relied on.
     *
     * @example git://github.com/sourcegraph/sourcegraph?sha#mydir1/mydir2
     */
    readonly uri: URL
}

/**
 * The full configuration value, containing all settings for the current subject.
 *
 * @template C The configuration schema.
 */
export interface Configuration<C extends object> {
    /**
     * Returns a value at a specific key in the configuration.
     *
     * @template C The configuration schema.
     * @template K Valid key on the configuration object.
     * @param key The name of the configuration property to get.
     * @returns The configuration value, or `undefined`.
     */
    get<K extends keyof C>(key: K): Readonly<C[K]> | undefined

    /**
     * Updates the configuration value for the given key. The updated configuration value is persisted by the
     * client.
     *
     * @template C The configuration schema.
     * @template K Valid key on the configuration object.
     * @param key The name of the configuration property to update.
     * @param value The new value, or undefined to remove it.
     * @returns A promise that resolves when the client acknowledges the update.
     */
    update<K extends keyof C>(key: K, value: C[K] | undefined): Promise<void>

    /**
     * The configuration value as a plain object.
     */
    readonly value: Readonly<C>
}

/**
 * A provider result represents the values that a provider, such as the {@link HoverProvider}, may return. The
 * result may be a single value, a Promise that resolves to a single value, a Subscribable that emits zero
 * or more values, or an AsyncIterable that yields zero or more values.
 */
export type ProviderResult<T> =
    // | T
    // | undefined
    // | null
    // | Promise<T | undefined | null>
    Observable<T | undefined | null>

/** The kinds of markup that can be used. */
export enum MarkupKind {
    PlainText = 'plaintext',
    Markdown = 'markdown',
}

/**
 * Human-readable text that supports various kinds of formatting.
 */
export interface MarkupContent {
    /** The marked up text. */
    value: string

    /**
     * The kind of markup used.
     *
     * @default MarkupKind.Markdown
     */
    kind?: MarkupKind
}

/** A badge holds the extra fields that can be attached to a providable type T via Badged<T>. */
export interface Badge {
    /**
     * Aggregable badges are concatenated and de-duplicated within a particular result set. These
     * values can briefly be used to describe some common property of the underlying result set.
     *
     * We currently use this to display whether a file in the file match locations pane contains
     * only precise or only search-based code navigation results.
     */
    aggregableBadges?: AggregableBadge[]
}

/**
 * Aggregable badges are concatenated and de-duplicated within a particular result set. These
 * values can briefly be used to describe some common property of the underlying result set.
 */
export interface AggregableBadge {
    /** The display text of the badge. */
    text: string

    /** If set, the badge becomes a link with this destination URL. */
    linkURL?: string

    /** Tooltip text to display when hovering over the badge. */
    hoverMessage?: string
}

/**
 * A wrapper around a providable type (hover text and locations) with additional context to enable
 * displaying badges next to the wrapped result value in the UI.
 */
export type Badged<T extends object> = T & Badge

/**
 * A hover represents additional information for a symbol or word. Hovers are rendered in a tooltip-like
 * widget.
 */
export interface Hover {
    /**
     * The contents of this hover.
     */
    contents: MarkupContent

    /**
     * The range to which this hover applies. When missing, the editor will use the range at the current
     * position or the current position itself.
     */
    range?: Range
}

export interface HoverProvider {
    provideHover(document: TextDocument, position: Position): ProviderResult<Badged<Hover>>
}

/**
 * The definition of a symbol represented as one or many [locations](#Location). For most programming languages
 * there is only one location at which a symbol is defined. If no definition can be found `null` is returned.
 */
export type Definition = Badged<Location> | Badged<Location>[] | null

/**
 * A definition provider implements the "go-to-definition" feature.
 */
export interface DefinitionProvider {
    /**
     * Provide the definition of the symbol at the given position and document.
     *
     * @param document The document in which the command was invoked.
     * @param position The position at which the command was invoked.
     * @returns A definition location, or an array of definitions, or `null` if there is no definition.
     */
    provideDefinition(document: TextDocument, position: Position): ProviderResult<Definition>
}

/**
 * Additional information and parameters for a references request.
 */
export interface ReferenceContext {
    /** Include the declaration of the current symbol. */
    includeDeclaration: boolean
}

/**
 * The reference provider interface defines the contract between extensions and
 * the [find references](https://code.visualstudio.com/docs/editor/editingevolved#_peek)-feature.
 */
export interface ReferenceProvider {
    /**
     * Provides a set of workspace-wide references for the given position in a document.
     *
     * @param document The document in which the command was invoked.
     * @param position The position at which the command was invoked.
     * @param context Additional information and parameters for the request.
     * @returns An array of reference locations.
     */
    provideReferences(
        document: TextDocument,
        position: Position,
        context: ReferenceContext
    ): ProviderResult<Badged<Location>[]>
}

/**
 * A location provider implements features such as "find implementations" and "find type definition". It is the
 * general form of {@link DefinitionProvider} and {@link ReferenceProvider}.
 */
export interface LocationProvider {
    /**
     * Provide related locations for the symbol at the given position and document.
     *
     * @param document The document in which the command was invoked.
     * @param position The position at which the command was invoked.
     * @returns Related locations, or `null` if there are none.
     */
    provideLocations(document: TextDocument, position: Position): ProviderResult<Location[]>
}

/**
 * A document highlight is a range inside a text document which deserves special attention.
 * Usually a document highlight is visualized by changing the background color of its range.
 */
export interface DocumentHighlight {
    /**
     * The range this highlight applies to.
     */
    range: Range

    /**
     * The highlight kind, default is text.
     */
    kind?: DocumentHighlightKind
}

/**
 * A document highlight kind.
 */
export enum DocumentHighlightKind {
    Text = 'text',
    Read = 'read',
    Write = 'write',
}

/**
 * A document highlight provider provides ranges to highlight in the current document like all
 * occurrences of a variable or all exit-points of a function.
 *
 * Providers are queried for document highlights on symbol hovers in any document matching
 * the document selector specified at registration time.
 */
export interface DocumentHighlightProvider {
    /**
     * Provide document highlights for the given position and document.
     *
     * @param document The document in which the command was invoked.
     * @param position The position at which the command was invoked.
     *
     * @returns An array of document highlights, or a thenable that resolves to document highlights.
     * The lack of a result can be signaled by returning `undefined`, `null`, or an empty array.
     */
    provideDocumentHighlights(document: TextDocument, position: Position): ProviderResult<DocumentHighlight[]>
}

export interface Position {
    isEqual(position: Position): boolean
    readonly line: number
    readonly character: number
}

export interface Range {
    readonly start: Position
    readonly end: Position
    contains(position: Position | Range): boolean
}

// NOTE(2022-09-08) We store global state at the module level because that was
// the easiest way to inline sourcegraph/code-intel-extensions into the main
// repository. The old extension code imported from the npm package
// 'sourcegraph' and it would have required a large refactoring to pass around
// the state for all methods. It would be nice to refactor the code one day to
// avoid storing state at the module level, but we had to deprecate extensions
// on a tight deadline so we decided not to do this refactoring during the
// initial migration.
let context: CodeIntelContext | undefined

export function requestGraphQL<T>(query: string, vars?: { [name: string]: unknown }): Promise<GraphQLResult<T>> {
    if (!context) {
        return Promise.reject(
            new Error(
                'code-intel: requestGraphQL not available. To fix this problem, call `updateCodeIntelContext` before invoking code-intel APIs.'
            )
        )
    }
    return lastValueFrom(
        context.requestGraphQL<T, any>({ request: query, variables: vars as any, mightContainPrivateInfo: true })
    )
}

export function getSetting<T>(key: string): T | undefined {
    if (context?.settings) {
        return context.settings(key)
    }
    return undefined
}

export function updateCodeIntelContext(newContext: CodeIntelContext): void {
    context = newContext
}

export interface CodeIntelContext extends Pick<PlatformContext, 'requestGraphQL' | 'telemetryService'> {
    settings: SettingsGetter
}

export type SettingsGetter = <T>(setting: string) => T | undefined

export function newSettingsGetter(settingsCascade: SettingsCascade<Settings>): SettingsGetter {
    return <T>(setting: string): T | undefined =>
        settingsCascade.final && (settingsCascade.final[setting] as T | undefined)
}

export interface ExtensionContext {
    /**
     * An object that maintains subscriptions to resources that should be freed when the extension is
     * deactivated.
     *
     * When an extension is deactivated, first its exported `deactivate` function is called (if one exists).
     * The `deactivate` function may be async, in which case deactivation blocks on it finishing. Next,
     * regardless of whether the `deactivate` function finished successfully or rejected with an error, all
     * unsubscribables passed to {@link ExtensionContext#subscriptions#add} are unsubscribed from.
     *
     * (An extension is deactivated when the user disables it, or after an arbitrary time period if its
     * activationEvents no longer evaluate to true.)
     */
    subscriptions: {
        /**
         * Mark a resource's teardown function to be called when the extension is deactivated.
         *
         * @param unsubscribable An {@link Unsubscribable} that frees (unsubscribes from) a resource, or a
         * plain function that does the same. Async functions are not supported. (If deactivation requires
         * async operations, make the `deactivate` function async; that is supported.)
         */
        add: (unsubscribable: Unsubscribable | (() => void)) => void
    }
}

export function logTelemetryEvent(
    eventName: string,
    eventProperties: { durationMs: number; languageId: string; repositoryId: number }
): void {
    context?.telemetryService?.log(eventName, eventProperties)
}
