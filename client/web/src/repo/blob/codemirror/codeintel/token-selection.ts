import {
    Annotation,
    type EditorState,
    type Extension,
    Prec,
    StateEffect,
    StateField,
    type Transaction,
    type TransactionSpec,
} from '@codemirror/state'
import { Decoration, EditorView, keymap } from '@codemirror/view'
import { from, merge, timer } from 'rxjs'
import { map, takeWhile } from 'rxjs/operators'

import type { LineOrPositionOrRange } from '@sourcegraph/common'

import { LoadingTooltip } from '../tooltips/LoadingTooltip'
import { lprToRange } from '../utils'

import {
    findOccurrenceRangeAt,
    getHoverTooltip,
    goToDefinitionAt,
    nextOccurrencePosition,
    prevOccurrencePosition,
} from './api'
import { ignoreDecorations } from './decorations'
import { showDocumentHighlights } from './document-highlights'
import { type TooltipSource, showTooltip } from './tooltips'

const tokenSelection = Annotation.define<boolean>()
const setTooltipSource = StateEffect.define<TooltipSource | null>()

const interactiveOccurrenceClass = 'interactive-occurrence'

const interactiveTokenDeco = Decoration.mark({
    // We need to add the 'focus-visible' here to
    // 1. style the focused occurrence with the same style we use for other elements
    //    (except in certain situations, see theme extension below)
    // 2. prevent the focus-visible polyfill from mutation CodeMirror controlled DOM,
    //    which causes its own focus issues.
    //    (the polyfill leaves focused elements alone which have the class set explicitly)
    class: `${interactiveOccurrenceClass} focus-visible`,
    attributes: {
        // Selected (focused) occurrence is the only focusable element in the editor.
        // This helps to maintain the focus position when editor is blurred and then focused again.
        tabindex: '0',
    },
})

/**
 * Returns `true` if the editor selection is empty or is inside the occurrence range.
 */
function shouldApplyFocusStyles(state: EditorState, range: { from: number; to: number }): boolean {
    if (state.selection.main.empty) {
        return true
    }

    const isEditorSelectionInsideOccurrenceRange =
        state.selection.main.from >= range.from && state.selection.main.to <= range.to
    return isEditorSelectionInsideOccurrenceRange
}

/**
 * Helper function for hiding the tooltip at the selected token.
 */
function hideTokenTooltip(view: EditorView): void {
    view.dispatch({ effects: setTooltipSource.of(null) })
}

/**
 * Helper function for showing a tooltip at the selected token.
 */
function showTokenTooltip(view: EditorView, source: TooltipSource): void {
    view.dispatch({ effects: setTooltipSource.of(source) })
}

/**
 * Returns true if the transaction is a click or an explicit token navigation transaction.
 */
function shouldUpdateSelectedToken(transaction: Transaction): boolean {
    return transaction.isUserEvent('select.pointer') || !!transaction.annotation(tokenSelection)
}

/**
 * Field for keeping track of the currently selected token for token navigation. The
 * field updates when the selection changes to a different token via clicks or explicit
 * token navigation events.
 * It also provides
 *   - keyboard events for showing tooltips via {@link showTooltip}.
 *   - document highlights for the selected token via {@link showDocumentHighlights}
 */
const selectedToken = StateField.define<{
    range: { from: number; to: number }
    tooltipSource?: TooltipSource | null
} | null>({
    create() {
        // TODO(fkling): The selected token should be initialized form the initial selection,
        // but at this point the syntax data used to determine the type of token might not
        // be available yet, causing false positives.
        return null
    },
    update(value, transaction) {
        if (shouldUpdateSelectedToken(transaction)) {
            const offset = transaction.newSelection.main.from
            if (!value || offset < value.range.from || value.range.to < offset) {
                const range = findOccurrenceRangeAt(transaction.state, offset)
                if (range) {
                    value = { range }
                } else if (value) {
                    value = { ...value, tooltipSource: null }
                }
            }
        }
        if (value) {
            for (const effect of transaction.effects) {
                if (effect.is(setTooltipSource) && effect.value !== value.tooltipSource) {
                    value = { ...value, tooltipSource: effect.value }
                }
            }
        }
        return value
    },
    provide: self => [
        /**
         * Register tooltip source when available.
         */
        showTooltip.computeN([self], state => {
            const field = state.field(self)
            return field?.tooltipSource ? [field.tooltipSource] : []
        }),

        /**
         * Show document highlights for selected token.
         */
        showDocumentHighlights.computeN([self], state => {
            const field = state.field(self)
            return field ? [field.range] : []
        }),

        /**
         * We can't add/remove any decorations inside the selected token, because
         * that causes the node to be recreated and loosing focus, which breaks
         * token keyboard navigation.
         * This facet should be used by all codeIntel extensions to ensure that any
         * conflicting decoration is removed.
         */
        ignoreDecorations.from(self, value => value?.range ?? null),

        /**
         * This extension is responsible for keeping the focus inside the editor so
         * that keyboard navigation continues to work.
         *
         * Because CodeMirror only renders the visible part of the document it's
         * possible that the line with the selected token gets removed. If the token
         * was focused the whole editor would loose focus.
         *
         * To prevent this, this extension ensures focus in the following situations:
         *   - a new token is selected -> decoration/token itself is focused
         *   - token is scrolled out of view -> editor content is selected
         */
        EditorView.updateListener.of(update => {
            const range = update.state.field(self)?.range
            const view = update.view
            if (range) {
                if (range !== update.startState.field(self)?.range) {
                    view.contentDOM.querySelector<HTMLElement>('.interactive-occurrence')?.focus()
                }
                if (
                    update.viewportChanged &&
                    !view.dom.contains(document.activeElement) &&
                    (!view.contentDOM.querySelector<HTMLElement>('.interactive-occurrence') ||
                        range.from < view.viewport.from)
                ) {
                    view.contentDOM.focus()
                }
            }
        }),

        // Prec.lowest causes the decoration to wrap around any other decoration that's inside the token,
        // ensuring that it's not broken up across multiple decorations (e.g. by search highlights)
        Prec.lowest(
            EditorView.decorations.compute([self, 'selection'], state => {
                const selectedRange = state.field(self)?.range
                if (!selectedRange) {
                    return Decoration.none
                }
                return Decoration.set(interactiveTokenDeco.range(selectedRange.from, selectedRange.to), true)
            })
        ),

        // Controls how the focused/selected occurrence should be styled. Don't show a focus ring
        // (controlled by following theme), if text selection is not contained within the focused
        // occurrence.
        EditorView.contentAttributes.compute([self, 'selection'], state => {
            const selectedRange = state.field(self)?.range
            return {
                class:
                    selectedRange && shouldApplyFocusStyles(state, selectedRange) ? 'focus-interactive-occurrence' : '',
            }
        }),

        EditorView.theme({
            // Disable focus style in certain situations. We do this via CSS instead of computing
            // different decorations prevent CodeMirror from recreating the corresponding DOM nodes,
            // which in turn avoids loosing focus and ensures keyboard navigation continues to work.
            // (if the DOM node that has focus is removed, focus moves to the body).
            '.cm-content:not(.focus-interactive-occurrence) .interactive-occurrence.focus-visible': {
                boxShadow: 'none',
            },
        }),

        /**
         * If there is a focused occurrence set editor's tabindex to -1, so that pressing Shift+Tab moves the focus
         * outside the editor instead of focusing the editor itself.
         *
         * Explicitly define extension precedence to override the [default tabindex value](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@728ea45d1cc063cd60cbd552e00929c09cb8ced8/-/blob/client/web/src/repo/blob/CodeMirrorBlob.tsx?L47&).
         */
        Prec.high(
            EditorView.contentAttributes.compute([self], state => ({
                tabindex: state.field(self) ? '-1' : '0',
            }))
        ),

        /**
         * Keybindings for showing/hiding a code intel toolip at this position
         */
        keymap.of([
            {
                key: 'Space',
                run(view) {
                    const selected = view.state.field(self)
                    if (!selected) {
                        return true
                    }

                    if (selected.tooltipSource) {
                        hideTokenTooltip(view)
                        return true
                    }

                    const loadingTooltip = new LoadingTooltip(selected.range.from, selected.range.to)
                    showTokenTooltip(
                        view,
                        // Show loading tooltip after 50ms, if the request is still pending
                        merge(
                            from(getHoverTooltip(view.state, selected.range.from)),
                            timer(50).pipe(map(() => loadingTooltip))
                        ).pipe(takeWhile(tooltip => tooltip !== loadingTooltip, true))
                    )
                    return true
                },
            },
            {
                key: 'Escape',
                run(view) {
                    hideTokenTooltip(view)
                    return true
                },
            },

            {
                key: 'Enter',
                run(view) {
                    const selected = getSelectedToken(view.state)
                    if (!selected) {
                        return false
                    }

                    showTokenTooltip(view, new LoadingTooltip(selected.from, selected.to))
                    void goToDefinitionAt(view, selected.from).finally(() => {
                        hideTokenTooltip(view)
                    })
                    return true
                },
            },
            {
                key: 'Mod-Enter',
                run(view) {
                    const selected = getSelectedToken(view.state)
                    if (!selected) {
                        return false
                    }

                    showTokenTooltip(view, new LoadingTooltip(selected.from, selected.to))
                    void goToDefinitionAt(view, selected.from, { newWindow: true }).finally(() => {
                        hideTokenTooltip(view)
                    })
                    return true
                },
            },
        ]),

        EditorView.domEventHandlers({
            /**
             * Keyboard event handlers defined via {@link keymap} facet do not work with the screen reader enabled while
             * keypress handlers defined via {@link EditorView.domEventHandlers} still work.
             */
            keydown: (event: KeyboardEvent, view: EditorView): boolean => {
                const from = view.state.field(self)?.range.from || view.viewport.from
                let offset: number | null = null

                switch (event.key) {
                    case 'ArrowLeft': {
                        offset = prevOccurrencePosition(view.state, from, 'character')
                        break
                    }
                    case 'ArrowRight': {
                        offset = nextOccurrencePosition(view.state, from, 'character')
                        break
                    }
                    case 'ArrowUp': {
                        offset = prevOccurrencePosition(view.state, from, 'line')
                        break
                    }
                    case 'ArrowDown': {
                        offset = nextOccurrencePosition(view.state, from, 'line')
                        break
                    }

                    default: {
                        return false
                    }
                }
                if (offset !== null) {
                    view.dispatch(setSelection(offset))
                }
                return true
            },
        }),
    ],
})

function setSelection(anchor: number): TransactionSpec {
    return { selection: { anchor }, annotations: tokenSelection.of(true), scrollIntoView: true }
}

/**
 * Returns the currently focused/selected token, if any.
 */
export function getSelectedToken(state: EditorState): { from: number; to: number } | null {
    return state.field(selectedToken)?.range ?? null
}

/**
 * Helper function for moving token selection to the provided position.
 */
export function syncSelection(view: EditorView, position: LineOrPositionOrRange): void {
    const { line, character, endCharacter, endLine } = position
    if (line && character && endCharacter && (!endLine || line === endLine)) {
        const range = lprToRange(view.state.doc, { line, character, endCharacter, endLine })
        if (range) {
            view.dispatch(setSelection(range.from))
        }
    }
}

export const selectedTokenExtension: Extension = selectedToken
