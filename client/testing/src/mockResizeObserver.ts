import ResizeObserver from 'resize-observer-polyfill'
import { vi } from 'vitest'

if ('ResizeObserver' in window === false) {
    // @ts-ignore
    window.ResizeObserver = ResizeObserver
}

vi.mock('use-resize-observer', () => vi.importActual('use-resize-observer/polyfilled'))
