import type { EditorView } from '@codemirror/view'
import { Subject, from, of } from 'rxjs'
import { switchMap, map, startWith, catchError } from 'rxjs/operators'
import { get, type Readable, readable } from 'svelte/store'

import { goto as svelteGoto } from '$app/navigation'
import { page } from '$app/stores'
import { addLineRangeQueryParameter, formatSearchParameters, toPositionOrRangeQueryParameter } from '$lib/common'
import {
    positionToOffset,
    type Definition,
    type GoToDefinitionOptions,
    type SelectedLineRange,
    showTemporaryTooltip,
    locationToURL,
    type DocumentInfo,
} from '$lib/web'

import type { BlobPage_Blob } from '../../routes/[...repo=reporev]/(validrev)/(code)/-/blob/[...path]/page.gql'

/**
 * The minimum number of milliseconds that must elapse before we handle a "Go to
 * definition request".  The motivation to impose a minimum latency on this
 * action is to give the user feedback that something happened if they rapidly
 * trigger "Go to definition" from the same location and the destination token
 * is already visible in the viewport.  Without this minimum latency, the user
 * gets no feedback that the destination is visible.  With this latency, the
 * source token (where the user clicks) gets briefly focused before the focus
 * moves back to the destination token. This small wiggle in the focus state
 * makes it easier to find the destination token.
 */
const MINIMUM_GO_TO_DEF_LATENCY_MILLIS = 20

export function updateSearchParamsWithLineInformation(
    currentSearchParams: URLSearchParams,
    range: SelectedLineRange
): string {
    const parameters = new URLSearchParams(currentSearchParams)
    parameters.delete('popover')

    let query: string | undefined

    if (range?.line !== range?.endLine && range?.endLine) {
        query = toPositionOrRangeQueryParameter({
            range: {
                start: { line: range.line },
                end: { line: range.endLine },
            },
        })
    } else if (range?.line) {
        query = toPositionOrRangeQueryParameter({ position: { line: range.line } })
    }

    return formatSearchParameters(addLineRangeQueryParameter(parameters, query))
}

export async function goToDefinition(
    documentInfo: DocumentInfo,
    view: EditorView,
    definition: Definition,
    options?: GoToDefinitionOptions
): Promise<void> {
    const goto = options?.newWindow ? (url: string, _options?: unknown) => window.open(url, '_blank') : svelteGoto
    const offset = positionToOffset(view.state.doc, definition.occurrence.range.start)

    switch (definition.type) {
        case 'none': {
            if (offset) {
                showTemporaryTooltip(view, 'No definition found', offset, 2000)
            }
            break
        }
        case 'at-definition': {
            if (offset) {
                showTemporaryTooltip(view, 'You are at the definition', offset, 2000)
            }
            break
        }
        case 'single': {
            interface DefinitionState {
                // The destination URL if we trigger `history.goBack()`.  We use this state
                // to avoid inserting redundant 'A->B->A->B' entries when the user triggers
                // "go to definition" twice in a row from the same location.
                previousURL?: string
            }

            const locationState = history.state as DefinitionState
            const hrefFrom = locationToURL(documentInfo, definition.from)
            // Don't push URLs into the history if the last goto-def
            // action was from the same URL same as this action. This
            // happens when the user repeatedly triggers goto-def, which
            // is easy to do when the definition URL is close to
            // where the action got triggered.
            const shouldPushHistory = locationState?.previousURL !== hrefFrom
            // Add browser history entry for reference location. This allows users
            // to easily jump back to the location they triggered 'go to definition'
            // from. Additionally this
            await goto(hrefFrom, {
                replaceState: !shouldPushHistory || get(page).url.pathname === hrefFrom,
            })

            setTimeout(() => {
                void goto(locationToURL(documentInfo, definition.destination), {
                    replaceState: !shouldPushHistory,
                    state: { previousURL: hrefFrom },
                })
            }, MINIMUM_GO_TO_DEF_LATENCY_MILLIS)
            break
        }
        case 'multiple': {
            void goto(locationToURL(documentInfo, definition.destination, 'def'))
            if (offset) {
                showTemporaryTooltip(view, 'Not supported yet: Multiple definitions', offset, 2000)
            }
            break
        }
    }
}

export function openReferences(
    view: EditorView,
    _documentInfo: DocumentInfo,
    occurrence: Definition['occurrence']
): void {
    const offset = positionToOffset(view.state.doc, occurrence.range.start)
    if (offset) {
        showTemporaryTooltip(view, 'Not supported yet: Find references', offset, 2000)
    }
}

export function openImplementations(
    view: EditorView,
    _documentInfo: DocumentInfo,
    occurrence: Definition['occurrence']
): void {
    const offset = positionToOffset(view.state.doc, occurrence.range.start)
    if (offset) {
        showTemporaryTooltip(view, 'Not supported yet: Find implementations', offset, 2000)
    }
}

interface CombinedBlobData {
    blob: BlobPage_Blob | null
    /**
     * JSON encoded highlighting information. Can be an empty string.
     */
    highlights: string
    blobPending: boolean
    highlightsPending: boolean
    blobError: Error | null
    highlightsError: Error | null
}

interface BlobDataHandler extends Readable<CombinedBlobData> {
    set(blob: PromiseLike<BlobPage_Blob | null>, highlight: PromiseLike<string | undefined>): void
}

/**
 * This store synchronizes the state of the blob data and the highlights. While new blob data is
 * loading, the old blob and highlights data is still available. Once the blob data is loaded, the
 * highlights are updated.
 */
export function createBlobDataHandler(): BlobDataHandler {
    const input = new Subject<{ blob: PromiseLike<BlobPage_Blob | null>; highlight: PromiseLike<string | undefined> }>()

    return {
        ...readable<CombinedBlobData>(
            {
                blob: null,
                highlights: '',
                blobPending: false,
                highlightsPending: false,
                blobError: null,
                highlightsError: null,
            },
            (_set, update) => {
                const subscription = input
                    .pipe(
                        switchMap(({ blob, highlight }) => {
                            return from(blob).pipe(
                                switchMap(blob => {
                                    return from(highlight).pipe(
                                        map((highlights = '') => ({
                                            highlights,
                                            highlightsPending: false,
                                            highlightsError: null,
                                        })),
                                        startWith({
                                            blob,
                                            blobPending: false,
                                            blobError: null,
                                            highlights: '',
                                            highlightsPending: true,
                                            highlightsError: null,
                                        }),
                                        catchError(error =>
                                            of({ highlights: '', highlightsPending: false, highlightsError: error })
                                        )
                                    )
                                }),
                                startWith({ blobPending: true }),
                                catchError(error => of({ blob: null, blobPending: false, blobError: error }))
                            )
                        })
                    )
                    .subscribe(updatedCombinedData => {
                        update(combinedData => ({ ...combinedData, ...updatedCombinedData }))
                    })
                return () => subscription.unsubscribe()
            }
        ),

        set(blob, highlight) {
            input.next({ blob, highlight })
        },
    }
}
