import { getResizeEventCoordinates } from './event/getResizeEventCoordinates'
import { intersects } from './rect/intersects'
import { PanelGroupDirection, type ResizeHandlerAction, type ResizeEvent } from './types'
import { resetGlobalCursorStyle, setGlobalCursorStyle } from './utils/cursor'
import { getInputType } from './utils/getInputType'
import { compare } from './vendor/stacking-order'

export type PointerHitAreaMargins = {
    coarse: number
    fine: number
}

export type SetResizeHandlerState = (action: ResizeHandlerAction, isActive: boolean, event: ResizeEvent) => void

export type ResizeHandlerData = {
    direction: PanelGroupDirection
    element: HTMLElement
    hitAreaMargins: PointerHitAreaMargins
    setResizeHandlerState: SetResizeHandlerState
}

export enum Exceed {
    NO_CONSTRAINT = 0,
    HORIZONTAL_MIN = 0b0001,
    HORIZONTAL_MAX = 0b0010,
    VERTICAL_MIN = 0b0100,
    VERTICAL_MAX = 0b1000,
}

export class PanelResizeHandleRegistry {
    static isPointerDown = false
    static isCoarsePointer = getInputType() === 'coarse'
    static intersectingHandles: ResizeHandlerData[] = []
    static ownerDocumentCounts: Map<Document, number> = new Map()
    static panelConstraintFlags: Map<string, Exceed> = new Map()
    static registeredResizeHandlers = new Set<ResizeHandlerData>()

    static reportConstraintsViolation(resizeHandleId: string, flag: Exceed) {
        PanelResizeHandleRegistry.panelConstraintFlags.set(resizeHandleId, flag)
    }

    static registerResizeHandle(
        resizeHandleId: string,
        element: HTMLElement,
        direction: PanelGroupDirection,
        hitAreaMargins: PointerHitAreaMargins,
        setResizeHandlerState: SetResizeHandlerState
    ) {
        const { ownerDocument } = element

        const data: ResizeHandlerData = {
            direction,
            element,
            hitAreaMargins,
            setResizeHandlerState,
        }

        const count = PanelResizeHandleRegistry.ownerDocumentCounts.get(ownerDocument) ?? 0
        PanelResizeHandleRegistry.ownerDocumentCounts.set(ownerDocument, count + 1)

        PanelResizeHandleRegistry.registeredResizeHandlers.add(data)

        PanelResizeHandleRegistry.updateListeners()

        return function unregisterResizeHandle() {
            PanelResizeHandleRegistry.panelConstraintFlags.delete(resizeHandleId)
            PanelResizeHandleRegistry.registeredResizeHandlers.delete(data)

            const count = PanelResizeHandleRegistry.ownerDocumentCounts.get(ownerDocument) ?? 1
            PanelResizeHandleRegistry.ownerDocumentCounts.set(ownerDocument, count - 1)

            PanelResizeHandleRegistry.updateListeners()

            if (count === 1) {
                PanelResizeHandleRegistry.ownerDocumentCounts.delete(ownerDocument)
            }
        }
    }

    static updateListeners() {
        PanelResizeHandleRegistry.ownerDocumentCounts.forEach((_, ownerDocument) => {
            const { body } = ownerDocument

            body.removeEventListener('contextmenu', PanelResizeHandleRegistry.handlePointerUp)
            body.removeEventListener('mousedown', PanelResizeHandleRegistry.handlePointerDown)
            body.removeEventListener('mouseleave', PanelResizeHandleRegistry.handlePointerMove)
            body.removeEventListener('mousemove', PanelResizeHandleRegistry.handlePointerMove)
            body.removeEventListener('touchmove', PanelResizeHandleRegistry.handlePointerMove)
            body.removeEventListener('touchstart', PanelResizeHandleRegistry.handlePointerDown)
        })

        window.removeEventListener('mouseup', PanelResizeHandleRegistry.handlePointerUp)
        window.removeEventListener('touchcancel', PanelResizeHandleRegistry.handlePointerUp)
        window.removeEventListener('touchend', PanelResizeHandleRegistry.handlePointerUp)

        if (PanelResizeHandleRegistry.registeredResizeHandlers.size > 0) {
            if (PanelResizeHandleRegistry.isPointerDown) {
                if (PanelResizeHandleRegistry.intersectingHandles.length > 0) {
                    PanelResizeHandleRegistry.ownerDocumentCounts.forEach((count, ownerDocument) => {
                        const { body } = ownerDocument

                        if (count > 0) {
                            body.addEventListener('contextmenu', PanelResizeHandleRegistry.handlePointerUp)
                            body.addEventListener('mouseleave', PanelResizeHandleRegistry.handlePointerMove)
                            body.addEventListener('mousemove', PanelResizeHandleRegistry.handlePointerMove)
                            body.addEventListener('touchmove', PanelResizeHandleRegistry.handlePointerMove, {
                                passive: false,
                            })
                        }
                    })
                }

                window.addEventListener('mouseup', PanelResizeHandleRegistry.handlePointerUp)
                window.addEventListener('touchcancel', PanelResizeHandleRegistry.handlePointerUp)
                window.addEventListener('touchend', PanelResizeHandleRegistry.handlePointerUp)
            } else {
                PanelResizeHandleRegistry.ownerDocumentCounts.forEach((count, ownerDocument) => {
                    const { body } = ownerDocument

                    if (count > 0) {
                        body.addEventListener('mousedown', PanelResizeHandleRegistry.handlePointerDown)
                        body.addEventListener('mousemove', PanelResizeHandleRegistry.handlePointerMove)
                        body.addEventListener('touchmove', PanelResizeHandleRegistry.handlePointerMove, {
                            passive: false,
                        })
                        body.addEventListener('touchstart', PanelResizeHandleRegistry.handlePointerDown)
                    }
                })
            }
        }
    }

    static handlePointerUp(event: ResizeEvent) {
        const { target } = event
        const { x, y } = getResizeEventCoordinates(event)

        PanelResizeHandleRegistry.panelConstraintFlags.clear()
        PanelResizeHandleRegistry.isPointerDown = false

        if (PanelResizeHandleRegistry.intersectingHandles.length > 0) {
            event.preventDefault()
        }

        PanelResizeHandleRegistry.updateResizeHandlerStates('up', event)
        PanelResizeHandleRegistry.recalculateIntersectingHandles({ target, x, y })
        PanelResizeHandleRegistry.updateCursor()

        PanelResizeHandleRegistry.updateListeners()
    }

    static handlePointerDown(event: ResizeEvent) {
        const { target } = event
        const { x, y } = getResizeEventCoordinates(event)

        PanelResizeHandleRegistry.isPointerDown = true

        PanelResizeHandleRegistry.recalculateIntersectingHandles({ target, x, y })
        PanelResizeHandleRegistry.updateListeners()

        if (PanelResizeHandleRegistry.intersectingHandles.length > 0) {
            PanelResizeHandleRegistry.updateResizeHandlerStates('down', event)

            event.preventDefault()
        }
    }

    static handlePointerMove(event: ResizeEvent) {
        const { x, y } = getResizeEventCoordinates(event)

        if (!PanelResizeHandleRegistry.isPointerDown) {
            const { target } = event

            // Recalculate intersecting handles whenever the pointer moves, except if it has already been pressed
            // at that point, the handles may not move with the pointer (depending on constraints)
            // but the same set of active handles should be locked until the pointer is released
            PanelResizeHandleRegistry.recalculateIntersectingHandles({ target, x, y })
        }

        PanelResizeHandleRegistry.updateResizeHandlerStates('move', event)

        // Update cursor based on return value(s) from active handles
        PanelResizeHandleRegistry.updateCursor()

        if (PanelResizeHandleRegistry.intersectingHandles.length > 0) {
            event.preventDefault()
        }
    }

    static updateResizeHandlerStates(action: ResizeHandlerAction, event: ResizeEvent) {
        PanelResizeHandleRegistry.registeredResizeHandlers.forEach(data => {
            const { setResizeHandlerState } = data

            const isActive = PanelResizeHandleRegistry.intersectingHandles.includes(data)

            setResizeHandlerState(action, isActive, event)
        })
    }

    static recalculateIntersectingHandles({ target, x, y }: { target: EventTarget | null; x: number; y: number }) {
        PanelResizeHandleRegistry.intersectingHandles.splice(0)

        let targetElement: HTMLElement | null = null
        if (target instanceof HTMLElement) {
            targetElement = target
        }

        PanelResizeHandleRegistry.registeredResizeHandlers.forEach(data => {
            const { element: dragHandleElement, hitAreaMargins } = data

            const dragHandleRect = dragHandleElement.getBoundingClientRect()
            const { bottom, left, right, top } = dragHandleRect

            const margin = PanelResizeHandleRegistry.isCoarsePointer ? hitAreaMargins.coarse : hitAreaMargins.fine

            const eventIntersects =
                x >= left - margin && x <= right + margin && y >= top - margin && y <= bottom + margin

            if (eventIntersects) {
                // TRICKY
                // We listen for pointers events at the root in order to support hit area margins
                // (determining when the pointer is close enough to an element to be considered a "hit")
                // Clicking on an element "above" a handle (e.g. a modal) should prevent a hit though
                // so at this point we need to compare stacking order of a potentially intersecting drag handle,
                // and the element that was actually clicked/touched
                if (
                    targetElement !== null &&
                    dragHandleElement !== targetElement &&
                    !dragHandleElement.contains(targetElement) &&
                    !targetElement.contains(dragHandleElement) &&
                    // Calculating stacking order has a cost, so we should avoid it if possible
                    // That is why we only check potentially intersecting handles,
                    // and why we skip if the event target is within the handle's DOM
                    compare(targetElement, dragHandleElement) > 0
                ) {
                    // If the target is above the drag handle, then we also need to confirm they overlap
                    // If they are beside each other (e.g. a panel and its drag handle) then the handle is still interactive
                    //
                    // It's not enough to compare only the target.
                    // The target might be a small element inside a larger container
                    // (For example, a SPAN or a DIV inside a larger modal dialog)
                    let currentElement: HTMLElement | null = targetElement
                    let didIntersect = false
                    while (currentElement) {
                        if (currentElement.contains(dragHandleElement)) {
                            break
                        } else if (intersects(currentElement.getBoundingClientRect(), dragHandleRect, true)) {
                            didIntersect = true
                            break
                        }

                        currentElement = currentElement.parentElement
                    }

                    if (didIntersect) {
                        return
                    }
                }

                PanelResizeHandleRegistry.intersectingHandles.push(data)
            }
        })
    }

    static updateCursor() {
        let intersectsHorizontal = false
        let intersectsVertical = false

        PanelResizeHandleRegistry.intersectingHandles.forEach(data => {
            const { direction } = data

            if (direction === 'horizontal') {
                intersectsHorizontal = true
            } else {
                intersectsVertical = true
            }
        })

        let constraintFlags = 0
        PanelResizeHandleRegistry.panelConstraintFlags.forEach(flag => {
            constraintFlags |= flag
        })

        if (intersectsHorizontal && intersectsVertical) {
            setGlobalCursorStyle('intersection', constraintFlags)
        } else if (intersectsHorizontal) {
            setGlobalCursorStyle('horizontal', constraintFlags)
        } else if (intersectsVertical) {
            setGlobalCursorStyle('vertical', constraintFlags)
        } else {
            resetGlobalCursorStyle()
        }
    }
}
