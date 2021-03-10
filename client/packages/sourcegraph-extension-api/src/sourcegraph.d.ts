/**
 * ## The Sourcegraph extension API
 *
 * Sourcegraph extensions enhance your code host, code reviews, and Sourcegraph itself by adding features such as:
 * - Code intelligence (go-to-definition, find references, hovers, etc.)
 * - Test coverage overlays
 * - Links to live traces, log output, and performance data for a line of code
 * - Git blame
 * - Usage examples for functions
 *
 * Check out the [extension authoring documentation](https://docs.sourcegraph.com/extensions/authoring) to get started.
 */
declare module 'sourcegraph' {
    export interface Unsubscribable {
        unsubscribe(): void
    }

    /**
     * @deprecated Use the global [native `URL` API](https://developer.mozilla.org/en-US/docs/Web/API/URL)
     */
    export const URI: typeof URL
    /**
     * @deprecated Use the global [native `URL` API](https://developer.mozilla.org/en-US/docs/Web/API/URL)
     */
    export type URI = URL

    export class Position {
        /** Zero-based line number. */
        readonly line: number

        /** Zero-based character on the line. */
        readonly character: number

        /**
         * Constructs a Position from a line and character.
         *
         * @param line A zero-based line value.
         * @param character A zero-based character value.
         */
        constructor(line: number, character: number)

        /**
         * Check if this position is before `other`.
         *
         * @param other A position.
         * @returns `true` if position is on a smaller line
         * or on the same line on a smaller character.
         */
        isBefore(other: Position): boolean

        /**
         * Check if this position is before or equal to `other`.
         *
         * @param other A position.
         * @returns `true` if position is on a smaller line
         * or on the same line on a smaller or equal character.
         */
        isBeforeOrEqual(other: Position): boolean

        /**
         * Check if this position is after `other`.
         *
         * @param other A position.
         * @returns `true` if position is on a greater line
         * or on the same line on a greater character.
         */
        isAfter(other: Position): boolean

        /**
         * Check if this position is after or equal to `other`.
         *
         * @param other A position.
         * @returns `true` if position is on a greater line
         * or on the same line on a greater or equal character.
         */
        isAfterOrEqual(other: Position): boolean

        /**
         * Check if this position is equal to `other`.
         *
         * @param other A position.
         * @returns `true` if the line and character of the given position are equal to
         * the line and character of this position.
         */
        isEqual(other: Position): boolean

        /**
         * Compare this to `other`.
         *
         * @param other A position.
         * @returns A number smaller than zero if this position is before the given position,
         * a number greater than zero if this position is after the given position, or zero when
         * this and the given position are equal.
         */
        compareTo(other: Position): number

        /**
         * Create a new position relative to this position.
         *
         * @param lineDelta Delta value for the line value, default is `0`.
         * @param characterDelta Delta value for the character value, default is `0`.
         * @returns A position which line and character is the sum of the current line and
         * character and the corresponding deltas.
         */
        translate(lineDelta?: number, characterDelta?: number): Position

        /**
         * Derived a new position relative to this position.
         *
         * @param change An object that describes a delta to this position.
         * @returns A position that reflects the given delta. Will return `this` position if the change
         * is not changing anything.
         */
        translate(change: { lineDelta?: number; characterDelta?: number }): Position

        /**
         * Create a new position derived from this position.
         *
         * @param line Value that should be used as line value, default is the [existing value](#Position.line)
         * @param character Value that should be used as character value, default is the [existing value](#Position.character)
         * @returns A position where line and character are replaced by the given values.
         */
        with(line?: number, character?: number): Position

        /**
         * Derived a new position from this position.
         *
         * @param change An object that describes a change to this position.
         * @returns A position that reflects the given change. Will return `this` position if the change
         * is not changing anything.
         */
        with(change: { line?: number; character?: number }): Position
    }

    /**
     * A range represents an ordered pair of two positions.
     * It is guaranteed that [start](#Range.start).isBeforeOrEqual([end](#Range.end))
     *
     * Range objects are __immutable__. Use the [with](#Range.with),
     * [intersection](#Range.intersection), or [union](#Range.union) methods
     * to derive new ranges from an existing range.
     */
    export class Range {
        /**
         * The start position. It is before or equal to [end](#Range.end).
         */
        readonly start: Position

        /**
         * The end position. It is after or equal to [start](#Range.start).
         */
        readonly end: Position

        /**
         * Create a new range from two positions. If `start` is not
         * before or equal to `end`, the values will be swapped.
         *
         * @param start A position.
         * @param end A position.
         */
        constructor(start: Position, end: Position)

        /**
         * Create a new range from number coordinates. It is a shorter equivalent of
         * using `new Range(new Position(startLine, startCharacter), new Position(endLine, endCharacter))`
         *
         * @param startLine A zero-based line value.
         * @param startCharacter A zero-based character value.
         * @param endLine A zero-based line value.
         * @param endCharacter A zero-based character value.
         */
        constructor(startLine: number, startCharacter: number, endLine: number, endCharacter: number)

        /**
         * `true` if `start` and `end` are equal.
         */
        isEmpty: boolean

        /**
         * `true` if `start.line` and `end.line` are equal.
         */
        isSingleLine: boolean

        /**
         * Check if a position or a range is contained in this range.
         *
         * @param positionOrRange A position or a range.
         * @returns `true` if the position or range is inside or equal
         * to this range.
         */
        contains(positionOrRange: Position | Range): boolean

        /**
         * Check if `other` equals this range.
         *
         * @param other A range.
         * @returns `true` when start and end are [equal](#Position.isEqual) to
         * start and end of this range.
         */
        isEqual(other: Range): boolean

        /**
         * Intersect `range` with this range and returns a new range or `undefined`
         * if the ranges have no overlap.
         *
         * @param range A range.
         * @returns A range of the greater start and smaller end positions. Will
         * return undefined when there is no overlap.
         */
        intersection(range: Range): Range | undefined

        /**
         * Compute the union of `other` with this range.
         *
         * @param other A range.
         * @returns A range of smaller start position and the greater end position.
         */
        union(other: Range): Range

        /**
         * Derived a new range from this range.
         *
         * @param start A position that should be used as start. The default value is the [current start](#Range.start).
         * @param end A position that should be used as end. The default value is the [current end](#Range.end).
         * @returns A range derived from this range with the given start and end position.
         * If start and end are not different `this` range will be returned.
         */
        with(start?: Position, end?: Position): Range

        /**
         * Derived a new range from this range.
         *
         * @param change An object that describes a change to this range.
         * @returns A range that reflects the given change. Will return `this` range if the change
         * is not changing anything.
         */
        with(change: { start?: Position; end?: Position }): Range
    }

    /**
     * Represents a text selection in an editor.
     */
    export class Selection extends Range {
        /**
         * The position at which the selection starts.
         * This position might be before or after [active](#Selection.active).
         */
        anchor: Position

        /**
         * The position of the cursor.
         * This position might be before or after [anchor](#Selection.anchor).
         */
        active: Position

        /**
         * Create a selection from two positions.
         *
         * @param anchor A position.
         * @param active A position.
         */
        constructor(anchor: Position, active: Position)

        /**
         * Create a selection from four coordinates.
         *
         * @param anchorLine A zero-based line value.
         * @param anchorCharacter A zero-based character value.
         * @param activeLine A zero-based line value.
         * @param activeCharacter A zero-based character value.
         */
        constructor(anchorLine: number, anchorCharacter: number, activeLine: number, activeCharacter: number)

        /**
         * A selection is reversed if [active](#Selection.active).isBefore([anchor](#Selection.anchor)).
         */
        isReversed: boolean
    }

    /**
     * Represents a location inside a resource, such as a line
     * inside a text file.
     */
    export class Location {
        /**
         * The resource identifier of this location.
         */
        readonly uri: URL

        /**
         * The document range of this location.
         */
        readonly range?: Range

        /**
         * Creates a new location object.
         *
         * @param uri The resource identifier.
         * @param rangeOrPosition The range or position. Positions will be converted to an empty range.
         */
        constructor(uri: URL, rangeOrPosition?: Range | Position)
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
         * extension](https://docs.sourcegraph.com/integration/browser_extension), the value is
         * `undefined` because determining the text contents (in general) is not possible without
         * additional access to the code host API. In the future, this limitation may be removed.
         */
        readonly text: string | undefined

        /**
         * Convert the position to a zero-based offset.
         *
         * The position will be adjusted using {@link TextDocument#validatePosition}.
         *
         * @param position A position.
         * @returns A valid zero-based offset.
         * @throws if {@link TextDocument#text} is undefined.
         */
        offsetAt(position: Position): number

        /**
         * Convert a zero-based offset to a position.
         *
         * @param offset A zero-based offset.
         * @returns A valid {@link Position}.
         * @throws if {@link TextDocument#text} is undefined.
         */
        positionAt(offset: number): Position

        /**
         * Ensure a position is contained in the range of this document. If not, adjust it so that
         * it is.
         *
         * @param position A position.
         * @returns The given position or a new, adjusted position.
         * @throws if {@link TextDocument#text} is undefined.
         */
        validatePosition(position: Position): Position

        /**
         * Ensure a range is completely contained in this document.
         *
         * @param range A range.
         * @returns The given range or a new, adjusted range.
         * @throws if {@link TextDocument#text} is undefined.
         */
        validateRange(range: Range): Range

        /**
         * Get the range of the word at the given position.
         *
         * The position will be adjusted using {@link TextDocument#validatePosition}.
         *
         * @param position A position.
         * @returns A range spanning a word, or `undefined`.
         */
        getWordRangeAtPosition(position: Position): Range | undefined

        /**
         * Get the substring of text content from the provided range.
         *
         * @param range A range
         * @returns The text inside the provided range, or the whole text if no range is provided
         */
        getText(range?: Range): string | undefined
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

    /**
     * Options for an input box displayed as a result of calling {@link Window#showInputBox}.
     */
    export interface InputBoxOptions {
        /**
         * The text that describes what input the user should provide.
         */
        prompt?: string

        /**
         * The pre-filled input value for the input box.
         */
        value?: string
    }

    export interface ProgressOptions {
        title?: string
    }

    export interface Progress {
        /** Optional message. If not set, the previous message is still shown. */
        message?: string

        /** Integer from 0 to 100. If not set, the previous percentage is still shown. */
        percentage?: number
    }

    export interface ProgressReporter {
        /**
         * Updates the progress display with a new message and/or percentage.
         */
        next(status: Progress): void

        /**
         * Turns the progress display into an error display for the given error or message.
         * Use if the operation failed.
         * No further progress updates can be sent after this.
         */
        error(error: any): void

        /**
         * Completes the progress bar and hides the display.
         * Sending a percentage of 100 has the same effect.
         * No further progress updates can be sent after this.
         */
        complete(): void
    }

    /**
     * A window in the client application that is running the extension.
     */
    export interface Window {
        /**
         * The user interface view components that are visible in the window.
         */
        visibleViewComponents: ViewComponent[]

        /**
         * The currently active view component in the window.
         */
        activeViewComponent: ViewComponent | undefined

        /**
         * An event that is fired when the active view component changes.
         */
        activeViewComponentChanges: Subscribable<ViewComponent | undefined>

        /**
         * Show a notification message to the user that does not require interaction or steal focus.
         *
         * @param message The message to show. Markdown is supported.
         * @param type a {@link NotificationType} affecting the display of the notification.
         */
        showNotification(message: string, type: NotificationType): void

        /**
         * Show progress in the window. Progress is shown while running the given callback
         * and while the promise it returned isn't resolved nor rejected.
         *
         * @param task A callback returning a promise. Progress state can be reported with
         * the provided [ProgressReporter](#ProgressReporter)-object.
         *
         * @returns The Promise the task-callback returned.
         */
        withProgress<R>(options: ProgressOptions, task: (reporter: ProgressReporter) => Promise<R>): Promise<R>

        /**
         * Show progress in the window. The returned ProgressReporter can be used to update the
         * progress bar, complete it or turn the notification into an error notification in case the operation failed.
         *
         * @returns A ProgressReporter that allows updating the progress display.
         */
        showProgress(options: ProgressOptions): Promise<ProgressReporter>

        /**
         * Show a modal message to the user that the user must dismiss before continuing.
         *
         * @param message The message to show.
         * @returns A promise that resolves when the user dismisses the message.
         */
        showMessage(message: string): Promise<void>

        /**
         * Displays an input box to ask the user for input.
         *
         * The returned value will be `undefined` if the input box was canceled (e.g., because the user pressed the
         * ESC key). Otherwise the returned value will be the string provided by the user.
         *
         * @param options Configures the behavior of the input box.
         * @returns The string provided by the user, or `undefined` if the input box was canceled.
         */
        showInputBox(options?: InputBoxOptions): Promise<string | undefined>
    }

    /**
     * A user interface component in an application window.
     *
     * Each {@link ViewComponent} has a distinct {@link ViewComponent#type} value that indicates what kind of
     * component it is ({@link CodeEditor}, etc.).
     */
    export type ViewComponent = CodeEditor | DirectoryViewer

    /**
     * A style for a {@link TextDocumentDecoration}.
     */
    export interface ThemableDecorationStyle {
        /** The CSS background-color property value for the line. */
        backgroundColor?: string

        /** The CSS border property value for the line. */
        border?: string

        /** The CSS border-color property value for the line. */
        borderColor?: string

        /** The CSS border-width property value for the line. */
        borderWidth?: string
    }

    /**
     * A text document decoration changes the appearance of a range in the document and/or adds other content to
     * it.
     */
    export interface TextDocumentDecoration extends ThemableDecorationStyle {
        /**
         * The range that the decoration applies to. Currently, decorations are
         * only applied only on the start line, and the entire line. Multiline
         * and intra-line ranges are not supported.
         */
        range: Range

        /**
         * If true, the decoration applies to all lines in the range (inclusive), even if not all characters on the
         * line are included.
         */
        isWholeLine?: boolean

        /** Content to display after the range. */
        after?: DecorationAttachmentRenderOptions

        /** Overwrite style for light themes. */
        light?: ThemableDecorationStyle

        /** Overwrite style for dark themes. */
        dark?: ThemableDecorationStyle
    }

    /**
     * A style for {@link DecorationAttachmentRenderOptions}.
     */
    export interface ThemableDecorationAttachmentStyle {
        /** The CSS background-color property value for the attachment. */
        backgroundColor?: string

        /** The CSS color property value for the attachment. */
        color?: string
    }

    /** A decoration attachment adds content after a {@link TextDocumentDecoration}. */
    export interface DecorationAttachmentRenderOptions extends ThemableDecorationAttachmentStyle {
        /** Text to display in the attachment. */
        contentText?: string

        /** Tooltip text to display when hovering over the attachment. */
        hoverMessage?: string

        /** If set, the attachment becomes a link with this destination URL. */
        linkURL?: string

        /** Overwrite style for light themes. */
        light?: ThemableDecorationAttachmentStyle

        /** Overwrite style for dark themes. */
        dark?: ThemableDecorationAttachmentStyle
    }

    /**
     * Represents a handle to a set of decorations.
     *
     * To get an instance of {@link TextDocumentDecorationType}, use
     * {@link sourcegraph.app.createDecorationType}
     */
    export interface TextDocumentDecorationType {
        /** An opaque identifier. */
        readonly key: string
    }

    export interface Directory {
        /**
         * The URI of the directory.
         *
         * @todo The format of this URI will be changed in the future. It must not be relied on.
         */
        readonly uri: URL
    }

    /**
     * A viewer for directories.
     *
     * This API is experimental and subject to change.
     */
    export interface DirectoryViewer {
        readonly type: 'DirectoryViewer'

        /**
         * The directory shown in the directory viewer.
         * This currently only exposes the URI of the directory.
         */
        readonly directory: Directory
    }

    /**
     * A text editor for code files (as opposed to a rich text editor for documents or other kinds of file format
     * editors).
     */
    export interface CodeEditor {
        /** The type tag for this kind of {@link ViewComponent}. */
        readonly type: 'CodeEditor' /** The internal ID of this code editor. */

        /**
         * The text document that is open in this editor. The document remains the same for the entire lifetime of
         * this editor.
         */
        readonly document: TextDocument

        /**
         * The primary selection in this text editor. This is equivalent to `CodeEditor.selections[0] || null`.
         *
         * @todo Make this non-readonly.
         */
        readonly selection: Selection | null

        /**
         * The selections in this text editor. A text editor has zero or more selections. The primary selection
         * ({@link CodeEditor#selection}), if any selections exist, is always at index 0.
         *
         * @todo Make this non-readonly.
         */
        readonly selections: Selection[]

        /**
         * An event that is fired when the selections in this text editor change.
         * The primary selection ({@link CodeEditor#selection}), if any selections exist,
         * is always at index 0 of the emitted array.
         */
        readonly selectionsChanges: Subscribable<Selection[]>

        /**
         * Add a set of decorations to this editor. If a set of decorations already exists with the given
         * {@link TextDocumentDecorationType}, they will be replaced.
         *
         * @see {@link TextDocumentDecorationType}
         * @see {@link sourcegraph.app.createDecorationType}
         *
         */
        setDecorations(decorationType: TextDocumentDecorationType, decorations: TextDocumentDecoration[]): void
    }

    /**
     * A panel view created by {@link sourcegraph.app.createPanelView}.
     */
    export interface PanelView extends Unsubscribable {
        /**
         * The title of the panel view.
         */
        title: string

        /**
         * The content to show in the panel view. Markdown is supported.
         */
        content: string

        /**
         * The priority of this panel view. A higher value means that the item is shown near the beginning (usually
         * the left side).
         */
        priority: number

        /**
         * Display the results of the location provider (with the given ID) in this panel below the
         * {@link PanelView#contents}.
         *
         * Experimental. Subject to change or removal without notice.
         *
         * @internal
         */
        component: { locationProvider: string } | null
    }

    export type ChartContent = LineChartContent<any, string> | BarChartContent<any, string> | PieChartContent<any>

    export interface ChartAxis<K extends keyof D, D extends object> {
        /** The key in the data object. */
        dataKey: K

        /** The scale of the axis. */
        scale?: 'time' | 'linear'

        /** The type of the data key. */
        type: 'number' | 'category'
    }

    export interface LineChartContent<D extends object, XK extends keyof D> {
        chart: 'line'

        /** An array of data objects, with one element for each step on the X axis. */
        data: D[]

        /** The series (lines) of the chart. */
        series: {
            /** The key in each data object for the values this line should be calculated from. */
            dataKey: keyof D

            /** The name of the line shown in the legend and tooltip. */
            name?: string

            /**
             * The link URLs for each data point.
             * A link URL should take the user to more details about the specific data point.
             */
            linkURLs?: string[]

            /** The CSS color of the line. */
            stroke?: string
        }[]

        xAxis: ChartAxis<XK, D>
    }

    export interface BarChartContent<D extends object, XK extends keyof D> {
        chart: 'bar'

        /** An array of data objects, with one element for each step on the X axis. */
        data: D[]

        /** The series of the chart. */
        series: {
            /** The key in each data object for the values this bar should be calculated from. */
            dataKey: keyof D

            /**
             * An optional stack id of each bar.
             * When two bars have the same same `stackId`, the two bars are stacked in order.
             */
            stackId?: string

            /** The name of the series, shown in the legend. */
            name?: string

            /**
             * The link URLs for each bar.
             * A link URL should take the user to more details about the specific data point.
             */
            linkURLs?: string[]

            /** The CSS fill color of the line. */
            fill?: string
        }[]

        xAxis: ChartAxis<XK, D>
    }

    export interface PieChartContent<D extends object> {
        chart: 'pie'

        pies: {
            /** The key of each sector's va lue. */
            dataKey: keyof D

            /** The key of each sector's name. */
            nameKey: keyof D

            /** The key of each sector's fill color. */
            fillKey?: keyof D

            /** An array of data objects, with one element for each pie sector. */
            data: D[]

            /** T he key of each sector's link URL. */
            linkURLKey?: keyof D
        }[]
    }

    /**
     * A view is a page or partial page.
     */
    export interface View {
        /** The title of the view. */
        title: string

        /** An optional subtitle displayed under the title. */
        subtitle?: string

        /**
         * The content sections of the view. The sections are rendered in order.
         *
         * Support for non-MarkupContent elements is experimental and subject to change or removal
         * without notice.
         */
        content: (
            | MarkupContent
            | ChartContent
            | { component: string; props: { [name: string]: string | number | boolean | null | undefined } }
        )[]
    }

    /**
     * A view provider registered with {@link sourcegraph.app.registerViewProvider}.
     */
    export type ViewProvider =
        | InsightsPageViewProvider
        | HomepageViewProvider
        | GlobalPageViewProvider
        | DirectoryViewProvider

    /**
     * Experimental view provider shown on the dashboard on the insights page.
     * This API is experimental and is subject to change or removal without notice.
     */
    export interface InsightsPageViewProvider {
        readonly where: 'insightsPage'

        /**
         * Provide content for the view.
         */
        provideView(context: {}): ProviderResult<View>
    }

    /**
     * Experimental view provider shown on the homepage (below the search box in the Sourcegraph web app).
     * This API is experimental and is subject to change or removal without notice.
     */
    export interface HomepageViewProvider {
        readonly where: 'homepage'

        /**
         * Provide content for the view.
         */
        provideView(context: {}): ProviderResult<View>
    }

    /**
     * Experimental global view provider. Global view providers are shown on a dedicated page in the app.
     * This API is experimental and is subject to change or removal without notice.
     */
    export interface GlobalPageViewProvider {
        readonly where: 'global/page'

        /**
         * Provide content for the view.
         *
         * @param params Parameters from the page (such as URL query parameters). The schema of these parameters is
         * experimental and subject to change without notice.
         * @returns The view content.
         */
        provideView(context: { [param: string]: string }): ProviderResult<View>
    }

    /**
     * Context passed to directory view providers.
     *
     * The schema of these parameters is experimental and subject to change without notice.
     */
    export interface DirectoryViewContext {
        /** The directory viewer displaying the view. */
        viewer: DirectoryViewer

        /** The workspace of the directory. */
        workspace: WorkspaceRoot
    }

    /**
     * Experimental view provider for directory pages.
     * This API is experimental and is subject to change or removal without notice.
     */
    export interface DirectoryViewProvider {
        readonly where: 'directory'

        /**
         * Provide content for a view.
         *
         * @param context The context of the directory. The schema of these parameters is experimental and subject to
         * change without notice.
         * @returns The view content.
         */
        provideView(context: DirectoryViewContext): ProviderResult<View>
    }

    export interface ThemableFileDecorationStyle {
        /** The CSS color property value for the text contet */
        color?: string

        /** Overwrite style for when the file is active */
        activeColor?: string
    }

    /** A decoration attachment adds content after a {@link FileDecoration}. */
    export interface FileDecorationAttachmentRenderOptions extends ThemableFileDecorationStyle {
        /** Text value to be displayed. This value should be very short to prevent truncation */
        contentText: string

        /** Tooltip text to display when hovering over the text content. */
        hoverMessage?: string

        /** Overwrite style for light themes. */
        light?: ThemableFileDecorationStyle

        /** Overwrite color for dark themes. */
        dark?: ThemableFileDecorationStyle
    }

    /**
     * A file decoration adds text content and/or a progress bar to files in a tree view
     */
    export interface FileDecoration {
        /** The resource identifier of this file */
        uri: string

        /** Whether to display the decoration on the sidebar file tree or tree page. If omitted, it will be displayed in both locations  */
        where?: 'sidebar' | 'page'

        /** An optional object that describes the text content contributed by the decoration */
        after?: FileDecorationAttachmentRenderOptions

        /**
         * Describes a meter bar like the [HTML5 `<meter>`
         * element](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/meter)
         * to be rendered after the file/directory name.
         */
        meter?: {
            /**
             * The current numeric value. This must be between the minimum and maximum values
             * (min attribute and max attribute) if they are specified.
             */
            value: number

            /**
             * The lower numeric bound of the measured range. This must be less than
             * the maximum value (max attribute), if specified. If unspecified, the
             * minimum value is 0.
             */
            min?: number

            /**
             * The upper numeric bound of the measured range. This must be greater
             * than the minimum value (min attribute), if specified. If unspecified,
             * the maximum value is 1.
             */
            max?: number

            /**
             * The upper numeric bound of the low end of the measured range. This
             * must be greater than the minimum value (min attribute), and it also
             * must be less than the high value and maximum value (high attribute
             * and max attribute, respectively), if any are specified. If
             * unspecified, or if less than the minimum value, the low value is
             * equal to the minimum value.
             */
            low?: number

            /**
             * The lower numeric bound of the high end of the measured range. This
             * must be less than the maximum value (max attribute), and it also must
             * be greater than the low value and minimum value (low attribute and
             * min attribute, respectively), if any are specified. If unspecified,
             * or if greater than the maximum value, the high value is equal to the
             * maximum value.
             */
            high?: number

            /**
             * This attribute indicates the optimal numeric value. It must be within
             * the range (as defined by the min attribute and max attribute). When
             * used with the low attribute and high attribute, it gives an
             * indication where along the range is considered preferable. For
             * example, if it is between the min attribute and the low attribute,
             * then the lower range is considered preferred. The browser may color
             * the meter's bar differently depending on whether the value is less
             * than or equal to the optimum value.
             */
            optimum?: number

            /** Tooltip text to display when hovering over the progress bar. */
            hoverMessage?: string
        }
    }

    /**
     * Context passed to file decoration providers.
     *
     * The schema of these parameters is experimental and subject to change without notice.
     */
    export interface FileDecorationContext {
        /** The uri of the file's parent */
        uri: string

        files: {
            /** The uri of the file */
            uri: string

            /** Whether this file is a directory */
            isDirectory: boolean

            /**
             * File path relative to repo root uri
             *
             * @todo Remove this once `parseRepoUri` is public
             * */
            path: string
        }[]
    }

    export interface FileDecorationProvider {
        provideFileDecorations: (fileDecorationContext: FileDecorationContext) => ProviderResult<FileDecoration[]>
    }

    /**
     * The client application that is running the extension.
     */
    export namespace app {
        /**
         * The currently active window, or `undefined`. The active window is the window that has focus, or when
         * none has focus, the window that was most recently focused.
         */
        export const activeWindow: Window | undefined

        /**
         * An event that is fired when the currently active window changes.
         */
        export const activeWindowChanges: Subscribable<Window | undefined>

        /**
         * All application windows that are accessible by the extension.
         *
         * @readonly
         */
        export const windows: Window[]

        /**
         * Create a panel view for the view contribution with the given {@link id}.
         *
         * @todo Consider requiring extensions to specify these statically in package.json's contributions section
         * to improve the activation experience.
         *
         * @param id The ID of the view. This may be shown to the user (e.g., in the URL fragment when the panel is
         * active).
         * @returns The panel view.
         */
        export function createPanelView(id: string): PanelView

        /**
         * Creates a decorationType that can be used to add decorations to code views.
         *
         * Use this to create a unique handle to a set of decorations, that can be applied to
         * text editors using {@link setDecorations}.
         */
        export function createDecorationType(): TextDocumentDecorationType

        /**
         * Register a view provider, which provides the contents of a view.
         *
         * This API is experimental and is subject to change or removal without notice.
         *
         * @param id The ID of the view.
         * @param provider A view provider.
         * @returns An unsubscribable to unregister this provider.
         */
        export function registerViewProvider(id: string, provider: ViewProvider): Unsubscribable

        /**
         * Register a file decoration provider
         */
        export function registerFileDecorationProvider(provider: FileDecorationProvider): Unsubscribable
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
     * The logical workspace that the extension is running in, which may consist of multiple folders, projects, and
     * repositories.
     */
    export namespace workspace {
        /**
         * All text documents currently known to the system.
         *
         * @readonly
         */
        export const textDocuments: TextDocument[]

        /**
         * An event that is fired when a new text document is opened.
         *
         * @deprecated Renamed to {@link workspace.openedTextDocuments}.
         */
        export const onDidOpenTextDocument: Subscribable<TextDocument>

        /**
         * An event that is fired when a new text document is opened.
         */
        export const openedTextDocuments: Subscribable<TextDocument>

        /**
         * The root directories of the workspace, if any.
         *
         * @example The repository that is currently being viewed is a root.
         * @todo Currently only a single root is supported.
         * @readonly
         */
        export const roots: readonly WorkspaceRoot[]

        /**
         * An event that is fired when a workspace root is added or removed from the workspace.
         *
         * @deprecated Renamed to {@link workspace.rootsChanges}.
         */
        export const onDidChangeRoots: Subscribable<void>

        /**
         * An event that is fired when a workspace root is added or removed from the workspace.
         */
        export const rootChanges: Subscribable<void>

        /**
         * The current version context of the workspace, if any.
         *
         * A version context is a set of repositories and revisions on a Sourcegraph instance.
         * When set, extensions use it to scope search queries, code intelligence actions, etc.
         *
         * See more information at http://docs.sourcegraph.com/user/search#version-contexts.
         */
        export const versionContext: string | undefined

        /**
         * An event that is fired when a workspace's version context changes.
         */
        export const versionContextChanges: Subscribable<string | undefined>
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

    interface ConfigurationService extends Subscribable<void> {
        /**
         * Returns the full configuration object.
         *
         * @template C The configuration schema.
         * @returns The full configuration object.
         */
        get<C extends object = { [key: string]: any }>(): Configuration<C>
    }

    /**
     * The configuration settings.
     *
     * It may be merged from the following sources of settings, in order:
     *
     * Default settings
     * Global settings
     * Organization settings (for all organizations the user is a member of)
     * User settings
     * Repository settings
     * Directory settings
     *
     * @todo Add a way to get/update configuration for a specific scope or subject.
     * @todo Support applying defaults to the configuration values.
     */
    export const configuration: ConfigurationService

    /**
     * A provider result represents the values that a provider, such as the {@link HoverProvider}, may return. The
     * result may be a single value, a Promise that resolves to a single value, a Subscribable that emits zero
     * or more values, or an AsyncIterable that yields zero or more values.
     */
    export type ProviderResult<T> =
        | T
        | undefined
        | null
        | Promise<T | undefined | null>
        | Subscribable<T | undefined | null>
        | AsyncIterable<T | undefined | null>

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

    /**
     * The type of a notification shown through {@link Window.showNotification}.
     */
    export enum NotificationType {
        /**
         * An error message.
         */
        Error = 1,
        /**
         * A warning message.
         */
        Warning = 2,
        /**
         * An info message.
         */
        Info = 3,
        /**
         * A log message.
         */
        Log = 4,
        /**
         * A success message.
         */
        Success = 5,
    }

    /**
     * A style for {@link BadgeAttachmentRenderOptions}.
     */
    export interface ThemableBadgeAttachmentStyle {
        /**
         * The icon (a base64-encoded image icon) to display next to the wrapped value.
         *
         * @deprecated Use {@link BadgeAttachmentRenderOptions#kind} to pick a predefined icon
         */
        icon?: string

        /**
         * The CSS background-color property value for the attachment.
         *
         * @deprecated Use {@link BadgeAttachmentRenderOptions#kind} to pick a predefined icon
         */
        backgroundColor?: string

        /**
         * The CSS color property value for the attachment.
         *
         * @deprecated Use {@link BadgeAttachmentRenderOptions#kind} to pick a predefined icon
         */
        color?: string
    }

    /** An attachment adds content to a hover tooltip or result in a locations panel. */
    export interface BadgeAttachmentRenderOptions extends ThemableBadgeAttachmentStyle {
        /** Predefined icons for badge attachments */
        kind: 'info' | 'error' | 'warning'

        /** Tooltip text to display when hovering over the attachment. */
        hoverMessage?: string

        /** If set, the attachment becomes a link with this destination URL. */
        linkURL?: string

        /**
         * Overwrite style for light themes.
         *
         * @deprecated Use {@link BadgeAttachmentRenderOptions#kind} to pick a predefined icon
         */
        light?: ThemableBadgeAttachmentStyle

        /**
         * Overwrite style for dark themes.
         *
         * @deprecated Use {@link BadgeAttachmentRenderOptions#kind} to pick a predefined icon
         */
        dark?: ThemableBadgeAttachmentStyle
    }

    /**
     * A wrapper around a providable type (currently hover and locations) with additional
     * context to enable displaying badges next to the wrapped result value in the UI.
     */
    export type Badged<T extends object> = T & {
        badge?: BadgeAttachmentRenderOptions
    }

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

        /**
         * Alerts that should be shown in this hover.
         */
        alerts?: Badged<HoverAlert>[]
    }

    export interface HoverAlert {
        /**
         * Text content to be shown on hovers. Since the alert is displayed inline,
         * multiparagraph content will be rendered on one line. It's recommended to
         * provide a brief message here, and place futher details in the badge or
         * provide a link.
         */
        summary: MarkupContent

        /**
         * When an alert has a dismissal type, dismissing it will prevent all alerts
         * of that type from being shown. If no type is provided, the alert is not
         * dismissible.
         */
        type?: string
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
     * A completion item is a suggestion to complete text that the user has typed.
     *
     * @see {@link CompletionItemProvider#provideCompletionItems}
     *
     * @deprecated
     */
    export interface CompletionItem {
        /**
         * The label of this completion item, which is rendered prominently. If no
         * {@link CompletionItem#insertText} is specified, the label is the text inserted when the
         * user selects this completion.
         */
        label: string

        /**
         * The description of this completion item, which is rendered less prominently but still
         * alongside the {@link CompletionItem#label}.
         */
        description?: string

        /**
         * A string to insert in a document when the user selects this completion. When not set, the
         * {@link CompletionItem#label} is used.
         */
        insertText?: string
    }

    /**
     * A collection of [completion items](#CompletionItem) to be presented in the editor.
     *
     * @deprecated
     */
    export interface CompletionList {
        /**
         * The list of completions.
         */
        items: CompletionItem[]
    }

    /**
     * A completion item provider provides suggestions to insert or apply at the cursor as the user
     * is typing.
     *
     * Providers are queried for completions as the user types in any document matching the document
     * selector specified at registration time.
     *
     * @deprecated
     */
    export interface CompletionItemProvider {
        /**
         * Provide completion items for the given position and document.
         *
         * @param document The document in which the command was invoked.
         * @param position The position at which the command was invoked.
         *
         * @returns An array of completions, a [completion list](#CompletionList), or a thenable that resolves to either.
         * The lack of a result can be signaled by returning `undefined`, `null`, or an empty array.
         */
        provideCompletionItems(document: TextDocument, position: Position): ProviderResult<CompletionList>
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

    export namespace languages {
        /**
         * Registers a hover provider, which returns a formatted hover message (intended for display in a tooltip)
         * when the user hovers on code.
         *
         * Multiple providers can be registered for a language. In that case, providers are queried in parallel and
         * the results are merged. A failing provider (rejected promise or exception) will not cause the whole
         * operation to fail.
         *
         * @param selector A selector that defines the documents this provider is applicable to.
         * @param provider A hover provider.
         * @returns An unsubscribable to unregister this provider.
         */
        export function registerHoverProvider(selector: DocumentSelector, provider: HoverProvider): Unsubscribable

        /**
         * Registers a definition provider.
         *
         * Multiple providers can be registered for a language. In that case, providers are queried in parallel and
         * the results are merged. A failing provider (rejected promise or exception) will not cause the whole
         * operation to fail.
         *
         * @param selector A selector that defines the documents this provider is applicable to.
         * @param provider A definition provider.
         * @returns An unsubscribable to unregister this provider.
         */
        export function registerDefinitionProvider(
            selector: DocumentSelector,
            provider: DefinitionProvider
        ): Unsubscribable

        /**
         * Registers a reference provider.
         *
         * Multiple providers can be registered for a language. In that case, providers are queried in parallel and
         * the results are merged. A failing provider (rejected promise or exception) will not cause the whole
         * operation to fail.
         *
         * @param selector A selector that defines the documents this provider is applicable to.
         * @param provider A reference provider.
         * @returns An unsubscribable to unregister this provider.
         */
        export function registerReferenceProvider(
            selector: DocumentSelector,
            provider: ReferenceProvider
        ): Unsubscribable

        /**
         * Registers a generic provider of a list of locations. It is the general form of
         * {@link registerDefinitionProvider} and {@link registerReferenceProvider}. It is intended for "find
         * implementations", "find type definition", and other similar features.
         *
         * The provider can be executed with the `executeLocationProvider` builtin command, passing the {@link id}
         * as the first argument. For more information, see
         * https://docs.sourcegraph.com/extensions/authoring/builtin_commands#executeLocationProvider.
         *
         * @param id An identifier for this location provider that distinguishes it from other location providers.
         * @param selector A selector that defines the documents this provider is applicable to.
         * @param provider A location provider.
         * @returns An unsubscribable to unregister this provider.
         */
        export function registerLocationProvider(
            id: string,
            selector: DocumentSelector,
            provider: LocationProvider
        ): Unsubscribable

        /**
         * Registers a completion item provider.
         *
         * Multiple providers can be registered with overlapping document selectors. In that case,
         * providers are queried in parallel and the results are merged. A failing provider will not
         * cause the whole operation to fail.
         *
         * @param selector A selector that defines the documents this provider applies to.
         * @param provider A completion item provider.
         * @returns An unsubscribable to unregister this provider.
         *
         * @deprecated
         */
        export function registerCompletionItemProvider(
            selector: DocumentSelector,
            provider: CompletionItemProvider
        ): Unsubscribable

        /**
         * Registers a document highlight provider.
         *
         * Multiple providers can be registered with overlapping document selectors. In that case,
         * providers are queried in parallel and the results are merged. A failing provider will not
         * cause the whole operation to fail.
         *
         * @param selector A selector that defines the documents this provider applies to.
         * @param provider A document hover provider.
         * @returns An unsubscribable to unregister this provider.
         */
        export function registerDocumentHighlightProvider(
            selector: DocumentSelector,
            provider: DocumentHighlightProvider
        ): Unsubscribable
    }

    /**
     * A query transformer alters a user's search query before executing a search.
     *
     * Query transformers allow extensions to define new search query operators and syntax, for example,
     * by matching strings in a query (e.g. `go.imports:`) and replacing them with a regular expression or string.
     */
    export interface QueryTransformer {
        /**
         * Transforms a search query into another, valid query. If there are no transformations to be made
         * the original query is returned.
         *
         * @param query A search query.
         */
        transformQuery(query: string): string | Promise<string>
    }

    /**
     * API for extensions to augment search functionality.
     */
    export namespace search {
        /**
         * Registers a query transformer.
         *
         * Multiple transformers can be registered. In that case, all transformations will be applied
         * and the result is a single query that has been altered by all transformers. The order in
         * which transforms are applied is not defined.
         *
         * @param provider A query transformer.
         */
        export function registerQueryTransformer(provider: QueryTransformer): Unsubscribable
    }

    /**
     * Commands are functions that are implemented and registered by extensions. Extensions can invoke any command
     * (including commands registered by other extensions). The extension can also define contributions (in
     * package.json), such as actions and menu items, that invoke a command.
     */
    export namespace commands {
        /**
         * Registers a command that can be invoked by an action or menu item, or directly (with
         * {@link commands.executeCommand}).
         *
         * @param command A unique identifier for the command.
         * @param callback A command function. If it returns a {@link Promise}, execution waits until it is resolved.
         * @returns Unsubscribable to unregister this command.
         * @throws Registering a command with an existing command identifier throws an error.
         */
        export function registerCommand(command: string, callback: (...args: any[]) => any): Unsubscribable

        /**
         * Executes the command with the given command identifier.
         *
         * @template T The result type of the command.
         * @param command Identifier of the command to execute.
         * @param rest Parameters passed to the command function.
         * @returns A {@link Promise} that resolves to the result of the given command.
         * @throws If no command exists with the given command identifier, an error is thrown.
         */
        export function executeCommand<T = any>(command: string, ...args: any[]): Promise<T>
    }

    export namespace graphQL {
        /**
         * Executes a [Sourcegraph GraphQL API](https://docs.sourcegraph.com/api/graphql) query or mutation on the associated Sourcegraph instance and returns a promise for the result.
         *
         * @template TResult The GraphQL result type
         * @template TVariables The type of the variables object
         * @param request The GraphQL request (query or mutation)
         * @param variables An object whose properties are GraphQL query name-value variable pairs
         * @returns A Promise for the result of the GraphQL request
         */
        export function execute<TResult, TVariables extends object>(
            request: string,
            variables: TVariables
        ): Promise<GraphQLResult<TResult>>

        export type GraphQLResult<T> = SuccessGraphQLResult<T> | ErrorGraphQLResult

        export interface SuccessGraphQLResult<T> {
            data: T
            errors: undefined
        }
        export interface ErrorGraphQLResult {
            data: undefined
            errors: GraphQLError[]
        }

        /**
         * A spec-compliant member of the GraphQL `errors` array.
         */
        export interface GraphQLError {
            /**
             * Every error must contain an entry with the key message with a string description of the error intended for
             * the developer as a guide to understand and correct the error.
             */
            message: string

            /**
             * If an error can be associated to a particular point in the requested GraphQL document, it should contain an
             * entry with the key locations with a list of locations, where each location is a map with the keys line and
             * column, both positive numbers starting from 1 which describe the beginning of an associated syntax element.
             */
            locations?: {
                line: number
                column: number
            }[]

            /**
             * If an error can be associated to a particular field in the GraphQL result, it must contain an entry with the
             * key path that details the path of the response field which experienced the error. This allows clients to
             * identify whether a null result is intentional or caused by a runtime error.
             *
             * This field should be a list of path segments starting at the root of the response and ending with the field
             * associated with the error. Path segments that represent fields should be strings, and path segments that
             * represent list indices should be 0‐indexed integers. If the error happens in an aliased field, the path to
             * the error should use the aliased name, since it represents a path in the response, not in the query.
             */
            path?: (string | number)[]

            /**
             * GraphQL services may provide an additional entry to errors with key extensions. This entry, if set, must
             * have a map as its value. This entry is reserved for implementors to add additional information to errors
             * however they see fit, and there are no additional restrictions on its contents.
             */
            extensions?: Record<string, unknown>
        }
    }

    /**
     * A description of the information available at a URL.
     */
    export interface LinkPreview {
        /**
         * The content of this link preview, which is shown next to the link.
         */
        content?: MarkupContent

        /**
         * The hover content of this link preview, which is shown when the cursor hovers the link.
         *
         * @todo Add support for Markdown. Currently only plain text is supported.
         */
        hover?: Pick<MarkupContent, 'value'> & { kind: MarkupKind.PlainText }
    }

    /**
     * Called to obtain a preview of the information available at a URL.
     */
    export interface LinkPreviewProvider {
        /**
         * Provides a preview of the information available at the URL of a link in a document.
         *
         * @todo Add a `context` parameter so that the provider knows what document contains the
         * link (so that it can handle links in code files differently from rendered Markdown
         * documents, for example).
         *
         * @param url The URL of the link to preview.
         */
        provideLinkPreview(url: URL): ProviderResult<LinkPreview>
    }

    /**
     * Extensions can customize how content is rendered.
     */
    export namespace content {
        /**
         * EXPERIMENTAL. This API is subject to change without notice and has no compatibility
         * guarantees.
         *
         * Registers a provider for link previews ({@link LinkPreviewProvider}) for all URLs in a
         * document matching the {@link urlMatchPattern}. A link preview is a description of the
         * information available at a URL.
         *
         * @todo Support a more powerful syntax for URL match patterns, such as Chrome's
         * (https://developer.chrome.com/extensions/match_patterns).
         *
         * @param urlMatchPattern A pattern that matches URLs for which the provider is called to
         * obtain a preview. Currently it matches all URLs that start with the match pattern (i.e.,
         * string prefix matches). No wildcards are supported.
         * @param provider The link preview provider.
         */
        export function registerLinkPreviewProvider(
            urlMatchPattern: string,
            provider: LinkPreviewProvider
        ): Unsubscribable
    }

    export interface ContextValues {
        [key: string]: string | number | boolean | null
    }

    /**
     * Internal API for Sourcegraph extensions. Most of these will be removed for the beta release of Sourcegraph
     * extensions. They are necessary now due to limitations in the extension API and its implementation that will
     * be addressed in the beta release.
     *
     * @internal
     * @hidden
     */
    export namespace internal {
        /**
         * Returns a promise that resolves when all pending messages have been sent to the client.
         * It helps enforce serialization of messages.
         *
         * @internal
         */
        export function sync(): Promise<void>

        /**
         * Updates context values for use in context expressions and contribution labels.
         *
         * @param updates The updates to apply to the context. If a context property's value is null, it is deleted from the context.
         */
        export function updateContext(updates: ContextValues): void

        /**
         * The URL to the Sourcegraph site that the user's session is associated with. This refers to
         * Sourcegraph.com (`https://sourcegraph.com`) by default, or a self-hosted instance of Sourcegraph.
         *
         * @todo Consider removing this when https://github.com/sourcegraph/sourcegraph/issues/566 is fixed.
         *
         * @example `https://sourcegraph.com`
         */
        export const sourcegraphURL: URL

        /**
         * The client application that is running this extension, either 'sourcegraph' for Sourcegraph or 'other'
         * for all other applications (such as GitHub, GitLab, etc.).
         *
         * @todo Consider removing this when https://github.com/sourcegraph/sourcegraph/issues/566 is fixed.
         */
        export const clientApplication: 'sourcegraph' | 'other'
    }

    /** Support types for {@link Subscribable}. */
    interface NextObserver<T> {
        closed?: boolean
        next: (value: T) => void
        error?: (err: any) => void
        complete?: () => void
    }
    interface ErrorObserver<T> {
        closed?: boolean
        next?: (value: T) => void
        error: (err: any) => void
        complete?: () => void
    }
    interface CompletionObserver<T> {
        closed?: boolean
        next?: (value: T) => void
        error?: (err: any) => void
        complete: () => void
    }
    type PartialObserver<T> = NextObserver<T> | ErrorObserver<T> | CompletionObserver<T>

    /**
     * A stream of values that may be subscribed to.
     */
    export interface Subscribable<T> {
        /**
         * Subscribes to the stream of values.
         *
         * @returns An unsubscribable that, when its {@link Unsubscribable#unsubscribe} method is called, causes
         * the subscription to stop reacting to the stream.
         */
        subscribe(observer?: PartialObserver<T>): Unsubscribable
        /** @deprecated Use an observer instead of a complete callback */
        subscribe(next: null | undefined, error: null | undefined, complete: () => void): Unsubscribable
        /** @deprecated Use an observer instead of an error callback */
        subscribe(next: null | undefined, error: (error: any) => void, complete?: (() => void) | null): Unsubscribable
        /** @deprecated Use an observer instead of a complete callback */
        // eslint-disable-next-line @typescript-eslint/unified-signatures
        subscribe(next: (value: T) => void, error: null | undefined, complete: () => void): Unsubscribable
        subscribe(
            next?: ((value: T) => void) | null,
            error?: ((error: any) => void) | null,
            complete?: (() => void) | null
        ): Unsubscribable
    }

    /**
     * The extension context is passed to the extension's activate function and contains utilities for the
     * extension lifecycle.
     *
     * @since Sourcegraph 3.0. Use `export function activate(ctx?: ExtensionContext) { ... }` for prior
     * versions (to ensure your code handles the pre-3.0-preview case when `ctx` is undefined).
     */
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
}
