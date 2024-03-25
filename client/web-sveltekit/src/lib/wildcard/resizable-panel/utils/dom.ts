export function getPanelGroupElement(id: string, rootElement: ParentNode | HTMLElement = document): HTMLElement | null {
    // If the root element is the PanelGroup
    if (rootElement instanceof HTMLElement && (rootElement as HTMLElement)?.dataset?.panelGroupId == id) {
        return rootElement as HTMLElement
    }

    // Else query children
    const element = rootElement.querySelector(`[data-panel-group][data-panel-group-id="${id}"]`)
    if (element) {
        return element as HTMLElement
    }
    return null
}

export function getResizeHandleElement(id: string, scope: ParentNode | HTMLElement = document): HTMLElement | null {
    const element = scope.querySelector(`[data-panel-resize-handle-id="${id}"]`)

    if (element) {
        return element as HTMLElement
    }

    return null
}

export function getResizeHandleElementsForGroup(
    groupId: string,
    scope: ParentNode | HTMLElement = document
): HTMLElement[] {
    return Array.from(scope.querySelectorAll(`[data-panel-resize-handle-id][data-panel-group-id="${groupId}"]`))
}

export function getResizeHandleElementIndex(
    groupId: string,
    id: string,
    scope: ParentNode | HTMLElement = document
): number | null {
    const handles = getResizeHandleElementsForGroup(groupId, scope)
    const index = handles.findIndex(handle => handle.getAttribute('data-panel-resize-handle-id') === id)
    return index ?? null
}
