import { Facet } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { from, merge, timer } from 'rxjs'
import { map, takeWhile } from 'rxjs/operators'

import type { LineOrPositionOrRange } from '@sourcegraph/common'

import { LoadingTooltip } from '../tooltips/LoadingTooltip'
import { offsetToUIPosition } from '../utils'

import { getHoverTooltip } from './api'
import { showTooltip } from './tooltips'

/**
 * Facet for specifying the location of the pinned tooltip/hovercard if any.
 *
 * NOTE: The support is split across two facets, this one and {@link pinnedRange}
 * because at the time {@link pinnedLocation} is provided, the code intel API
 * might not be available yet.
 */
export const pinnedLocation = Facet.define<LineOrPositionOrRange | null, LineOrPositionOrRange | null>({
    combine(values) {
        return values[0] ?? null
    },
})

/**
 * The pinned range, after it was validated that an interactive occurrence exists at this position.
 * This should only be used by the main code intel extension. See note in {@link pinnedLocation}.
 */
export const pinnedRange = Facet.define<{ from: number; to: number } | null, { from: number; to: number } | null>({
    combine(value) {
        return value[0] ?? null
    },

    enables: self => [
        // Provide tooltip for pinned range
        showTooltip.computeN([self], state => {
            const range = state.facet(self)
            if (range) {
                const loadingTooltip = new LoadingTooltip(range.from, range.to)
                return [
                    // Show loading tooltip after 50ms if the hover tooltip is not yet available
                    merge(from(getHoverTooltip(state, range.from)), timer(50).pipe(map(() => loadingTooltip))).pipe(
                        takeWhile(tooltip => tooltip !== loadingTooltip, true)
                    ),
                ]
            }
            return []
        }),

        // Hide tooltip when selection moves outside of pinned occurrence
        EditorView.updateListener.of(update => {
            const range = update.state.facet(self)
            if (
                update.selectionSet &&
                range &&
                (update.state.selection.main.from <= range.from || range.to <= update.state.selection.main.from)
            ) {
                update.state.facet(pinConfig).onUnpin?.(offsetToUIPosition(update.state.doc, range.from))
            }
        }),
    ],
})

export interface PinConfig {
    /**
     * Called when the tooltip at the specified position should be pinned.
     */
    onPin?: (position: { line: number; character: number }) => void
    /**
     * Called when the specified position should be unpinned.
     */
    onUnpin?: (position: { line: number; character: number }) => void
}

const emptyConfig: PinConfig = {}

/**
 * Facet for providing handlers for pinning an unpining tooltips.
 */
export const pinConfig = Facet.define<PinConfig, PinConfig>({
    combine(value) {
        return value[0] ?? emptyConfig
    },
})
