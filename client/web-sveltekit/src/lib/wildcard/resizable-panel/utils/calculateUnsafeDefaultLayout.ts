import type { PanelInfo } from '../types'

export function calculateUnsafeDefaultLayout(panels: PanelInfo[]) {
    const layout = new Array<number>(panels.length)

    let numPanelsWithSizes = 0
    let remainingSize = 100

    // Distribute default sizes first
    for (let index = 0; index < panels.length; index++) {
        const { defaultSize } = panels[index].constraints

        if (defaultSize !== undefined) {
            numPanelsWithSizes++
            layout[index] = defaultSize
            remainingSize -= defaultSize
        }
    }

    // Remaining size should be distributed evenly between panels without default sizes
    for (let index = 0; index < panels.length; index++) {
        const { defaultSize } = panels[index].constraints

        if (defaultSize !== undefined) {
            continue
        }

        const numRemainingPanels = panels.length - numPanelsWithSizes
        const size = remainingSize / numRemainingPanels

        numPanelsWithSizes++
        layout[index] = size
        remainingSize -= size
    }

    return layout
}
