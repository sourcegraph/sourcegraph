import type { Readable, Unsubscriber, Writable } from 'svelte/store'

export enum PanelGroupDirection {
    Horizontal = 'horizontal',
    Vertical = 'vertical',
}

export type PanelId = string
export type PanelSize = number
export type PanelsLayout = PanelSize[]
export type ResizeHandlerAction = 'down' | 'move' | 'up'
export type Direction = `${PanelGroupDirection}` | PanelGroupDirection

// TODO [VK]: Keyboard events are not supported but will be in the next iteration
export type ResizeEvent = KeyboardEvent | MouseEvent | TouchEvent

export interface PanelInfo {
    id: PanelId
    order: number | undefined
    constraints: PanelConstraints
    getPanelElement: () => HTMLElement
}

export interface PanelConstraints {
    defaultSize?: number | undefined
    maxSize?: number | undefined
    minSize?: number | undefined
    collapsedSize?: number | undefined
    collapsible?: boolean | undefined
}

export interface DragState {
    dragHandleId: string
    dragHandleRect: DOMRect
    initialCursorPosition: number
    initialLayout: number[]
}

export interface PanelGroupContext {
    groupId: string
    panelsStore: Writable<PanelInfo[]>
    dragStateStore: Writable<DragState | null>
    direction: PanelGroupDirection
    registerPanel: (panel: PanelInfo) => Unsubscriber
    getResizeHandler: (handleId: string) => (event: ResizeEvent) => void
    startDragging: (dragHandleId: string, event: ResizeEvent) => void
    stopDragging: () => void
    getPanelGroupElement: () => HTMLElement
    getPanelStyles: (id: PanelId) => Readable<string>
}
