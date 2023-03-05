import { EditorView, Tooltip, TooltipView } from '@codemirror/view'

import { Position } from '@sourcegraph/shared/src/codeintel/legacy-extensions/api'

import { getCodeIntelTooltipState, setFocusedOccurrenceTooltip } from '../token-selection/code-intel-tooltips'

class TemporaryTooltip implements Tooltip {
    public readonly above = true
    constructor(
        public readonly message: string,
        public readonly pos: number,
        public readonly arrow: boolean | undefined
    ) {}
    public create(): TooltipView {
        const div = document.createElement('div')
        div.classList.add('tmp-tooltip')
        div.textContent = this.message
        return { dom: div }
    }
}

/**
 * Displays a simple tooltip that automatically hides after the provided timeout.
 * As temporary tooltips are only shown for selected (focused) occurrences currently,
 * we use {@link setFocusedOccurrenceTooltip} to update {@link codeIntelTooltipsState}.
 */
export function showTemporaryTooltip(
    view: EditorView,
    message: string,
    position: Position,
    clearTimeout: number,
    options?: {
        arrow?: boolean
    }
): void {
    const line = view.state.doc.line(position.line + 1)
    const pos = line.from + position.character + 1
    const tooltip = new TemporaryTooltip(message, pos, options?.arrow)
    view.dispatch({ effects: setFocusedOccurrenceTooltip.of(tooltip) })
    setTimeout(() => {
        // close loading tooltip if any
        const current = getCodeIntelTooltipState(view, 'focus')
        if (current?.tooltip === tooltip) {
            view.dispatch({ effects: setFocusedOccurrenceTooltip.of(null) })
        }
    }, clearTimeout)
}
