import { useMemo, useState } from 'react'

import {
    Compartment,
    EditorState,
    type Extension,
    Facet,
    Prec,
    StateEffect,
    StateField,
    type Transaction,
} from '@codemirror/state'
import { EditorView, type KeyBinding, keymap } from '@codemirror/view'

import { placeholderConfig } from '../codemirror/placeholder'

export interface ModeDefinition {
    name: string
    keybinding?: Omit<KeyBinding, 'run' | 'scope' | 'any' | 'shift'>
    placeholder?: string
}

class SelectedModeState {
    constructor(
        public readonly selectedMode: ModeDefinition | null = null,
        public readonly previousInput: string | null = null
    ) {}

    public update(transaction: Transaction): SelectedModeState {
        // Aliasing makes it easier to update the state
        // eslint-disable-next-line @typescript-eslint/no-this-alias,unicorn/no-this-assignment
        let state: SelectedModeState = this
        const modes = transaction.state.facet(modesFacet)

        for (const effect of transaction.effects) {
            if (effect.is(setModeEffect)) {
                if (!effect.value) {
                    state = new SelectedModeState()
                } else if (state.selectedMode?.name !== effect.value) {
                    const mode = modes.find(mode => mode.name === effect.value)
                    state = mode
                        ? new SelectedModeState(mode, transaction.startState.sliceDoc())
                        : new SelectedModeState()
                }
            }
        }

        if (state.selectedMode && !modes.includes(state.selectedMode)) {
            // Availabel modes might have been changed, in which case we need to
            // update the state.
            const mode = modes.find(mode => mode.name === state.selectedMode?.name)
            if (mode) {
                state = new SelectedModeState(mode, state.previousInput)
            }
        }

        return state
    }
}

export const setModeEffect = StateEffect.define<string | null>()
const selectedModeField = StateField.define<SelectedModeState>({
    create() {
        return new SelectedModeState()
    },
    update(selectedMode, transaction) {
        return selectedMode.update(transaction)
    },
    provide(field) {
        return [
            Prec.highest(
                placeholderConfig.computeN([field], state => {
                    const selectedMode = state.field(field).selectedMode
                    if (!selectedMode?.placeholder) {
                        return []
                    }
                    return [{ content: selectedMode.placeholder }]
                })
            ),
            EditorView.contentAttributes.compute([field], state => {
                const selectedMode = state.field(field).selectedMode
                return {
                    class: selectedMode ? `sg-mode-${selectedMode.name}` : '',
                }
            }),
        ]
    },
})

export const modesFacet = Facet.define<ModeDefinition[], ModeDefinition[]>({
    combine(modes) {
        return modes.flat()
    },
    enables(facet) {
        return [
            selectedModeField,
            Prec.highest([
                keymap.compute([facet], state => {
                    const modes = state.facet(facet)
                    return [
                        {
                            key: 'Escape',
                            run: clearMode,
                        },
                        ...modes
                            .filter(mode => mode.keybinding)
                            .map(
                                (mode): KeyBinding => ({
                                    ...mode.keybinding,
                                    run: view => setMode(view, mode.name),
                                })
                            ),
                    ]
                }),
            ]),
        ]
    },
})

export function modeChanged({ startState, state }: Transaction): boolean {
    return getSelectedMode(startState) !== getSelectedMode(state)
}

export function getSelectedMode(state: EditorState): ModeDefinition | null {
    return state.field(selectedModeField, false)?.selectedMode ?? null
}

export function setMode(view: EditorView, name: string | null | ((mode: string | null) => string | null)): boolean {
    const resolvedName = typeof name === 'function' ? name(getSelectedMode(view.state)?.name ?? null) : name

    if (resolvedName === null) {
        return clearMode(view)
    }

    view.dispatch({
        effects: setModeEffect.of(resolvedName),
        // Clear input
        changes: { from: 0, to: view.state.doc.length, insert: '' },
        // It seems that setting the selection explicitly
        // ensures that the cursor is rendered correctly after the widget decoration.
        selection: { anchor: 0 },
    })
    return true
}

function clearMode(view: EditorView, restoreInput = true): boolean {
    const state = view.state.field(selectedModeField, false)
    if (state?.selectedMode) {
        const changes = restoreInput
            ? { from: 0, to: view.state.doc.length, insert: state.previousInput ?? '' }
            : undefined
        view.dispatch({
            effects: setModeEffect.of(null),
            changes,
            selection: changes ? { anchor: changes.insert.length } : undefined,
        })
        return true
    }
    return false
}

const isSetModeEffect = (effect: StateEffect<unknown>): effect is StateEffect<string | null> => effect.is(setModeEffect)

/**
 * The provided extensions are only enabled when the specified modes are active
 * or, when `null` is passed, when no mode is active.
 */
export function modeScope(extension: Extension, modes: (string | null)[]): Extension {
    const compartment = new Compartment()
    return [
        compartment.of(extension),
        EditorState.transactionExtender.of(transaction => {
            const effect = transaction.effects.find(isSetModeEffect)
            if (!effect) {
                return null
            }
            return {
                effects: compartment.reconfigure(modes.includes(effect.value) ? extension : []),
            }
        }),
    ]
}

/**
 * Extension to listen to mode changes.
 */
export function onModeChange(
    callback: (view: EditorView, mode: string | null, prevMode: string | null) => void
): Extension {
    return EditorView.updateListener.of(update => {
        const selectedMode = getSelectedMode(update.state)
        const prevSelectedMode = getSelectedMode(update.startState)
        if (selectedMode !== prevSelectedMode) {
            return callback(update.view, selectedMode?.name ?? null, prevSelectedMode?.name ?? null)
        }
    })
}

/**
 * React hook to integrate with the input mode extension. The returned extension has to be passed
 * to the CodeMirror instance when initialized. The hook works like `useState`: The first value
 * is the currently enabled mode (or `null` if no mode is enabled), the second value is a setter
 * for changing the mode.
 */
export function useInputMode(): [
    string | null,
    (view: EditorView, mode: string | null | ((mode: string | null) => string | null)) => void,
    Extension
] {
    const [mode, set] = useState<string | null>(null)

    const extension: Extension = useMemo(() => onModeChange((_, mode) => set(mode)), [set])

    return [mode, setMode, extension]
}
