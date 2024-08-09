import type { EditorView } from '@codemirror/view'
import { get } from 'svelte/store'

import { goto as svelteGoto } from '$app/navigation'
import { page } from '$app/stores'
import { Occurrence, toPrettyBlobURL } from '$lib/shared'
import {
    positionToOffset,
    type Definition,
    type GoToDefinitionOptions,
    showTemporaryTooltip,
    locationToURL,
    type DocumentInfo,
} from '$lib/web'

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

/**
 * This will either:
 * - Show a tooltip indicating that no definition was found or that the user is already at the definition.
 * - Go to the definition if it is a single definition.
 * - Show a tooltip indicating that multiple definitions were found (but do nothing else).
 */
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
                showTemporaryTooltip(view, 'Multiple definitions found', offset, 2000)
            }
            break
        }
    }
}

export function openReferences(view: EditorView, documentInfo: DocumentInfo, occurrence: Occurrence): void {
    const url = toPrettyBlobURL({
        repoName: documentInfo.repoName,
        revision: documentInfo.revision,
        commitID: documentInfo.commitID,
        filePath: documentInfo.filePath,
        range: occurrence.range.withIncrementedValues(),
        viewState: 'references',
    })
    svelteGoto(url)
}
