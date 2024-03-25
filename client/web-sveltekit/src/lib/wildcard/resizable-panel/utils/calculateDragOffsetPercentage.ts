import { getResizeEventCursorPosition } from '../event/getResizeEventCursorPosition'
import { PanelGroupDirection } from '../types'
import type { DragState, ResizeEvent } from '../types'

import { assert } from './assert'
import { getPanelGroupElement, getResizeHandleElement } from './dom'

export function calculateDragOffsetPercentage(
    event: ResizeEvent,
    dragHandleId: string,
    direction: PanelGroupDirection,
    initialDragState: DragState,
    panelGroupElement: HTMLElement
): number {
    const isHorizontal = direction === 'horizontal'

    const handleElement = getResizeHandleElement(dragHandleId, panelGroupElement)
    assert(handleElement, `No resize handle element found for id "${dragHandleId}"`)

    const groupId = handleElement.getAttribute('data-panel-group-id')
    assert(groupId, `Resize handle element has no group id attribute`)

    let { initialCursorPosition } = initialDragState

    const cursorPosition = getResizeEventCursorPosition(direction, event)

    const groupElement = getPanelGroupElement(groupId, panelGroupElement)
    assert(groupElement, `No group element found for id "${groupId}"`)

    const groupRect = groupElement.getBoundingClientRect()
    const groupSizeInPixels = isHorizontal ? groupRect.width : groupRect.height

    const offsetPixels = cursorPosition - initialCursorPosition
    const offsetPercentage = (offsetPixels / groupSizeInPixels) * 100

    return offsetPercentage
}
