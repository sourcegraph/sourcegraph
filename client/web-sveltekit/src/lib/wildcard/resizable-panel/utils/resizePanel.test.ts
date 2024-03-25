import { describe, expect, it } from 'vitest'

import { resizePanel } from './resizePanel'

describe('resizePanel', () => {
    it('should not collapse (or expand) until a panel size dips below the halfway point between min size and collapsed size', () => {
        expect(
            resizePanel({
                panelConstraints: [
                    {
                        collapsible: true,
                        collapsedSize: 10,
                        minSize: 20,
                    },
                ],
                panelIndex: 0,
                size: 15,
            })
        ).toBe(20)

        expect(
            resizePanel({
                panelConstraints: [
                    {
                        collapsible: true,
                        collapsedSize: 10,
                        minSize: 20,
                    },
                ],
                panelIndex: 0,
                size: 14,
            })
        ).toBe(10)

        expect(
            resizePanel({
                panelConstraints: [
                    {
                        collapsible: true,
                        minSize: 20,
                    },
                ],
                panelIndex: 0,
                size: 10,
            })
        ).toBe(20)

        expect(
            resizePanel({
                panelConstraints: [
                    {
                        collapsible: true,
                        minSize: 20,
                    },
                ],
                panelIndex: 0,
                size: 9,
            })
        ).toBe(0)
    })
})
