import { isKeyDown } from '../event'
import { PanelGroupDirection } from '../types'
import type { DragState, ResizeEvent } from '../types'

import { calculateDragOffsetPercentage } from './calculateDragOffsetPercentage'

// https://developer.mozilla.org/en-US/docs/Web/API/MouseEvent/movementX
export function calculateDeltaPercentage(
    event: ResizeEvent,
    dragHandleId: string,
    direction: PanelGroupDirection,
    initialDragState: DragState | null,
    keyboardResizeBy: number | null,
    panelGroupElement: HTMLElement
): number {
    if (isKeyDown(event)) {
        const isHorizontal = direction === 'horizontal'

        let delta = 0
        if (event.shiftKey) {
            delta = 100
        } else if (keyboardResizeBy != null) {
            delta = keyboardResizeBy
        } else {
            delta = 10
        }

        let movement = 0
        switch (event.key) {
            case 'ArrowDown':
                movement = isHorizontal ? 0 : delta
                break
            case 'ArrowLeft':
                movement = isHorizontal ? -delta : 0
                break
            case 'ArrowRight':
                movement = isHorizontal ? delta : 0
                break
            case 'ArrowUp':
                movement = isHorizontal ? 0 : -delta
                break
            case 'End':
                movement = 100
                break
            case 'Home':
                movement = -100
                break
        }

        return movement
    } else {
        if (initialDragState == null) {
            return 0
        }

        return calculateDragOffsetPercentage(event, dragHandleId, direction, initialDragState, panelGroupElement)
    }
}
