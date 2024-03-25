import type { ResizeEvent } from '../types'

export function isKeyDown(event: ResizeEvent): event is KeyboardEvent {
    return event.type === 'keydown'
}

export function isMouseEvent(event: ResizeEvent): event is MouseEvent {
    return event.type.startsWith('mouse')
}

export function isTouchEvent(event: ResizeEvent): event is TouchEvent {
    return event.type.startsWith('touch')
}
