import { type EditorState, Facet } from '@codemirror/state'
import type { EditorView, Tooltip, TooltipView } from '@codemirror/view'
import { type Observable, from } from 'rxjs'
import { map, startWith } from 'rxjs/operators'

import type { HoverMerged } from '@sourcegraph/client-api'
import { isMacPlatform } from '@sourcegraph/common'
import { MarkupKind } from '@sourcegraph/extension-api-classes'
import type { Range } from '@sourcegraph/extension-api-types'
import type { ActionItemAction } from '@sourcegraph/shared/src/actions/ActionItem'
import {
    type CodeIntelAPI,
    findLanguageMatchingDocument,
    hasFindImplementationsSupport,
} from '@sourcegraph/shared/src/codeintel/api'
import type { Occurrence } from '@sourcegraph/shared/src/codeintel/scip'
import { makeRepoGitURI, parseRepoGitURI } from '@sourcegraph/shared/src/util/url'
import type { UIRangeSpec } from '@sourcegraph/shared/src/util/url'

import type { WebHoverOverlayProps } from '../../../../components/WebHoverOverlay'
import { syntaxHighlight } from '../highlight'
import {
    contains,
    isInteractiveOccurrence,
    occurrenceAt,
    positionAtCmPosition,
    rangeToCmSelection,
    closestOccurrenceByCharacter,
} from '../occurrence-utils'
import { isRegularEvent, locationToURL, positionToOffset } from '../utils'

/**
 * Hover information received from a hover source.
 */
type HoverData = Pick<WebHoverOverlayProps, 'hoverOrError' | 'actionsOrError'>

export interface TooltipViewOptions {
    readonly view: EditorView
    readonly token: UIRangeSpec['range']
    readonly hovercardData: Observable<HoverData>
}

export interface Location {
    readonly repoName: string
    readonly filePath: string
    readonly revision?: string
    readonly range: Range
}

export type Definition =
    | {
          type: 'at-definition' | 'single' | 'multiple'
          readonly destination: Location
          readonly from: Location
          occurrence: Occurrence
      }
    | { type: 'none'; occurrence: Occurrence }

export interface GoToDefinitionOptions {
    newWindow: boolean
}

export interface DocumentInfo {
    repoName: string
    filePath: string
    commitID: string
    revision: string
    languages: string[]
}

export interface CodeIntelAPIConfig {
    /**
     * Reference to the code intel API.
     */
    api: CodeIntelAPI
    /**
     * Information about the current document.
     */
    documentInfo: DocumentInfo

    /**
     * Called to create the code intel tooltip.
     */
    createTooltipView: (options: TooltipViewOptions) => TooltipView

    /**
     * Called when any action triggers "go to definition".
     */
    goToDefinition(view: EditorView, definition: Definition, options?: GoToDefinitionOptions): void

    /**
     * Called when any action wants to show references for the provided occurrence.
     */
    openReferences(view: EditorView, documentInfo: DocumentInfo, occurrence: Occurrence): void

    /**
     * Called when any action wants to show implementations for the provided occurrence.
     */
    openImplementations(view: EditorView, documentInfo: DocumentInfo, occurrence: Occurrence): void
}

/**
 * Wrapper around the code intel API. It translates CodeMirror document positions to occurrences and adds
 * a caching layer to speed up subsequent access.
 * Extensions should access the methods of this class via the corresponding functions exported by this
 * module, e.g. {@link hasDefinitionAt} or {@link getDocumentHighlights}.
 */
export class CodeIntelAPIAdapter {
    private hoverCache = new Map<Occurrence, Promise<(Tooltip & { end: number }) | null>>()
    private documentHighlightCache = new Map<Occurrence, Promise<{ from: number; to: number }[]>>()
    private definitionCache = new Map<Occurrence, Promise<Definition>>()
    private occurrenceCache = new Map<
        number,
        { occurrence: Occurrence | null; range: { from: number; to: number } | null }
    >()
    private documentURI: string
    private hasCodeIntelligenceSupport: boolean

    constructor(private config: CodeIntelAPIConfig) {
        this.documentURI = makeRepoGitURI({
            repoName: config.documentInfo.repoName,
            filePath: config.documentInfo.filePath,
            commitID: config.documentInfo.commitID,
        })
        this.hasCodeIntelligenceSupport = !!findLanguageMatchingDocument({ uri: this.documentURI })
    }

    public findOccurrenceAt(
        offset: number,
        state: EditorState
    ): { occurrence: Occurrence | null; range: { from: number; to: number } | null } {
        if (!this.hasCodeIntelligenceSupport) {
            return { occurrence: null, range: null }
        }

        const fromCache = this.occurrenceCache.get(offset)
        if (fromCache) {
            return fromCache
        }

        let occurrence = occurrenceAt(state, offset) ?? null
        if (occurrence && !isInteractiveOccurrence(occurrence)) {
            occurrence = null
        }
        const range = occurrence ? rangeToCmSelection(state.doc, occurrence.range) : null
        for (let i = range?.from ?? offset, to = range?.to ?? offset; i <= to; i++) {
            this.occurrenceCache.set(offset, { occurrence, range })
        }

        return { occurrence, range }
    }

    public getDefinition(state: EditorState, occurrence: Occurrence): Promise<Definition> {
        const occurrences = state.facet(syntaxHighlight).occurrences
        const fromCache = this.definitionCache.get(occurrence)
        if (fromCache) {
            return fromCache
        }

        const { documentInfo } = this.config
        const from = {
            ...documentInfo,
            range: occurrence.range,
        }

        const promise: Promise<Definition> = this.config.api
            .getDefinition(
                {
                    textDocument: { uri: this.documentURI },
                    position: occurrence.range.start,
                },
                { referenceOccurrence: occurrence, documentOccurrences: occurrences }
            )
            .then(locations => {
                for (const location of locations) {
                    const { repoName, filePath, revision } = parseRepoGitURI(location.uri)
                    if (
                        filePath &&
                        location.range &&
                        repoName === documentInfo.repoName &&
                        filePath === documentInfo.filePath &&
                        documentInfo.commitID === revision &&
                        contains(location.range, occurrence.range.start)
                    ) {
                        return {
                            type: 'at-definition',
                            from,
                            destination: {
                                repoName,
                                filePath,
                                revision,
                                range: location.range,
                            },
                            occurrence,
                        }
                    }
                }
                if (locations.length === 1) {
                    const location = locations[0]
                    const { repoName, filePath, revision } = parseRepoGitURI(location.uri)
                    if (!(filePath && location.range)) {
                        return { type: 'none', occurrence }
                    }
                    return {
                        type: 'single',
                        from,
                        destination: {
                            repoName,
                            filePath,
                            revision,
                            range: location.range,
                        },
                        occurrence,
                    }
                }
                return locations.length === 0
                    ? { type: 'none', occurrence }
                    : { type: 'multiple', from, destination: from, occurrence }
            })
        this.definitionCache.set(occurrence, promise)

        return promise
    }

    public getDocumentHighlights(state: EditorState, offset: number): Promise<{ from: number; to: number }[]> {
        const occurrence = this.findOccurrenceAt(offset, state).occurrence
        if (!occurrence) {
            return Promise.resolve([])
        }
        const fromCache = this.documentHighlightCache.get(occurrence)
        if (fromCache) {
            return fromCache
        }
        const promise = this.config.api
            .getDocumentHighlights({
                textDocument: { uri: this.documentURI },
                position: occurrence.range.start,
            })
            .then(result =>
                result
                    .map(({ range }) => ({
                        from: positionToOffset(state.doc, range.start),
                        to: positionToOffset(state.doc, range.end),
                    }))
                    .filter((range): range is { from: number; to: number } => range.to !== null && range.from !== null)
            )
        this.documentHighlightCache.set(occurrence, promise)
        return promise
    }

    public getHoverTooltip(state: EditorState, offset: number): Promise<(Tooltip & { end: number }) | null> {
        const { occurrence, range } = this.findOccurrenceAt(offset, state)
        if (!occurrence || !range) {
            return Promise.resolve(null)
        }
        const fromCache = this.hoverCache.get(occurrence)
        if (fromCache) {
            return fromCache
        }

        // Preload definition
        void this.getDefinition(state, occurrence)

        const tooltip = this.config.api
            .getHover({
                position: occurrence.range.start,
                textDocument: { uri: this.documentURI },
            })
            .then((result): (Tooltip & { end: number }) | null => {
                let markdownContents: string =
                    result === null || result === undefined || result.contents.length === 0
                        ? ''
                        : result.contents
                              .map(({ value }) => value)
                              .join('\n\n----\n\n')
                              .trimEnd()
                const precise = isPrecise(result)
                if (!precise && markdownContents.length > 0 && !isInteractiveOccurrence(occurrence)) {
                    return null
                }
                if (markdownContents === '' && isInteractiveOccurrence(occurrence)) {
                    markdownContents = 'No hover information available'
                }
                return markdownContents
                    ? {
                          pos: range.from,
                          end: range.to,
                          above: true,
                          create: view =>
                              this.config.createTooltipView({
                                  view,
                                  token: occurrence.range.withIncrementedValues(),
                                  hovercardData: this.getHoverActions(view, occurrence, precise).pipe(
                                      map(actions => ({
                                          actionsOrError: actions,
                                          hoverOrError: {
                                              range: occurrence.range,
                                              aggregatedBadges: result?.aggregatedBadges,
                                              contents: [
                                                  {
                                                      value: markdownContents,
                                                      kind: MarkupKind.Markdown,
                                                  },
                                              ],
                                          },
                                      }))
                                  ),
                              }),
                      }
                    : null
            })

        this.hoverCache.set(occurrence, tooltip)
        return tooltip
    }

    private getHoverActions(
        view: EditorView,
        occurrence: Occurrence,
        isPrecise: boolean
    ): Observable<HoverData['actionsOrError']> {
        // Trigger "Go to definition" to identify if this hover message
        // is already at the definition or if there are no references.
        return from(this.getDefinition(view.state, occurrence)).pipe(
            // To prevent the "Go to definition" from delaying the loading of
            // the popover, we provide an instant result that doesn't handle the
            // "No definition found" or "You are at the definition" cases. This
            // instant result gets dynamically replaced by the actual result once
            // it finishes loading.
            startWith({ type: 'initial' as const }),
            map(definition => {
                const actions: ActionItemAction[] = []
                if (definition.type === 'none' || definition.type === 'at-definition') {
                    actions.push({
                        disabledWhen: true,
                        active: true,
                        action: {
                            id: 'go-to-definition',
                            title: '',
                            disabledTitle:
                                definition.type === 'none' ? 'No definition found' : 'You are at the definition',
                            command: 'open',
                        },
                    })
                } else if (definition.type === 'initial') {
                    actions.push({
                        active: true,
                        action: {
                            id: 'go-to-definition',
                            title: 'Go to definition',
                            command: 'invokeFunction-new',
                            commandArguments: [() => this.goToDefinitionAtOccurrence(view, occurrence)],
                        },
                    })
                } else {
                    actions.push({
                        active: true,
                        action: {
                            id: 'go-to-definition',
                            title: 'Go to definition',
                            command: 'open',
                            commandArguments: [
                                locationToURL(this.config.documentInfo, definition.destination) ?? '',
                                (event: MouseEvent | KeyboardEvent): boolean => {
                                    if (isRegularEvent(event)) {
                                        // "regular events" are basic clicks with the main button or keyboard
                                        // events without modifier keys.
                                        // We treat these the same way as Cmd-Click on the token itself.
                                        event.preventDefault()
                                        void this.goToDefinitionAtOccurrence(view, occurrence)
                                        return true
                                    }
                                    // Don't override `onSelect` unless it's a regular event with modifier keys
                                    // or with non-main buttons.
                                    // We do this to fallback to the browser's default behavior for links, for example to allow
                                    // the user to open the definition in a new browser tab.
                                    return false
                                },
                            ],
                        },
                    })
                }
                actions.push({
                    active: true,
                    action: {
                        id: 'findReferences',
                        title: 'Find references',
                        command: 'invokeFunction-new',
                        commandArguments: [
                            () => this.config.openReferences(view, this.config.documentInfo, occurrence),
                        ],
                    },
                })

                if (isPrecise && this.config.documentInfo.languages.some(hasFindImplementationsSupport)) {
                    actions.push({
                        active: true,
                        action: {
                            id: 'findImplementations',
                            title: 'Find implementations',
                            command: 'invokeFunction-new',
                            commandArguments: [
                                () => this.config.openImplementations(view, this.config.documentInfo, occurrence),
                            ],
                        },
                    })
                }
                actions.push({
                    active: true,
                    action: {
                        id: 'goToDefinition.help',
                        title: '?', // special marker for the MDI "Help" icon.
                        description: `Go to definition with ${modifierClickDescription}, long-click, or by pressing Enter with the keyboard. Display this popover by pressing Space with the keyboard.`,
                        command: '',
                    },
                })
                return actions
            })
        )
    }

    public async goToDefinitionAtOccurrence(
        view: EditorView,
        occurrence: Occurrence,
        options?: GoToDefinitionOptions
    ): Promise<void> {
        this.config.goToDefinition(view, await this.getDefinition(view.state, occurrence), options)
    }
}

const modifierClickDescription = isMacPlatform() ? 'cmd+click' : 'ctrl+click'

/**
 * Returns true if any of the code intel information is precise data.
 */
function isPrecise(hover: HoverMerged | null | undefined): boolean {
    for (const badge of hover?.aggregatedBadges || []) {
        if (badge.text === 'precise') {
            return true
        }
    }
    return false
}

/**
 * Facet for registering the code intel API.
 */
export const codeIntelAPIAdapter = Facet.define<CodeIntelAPIAdapter, CodeIntelAPIAdapter | null>({
    combine(values) {
        return values[0] ?? null
    },
})

/**
 * Helper function for getting a reference to the current code intel API.
 * Throws an error if it is not set.
 */
function getCodeIntelAPIAdapter(state: EditorState): CodeIntelAPIAdapter {
    const apiAdapter = state.facet(codeIntelAPIAdapter)
    if (!apiAdapter) {
        throw new Error('A CodeIntelAPI instance has to be provided via the `codeIntelAPI` facet.')
    }
    return apiAdapter
}

/**
 * Returns true if the token at this position has a definition. Lookup is cached.
 */
export async function hasDefinitionAt(state: EditorState, offset: number): Promise<boolean> {
    const apiAdapter = getCodeIntelAPIAdapter(state)
    const occurrence = apiAdapter.findOccurrenceAt(offset, state).occurrence
    return !!occurrence && (await apiAdapter.getDefinition(state, occurrence)).type !== 'none'
}

/**
 * Returns the document range for the interactive occurrence at this position, if any.
 * Lookup is cached.
 */
export function findOccurrenceRangeAt(state: EditorState, offset: number): { from: number; to: number } | null {
    return getCodeIntelAPIAdapter(state).findOccurrenceAt(offset, state).range
}

/**
 * Return the document highlights for the occurrence at this position.
 * Lookup is cached.
 */
export function getDocumentHighlights(state: EditorState, offset: number): Promise<{ from: number; to: number }[]> {
    return getCodeIntelAPIAdapter(state).getDocumentHighlights(state, offset)
}

/**
 * Get code intel tooltip at this position.
 * Looltip is cached.
 */
export function getHoverTooltip(state: EditorState, offset: number): Promise<(Tooltip & { end: number }) | null> {
    return getCodeIntelAPIAdapter(state).getHoverTooltip(state, offset)
}

/**
 * Trigger "got to definition" behavior at this position. Won't do anything
 * if there is no interactive occurrence at this position.
 */
export async function goToDefinitionAt(
    view: EditorView,
    offset: number,
    options?: GoToDefinitionOptions
): Promise<void> {
    const apiAdapter = getCodeIntelAPIAdapter(view.state)
    const occurrence = apiAdapter.findOccurrenceAt(offset, view.state).occurrence
    if (occurrence) {
        await apiAdapter.goToDefinitionAtOccurrence(view, occurrence, options)
    }
}

/**
 * Finds the next/previous occurrence by line or character starting at the provided position.
 */
export function nextOccurrencePosition(
    state: EditorState,
    from: number,
    step: 'line' | 'character',
    direction: 'next' | 'previous' = 'next'
): number | null {
    const position = positionAtCmPosition(state.doc, from)
    const table = state.facet(syntaxHighlight)
    let occurrence = null
    if (step === 'character') {
        occurrence = closestOccurrenceByCharacter(
            position.line,
            table,
            position,
            direction === 'next'
                ? occurrence => occurrence.range.start.isGreater(position)
                : occurrence => occurrence.range.start.isSmaller(position)
        )
    } else {
        const next = direction === 'next'
        const start = position.line + (next ? 1 : -1)
        const increment = next ? 1 : -1

        for (let line = start; line >= 0 && line < table.lineIndex.length; line += increment) {
            occurrence = closestOccurrenceByCharacter(line, table, position)
            if (occurrence) {
                break
            }
        }
    }
    return occurrence ? positionToOffset(state.doc, occurrence.range.start) : null
}

/**
 * Convenience function around {@link nextOccurrencePosition} for finding the previous occurrence.
 */
export function prevOccurrencePosition(state: EditorState, from: number, step: 'line' | 'character'): number | null {
    return nextOccurrencePosition(state, from, step, 'previous')
}
