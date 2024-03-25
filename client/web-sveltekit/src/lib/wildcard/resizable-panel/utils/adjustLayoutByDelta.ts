import { fuzzyCompareNumbers, fuzzyLayoutsEqual, fuzzyNumbersEqual } from '../numbers'
import type { PanelConstraints } from '../types'

import { assert } from './assert'
import { resizePanel } from './resizePanel'

// All units must be in percentages; pixel values should be pre-converted
export function adjustLayoutByDelta({
    delta,
    initialLayout,
    panelConstraints: panelConstraintsArray,
    pivotIndices,
    prevLayout,
    trigger,
}: {
    delta: number
    initialLayout: number[]
    panelConstraints: PanelConstraints[]
    pivotIndices: number[]
    prevLayout: number[]
    trigger: 'imperative-api' | 'keyboard' | 'mouse-or-touch'
}): number[] {
    if (fuzzyNumbersEqual(delta, 0)) {
        return initialLayout
    }

    const nextLayout = [...initialLayout]

    const [firstPivotIndex, secondPivotIndex] = pivotIndices
    assert(firstPivotIndex != null, 'Invalid first pivot index')
    assert(secondPivotIndex != null, 'Invalid second pivot index')

    let deltaApplied = 0

    // A resizing panel affects the panels before or after it.
    //
    // A negative delta means the panel(s) immediately after the resize handle should grow/expand by decreasing its offset.
    // Other panels may also need to shrink/contract (and shift) to make room, depending on the min weights.
    //
    // A positive delta means the panel(s) immediately before the resize handle should "expand".
    // This is accomplished by shrinking/contracting (and shifting) one or more of the panels after the resize handle.

    {
        // If this is a resize triggered by a keyboard event, our logic for expanding/collapsing is different.
        // We no longer check the halfway threshold because this may prevent the panel from expanding at all.
        if (trigger === 'keyboard') {
            {
                // Check if we should expand a collapsed panel
                const index = delta < 0 ? secondPivotIndex : firstPivotIndex
                const panelConstraints = panelConstraintsArray[index]
                assert(panelConstraints, `Panel constraints not found for index ${index}`)

                const { collapsedSize = 0, collapsible, minSize = 0 } = panelConstraints

                if (collapsible) {
                    const prevSize = initialLayout[index]
                    assert(prevSize != null, `Previous layout not found for panel index ${index}`)

                    if (fuzzyNumbersEqual(prevSize, collapsedSize)) {
                        const localDelta = minSize - prevSize

                        if (fuzzyCompareNumbers(localDelta, Math.abs(delta)) > 0) {
                            delta = delta < 0 ? 0 - localDelta : localDelta
                        }
                    }
                }
            }

            {
                // Check if we should collapse a panel at its minimum size
                const index = delta < 0 ? firstPivotIndex : secondPivotIndex
                const panelConstraints = panelConstraintsArray[index]
                assert(panelConstraints, `No panel constraints found for index ${index}`)

                const { collapsedSize = 0, collapsible, minSize = 0 } = panelConstraints

                if (collapsible) {
                    const prevSize = initialLayout[index]
                    assert(prevSize != null, `Previous layout not found for panel index ${index}`)

                    if (fuzzyNumbersEqual(prevSize, minSize)) {
                        const localDelta = prevSize - collapsedSize

                        if (fuzzyCompareNumbers(localDelta, Math.abs(delta)) > 0) {
                            delta = delta < 0 ? 0 - localDelta : localDelta
                        }
                    }
                }
            }
        }
    }

    {
        // Pre-calculate max available delta in the opposite direction of our pivot.
        // This will be the maximum amount we're allowed to expand/contract the panels in the primary direction.
        // If this amount is less than the requested delta, adjust the requested delta.
        // If this amount is greater than the requested delta, that's useful information too–
        // as an expanding panel might change from collapsed to min size.

        const increment = delta < 0 ? 1 : -1

        let index = delta < 0 ? secondPivotIndex : firstPivotIndex
        let maxAvailableDelta = 0

        while (true) {
            const prevSize = initialLayout[index]
            assert(prevSize != null, `Previous layout not found for panel index ${index}`)

            const maxSafeSize = resizePanel({
                panelConstraints: panelConstraintsArray,
                panelIndex: index,
                size: 100,
            })
            const delta = maxSafeSize - prevSize

            maxAvailableDelta += delta
            index += increment

            if (index < 0 || index >= panelConstraintsArray.length) {
                break
            }
        }

        const minAbsDelta = Math.min(Math.abs(delta), Math.abs(maxAvailableDelta))
        delta = delta < 0 ? 0 - minAbsDelta : minAbsDelta
    }

    {
        // Delta added to a panel needs to be subtracted from other panels (within the constraints that those panels allow).

        const pivotIndex = delta < 0 ? firstPivotIndex : secondPivotIndex
        let index = pivotIndex
        while (index >= 0 && index < panelConstraintsArray.length) {
            const deltaRemaining = Math.abs(delta) - Math.abs(deltaApplied)

            const prevSize = initialLayout[index]
            assert(prevSize != null, `Previous layout not found for panel index ${index}`)

            const unsafeSize = prevSize - deltaRemaining
            const safeSize = resizePanel({
                panelConstraints: panelConstraintsArray,
                panelIndex: index,
                size: unsafeSize,
            })

            if (!fuzzyNumbersEqual(prevSize, safeSize)) {
                deltaApplied += prevSize - safeSize

                nextLayout[index] = safeSize

                if (
                    deltaApplied.toPrecision(3).localeCompare(Math.abs(delta).toPrecision(3), undefined, {
                        numeric: true,
                    }) >= 0
                ) {
                    break
                }
            }

            if (delta < 0) {
                index--
            } else {
                index++
            }
        }
    }

    // If we were unable to resize any of the panels panels, return the previous state.
    // This will essentially bailout and ignore e.g. drags past a panel's boundaries
    if (fuzzyLayoutsEqual(prevLayout, nextLayout)) {
        return prevLayout
    }

    {
        // Now distribute the applied delta to the panels in the other direction
        const pivotIndex = delta < 0 ? secondPivotIndex : firstPivotIndex

        const prevSize = initialLayout[pivotIndex]
        assert(prevSize != null, `Previous layout not found for panel index ${pivotIndex}`)

        const unsafeSize = prevSize + deltaApplied
        const safeSize = resizePanel({
            panelConstraints: panelConstraintsArray,
            panelIndex: pivotIndex,
            size: unsafeSize,
        })

        // Adjust the pivot panel before, but only by the amount that surrounding panels were able to shrink/contract.
        nextLayout[pivotIndex] = safeSize

        // Edge case where expanding or contracting one panel caused another one to change collapsed state
        if (!fuzzyNumbersEqual(safeSize, unsafeSize)) {
            let deltaRemaining = unsafeSize - safeSize

            const pivotIndex = delta < 0 ? secondPivotIndex : firstPivotIndex
            let index = pivotIndex
            while (index >= 0 && index < panelConstraintsArray.length) {
                const prevSize = nextLayout[index]
                assert(prevSize != null, `Previous layout not found for panel index ${index}`)

                const unsafeSize = prevSize + deltaRemaining
                const safeSize = resizePanel({
                    panelConstraints: panelConstraintsArray,
                    panelIndex: index,
                    size: unsafeSize,
                })

                if (!fuzzyNumbersEqual(prevSize, safeSize)) {
                    deltaRemaining -= safeSize - prevSize

                    nextLayout[index] = safeSize
                }

                if (fuzzyNumbersEqual(deltaRemaining, 0)) {
                    break
                }

                if (delta > 0) {
                    index--
                } else {
                    index++
                }
            }
        }
    }

    const totalSize = nextLayout.reduce((total, size) => size + total, 0)

    // If our new layout doesn't add up to 100%, that means the requested delta can't be applied
    // In that case, fall back to our most recent valid layout
    if (!fuzzyNumbersEqual(totalSize, 100)) {
        return prevLayout
    }

    return nextLayout
}
