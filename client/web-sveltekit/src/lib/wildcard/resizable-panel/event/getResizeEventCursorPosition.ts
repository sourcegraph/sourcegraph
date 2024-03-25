import type { ResizeEvent } from '../types'
import { PanelGroupDirection } from '../types'

import { getResizeEventCoordinates } from './getResizeEventCoordinates'

export function getResizeEventCursorPosition(direction: PanelGroupDirection, event: ResizeEvent): number {
    const isHorizontal = direction === 'horizontal'

    const { x, y } = getResizeEventCoordinates(event)

    return isHorizontal ? x : y
}
