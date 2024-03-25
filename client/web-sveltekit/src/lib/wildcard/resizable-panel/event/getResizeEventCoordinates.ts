import { isMouseEvent, isTouchEvent } from '.'
import type { ResizeEvent } from './../types'

export function getResizeEventCoordinates(event: ResizeEvent) {
    if (isMouseEvent(event)) {
        return {
            x: event.clientX,
            y: event.clientY,
        }
    } else if (isTouchEvent(event)) {
        const touch = event.touches[0]
        if (touch && touch.clientX && touch.clientY) {
            return {
                x: touch.clientX,
                y: touch.clientY,
            }
        }
    }

    return {
        x: Infinity,
        y: Infinity,
    }
}
