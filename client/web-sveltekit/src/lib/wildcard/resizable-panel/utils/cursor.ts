import { Exceed } from '../PanelResizeHandleRegistry'

export type CursorState = 'horizontal' | 'intersection' | 'vertical'

let currentCursorStyle: string | null = null
let styleElement: HTMLStyleElement | null = null

export function getCursorStyle(state: CursorState, constraintFlags: number): string {
    if (constraintFlags) {
        const horizontalMin = (constraintFlags & Exceed.HORIZONTAL_MIN) !== 0
        const horizontalMax = (constraintFlags & Exceed.HORIZONTAL_MAX) !== 0
        const verticalMin = (constraintFlags & Exceed.VERTICAL_MIN) !== 0
        const verticalMax = (constraintFlags & Exceed.VERTICAL_MAX) !== 0

        if (horizontalMin) {
            if (verticalMin) {
                return 'se-resize'
            } else if (verticalMax) {
                return 'ne-resize'
            } else {
                return 'e-resize'
            }
        } else if (horizontalMax) {
            if (verticalMin) {
                return 'sw-resize'
            } else if (verticalMax) {
                return 'nw-resize'
            } else {
                return 'w-resize'
            }
        } else if (verticalMin) {
            return 's-resize'
        } else if (verticalMax) {
            return 'n-resize'
        }
    }

    switch (state) {
        case 'horizontal':
            return 'ew-resize'
        case 'intersection':
            return 'move'
        case 'vertical':
            return 'ns-resize'
    }
}

export function resetGlobalCursorStyle() {
    if (styleElement !== null) {
        document.head.removeChild(styleElement)

        currentCursorStyle = null
        styleElement = null
    }
}

export function setGlobalCursorStyle(state: CursorState, constraintFlags: number) {
    const style = getCursorStyle(state, constraintFlags)

    if (currentCursorStyle === style) {
        return
    }

    currentCursorStyle = style

    if (styleElement === null) {
        styleElement = document.createElement('style')

        document.head.appendChild(styleElement)
    }

    styleElement.innerHTML = `*{cursor: ${style}!important;}`
}
