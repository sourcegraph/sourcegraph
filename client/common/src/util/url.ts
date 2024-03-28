import { tryCatch } from '../errors'

/**
 * Represents a line, a position, a line range, or a position range. It forbids
 * just a character, or a range from a line to a position or vice versa (such as
 * "L1-2:3" or "L1:2-3"), none of which would make much sense.
 *
 * 1-indexed.
 *
 * For backward compatibility, `character` and `endCharacter` are allowed to be 0,
 * which is the same as not being set.
 */
export type LineOrPositionOrRange =
    | { line?: undefined; character?: undefined; endLine?: undefined; endCharacter?: undefined }
    | { line: number; character?: number; endLine?: undefined; endCharacter?: undefined }
    | { line: number; character?: undefined; endLine?: number; endCharacter?: undefined }
    | { line: number; character: number; endLine: number; endCharacter: number }

/**
 * Parses a string like that encodes a line or a position.
 * Examples of valid input:
 * L1          -  line:     line 1
 * L1:2        -  position: line 1, character 2
 *
 * If the line is invalid (e.g. L0), the return value will be an empty object.
 * If the character is invalid (e.g. L1:0), the return value will be a line.
 *
 * @param lineOrPosition a string like that encodes a line or a position
 * @returns the parsed line or position, or an empty object if the input is invalid
 */
function parseLineOrPosition(
    lineOrPosition: string
): { line?: undefined; character?: undefined } | { line: number; character?: number } {
    if (lineOrPosition.startsWith('L')) {
        lineOrPosition = lineOrPosition.slice(1)
    }
    const parts = lineOrPosition.split(':', 2)
    const line = parts.length >= 1 ? parseInt(parts[0], 10) : undefined
    const character = parts.length === 2 ? parseInt(parts[1], 10) : undefined

    if (line === undefined || isNaN(line) || line <= 0) {
        return {}
    }
    if (character === undefined || isNaN(character) || character <= 0) {
        return { line }
    }
    return { line, character }
}

/**
 * Parses a string like that encodes a line, a position, a line range, or a position range.
 *
 * Examples of valid input:
 * L1          -  line:           line 1
 * L1:2        -  position:       line 1, character 2
 * L1-2        -  line range:     line 1 to line 2
 * L1:2-3:4    -  position range: line 1, character 2 to line 3, character 4
 *
 * Other combinations of lines or positions are not valid.
 *
 * If the range is empty, e.g. L1:2-1:2, the output return value will be simplified to a position, e.g. L1:2.
 *
 * @param range a string like that encodes a line, a position, a line range, or a position range
 * @returns the parsed line, position, line range, or position range, or an empty object if the input is invalid
 */
function parseLineOrPositionOrRange(range: string): LineOrPositionOrRange {
    if (!/^(L\d+(:\d+)?(-L?\d+(:\d+)?)?)?$/.test(range)) {
        return {} // invalid
    }

    // Parse the line or position range, ensuring we don't get an inconsistent result
    // (such as L1-2:3, a range from a line to a position).
    let line: number | undefined // 17
    let character: number | undefined // 19
    let endLine: number | undefined // 21
    let endCharacter: number | undefined // 23
    if (range.startsWith('L')) {
        const positionOrRangeString = range.slice(1)
        const [startString, endString] = positionOrRangeString.split('-', 2)
        if (startString) {
            const parsed = parseLineOrPosition(startString)
            line = parsed.line
            character = parsed.character
        }
        if (endString) {
            const parsed = parseLineOrPosition(endString)
            endLine = parsed.line
            endCharacter = parsed.character
        }
    }
    if (line === undefined || (endLine !== undefined && typeof character !== typeof endCharacter)) {
        return {}
    }
    if (character === undefined) {
        return endLine === undefined ? { line } : { line, endLine }
    }
    if (endLine === undefined || endCharacter === undefined) {
        return { line, character }
    }
    return simplifyRange({ line, character, endLine, endCharacter })
}

/**
 * Simplifies a line or a position range. If the input represents an empty range,
 * e.g. `{ line: 1, character: 2, endLine: 1, endCharacter: 2 }`, the output
 * will be converted into a position, e.g. `{ line: 1, character: 2 }`.
 * If the input is invalid, an empty object is returned.
 *
 * See {@link LineOrPositionOrRange}.
 *
 * @param range the line, position, line range, or position range to simplify
 * @returns the simplified line, position, line range, or position range
 */
function simplifyRange(range: LineOrPositionOrRange): LineOrPositionOrRange {
    if (range.line === undefined) {
        return {}
    }
    // Treat character 0 or endCharacter 0 as 'not set'
    if (range.character === 0 || range.endCharacter === 0) {
        return { line: range.line, endLine: range.endLine }
    }
    if (range.line === range.endLine && range.character === range.endCharacter) {
        return { line: range.line, character: range.character }
    }
    return range
}

/**
 * Formats a line, a position, a line range, or a position range as a string. The output
 * is suitable for use in a URL hash or search parameter and can be parsed with
 * {@link parseLineOrPositionOrRange}.
 * If the input represents an empty range, e.g. `{ line: 1, character: 2, endLine: 1, endCharacter: 2 }`,
 * the output will be converted into a position, e.g. `L1:2`.
 *
 * See {@link LineOrPositionOrRange}.
 *
 * @param lpr the line, position, line range, or position range to format
 * @returns the formatted line, position, line range, or position range
 */
function formatLineOrPositionOrRange(lpr: LineOrPositionOrRange): string {
    lpr = simplifyRange(lpr)
    if (lpr.line === undefined) {
        return ''
    }
    if (lpr.character === undefined) {
        return `L${lpr.line}${lpr.endLine ? `-${lpr.endLine}` : ''}`
    }
    return `L${lpr.line}:${lpr.character}${lpr.endLine ? `-${lpr.endLine}:${lpr.endCharacter}` : ''}`
}

/**
 * Adds, updates or removes a line or position range in search parameters.
 *
 * @param inputParams the URL's search parameters
 * @param lpr the line or position range to add or update
 * @returns the updated search parameters
 */
function addOrUpdateLineRange(inputParams: URLSearchParams, lpr: LineOrPositionOrRange | null): URLSearchParams {
    const params = new URLSearchParams(inputParams)
    const range = lpr ? formatLineOrPositionOrRange(lpr) : ''

    // Remove existing line range if it exists
    const existingLineRangeKey = findLineKeyInSearchParameters(params)
    if (existingLineRangeKey) {
        params.delete(existingLineRangeKey)
    }

    return range !== '' ? new URLSearchParams([[range, ''], ...params]) : params
}

/**
 * Tells if the given fragment component is a legacy blob hash component or not.
 * Legacy fragments have the structure `#L<line>:<character>-<line>:<character>$<viewState>`.
 *
 * @param hash The URL fragment.
 */
export function isLegacyFragment(hash: string): boolean {
    if (hash.startsWith('#')) {
        hash = hash.slice(1)
    }
    return (
        hash !== '' &&
        !hash.includes('=') &&
        (hash.includes('$info') ||
            hash.includes('$def') ||
            hash.includes('$references') ||
            hash.includes('$impl') ||
            hash.includes('$history'))
    )
}

/**
 * Parses the URL fragment (hash) portion, which consists of a line, position, or range in the file, plus an
 * optional "viewState" parameter (that encodes other view state, such as for the panel).
 *
 * For example, in the URL fragment "#L17:19-21:23$foo:bar", the "viewState" is "foo:bar".
 *
 * NOTE: Prefer to use {@link SourcegraphURL} instead of this function.
 *
 * @template V The type that describes the view state (typically a union of string constants). There is no runtime check that the return value satisfies V.
 */
function parseHash<V extends string>(hash: string): LineOrPositionOrRange & { viewState?: V } {
    if (hash.startsWith('#')) {
        hash = hash.slice('#'.length)
    }

    if (!isLegacyFragment(hash)) {
        // Modern hash parsing logic (e.g. for hashes like `"#L17:19-21:23&tab=foo:bar"`:
        const searchParameters = new URLSearchParams(hash)
        const existingLineRangeKey = findLineKeyInSearchParameters(searchParameters)
        const lpr: LineOrPositionOrRange & { viewState?: V } = existingLineRangeKey
            ? parseLineOrPositionOrRange(existingLineRangeKey)
            : {}
        if (searchParameters.get('tab')) {
            lpr.viewState = searchParameters.get('tab') as V
        }
        return lpr
    }

    // Legacy hash parsing logic (e.g. for hashes like "#L17:19-21:23$foo:bar" where the "viewState" is "foo:bar"):
    if (!/^(L\d+(:\d+)?(-\d+(:\d+)?)?)?(\$.*)?$/.test(hash)) {
        // invalid or empty hash
        return {}
    }
    const lineCharModalInfo = hash.split('$', 2) // e.g. "L17:19-21:23$references"
    const lpr: LineOrPositionOrRange & { viewState?: V } = parseLineOrPositionOrRange(lineCharModalInfo[0])
    if (lineCharModalInfo[1]) {
        lpr.viewState = lineCharModalInfo[1] as V
    }
    return lpr
}

/**
 * Finds an existing line range search parameter like "L1-2:3"
 */
function findLineKeyInSearchParameters(searchParameters: URLSearchParams): string | undefined {
    for (const key of searchParameters.keys()) {
        if (key.startsWith('L')) {
            return key
        }
        break
    }
    return undefined
}

/**
 * Stringifies the provided search parameters, replaces encoded `/` and `:` characters,
 * and removes trailing `=`.
 *
 * E.g. L1%3A2 => L1:2
 */
function formatSearchParameters(searchParameters: string): string {
    return searchParameters.replaceAll('%2F', '/').replaceAll('%3A', ':').replaceAll('=&', '&').replace(/=$/, '')
}

/**
 * This class encapsulates the logic for creating and manipulating Soucegraph URLs.
 * Not all methods are applicable to all types of URLs. If a method is not applicable
 * to the URL, the operation will be a no-op.
 *
 * See the individual method documentation for details.
 *
 * Using this class to manipulate URLs is preferred over manual string manipulation
 * because it ensures that the URL is prettified when converted to a string.
 */
export class SourcegraphURL {
    private url: URL
    private hasPathname: boolean
    private hasOrigin: boolean

    private constructor(url: string | URL) {
        this.url = typeof url === 'string' ? new URL(url, 'http://0.0.0.0/') : new URL(url)
        this.hasPathname = !(typeof url === 'string' && /^[#?]/.test(url))
        this.hasOrigin = typeof url !== 'string' || /^https?:/.test(url)
    }

    /**
     * Creates a new SourcegraphURL instance from a string, URL, URLSearchParams, or location object.
     *
     * When converting the URL back to a string, the string representation depends on the input as well
     * as the changes that have been made to the URL.
     *
     * If the input contains an origin, the output will too.
     * If the input contains a pathname, the output will too.
     *
     * This should make the output fairly predictable.
     */
    public static from(
        url: string | URL | URLSearchParams | { pathname?: string; search?: string | URLSearchParams; hash?: string }
    ): SourcegraphURL {
        if (typeof url === 'string' || url instanceof URL) {
            return new SourcegraphURL(url)
        }
        if (url instanceof URLSearchParams) {
            return new SourcegraphURL(`?${formatSearchParameters(url.toString())}`)
        }
        return SourcegraphURL.fromLocation(url)
    }

    private static fromLocation(location: {
        pathname?: string
        search?: string | URLSearchParams
        hash?: string
    }): SourcegraphURL {
        let { pathname = '', search = '', hash = '' } = location
        if (search) {
            if (typeof search === 'string') {
                if (!search.startsWith('?')) {
                    search = `?${search}`
                }
            } else {
                search = `?${search.toString()}`
            }
        }
        if (hash && !hash.startsWith('#')) {
            hash = `#${hash}`
        }
        return new SourcegraphURL(`${pathname}${search}${hash}`)
    }

    // Mutation methods

    /**
     * Adds or updates a line or position range in a URL's search parameters.
     *
     * Example:
     * ```
     * const url = new SourcegraphURL('/foo?bar')
     * url.addOrUpdateLineRange({ line: 24, character: 24 })
     * url.toString() // => '/foo?L24:24&bar'
     * ```
     *
     * @param href the URL to update
     * @param lpr the line or position range to add or update
     * @returns the updated URL
     */
    public setLineRange(lpr: LineOrPositionOrRange | null): this {
        this.url.search = addOrUpdateLineRange(this.url.searchParams, lpr).toString()
        return this
    }

    /**
     * Sets the view state, using the modern hash format.
     *
     * Example:
     * ```
     * const url = new SourcegraphURL('/foo?bar')
     * url.setViewState('references')
     * url.toString() // => '/foo?bar#L1:2-3:4&tab=references'
     * ```
     *
     * @template V The type that describes the view state (typically a union of string constants).
     * @param viewState the view state to set
     */
    public setViewState<V extends string = string>(viewState: V | undefined): this {
        // Try to preserve existing hash params
        const hashParams = new URLSearchParams(this.url.hash.slice(1))
        if (!viewState && !hashParams.has('tab')) {
            // Nothing to do
            return this
        }
        if (viewState) {
            hashParams.set('tab', viewState)
        } else {
            hashParams.delete('tab')
        }
        this.url.hash = hashParams.toString()
        return this
    }

    /**
     * Adds a search parameter to the URL.
     */
    public setSearchParameter(key: string, value: string): this {
        this.url.searchParams.set(key, value)
        return this
    }

    /**
     * Removes a search parameter from the URL.
     */
    public deleteSearchParameter(key: string): this {
        this.url.searchParams.delete(key)
        return this
    }

    // Accessors
    //
    /**
     * Parses the encoded line range from the URL's search parameters or hash.
     * A line range is often present in file URLs to indicate the selected lines or positions.
     *
     * If the URL contains a line range in both the search parameters and the hash,
     * the search parameters take precedence.
     *
     * If the line range is "empty" (e.g. L1:2-1:2), the return value will be simplified
     * to a position (e.g. L1:2).
     *
     * Examples of valid line or position ranges:
     *
     *   ?L1 => { line: 1 }
     *   ?L1:2 => { line: 1, character: 2 }
     *   ?L1-2 => { line: 1, endLine: 2 }
     *   ?L1:2-3:4 => { line: 1, character: 2, endLine: 3, endCharacter: 4 }
     *   ?L1:2-1:2 => { line: 1, character: 2 }
     *   #L1 => { line: 1 }
     *   #L1:2 => { line: 1, character: 2 }
     *   #L1-2 => { line: 1, endLine: 2 }
     *   #L1:2-3:4 => { line: 1, character: 2, endLine: 3, endCharacter: 4 }
     *   #L1:2-1:2 => { line: 1, character: 2 }
     *
     * @returns the parsed line or position range, or an empty object if the input is invalid
     */
    public get lineRange(): LineOrPositionOrRange {
        const existingLineRangeKey = findLineKeyInSearchParameters(this.url.searchParams)
        if (existingLineRangeKey) {
            return parseLineOrPositionOrRange(existingLineRangeKey)
        }
        return parseHash(this.url.hash)
    }

    /**
     * Parses the view state from the URL.
     *
     * The view state is often present in file URLs to indicate the selected tab.
     *
     * The function supports both legacy and modern hash formats:
     * - Legacy: `#L1:2-3:4$references`
     * - Modern: `#L1:2-3:4&tab=references`
     *
     *  @returns the parsed view state, or undefined if the input is invalid
     */
    public get viewState(): string | undefined {
        return parseHash(this.url.hash).viewState
    }

    /**
     * The pathname of the URL.
     */
    public get pathname(): string {
        return this.hasPathname ? this.url.pathname : ''
    }

    /**
     * The search parameters of the URL.
     */
    public get searchParams(): URLSearchParams {
        return this.url.searchParams
    }

    /**
     * The search parameters of the URL as a string.
     * Search parameters are prettied.
     */
    public get search(): string {
        return formatSearchParameters(this.url.search)
    }

    /**
     * The hash of the URL.
     */
    public get hash(): string {
        return formatSearchParameters(this.url.hash)
    }

    /**
     * Returns a string representation of the URL. The output
     * depends on the original input and the changes that have been
     * made to the URL.
     * E.g. if the original input did not include a pathname, the output
     * won't either.
     *
     * The stringified URL is prettified.
     */
    public toString(): string {
        return (
            (this.hasOrigin ? this.url.origin : '') +
            (this.hasPathname ? this.url.pathname : '') +
            this.search +
            this.hash
        )
    }
}

/**
 * Encodes revision with encodeURIComponent, except that slashes ('/') are preserved,
 * because they are not ambiguous in any of the current places where used, and URLs
 * for (e.g.) branches with slashes look a lot nicer with '/' than '%2F'.
 */
export function escapeRevspecForURL(revision: string): string {
    return encodeURIPathComponent(revision)
}

/**
 * %-Encodes a path component of a URI.
 *
 * It encodes all special characters except forward slashes and the plus sign `+`. The plus sign only has meaning
 * as a space in the query component of a URL, because its special meaning is defined for the
 * `application/x-www-form-urlencoded` MIME type, which is used for queries. It is not part of the general
 * `%`-encoding for URLs.
 */
export const encodeURIPathComponent = (component: string): string =>
    component.split('/').map(encodeURIComponent).join('/').replaceAll('%2B', '+')

/**
 * Returns true if the given URL points outside the current site.
 */
export const isExternalLink = (
    url: string,
    windowLocation__testingOnly: Pick<URL, 'origin' | 'href'> = window.location
): boolean =>
    !!tryCatch(() => new URL(url, windowLocation__testingOnly.href).origin !== windowLocation__testingOnly.origin)
