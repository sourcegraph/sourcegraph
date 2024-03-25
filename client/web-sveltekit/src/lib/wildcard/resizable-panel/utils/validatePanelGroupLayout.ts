import { fuzzyNumbersEqual } from '../numbers'
import type { PanelConstraints, PanelsLayout } from '../types'

import { assert } from './assert'

export function validatePanelGroupLayout(layout: PanelsLayout, constraints: PanelConstraints[]): PanelsLayout {
    const nextLayout = [...layout]
    const nextLayoutTotalSize = nextLayout.reduce((accumulated, current) => accumulated + current, 0)

    // Validate layout expectations
    if (nextLayout.length !== constraints.length) {
        throw new Error(`Invalid ${constraints.length} panel layout: ${nextLayout.map(size => `${size}%`).join(', ')}`)
    } else if (!fuzzyNumbersEqual(nextLayoutTotalSize, 100)) {
        for (let index = 0; index < constraints.length; index++) {
            const unsafeSize = nextLayout[index]
            assert(unsafeSize !== undefined, `No layout data found for index ${index}`)
            const safeSize = (100 / nextLayoutTotalSize) * unsafeSize
            nextLayout[index] = safeSize
        }
    }

    return nextLayout
}
