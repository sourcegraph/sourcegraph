import type { DragState, PanelInfo, PanelsLayout } from '../types'

interface Input {
    dragState: DragState | null
    layout: PanelsLayout
    panels: PanelInfo[]
    panelIndex: number
    precision?: number
}

export function computePanelFlexBoxStyle(input: Input): string {
    const { dragState, layout, panels, panelIndex, precision = 3 } = input

    const size = layout[panelIndex]

    let flexGrow

    // Special case: Single panel group should always fill full width/height
    if (size === undefined || panels.length === 1) {
        flexGrow = '1'
    } else {
        flexGrow = size.toPrecision(precision)
    }

    // Disable pointer events inside a panel during resize
    // This avoid edge cases like nested iframes
    return `
        flex-grow: ${flexGrow};
        flex-basis: 0;
        flex-shrink: 0;
        overflow: hidden;
        pointer-events: ${dragState !== undefined ? 'none' : 'unset'}
    `
}
